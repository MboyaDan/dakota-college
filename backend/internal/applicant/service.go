// Package applicant lets people who apply online sign in without a password
// (D-96): they ask for a one-time link by email, or enter their application
// reference number with the same email to get one. Following the link starts
// an ordinary session for a user of kind "applicant", created on first use.
//
// Requests never reveal whether an email address or reference exists: the
// answer and its timing are the same, and links are emailed in the
// background.
package applicant

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/notify"
)

const (
	// LinkTTL is how long a sign-in link works.
	LinkTTL = 30 * time.Minute
	// DefaultName is used for a new account when no name was given; the
	// application form collects the real name.
	DefaultName   = "Applicant"
	MaxNameLength = 120
)

var ErrInvalidLink = errors.New("invalid or expired sign-in link")

type Service struct {
	pool       *pgxpool.Pool
	auth       *auth.Service
	notify     *notify.Service
	logger     *slog.Logger
	appBaseURL string
	now        func() time.Time

	byEmail *auth.Limiter
	byIP    *auth.Limiter
}

type Options struct {
	Pool       *pgxpool.Pool
	Auth       *auth.Service
	Notify     *notify.Service
	Logger     *slog.Logger
	AppBaseURL string
	// Now is for tests; defaults to time.Now.
	Now func() time.Time
}

func NewService(o Options) *Service {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		pool: o.Pool, auth: o.Auth, notify: o.Notify, logger: o.Logger,
		appBaseURL: strings.TrimRight(o.AppBaseURL, "/"), now: now,
		// Same shape as password resets: a few per address, and a generous
		// per-IP limit because requests may all come from the frontend (R-23).
		byEmail: auth.NewLimiter(5, time.Hour, now),
		byIP:    auth.NewLimiter(30, time.Hour, now),
	}
}

func (s *Service) allow(email string, req audit.Request) bool {
	ipKey := "ip"
	if req.IP != nil {
		ipKey = req.IP.String()
	}
	return s.byIP.Allow(ipKey) && s.byEmail.Allow(email)
}

// canSignIn is true for an email with no account yet, or an active
// applicant account. Staff and student accounts never get applicant links.
func canSignIn(u dbq.GetUserForLoginRow, err error) (bool, error) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return true, nil
	case err != nil:
		return false, err
	default:
		return u.Kind == "applicant" && u.Status == "active", nil
	}
}

// RequestLink emails a sign-in link. It returns nil whenever the request is
// well formed, including when nothing was sent.
func (s *Service) RequestLink(ctx context.Context, email, name, next string, req audit.Request) error {
	email = auth.NormaliseEmail(email)
	name = strings.Join(strings.Fields(name), " ")
	if !s.allow(email, req) {
		s.logger.WarnContext(ctx, "applicant sign-in link rate limited", slog.String("request_id", req.RequestID))
		return nil
	}
	q := dbq.New(s.pool)
	u, err := q.GetUserForLogin(ctx, email)
	ok, err := canSignIn(u, err)
	if err != nil || !ok {
		return err
	}
	if u.FullName != "" {
		name = u.FullName
	}
	link, err := s.createLink(ctx, email, name, nil, CleanNext(next), req)
	if err != nil {
		return err
	}
	return s.notify.Email(ctx, notify.Message{
		Template: notify.ApplicantSignInLink, To: email,
		Vars:        map[string]string{"name": greeting(name), "link": link, "minutes": minutes()},
		RelatedType: "applicant_email",
	})
}

// ResumeByReference emails a sign-in link to the owner of the application,
// but only when the email given matches theirs. It returns nil whenever the
// request is well formed, including when nothing was sent.
func (s *Service) ResumeByReference(ctx context.Context, reference, email string, req audit.Request) error {
	email = auth.NormaliseEmail(email)
	reference = strings.ToUpper(strings.TrimSpace(reference))
	if !s.allow(email, req) {
		s.logger.WarnContext(ctx, "applicant resume rate limited", slog.String("request_id", req.RequestID))
		return nil
	}
	app, err := dbq.New(s.pool).GetApplicationForResume(ctx, reference)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if auth.NormaliseEmail(app.Email) != email || app.Kind != "applicant" || app.Status != "active" {
		return nil
	}
	link, err := s.createLink(ctx, email, "", &app.ID, "/apply/applications/"+app.ID.String(), req)
	if err != nil {
		return err
	}
	return s.notify.Email(ctx, notify.Message{
		Template: notify.ApplicantResumeLink, To: email,
		Vars:        map[string]string{"name": greeting(app.FullName), "reference": app.Reference, "link": link, "minutes": minutes()},
		RelatedType: "application", RelatedID: app.ID.String(),
	})
}

func (s *Service) createLink(ctx context.Context, email, name string, applicationID *uuid.UUID, next string, req audit.Request) (string, error) {
	token, hash := auth.NewToken()
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := q.CreateApplicantSignInLink(ctx, dbq.CreateApplicantSignInLinkParams{
			Email: email, FullName: name, ApplicationID: applicationID, NextPath: next, TokenHash: hash,
			ExpiresAt: s.now().Add(LinkTTL), RequestedIp: req.IP,
		}); err != nil {
			return err
		}
		details := map[string]any{"resume": applicationID != nil}
		return audit.Write(ctx, q, req, audit.Entry{Action: "applicant.sign_in_link_requested", EntityType: "applicant_email", EntityID: email, Details: details})
	})
	if err != nil {
		return "", err
	}
	// The link opens a page that posts the token back when the person clicks;
	// following the link alone signs nobody in (mail scanners open links).
	return s.appBaseURL + "/apply/continue?token=" + token, nil
}

// SignInResult is a new applicant session.
type SignInResult struct {
	*auth.LoginResult
	// ApplicationID is set when the link came from a reference number.
	ApplicationID *uuid.UUID
	// NextPath is where to go after signing in ("" for the account page).
	NextPath string
	Created  bool
}

// SignIn spends a link and starts a session, creating the applicant account
// the first time. Every outstanding link for the address is spent with it.
func (s *Service) SignIn(ctx context.Context, token string, req audit.Request) (*SignInResult, error) {
	if token == "" || len(token) > 128 {
		return nil, ErrInvalidLink
	}
	var out SignInResult
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		link, err := q.GetApplicantSignInLinkForUpdate(ctx, auth.HashToken(token))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidLink
		}
		if err != nil {
			return err
		}
		if link.UsedAt != nil || !s.now().Before(link.ExpiresAt) {
			return ErrInvalidLink
		}

		if err := q.LockApplicantEmail(ctx, link.Email); err != nil {
			return err
		}
		u, err := q.GetUserForLogin(ctx, link.Email)
		ok, err := canSignIn(u, err)
		if err != nil {
			return err
		}
		if !ok {
			return ErrInvalidLink
		}
		userID, fullName := u.ID, u.FullName
		if u.ID == uuid.Nil {
			fullName = link.FullName
			if fullName == "" {
				fullName = DefaultName
			}
			if userID, err = q.CreateApplicantUser(ctx, dbq.CreateApplicantUserParams{Email: link.Email, FullName: fullName}); err != nil {
				return err
			}
			out.Created = true
		}
		if err := q.UseApplicantSignInLinks(ctx, link.Email); err != nil {
			return err
		}
		if err := q.RecordLoginSuccess(ctx, userID); err != nil {
			return err
		}
		res, err := s.auth.StartSession(ctx, q, userID, req)
		if err != nil {
			return err
		}
		res.Principal.Email, res.Principal.FullName, res.Principal.Kind = link.Email, fullName, "applicant"
		out.LoginResult, out.ApplicationID, out.NextPath = res, link.ApplicationID, link.NextPath

		r := req
		r.ActorID = &userID
		return audit.Write(ctx, q, r, audit.Entry{
			Action: "applicant.signed_in", EntityType: "user", EntityID: userID.String(),
			Details: map[string]any{"account_created": out.Created},
		})
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Application is one of the applicant's own applications.
type Application struct {
	ID             uuid.UUID  `json:"id"`
	Reference      string     `json:"reference"`
	Status         string     `json:"status"`
	ProgrammeTitle string     `json:"programme_title"`
	ProgrammeSlug  string     `json:"programme_slug"`
	IntakeName     *string    `json:"intake_name"`
	IntakeStartsOn *string    `json:"intake_starts_on"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	SubmittedAt    *time.Time `json:"submitted_at"`
}

// Applications lists what the signed-in applicant owns, and nothing else.
func (s *Service) Applications(ctx context.Context, userID uuid.UUID) ([]Application, error) {
	rows, err := dbq.New(s.pool).ListApplicantApplications(ctx, &userID)
	if err != nil {
		return nil, err
	}
	out := make([]Application, 0, len(rows))
	for _, r := range rows {
		a := Application{
			ID: r.ID, Reference: r.Reference, Status: r.Status, ProgrammeTitle: r.ProgrammeTitle,
			ProgrammeSlug: r.ProgrammeSlug, IntakeName: r.IntakeName, CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt, SubmittedAt: r.SubmittedAt,
		}
		if r.IntakeStartsOn != nil {
			d := r.IntakeStartsOn.Format(time.DateOnly)
			a.IntakeStartsOn = &d
		}
		out = append(out, a)
	}
	return out, nil
}

// CleanNext keeps a return path only if it is a plain path under /apply/,
// so a sign-in link can never send anyone to another site.
func CleanNext(next string) string {
	if len(next) > 300 || !strings.HasPrefix(next, "/apply/") || strings.ContainsAny(next, "\\\r\n\t#") ||
		strings.Contains(next, "//") || strings.Contains(next, "..") {
		return ""
	}
	for _, r := range next {
		if r < 0x21 || r > 0x7e {
			return ""
		}
	}
	return next
}

func greeting(name string) string {
	if name == "" || name == DefaultName {
		return "there"
	}
	return name
}

func minutes() string { return strconv.Itoa(int(LinkTTL / time.Minute)) }
