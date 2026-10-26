package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/config"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/mail"
)

const (
	// MaxFailedLogins wrong passwords in a row lock the account for LockoutDuration.
	MaxFailedLogins = 5
	LockoutDuration = 15 * time.Minute
	ResetTokenTTL   = time.Hour
	// MaxPasswordLength caps Argon2id input so huge bodies cannot burn CPU.
	MaxPasswordLength = 256
	touchInterval     = time.Minute
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTooManyAttempts    = errors.New("too many attempts")
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrInvalidResetToken  = errors.New("invalid or expired reset token")
	ErrWeakPassword       = fmt.Errorf("password must be %d to %d characters", MinPasswordLength, MaxPasswordLength)
)

// Principal is the signed-in user for one request. Permissions are read from
// the database on every request (brief 12.1), so role changes apply at once.
type Principal struct {
	SessionID   uuid.UUID
	UserID      uuid.UUID
	Email       string
	FullName    string
	Kind        string
	Permissions []string
	CSRFToken   string
}

func (p *Principal) Has(permission string) bool {
	for _, k := range p.Permissions {
		if k == permission {
			return true
		}
	}
	return false
}

// CSRFTokenFor derives the CSRF token from the session token. Only someone
// holding the session cookie can compute it, and nothing extra is stored.
func CSRFTokenFor(sessionToken string) string {
	mac := hmac.New(sha256.New, []byte(sessionToken))
	mac.Write([]byte("dakota-csrf-v1"))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// ValidCSRF compares in constant time.
func ValidCSRF(sessionToken, presented string) bool {
	return presented != "" && hmac.Equal([]byte(CSRFTokenFor(sessionToken)), []byte(presented))
}

type Service struct {
	pool       *pgxpool.Pool
	mailer     mail.Mailer
	logger     *slog.Logger
	session    config.SessionConfig
	appBaseURL string
	now        func() time.Time

	loginByEmail *Limiter
	loginByIP    *Limiter
	resetByEmail *Limiter
	resetByIP    *Limiter
	dummyHash    string
}

type Options struct {
	Pool       *pgxpool.Pool
	Mailer     mail.Mailer
	Logger     *slog.Logger
	Session    config.SessionConfig
	AppBaseURL string
	// Now is for tests; defaults to time.Now.
	Now func() time.Time
}

func NewService(o Options) (*Service, error) {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	// Verified against when the account does not exist, so a wrong email
	// takes as long as a wrong password (no account enumeration by timing).
	dummy, err := HashPassword("dakota-timing-equaliser-not-a-password")
	if err != nil {
		return nil, err
	}
	return &Service{
		pool: o.Pool, mailer: o.Mailer, logger: o.Logger, session: o.Session,
		appBaseURL: strings.TrimRight(o.AppBaseURL, "/"), now: now,
		// Per email: brute force on one account. Per IP: spraying across
		// accounts. The IP limit is generous because every request may arrive
		// from the frontend server's address (R-23).
		loginByEmail: NewLimiter(MaxFailedLogins, LockoutDuration, now),
		loginByIP:    NewLimiter(100, LockoutDuration, now),
		resetByEmail: NewLimiter(3, time.Hour, now),
		resetByIP:    NewLimiter(30, time.Hour, now),
		dummyHash:    dummy,
	}, nil
}

func NormaliseEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

type LoginResult struct {
	Token     string
	ExpiresAt time.Time
	Principal *Principal
}

// Login checks the password and starts a session. Every outcome is audited.
// Wrong email and wrong password return the same error.
func (s *Service) Login(ctx context.Context, email, password string, req audit.Request) (*LoginResult, error) {
	email = NormaliseEmail(email)
	q := dbq.New(s.pool)
	fail := func(reason string, userID *uuid.UUID, err error) (*LoginResult, error) {
		details := map[string]any{"reason": reason}
		entityID := ""
		if userID != nil {
			entityID = userID.String()
		}
		r := req
		r.ActorID = userID
		if aerr := audit.Write(ctx, q, r, audit.Entry{Action: "auth.login_failed", EntityType: "user", EntityID: entityID, Details: details}); aerr != nil {
			return nil, aerr
		}
		return nil, err
	}

	ipKey := "ip"
	if req.IP != nil {
		ipKey = req.IP.String()
	}
	if !s.loginByIP.Allow(ipKey) || !s.loginByEmail.Allow(email) {
		return fail("rate_limited", nil, ErrTooManyAttempts)
	}
	if len(password) > MaxPasswordLength {
		return fail("password_too_long", nil, ErrInvalidCredentials)
	}

	u, err := q.GetUserForLogin(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		_, _ = VerifyPassword(password, s.dummyHash)
		return fail("unknown_email", nil, ErrInvalidCredentials)
	}
	if err != nil {
		return nil, err
	}
	if u.LockedUntil != nil && u.LockedUntil.After(s.now()) {
		return fail("locked", &u.ID, ErrTooManyAttempts)
	}
	if u.Status != "active" || u.PasswordHash == nil {
		_, _ = VerifyPassword(password, s.dummyHash)
		return fail("inactive_or_no_password", &u.ID, ErrInvalidCredentials)
	}

	ok, err := VerifyPassword(password, *u.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		res, err := q.RecordLoginFailure(ctx, dbq.RecordLoginFailureParams{
			ID: u.ID, MaxFailures: MaxFailedLogins, LockUntil: s.now().Add(LockoutDuration),
		})
		if err != nil {
			return nil, err
		}
		if res.LockedUntil != nil && res.LockedUntil.After(s.now()) {
			return fail("wrong_password_locked", &u.ID, ErrTooManyAttempts)
		}
		return fail("wrong_password", &u.ID, ErrInvalidCredentials)
	}

	var res *LoginResult
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tq := dbq.New(tx)
		if err := tq.RecordLoginSuccess(ctx, u.ID); err != nil {
			return err
		}
		if res, err = s.StartSession(ctx, tq, u.ID, req); err != nil {
			return err
		}
		r := req
		r.ActorID = &u.ID
		return audit.Write(ctx, tq, r, audit.Entry{Action: "auth.login_succeeded", EntityType: "user", EntityID: u.ID.String()})
	})
	if err != nil {
		return nil, err
	}
	s.loginByEmail.Reset(email)

	perms, err := q.ListUserPermissionKeys(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	res.Principal.Email, res.Principal.FullName, res.Principal.Kind = u.Email, u.FullName, u.Kind
	res.Principal.Permissions = perms
	return res, nil
}

// StartSession creates a session for userID inside the caller's transaction.
// The Principal carries only the session and user ids and the CSRF token;
// the caller fills in the rest. Applicant sign-in links use it too.
func (s *Service) StartSession(ctx context.Context, q *dbq.Queries, userID uuid.UUID, req audit.Request) (*LoginResult, error) {
	token, tokenHash := NewToken()
	expires := s.now().Add(s.session.TTL)
	var ua *string
	if req.UserAgent != "" {
		ua = &req.UserAgent
	}
	sessionID, err := q.CreateSession(ctx, dbq.CreateSessionParams{
		TokenHash: tokenHash, UserID: userID, ExpiresAt: expires, Ip: req.IP, UserAgent: ua,
	})
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		Token:     token,
		ExpiresAt: expires,
		Principal: &Principal{SessionID: sessionID, UserID: userID, Permissions: []string{}, CSRFToken: CSRFTokenFor(token)},
	}, nil
}

// Authenticate resolves a session cookie to a Principal. Revoked, expired,
// idle and disabled-user sessions are all ErrUnauthenticated.
func (s *Service) Authenticate(ctx context.Context, token string) (*Principal, error) {
	if token == "" || len(token) > 128 {
		return nil, ErrUnauthenticated
	}
	q := dbq.New(s.pool)
	sess, err := q.GetSessionByTokenHash(ctx, HashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	now := s.now()
	switch {
	case sess.RevokedAt != nil, sess.Status != "active", !now.Before(sess.ExpiresAt):
		return nil, ErrUnauthenticated
	case now.Sub(sess.LastSeenAt) > s.session.IdleTimeout:
		if err := q.RevokeSession(ctx, sess.ID); err != nil {
			return nil, err
		}
		return nil, ErrUnauthenticated
	}
	if now.Sub(sess.LastSeenAt) > touchInterval {
		if err := q.TouchSession(ctx, sess.ID); err != nil {
			return nil, err
		}
	}
	perms, err := q.ListUserPermissionKeys(ctx, sess.UserID)
	if err != nil {
		return nil, err
	}
	return &Principal{
		SessionID: sess.ID, UserID: sess.UserID, Email: sess.Email, FullName: sess.FullName, Kind: sess.Kind,
		Permissions: perms, CSRFToken: CSRFTokenFor(token),
	}, nil
}

func (s *Service) Logout(ctx context.Context, p *Principal, req audit.Request) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := q.RevokeSession(ctx, p.SessionID); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "auth.logout", EntityType: "session", EntityID: p.SessionID.String()})
	})
}

// RequestPasswordReset emails a single-use link if the account exists and is
// active. It always returns nil for unknown or limited requests, so the
// response never reveals whether an email is registered.
func (s *Service) RequestPasswordReset(ctx context.Context, email string, req audit.Request) error {
	email = NormaliseEmail(email)
	ipKey := "ip"
	if req.IP != nil {
		ipKey = req.IP.String()
	}
	if !s.resetByIP.Allow(ipKey) || !s.resetByEmail.Allow(email) {
		s.logger.WarnContext(ctx, "password reset rate limited", slog.String("request_id", req.RequestID))
		return nil
	}

	q := dbq.New(s.pool)
	u, err := q.GetUserForLogin(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if u.Status != "active" {
		return nil
	}

	token, hash := NewToken()
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tq := dbq.New(tx)
		if err := tq.CreatePasswordReset(ctx, dbq.CreatePasswordResetParams{
			UserID: u.ID, TokenHash: hash, ExpiresAt: s.now().Add(ResetTokenTTL), RequestedIp: req.IP,
		}); err != nil {
			return err
		}
		return audit.Write(ctx, tq, req, audit.Entry{Action: "auth.password_reset_requested", EntityType: "user", EntityID: u.ID.String()})
	})
	if err != nil {
		return err
	}

	msg := mail.Message{
		To:      u.Email,
		Subject: "Reset your Dakota Training College password",
		Text: fmt.Sprintf("Hello %s,\n\nWe received a request to reset your password. Open this link within one hour to choose a new one:\n\n%s%s\n\nIf you did not ask for this, you can ignore this email. Your password has not changed.\n\nDakota Training College\n",
			u.FullName, s.appBaseURL, ResetPath(token, u.Kind)),
	}
	// Sent in the background so response time does not reveal whether the
	// account exists. Stage 16 moves email onto a retrying queue.
	go func() {
		sendCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := s.mailer.Send(sendCtx, msg); err != nil {
			s.logger.Error("password reset email failed", slog.String("user_id", u.ID.String()), slog.String("error", err.Error()))
		}
	}()
	return nil
}

// ConfirmPasswordReset sets a new password from a valid link, spends every
// outstanding link for the user and signs them out everywhere.
func (s *Service) ConfirmPasswordReset(ctx context.Context, token, newPassword string, req audit.Request) error {
	if len(newPassword) < MinPasswordLength || len(newPassword) > MaxPasswordLength {
		return ErrWeakPassword
	}
	if token == "" || len(token) > 128 {
		return ErrInvalidResetToken
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		pr, err := q.GetPasswordResetForUpdate(ctx, HashToken(token))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidResetToken
		}
		if err != nil {
			return err
		}
		if pr.UsedAt != nil || !s.now().Before(pr.ExpiresAt) || pr.Status != "active" {
			return ErrInvalidResetToken
		}
		if err := q.SetUserPassword(ctx, dbq.SetUserPasswordParams{ID: pr.UserID, PasswordHash: &hash}); err != nil {
			return err
		}
		if err := q.UseUserPasswordResets(ctx, pr.UserID); err != nil {
			return err
		}
		revoked, err := q.RevokeUserSessions(ctx, pr.UserID)
		if err != nil {
			return err
		}
		r := req
		r.ActorID = &pr.UserID
		return audit.Write(ctx, q, r, audit.Entry{
			Action: "auth.password_reset_completed", EntityType: "user", EntityID: pr.UserID.String(),
			Details: map[string]any{"sessions_revoked": revoked},
		})
	})
}
