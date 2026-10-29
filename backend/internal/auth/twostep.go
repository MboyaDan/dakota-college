package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// Two-step sign-in (Stage 17b, brief 13, D-206 to D-210). Optional for each
// staff member: once on, the password alone no longer opens a session.
const (
	// ChallengeTTL is how long a person has to type their code after a
	// correct password.
	ChallengeTTL = 5 * time.Minute
	// MaxChallengeAttempts wrong codes end the challenge; they also count
	// towards the account lockout (MaxFailedLogins).
	MaxChallengeAttempts = 5
)

var (
	ErrInvalidCode      = errors.New("invalid code")
	ErrChallengeExpired = errors.New("sign-in step expired")
	ErrTwoStepEnabled   = errors.New("two-step sign-in is already on")
	ErrTwoStepNotSetUp  = errors.New("two-step sign-in is not set up")
)

// SecondStepRequired is returned by Login when the password was right and
// the account uses two-step sign-in. Any caller that does not handle it
// fails closed: no session exists yet.
type SecondStepRequired struct {
	Token     string
	ExpiresAt time.Time
}

func (e *SecondStepRequired) Error() string { return "second sign-in step required" }

// twoStepRecommended are the permissions whose holders are asked to turn
// two-step sign-in on (brief 13: finance, registrar and admin roles),
// chosen by permission, not role name.
var twoStepRecommended = []string{
	"system.admin", "users.manage", "roles.manage",
	"admissions.enrol", "students.update", "academics.results.release",
	"finance.invoice", "finance.payment.record", "finance.payment.confirm", "finance.adjust", "finance.mpesa.setup",
}

// TwoStepRecommended reports whether p holds a permission that makes
// two-step sign-in worth turning on.
func TwoStepRecommended(p *Principal) bool {
	return slices.ContainsFunc(twoStepRecommended, p.Has)
}

type TwoStepStatus struct {
	Enabled           bool
	EnabledAt         *time.Time
	RecoveryCodesLeft int64
	Recommended       bool
}

func (s *Service) TwoStepStatus(ctx context.Context, p *Principal) (TwoStepStatus, error) {
	out := TwoStepStatus{Recommended: TwoStepRecommended(p)}
	q := dbq.New(s.pool)
	t, err := q.GetUserTOTP(ctx, p.UserID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && t.ConfirmedAt == nil) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.Enabled, out.EnabledAt = true, t.ConfirmedAt
	out.RecoveryCodesLeft, err = q.CountUserRecoveryCodes(ctx, p.UserID)
	return out, err
}

// checkPassword asks for the password again before a change to two-step
// sign-in, so an unattended signed-in screen is not enough. Wrong passwords
// share the sign-in limit for the account.
func (s *Service) checkPassword(ctx context.Context, q *dbq.Queries, p *Principal, password string) error {
	if !s.loginByEmail.Allow(NormaliseEmail(p.Email)) {
		return ErrTooManyAttempts
	}
	hash, err := q.GetUserPasswordHash(ctx, p.UserID)
	if err != nil {
		return err
	}
	if hash == nil || len(password) > MaxPasswordLength {
		return ErrInvalidCredentials
	}
	ok, err := VerifyPassword(password, *hash)
	if err != nil {
		return fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return ErrInvalidCredentials
	}
	s.loginByEmail.Reset(NormaliseEmail(p.Email))
	return nil
}

type TwoStepSetup struct {
	// Secret is grouped in fours for typing into an app; URI is for the QR code.
	Secret string
	URI    string
}

// BeginTwoStep makes a new authenticator secret. Nothing changes at sign-in
// until ConfirmTwoStep proves the app has it.
func (s *Service) BeginTwoStep(ctx context.Context, p *Principal, password string, req audit.Request) (*TwoStepSetup, error) {
	q := dbq.New(s.pool)
	if err := s.checkPassword(ctx, q, p, password); err != nil {
		return nil, err
	}
	secret := make([]byte, totpSecretSize)
	_, _ = rand.Read(secret)
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tq := dbq.New(tx)
		if t, err := tq.GetUserTOTPForUpdate(ctx, p.UserID); err == nil && t.ConfirmedAt != nil {
			return ErrTwoStepEnabled
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err := tq.StartUserTOTP(ctx, dbq.StartUserTOTPParams{UserID: p.UserID, SecretSealed: s.box.seal(p.UserID, secret)}); err != nil {
			return err
		}
		return audit.Write(ctx, tq, req, audit.Entry{Action: "auth.two_step_setup_started", EntityType: "user", EntityID: p.UserID.String()})
	})
	if err != nil {
		return nil, err
	}
	return &TwoStepSetup{Secret: formatSecret(secret), URI: otpauthURI(secret, p.Email)}, nil
}

// ConfirmTwoStep turns two-step sign-in on once the person types a code from
// their app. It returns the recovery codes, which are never shown again, and
// signs the person out of their other sessions.
func (s *Service) ConfirmTwoStep(ctx context.Context, p *Principal, code string, req audit.Request) ([]string, error) {
	if !s.loginByEmail.Allow(NormaliseEmail(p.Email)) {
		return nil, ErrTooManyAttempts
	}
	var codes []string
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		t, err := q.GetUserTOTPForUpdate(ctx, p.UserID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTwoStepNotSetUp
		}
		if err != nil {
			return err
		}
		if t.ConfirmedAt != nil {
			return ErrTwoStepEnabled
		}
		secret, err := s.box.open(p.UserID, t.SecretSealed)
		if err != nil {
			return ErrTwoStepNotSetUp
		}
		step, ok := matchTOTP(secret, code, s.now(), t.LastStep)
		if !ok {
			return ErrInvalidCode
		}
		if err := q.ConfirmUserTOTP(ctx, dbq.ConfirmUserTOTPParams{UserID: p.UserID, LastStep: step}); err != nil {
			return err
		}
		if codes, err = replaceRecoveryCodes(ctx, q, p.UserID); err != nil {
			return err
		}
		revoked, err := q.RevokeOtherUserSessions(ctx, dbq.RevokeOtherUserSessionsParams{UserID: p.UserID, KeepID: p.SessionID})
		if err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{
			Action: "auth.two_step_enabled", EntityType: "user", EntityID: p.UserID.String(),
			Details: map[string]any{"other_sessions_revoked": revoked},
		})
	})
	if err != nil {
		return nil, err
	}
	s.loginByEmail.Reset(NormaliseEmail(p.Email))
	return codes, nil
}

// RegenerateRecoveryCodes replaces every recovery code, used or not.
func (s *Service) RegenerateRecoveryCodes(ctx context.Context, p *Principal, password string, req audit.Request) ([]string, error) {
	q := dbq.New(s.pool)
	if err := s.checkPassword(ctx, q, p, password); err != nil {
		return nil, err
	}
	var codes []string
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tq := dbq.New(tx)
		t, err := tq.GetUserTOTPForUpdate(ctx, p.UserID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && t.ConfirmedAt == nil) {
			return ErrTwoStepNotSetUp
		}
		if err != nil {
			return err
		}
		if codes, err = replaceRecoveryCodes(ctx, tq, p.UserID); err != nil {
			return err
		}
		return audit.Write(ctx, tq, req, audit.Entry{Action: "auth.recovery_codes_replaced", EntityType: "user", EntityID: p.UserID.String()})
	})
	return codes, err
}

// DisableTwoStep turns two-step sign-in off for the signed-in person.
func (s *Service) DisableTwoStep(ctx context.Context, p *Principal, password string, req audit.Request) error {
	q := dbq.New(s.pool)
	if err := s.checkPassword(ctx, q, p, password); err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tq := dbq.New(tx)
		n, err := RemoveTwoStep(ctx, tq, p.UserID)
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrTwoStepNotSetUp
		}
		return audit.Write(ctx, tq, req, audit.Entry{Action: "auth.two_step_disabled", EntityType: "user", EntityID: p.UserID.String()})
	})
}

// RemoveTwoStep deletes a person's authenticator and recovery codes inside
// the caller's transaction. It returns 0 if there was no authenticator. An
// admin reset (access.Service) and the command line use it too.
func RemoveTwoStep(ctx context.Context, q *dbq.Queries, userID uuid.UUID) (int64, error) {
	if err := q.DeleteUserRecoveryCodes(ctx, userID); err != nil {
		return 0, err
	}
	return q.DeleteUserTOTP(ctx, userID)
}

func replaceRecoveryCodes(ctx context.Context, q *dbq.Queries, userID uuid.UUID) ([]string, error) {
	if err := q.DeleteUserRecoveryCodes(ctx, userID); err != nil {
		return nil, err
	}
	codes := make([]string, RecoveryCodeCount)
	for i := range codes {
		codes[i] = newRecoveryCode()
		if err := q.InsertUserRecoveryCode(ctx, dbq.InsertUserRecoveryCodeParams{UserID: userID, CodeHash: recoveryCodeHash(codes[i])}); err != nil {
			return nil, err
		}
	}
	return codes, nil
}

// usesTwoStep reports whether a confirmed authenticator exists.
func usesTwoStep(ctx context.Context, q *dbq.Queries, userID uuid.UUID) (bool, error) {
	t, err := q.GetUserTOTP(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return t.ConfirmedAt != nil, nil
}

// startChallenge records a correct password that still needs a code. The
// failed-sign-in count is not cleared yet, so wrong codes keep counting
// towards the lockout across fresh passwords.
func (s *Service) startChallenge(ctx context.Context, q *dbq.Queries, userID uuid.UUID, req audit.Request) (*SecondStepRequired, error) {
	token, hash := NewToken()
	expires := s.now().Add(ChallengeTTL)
	var ua *string
	if req.UserAgent != "" {
		ua = &req.UserAgent
	}
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tq := dbq.New(tx)
		if err := tq.DeleteOldLoginChallenges(ctx); err != nil {
			return err
		}
		if err := tq.CreateLoginChallenge(ctx, dbq.CreateLoginChallengeParams{
			TokenHash: hash, UserID: userID, ExpiresAt: expires, Ip: req.IP, UserAgent: ua,
		}); err != nil {
			return err
		}
		r := req
		r.ActorID = &userID
		return audit.Write(ctx, tq, r, audit.Entry{Action: "auth.password_accepted", EntityType: "user", EntityID: userID.String(),
			Details: map[string]any{"second_step": "pending"}})
	})
	if err != nil {
		return nil, err
	}
	return &SecondStepRequired{Token: token, ExpiresAt: expires}, nil
}

// CompleteSecondStep checks the code for a sign-in challenge and starts the
// session. code is a six-digit authenticator code or a recovery code.
func (s *Service) CompleteSecondStep(ctx context.Context, challengeToken, code string, req audit.Request) (*LoginResult, error) {
	if challengeToken == "" || len(challengeToken) > 128 || len(code) > 64 {
		return nil, ErrChallengeExpired
	}
	var (
		res     *LoginResult
		failure error
		email   string
		userID  uuid.UUID
	)
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		ch, err := q.GetLoginChallengeForUpdate(ctx, HashToken(challengeToken))
		if errors.Is(err, pgx.ErrNoRows) {
			failure = ErrChallengeExpired
			return nil
		}
		if err != nil {
			return err
		}
		email, userID = ch.Email, ch.UserID
		r := req
		r.ActorID = &ch.UserID
		fail := func(reason string, err error) error {
			failure = err
			return audit.Write(ctx, q, r, audit.Entry{Action: "auth.login_failed", EntityType: "user", EntityID: ch.UserID.String(),
				Details: map[string]any{"reason": reason, "step": "second"}})
		}
		if ch.UsedAt != nil || !s.now().Before(ch.ExpiresAt) || ch.Attempts >= MaxChallengeAttempts || ch.Status != "active" {
			return fail("challenge_expired", ErrChallengeExpired)
		}
		// The password step already counted towards the in-memory limit;
		// wrong codes count towards the database lockout below.
		u, err := q.GetUserForLogin(ctx, ch.Email)
		if err != nil {
			return err
		}
		if u.LockedUntil != nil && u.LockedUntil.After(s.now()) {
			return fail("locked", ErrTooManyAttempts)
		}

		method, ok, err := s.checkSecondStep(ctx, q, ch.UserID, code)
		if err != nil {
			return err
		}
		if !ok {
			if err := q.CountLoginChallengeAttempt(ctx, ch.ID); err != nil {
				return err
			}
			lock, err := q.RecordLoginFailure(ctx, dbq.RecordLoginFailureParams{
				ID: ch.UserID, MaxFailures: MaxFailedLogins, LockUntil: s.now().Add(LockoutDuration),
			})
			if err != nil {
				return err
			}
			if lock.LockedUntil != nil && lock.LockedUntil.After(s.now()) {
				return fail("wrong_code_locked", ErrTooManyAttempts)
			}
			return fail("wrong_code", ErrInvalidCode)
		}

		if err := q.UseLoginChallenge(ctx, ch.ID); err != nil {
			return err
		}
		if err := q.RecordLoginSuccess(ctx, ch.UserID); err != nil {
			return err
		}
		if res, err = s.StartSession(ctx, q, ch.UserID, req); err != nil {
			return err
		}
		details := map[string]any{"second_step": method}
		if method == "recovery_code" {
			left, err := q.CountUserRecoveryCodes(ctx, ch.UserID)
			if err != nil {
				return err
			}
			details["recovery_codes_left"] = left
		}
		return audit.Write(ctx, q, r, audit.Entry{Action: "auth.login_succeeded", EntityType: "user", EntityID: ch.UserID.String(), Details: details})
	})
	if err != nil {
		return nil, err
	}
	if failure != nil {
		return nil, failure
	}
	s.loginByEmail.Reset(email)
	return s.fillPrincipal(ctx, res, userID)
}

// checkSecondStep tries code as a recovery code or an authenticator code.
// A matched code is spent inside the caller's transaction.
func (s *Service) checkSecondStep(ctx context.Context, q *dbq.Queries, userID uuid.UUID, code string) (method string, ok bool, err error) {
	if looksLikeRecoveryCode(code) {
		n, err := q.UseUserRecoveryCode(ctx, dbq.UseUserRecoveryCodeParams{UserID: userID, CodeHash: recoveryCodeHash(code)})
		return "recovery_code", n == 1, err
	}
	t, err := q.GetUserTOTPForUpdate(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "authenticator", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if t.ConfirmedAt == nil {
		return "authenticator", false, nil
	}
	secret, err := s.box.open(userID, t.SecretSealed)
	if err != nil {
		// A changed TOTP_KEY: only a recovery code or an admin reset helps.
		s.logger.ErrorContext(ctx, "two-step secret could not be opened (was TOTP_KEY changed?)")
		return "authenticator", false, nil
	}
	step, ok := matchTOTP(secret, code, s.now(), t.LastStep)
	if !ok {
		return "authenticator", false, nil
	}
	return "authenticator", true, q.SetUserTOTPStep(ctx, dbq.SetUserTOTPStepParams{UserID: userID, LastStep: step})
}

// fillPrincipal completes a new session's Principal with the user's details.
func (s *Service) fillPrincipal(ctx context.Context, res *LoginResult, userID uuid.UUID) (*LoginResult, error) {
	q := dbq.New(s.pool)
	u, err := q.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	perms, err := q.ListUserPermissionKeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	res.Principal.Email, res.Principal.FullName, res.Principal.Kind = u.Email, u.FullName, u.Kind
	res.Principal.Permissions = perms
	return res, nil
}
