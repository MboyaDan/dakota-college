package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

var ErrAlreadySetUp = errors.New("a Super Admin already exists; first-run setup is only for an empty system")

// CreateFirstSuperAdmin creates the one Super Admin a new system needs
// (CLAUDE.md: production gets one Super Admin from environment variables).
// It refuses once any Super Admin exists, so it cannot be used to take over
// a running system.
func CreateFirstSuperAdmin(ctx context.Context, pool *pgxpool.Pool, email, fullName, password string) (uuid.UUID, error) {
	email = NormaliseEmail(email)
	if _, err := mail.ParseAddress(email); err != nil || email == "" {
		return uuid.Nil, fmt.Errorf("SUPER_ADMIN_EMAIL %q is not a valid email address", email)
	}
	if len(fullName) == 0 {
		return uuid.Nil, errors.New("SUPER_ADMIN_NAME is required")
	}
	if len(password) < MinPasswordLength || len(password) > MaxPasswordLength {
		return uuid.Nil, fmt.Errorf("SUPER_ADMIN_PASSWORD: %w", ErrWeakPassword)
	}
	hash, err := HashPassword(password)
	if err != nil {
		return uuid.Nil, err
	}

	var userID uuid.UUID
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		roleID, err := q.LockSuperAdminRole(ctx)
		if err != nil {
			return fmt.Errorf("super_admin role missing; run migrations first: %w", err)
		}
		n, err := q.CountSuperAdmins(ctx)
		if err != nil {
			return err
		}
		if n > 0 {
			return ErrAlreadySetUp
		}
		if userID, err = q.CreateStaffUser(ctx, dbq.CreateStaffUserParams{Email: email, FullName: fullName}); err != nil {
			return fmt.Errorf("create user (does %s already exist?): %w", email, err)
		}
		if err := q.SetUserPassword(ctx, dbq.SetUserPasswordParams{ID: userID, PasswordHash: &hash}); err != nil {
			return err
		}
		if err := q.AssignRole(ctx, dbq.AssignRoleParams{UserID: userID, RoleID: roleID}); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Request{ActorID: &userID, RequestID: "setup"}, audit.Entry{
			Action: "setup.super_admin_created", EntityType: "user", EntityID: userID.String(),
		})
	})
	return userID, err
}
