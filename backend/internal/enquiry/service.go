// Package enquiry receives questions sent from the public website (brief 6.2)
// and emails them to the right office. The staff inbox comes in Stage 8.
//
// Spam protection is built in, with no third-party service (D-70):
//   - a hidden honeypot field that people never fill in;
//   - a minimum time between showing the form and sending it;
//   - at most two links in a message;
//   - a limit per email address or phone number, and an overall hourly cap.
//
// Honeypot and too-fast submissions get the same answer as real ones, so a
// bot learns nothing, but they are not saved or emailed.
package enquiry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/mail"
)

const (
	// MinFillTime is the shortest time a person takes to fill in the form.
	MinFillTime = 3 * time.Second
	// MaxLinks is the most links a message may contain.
	MaxLinks = 2

	perContactMax = 3
	overallMax    = 60
)

var (
	// ErrTooMany means this email address or phone number sent too many
	// enquiries in the last hour.
	ErrTooMany = errors.New("too many enquiries from this contact")
	// ErrBusy means the hourly cap for all enquiries is reached.
	ErrBusy = errors.New("too many enquiries overall")
)

// QueryTypes are the choices on the contact form, with their labels.
var QueryTypes = map[string]string{
	"admissions":   "Admissions",
	"programmes":   "Programmes",
	"fees":         "Fees",
	"student_life": "Student life",
	"alumni":       "Alumni (transcripts and certificates)",
	"other":        "Other",
}

// recipientKey is the site setting holding each office's email address.
// contact.email is the fallback when an office has none.
var recipientKey = map[string]string{
	"admissions":   "contact.admissions_email",
	"programmes":   "contact.admissions_email",
	"fees":         "contact.finance_email",
	"student_life": "contact.email",
	"alumni":       "contact.email",
	"other":        "contact.email",
}

type Submission struct {
	FullName      string
	Email         string
	Phone         string
	QueryType     string
	ProgrammeSlug string
	Message       string
	// Website is the honeypot: hidden from people, so it must stay empty.
	Website string
	// Elapsed is how long the form was open before it was sent.
	Elapsed time.Duration
}

type Service struct {
	pool       *pgxpool.Pool
	mailer     mail.Mailer
	logger     *slog.Logger
	perContact *auth.Limiter
	overall    *auth.Limiter
}

func NewService(pool *pgxpool.Pool, mailer mail.Mailer, logger *slog.Logger) *Service {
	return &Service{
		pool: pool, mailer: mailer, logger: logger,
		perContact: auth.NewLimiter(perContactMax, time.Hour, nil),
		overall:    auth.NewLimiter(overallMax, time.Hour, nil),
	}
}

func (s Submission) trimmed() Submission {
	s.FullName = strings.TrimSpace(s.FullName)
	s.Email = strings.TrimSpace(s.Email)
	s.Phone = strings.TrimSpace(s.Phone)
	s.ProgrammeSlug = strings.TrimSpace(s.ProgrammeSlug)
	s.Message = strings.TrimSpace(s.Message)
	return s
}

func (s Submission) validate() content.Problems {
	var p content.Problems
	p.Required("full_name", s.FullName, 120)
	p.Email("email", s.Email)
	p.Optional("email", s.Email, 254)
	p.Phone("phone", s.Phone)
	if s.Email == "" && s.Phone == "" {
		p.Add("email", "Give an email address or a phone number, so we can reply.")
	}
	if _, ok := QueryTypes[s.QueryType]; !ok {
		p.Add("query_type", "Choose what your question is about.")
	}
	p.Required("message", s.Message, 5000)
	if countLinks(s.Message) > MaxLinks {
		p.Add("message", fmt.Sprintf("Remove some links: a message can contain at most %d.", MaxLinks))
	}
	return p
}

func countLinks(s string) int {
	s = strings.ToLower(s)
	return strings.Count(s, "http://") + strings.Count(s, "https://") + strings.Count(s, "www.")
}

// contactKey identifies the sender for the per-contact limit.
func contactKey(s Submission) string {
	if s.Email != "" {
		return "e:" + strings.ToLower(s.Email)
	}
	return "p:" + strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, s.Phone)
}

// Submit checks, saves and emails an enquiry. It returns a
// *content.ValidationError for fixable input, ErrTooMany or ErrBusy when a
// limit is reached, and nil both when the enquiry was saved and when it was
// dropped as spam.
func (s *Service) Submit(ctx context.Context, sub Submission, req audit.Request) error {
	sub = sub.trimmed()
	if sub.Website != "" || sub.Elapsed < MinFillTime {
		s.logger.Info("enquiry dropped as likely spam", slog.Bool("honeypot", sub.Website != ""),
			slog.Duration("elapsed", sub.Elapsed), slog.String("request_id", req.RequestID))
		return nil
	}
	problems := sub.validate()

	q := dbq.New(s.pool)
	var programmeID *uuid.UUID
	programmeTitle := ""
	if sub.ProgrammeSlug != "" {
		prog, err := q.GetPublishedProgrammeBySlug(ctx, sub.ProgrammeSlug)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			problems.Add("programme", "That programme is not offered now. Choose another, or leave it empty.")
		case err != nil:
			return err
		default:
			programmeID, programmeTitle = &prog.ID, prog.Title
		}
	}
	if err := problems.Err(); err != nil {
		return err
	}

	if !s.perContact.Allow(contactKey(sub)) {
		return ErrTooMany
	}
	if !s.overall.Allow("all") {
		return ErrBusy
	}

	row, err := q.InsertEnquiry(ctx, dbq.InsertEnquiryParams{
		FullName: sub.FullName, Email: optional(sub.Email), Phone: optional(sub.Phone), QueryType: sub.QueryType,
		ProgrammeID: programmeID, Message: sub.Message, Ip: req.IP, UserAgent: optional(req.UserAgent),
	})
	if err != nil {
		return err
	}

	to, err := s.recipient(ctx, q, sub.QueryType)
	if err != nil {
		return err
	}
	if to == "" {
		s.logger.Warn("enquiry saved but not emailed: no office email in site settings", slog.String("enquiry_id", row.ID.String()))
		return nil
	}
	msg := mail.Message{
		To:      to,
		Subject: "New website enquiry: " + QueryTypes[sub.QueryType],
		Text:    emailText(sub, programmeTitle, row.CreatedAt),
		ReplyTo: sub.Email,
	}
	// Sent in the background; the enquiry is already saved. Stage 16 moves
	// email onto a retrying queue.
	go func() {
		sendCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := s.mailer.Send(sendCtx, msg); err != nil {
			s.logger.Error("enquiry email failed", slog.String("enquiry_id", row.ID.String()), slog.String("error", err.Error()))
		}
	}()
	return nil
}

func (s *Service) recipient(ctx context.Context, q *dbq.Queries, queryType string) (string, error) {
	key := recipientKey[queryType]
	rows, err := q.GetSiteSettings(ctx, []string{key, "contact.email"})
	if err != nil {
		return "", err
	}
	values := map[string]string{}
	for _, r := range rows {
		values[r.Key] = r.Value
	}
	if v := values[key]; v != "" {
		return v, nil
	}
	return values["contact.email"], nil
}

func emailText(sub Submission, programme string, at time.Time) string {
	var b strings.Builder
	b.WriteString("A visitor sent this question from the website contact form.\n\n")
	fmt.Fprintf(&b, "About:     %s\n", QueryTypes[sub.QueryType])
	if programme != "" {
		fmt.Fprintf(&b, "Programme: %s\n", programme)
	}
	fmt.Fprintf(&b, "Name:      %s\n", sub.FullName)
	if sub.Email != "" {
		fmt.Fprintf(&b, "Email:     %s\n", sub.Email)
	}
	if sub.Phone != "" {
		fmt.Fprintf(&b, "Phone:     %s\n", sub.Phone)
	}
	fmt.Fprintf(&b, "Sent:      %s (Nairobi time)\n\n", at.In(time.FixedZone("EAT", 3*60*60)).Format("2 January 2006, 15:04"))
	b.WriteString("Message:\n")
	b.WriteString(sub.Message)
	b.WriteString("\n\n")
	if sub.Email != "" {
		b.WriteString("Reply to this email to answer them directly. ")
	} else {
		b.WriteString("They gave no email address: please call or text them. ")
	}
	b.WriteString("The enquiry is also saved in the staff portal.\n")
	return b.String()
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
