// Package access administers staff accounts and roles. It enforces the
// protections in brief section 11 (AC-12):
//   - the last active Super Admin can never be removed or disabled;
//   - system.admin is never grantable through role configuration;
//   - only a holder of system.admin can give or take a role that carries it;
//   - a role in use, or a starter role, cannot be deleted.
package access

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

const (
	SystemAdmin    = "system.admin"
	superAdminRole = "super_admin"
)

var (
	ErrNotFound                = errors.New("not found")
	ErrLastSuperAdmin          = errors.New("this would leave no active Super Admin")
	ErrNeedsSystemAdmin        = errors.New("only a Super Admin can give or remove a role that includes system.admin")
	ErrSystemAdminNotGrantable = errors.New("system.admin cannot be added to a role")
	ErrProtectedRole           = errors.New("this role is protected")
	ErrRoleInUse               = errors.New("this role is assigned to users; remove it from them first")
	ErrUnknownRole             = errors.New("unknown role")
	ErrUnknownPermission       = errors.New("unknown permission")
	ErrSelf                    = errors.New("you cannot disable your own account")
	ErrDuplicate               = errors.New("already exists")
)

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func (s *Service) ListUsers(ctx context.Context) ([]dbq.ListUsersRow, error) {
	return dbq.New(s.pool).ListUsers(ctx)
}

func (s *Service) ListRoles(ctx context.Context) ([]dbq.ListRolesRow, error) {
	return dbq.New(s.pool).ListRoles(ctx)
}

func (s *Service) ListPermissions(ctx context.Context) ([]dbq.Permission, error) {
	return dbq.New(s.pool).ListPermissions(ctx)
}

// inTx runs fn holding the super_admin role lock, so concurrent changes that
// could each remove "another" Super Admin are serialised. It refuses to
// commit a change that takes the number of active Super Admins to zero.
func (s *Service) inTx(ctx context.Context, fn func(q *dbq.Queries) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if _, err := q.LockSuperAdminRole(ctx); err != nil {
			return err
		}
		before, err := q.CountActiveSuperAdmins(ctx)
		if err != nil {
			return err
		}
		if err := fn(q); err != nil {
			return err
		}
		after, err := q.CountActiveSuperAdmins(ctx)
		if err != nil {
			return err
		}
		if before > 0 && after == 0 {
			return ErrLastSuperAdmin
		}
		return nil
	})
}

func normaliseKeys(keys []string) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if k = strings.TrimSpace(k); k != "" && !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

// checkSystemAdminChange refuses giving or removing a role that carries
// system.admin unless the actor holds it.
func checkSystemAdminChange(ctx context.Context, q *dbq.Queries, actor *auth.Principal, changed []string) error {
	if len(changed) == 0 || actor.Has(SystemAdmin) {
		return nil
	}
	carries, err := q.RolesGrantPermission(ctx, dbq.RolesGrantPermissionParams{RoleKeys: changed, PermissionKey: SystemAdmin})
	if err != nil {
		return err
	}
	if carries {
		return ErrNeedsSystemAdmin
	}
	return nil
}

func (s *Service) assignRoles(ctx context.Context, q *dbq.Queries, actor *auth.Principal, userID uuid.UUID, keys []string) (before []string, err error) {
	roles, err := q.GetRolesByKeys(ctx, keys)
	if err != nil {
		return nil, err
	}
	if len(roles) != len(keys) {
		return nil, ErrUnknownRole
	}
	if before, err = q.ListUserRoleKeys(ctx, userID); err != nil {
		return nil, err
	}
	var changed []string
	for _, k := range keys {
		if !slices.Contains(before, k) {
			changed = append(changed, k)
		}
	}
	for _, k := range before {
		if !slices.Contains(keys, k) {
			changed = append(changed, k)
		}
	}
	if err := checkSystemAdminChange(ctx, q, actor, changed); err != nil {
		return nil, err
	}
	if err := q.DeleteUserRoles(ctx, userID); err != nil {
		return nil, err
	}
	for _, r := range roles {
		if err := q.AssignRole(ctx, dbq.AssignRoleParams{UserID: userID, RoleID: r.ID, GrantedBy: &actor.UserID}); err != nil {
			return nil, err
		}
	}
	return before, nil
}

// CreateStaffUser adds a staff account with no password; the person sets one
// through the password reset email.
func (s *Service) CreateStaffUser(ctx context.Context, actor *auth.Principal, email, fullName string, roleKeys []string, req audit.Request) (uuid.UUID, error) {
	email = auth.NormaliseEmail(email)
	fullName = strings.TrimSpace(fullName)
	roleKeys = normaliseKeys(roleKeys)
	var id uuid.UUID
	err := s.inTx(ctx, func(q *dbq.Queries) error {
		var err error
		if id, err = q.CreateStaffUser(ctx, dbq.CreateStaffUserParams{Email: email, FullName: fullName}); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return fmt.Errorf("a user with this email %w", ErrDuplicate)
			}
			return err
		}
		if _, err := s.assignRoles(ctx, q, actor, id, roleKeys); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{
			Action: "user.created", EntityType: "user", EntityID: id.String(),
			Details: map[string]any{"roles": roleKeys},
		})
	})
	return id, err
}

func (s *Service) SetUserRoles(ctx context.Context, actor *auth.Principal, userID uuid.UUID, roleKeys []string, req audit.Request) error {
	roleKeys = normaliseKeys(roleKeys)
	return s.inTx(ctx, func(q *dbq.Queries) error {
		if _, err := q.GetUserByID(ctx, userID); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		before, err := s.assignRoles(ctx, q, actor, userID, roleKeys)
		if err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{
			Action: "user.roles_changed", EntityType: "user", EntityID: userID.String(),
			Details: map[string]any{"from": before, "to": roleKeys},
		})
	})
}

// SetUserActive enables or disables an account. Disabling signs the person out
// everywhere. People are never deleted (brief 10).
func (s *Service) SetUserActive(ctx context.Context, actor *auth.Principal, userID uuid.UUID, active bool, req audit.Request) error {
	if !active && userID == actor.UserID {
		return ErrSelf
	}
	status, action := "disabled", "user.disabled"
	if active {
		status, action = "active", "user.enabled"
	}
	return s.inTx(ctx, func(q *dbq.Queries) error {
		if _, err := q.GetUserByID(ctx, userID); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		roles, err := q.ListUserRoleKeys(ctx, userID)
		if err != nil {
			return err
		}
		if err := checkSystemAdminChange(ctx, q, actor, roles); err != nil {
			return err
		}
		if err := q.SetUserStatus(ctx, dbq.SetUserStatusParams{ID: userID, Status: status}); err != nil {
			return err
		}
		if !active {
			if _, err := q.RevokeUserSessions(ctx, userID); err != nil {
				return err
			}
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: action, EntityType: "user", EntityID: userID.String()})
	})
}

func (s *Service) validatePermissions(ctx context.Context, q *dbq.Queries, keys []string) error {
	if slices.Contains(keys, SystemAdmin) {
		return ErrSystemAdminNotGrantable
	}
	n, err := q.CountPermissions(ctx, keys)
	if err != nil {
		return err
	}
	if int(n) != len(keys) {
		return ErrUnknownPermission
	}
	return nil
}

func (s *Service) CreateRole(ctx context.Context, key, name, description string, permissionKeys []string, req audit.Request) (uuid.UUID, error) {
	permissionKeys = normaliseKeys(permissionKeys)
	var id uuid.UUID
	err := s.inTx(ctx, func(q *dbq.Queries) error {
		if err := s.validatePermissions(ctx, q, permissionKeys); err != nil {
			return err
		}
		var err error
		if id, err = q.CreateRole(ctx, dbq.CreateRoleParams{Key: strings.TrimSpace(key), Name: strings.TrimSpace(name), Description: strings.TrimSpace(description)}); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return fmt.Errorf("a role with this key %w", ErrDuplicate)
			}
			return err
		}
		for _, p := range permissionKeys {
			if err := q.InsertRolePermission(ctx, dbq.InsertRolePermissionParams{RoleID: id, PermissionKey: p}); err != nil {
				return err
			}
		}
		return audit.Write(ctx, q, req, audit.Entry{
			Action: "role.created", EntityType: "role", EntityID: id.String(),
			Details: map[string]any{"key": key, "permissions": permissionKeys},
		})
	})
	return id, err
}

// SetRolePermissions replaces a role's permissions. The Super Admin role is
// fixed (it always holds everything) and system.admin can never be added.
func (s *Service) SetRolePermissions(ctx context.Context, roleID uuid.UUID, permissionKeys []string, req audit.Request) error {
	permissionKeys = normaliseKeys(permissionKeys)
	return s.inTx(ctx, func(q *dbq.Queries) error {
		role, err := q.GetRoleByID(ctx, roleID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if role.Key == superAdminRole {
			return ErrProtectedRole
		}
		if err := s.validatePermissions(ctx, q, permissionKeys); err != nil {
			return err
		}
		if err := q.DeleteRolePermissions(ctx, roleID); err != nil {
			return err
		}
		for _, p := range permissionKeys {
			if err := q.InsertRolePermission(ctx, dbq.InsertRolePermissionParams{RoleID: roleID, PermissionKey: p}); err != nil {
				return err
			}
		}
		return audit.Write(ctx, q, req, audit.Entry{
			Action: "role.permissions_changed", EntityType: "role", EntityID: roleID.String(),
			Details: map[string]any{"key": role.Key, "permissions": permissionKeys},
		})
	})
}

func (s *Service) DeleteRole(ctx context.Context, roleID uuid.UUID, req audit.Request) error {
	return s.inTx(ctx, func(q *dbq.Queries) error {
		role, err := q.GetRoleByID(ctx, roleID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if role.IsSystem {
			return ErrProtectedRole
		}
		n, err := q.CountUsersWithRole(ctx, roleID)
		if err != nil {
			return err
		}
		if n > 0 {
			return ErrRoleInUse
		}
		if err := q.DeleteRole(ctx, roleID); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{
			Action: "role.deleted", EntityType: "role", EntityID: roleID.String(), Details: map[string]any{"key": role.Key},
		})
	})
}
