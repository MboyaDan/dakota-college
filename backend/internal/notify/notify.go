// Package notify sends messages to applicants and students from editable
// templates (brief 10, Stage 8b) through the message queue (Stage 16a).
// Email is sent now; text messages are recorded as skipped until an SMS
// provider is chosen (DG-03).
//
// Templates use {placeholder} names, not a template language, so staff can
// reword them safely: only the placeholders listed for a template are
// allowed, and they are checked when a template is saved.
//
// Every message gets a row in notifications with its status. The text is
// kept, encrypted, only until it is sent (see package outbox), because some
// messages carry sign-in links.
package notify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/mail"
	"github.com/MboyaDan/dakota-college/backend/internal/outbox"
)

// PermManage lets staff edit template text.
const PermManage = "notifications.manage"

// Template keys used by the code.
const (
	ApplicantSignInLink  = "applicant.sign_in_link"
	ApplicantResumeLink  = "applicant.resume_link"
	ApplicationStarted   = "application.started"
	ApplicationSubmitted = "application.submitted"
	// Stage 10a: decisions and enrolment.
	ApplicationInfoRequested = "application.info_requested"
	ApplicationOffered       = "application.offered"
	ApplicationRejected      = "application.rejected"
	ApplicationEnrolled      = "application.enrolled"
	// OfferLetter is printed (channel "letter"), never sent as a message.
	OfferLetter = "offer.letter"
	// Stage 12a: the student portal.
	StudentRequestUpdated       = "student.request_updated"
	StudentProfileChangeDecided = "student.profile_change_decided"
)

// Placeholders lists what each template may use. A template saved with any
// other {name} is refused, so a message never goes out with a raw brace.
var Placeholders = map[string][]string{
	ApplicantSignInLink:         {"name", "link", "minutes"},
	ApplicantResumeLink:         {"name", "reference", "link", "minutes"},
	ApplicationStarted:          {"name", "reference", "programme", "link"},
	ApplicationSubmitted:        {"name", "reference", "programme", "link"},
	ApplicationInfoRequested:    {"name", "reference", "programme", "message", "link"},
	ApplicationOffered:          {"name", "reference", "programme", "intake", "respond_by", "link"},
	ApplicationRejected:         {"name", "reference", "programme", "reason", "link"},
	ApplicationEnrolled:         {"name", "reference", "programme", "student_number", "link"},
	StudentRequestUpdated:       {"name", "kind", "status", "response", "link"},
	StudentProfileChangeDecided: {"name", "decision", "reason", "link"},
	OfferLetter:                 {"name", "reference", "programme", "intake", "starts_on", "respond_by"},
}

// needsLink are the templates whose whole purpose is the link in them.
var needsLink = []string{ApplicantSignInLink, ApplicantResumeLink}

const (
	MaxSubjectLength = 150
	MaxEmailLength   = 5000
	// MaxSMSLength is two SMS segments; the database enforces it too.
	MaxSMSLength = 320
)

var placeholderPattern = regexp.MustCompile(`\{([a-z_]+)\}`)

// Render fills placeholders. Names without a value are left as they are;
// CheckPlaceholders stops that from happening with saved templates.
func Render(text string, vars map[string]string) string {
	return placeholderPattern.ReplaceAllStringFunc(text, func(m string) string {
		if v, ok := vars[m[1:len(m)-1]]; ok {
			return v
		}
		return m
	})
}

// unknownPlaceholders returns the {names} in text that key does not allow.
func unknownPlaceholders(key, text string) []string {
	var bad []string
	for _, m := range placeholderPattern.FindAllStringSubmatch(text, -1) {
		if !slices.Contains(Placeholders[key], m[1]) && !slices.Contains(bad, m[1]) {
			bad = append(bad, m[1])
		}
	}
	return bad
}

type Service struct {
	pool   *pgxpool.Pool
	queue  *outbox.Queue
	logger *slog.Logger
}

func NewService(pool *pgxpool.Pool, queue *outbox.Queue, logger *slog.Logger) *Service {
	return &Service{pool: pool, queue: queue, logger: logger}
}

// Message is one notification to one person.
type Message struct {
	Template string
	To       string
	Vars     map[string]string
	// RelatedType and RelatedID point at what the message is about, for the
	// log (for example "application" and its id).
	RelatedType string
	RelatedID   string
	// ValidFor is how long the message may wait to be sent: the lifetime of
	// a link in it. Zero means the queue's default.
	ValidFor time.Duration
}

func (m Message) item() outbox.Item {
	return outbox.Item{TemplateKey: m.Template, RelatedType: m.RelatedType, RelatedID: m.RelatedID, ValidFor: m.ValidFor}
}

// Email renders the email template and queues it. The queue sends it in the
// background and retries failures, so the response time never depends on
// the mail server (and never reveals whether an account exists).
func (s *Service) Email(ctx context.Context, m Message) error {
	t, err := dbq.New(s.pool).GetNotificationTemplate(ctx, dbq.GetNotificationTemplateParams{Key: m.Template, Channel: "email"})
	if err != nil {
		return fmt.Errorf("template %s: %w", m.Template, err)
	}
	_, err = s.queue.Email(ctx, m.item(), mail.Message{
		To: m.To,
		// Values could hold a line break; a subject never may.
		Subject: strings.Join(strings.Fields(Render(t.Subject, m.Vars)), " "),
		Text:    Render(t.Body, m.Vars),
	})
	return err
}

// SMS queues the text message, or records it as skipped while no SMS
// provider is set up (DG-03).
func (s *Service) SMS(ctx context.Context, m Message) error {
	text := ""
	if s.queue.SMSEnabled() {
		t, err := dbq.New(s.pool).GetNotificationTemplate(ctx, dbq.GetNotificationTemplateParams{Key: m.Template, Channel: "sms"})
		if err != nil {
			return fmt.Errorf("sms template %s: %w", m.Template, err)
		}
		text = Render(t.Body, m.Vars)
	}
	_, err := s.queue.SMS(ctx, m.item(), m.To, text)
	return err
}

// Template is one editable message for the staff portal.
type Template struct {
	Key           string    `json:"key"`
	Channel       string    `json:"channel"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Subject       string    `json:"subject"`
	Body          string    `json:"body"`
	Placeholders  []string  `json:"placeholders"`
	MaxLength     int       `json:"max_length"`
	Sending       bool      `json:"sending"`
	UpdatedAt     time.Time `json:"updated_at"`
	UpdatedByName *string   `json:"updated_by_name"`
}

func (s *Service) ListTemplates(ctx context.Context) ([]Template, error) {
	rows, err := dbq.New(s.pool).ListNotificationTemplates(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Template, 0, len(rows))
	for _, r := range rows {
		out = append(out, Template{
			Key: r.Key, Channel: r.Channel, Name: r.Name, Description: r.Description, Subject: r.Subject, Body: r.Body,
			Placeholders: Placeholders[r.Key], MaxLength: maxLength(r.Channel), Sending: r.Channel == "email" || (r.Channel == "sms" && s.queue.SMSEnabled()),
			UpdatedAt: r.UpdatedAt, UpdatedByName: r.UpdatedByName,
		})
	}
	return out, nil
}

// Letter returns a printed letter's heading and body with placeholders
// filled in.
func (s *Service) Letter(ctx context.Context, key string, vars map[string]string) (heading, body string, err error) {
	t, err := dbq.New(s.pool).GetNotificationTemplate(ctx, dbq.GetNotificationTemplateParams{Key: key, Channel: "letter"})
	if err != nil {
		return "", "", fmt.Errorf("letter template %s: %w", key, err)
	}
	return strings.Join(strings.Fields(Render(t.Subject, vars)), " "), Render(t.Body, vars), nil
}

func maxLength(channel string) int {
	if channel == "sms" {
		return MaxSMSLength
	}
	return MaxEmailLength
}

// ErrNotFound means there is no template with that key and channel.
var ErrNotFound = errors.New("template not found")

// UpdateTemplate saves new text after checking lengths and placeholders.
// Problems come back as a *content.ValidationError.
func (s *Service) UpdateTemplate(ctx context.Context, key, channel, subject, body string, req audit.Request) error {
	if _, ok := Placeholders[key]; !ok || (channel != "email" && channel != "sms" && channel != "letter") {
		return ErrNotFound
	}
	subject, body = strings.TrimSpace(subject), strings.TrimSpace(body)
	var p content.Problems
	if channel == "email" || channel == "letter" {
		p.Required("subject", subject, MaxSubjectLength)
		if strings.ContainsAny(subject, "\r\n") {
			p.Add("subject", "Keep the subject on one line.")
		}
	} else {
		subject = ""
	}
	p.Required("body", body, maxLength(channel))
	allowed := strings.Join(Placeholders[key], "}, {")
	for _, f := range []struct{ field, text string }{{"subject", subject}, {"body", body}} {
		if bad := unknownPlaceholders(key, f.text); len(bad) > 0 {
			p.Add(f.field, fmt.Sprintf("Remove {%s}: this message can only use {%s}.", strings.Join(bad, "}, {"), allowed))
		}
	}
	if slices.Contains(needsLink, key) && !strings.Contains(body, "{link}") {
		p.Add("body", "Keep {link} in the message: it is how the applicant signs in.")
	}
	if err := p.Err(); err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		n, err := q.UpdateNotificationTemplate(ctx, dbq.UpdateNotificationTemplateParams{
			Key: key, Channel: channel, Subject: subject, Body: body, UpdatedBy: req.ActorID,
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return audit.Write(ctx, q, req, audit.Entry{
			Action: "notifications.template_updated", EntityType: "notification_template", EntityID: key + "/" + channel,
			Details: map[string]any{"body_length": utf8.RuneCountInString(body)},
		})
	})
}
