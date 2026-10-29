package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/access"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
)

type adminHandlers struct {
	svc    *access.Service
	auth   *auth.Service
	logger *slog.Logger
}

var roleKeyPattern = regexp.MustCompile(`^[a-z][a-z_]{1,48}$`)

// accessError maps access errors to responses that say what to do.
func accessError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, access.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", "That user or role does not exist.")
	case errors.Is(err, access.ErrLastSuperAdmin):
		writeError(c, http.StatusConflict, "last_super_admin", "This would leave no active Super Admin. Give another person the Super Admin role first.")
	case errors.Is(err, access.ErrNeedsSystemAdmin):
		writeError(c, http.StatusForbidden, "needs_super_admin", "Only a Super Admin can give or remove the Super Admin role.")
	case errors.Is(err, access.ErrSystemAdminNotGrantable):
		writeError(c, http.StatusUnprocessableEntity, "system_admin_not_grantable", "system.admin cannot be added to a role. It belongs only to the Super Admin role.")
	case errors.Is(err, access.ErrProtectedRole):
		writeError(c, http.StatusConflict, "protected_role", "This role is protected and cannot be changed this way.")
	case errors.Is(err, access.ErrRoleInUse):
		writeError(c, http.StatusConflict, "role_in_use", "This role is assigned to people. Remove it from them before deleting it.")
	case errors.Is(err, access.ErrUnknownRole):
		writeError(c, http.StatusUnprocessableEntity, "unknown_role", "One of the roles does not exist. Reload the page and choose again.")
	case errors.Is(err, access.ErrUnknownPermission):
		writeError(c, http.StatusUnprocessableEntity, "unknown_permission", "One of the permissions does not exist. Reload the page and choose again.")
	case errors.Is(err, access.ErrNotStaff):
		writeError(c, http.StatusConflict, "not_staff_account", "Roles can only be given to staff accounts.")
	case errors.Is(err, access.ErrSelf):
		writeError(c, http.StatusConflict, "cannot_disable_self", "You cannot disable your own account. Ask another administrator.")
	case errors.Is(err, access.ErrSelfTwoStep):
		writeError(c, http.StatusConflict, "cannot_reset_self", "To turn off your own two-step sign-in, use Your account, where you confirm your password.")
	case errors.Is(err, access.ErrNoTwoStep):
		writeError(c, http.StatusConflict, "two_step_not_on", "This person does not use two-step sign-in, so there is nothing to reset.")
	case errors.Is(err, access.ErrDuplicate):
		writeError(c, http.StatusConflict, "already_exists", "That already exists. Use a different email or key.")
	default:
		internalError(c, err)
	}
}

func pathID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusNotFound, "not_found", "That user or role does not exist.")
		return uuid.Nil, false
	}
	return id, true
}

type adminUser struct {
	ID       string   `json:"id"`
	Email    string   `json:"email"`
	FullName string   `json:"full_name"`
	Status   string   `json:"status"`
	RoleKeys []string `json:"role_keys"`
	// TwoStepEnabled: the person signs in with a code as well (Stage 17b).
	TwoStepEnabled bool       `json:"two_step_enabled"`
	LastLoginAt    *time.Time `json:"last_login_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (h adminHandlers) listUsers(c *gin.Context) {
	rows, err := h.svc.ListUsers(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	out := make([]adminUser, 0, len(rows))
	for _, r := range rows {
		out = append(out, adminUser{ID: r.ID.String(), Email: r.Email, FullName: r.FullName, Status: r.Status, RoleKeys: r.RoleKeys, TwoStepEnabled: r.TwoStepEnabled, LastLoginAt: r.LastLoginAt, CreatedAt: r.CreatedAt})
	}
	c.JSON(http.StatusOK, gin.H{"users": out})
}

func (h adminHandlers) createUser(c *gin.Context) {
	var body struct {
		Email    string   `json:"email" binding:"required,email,max=254"`
		FullName string   `json:"full_name" binding:"required,min=1,max=200"`
		RoleKeys []string `json:"role_keys" binding:"max=20"`
	}
	if !bindJSON(c, &body) {
		return
	}
	id, err := h.svc.CreateStaffUser(c.Request.Context(), principal(c), body.Email, body.FullName, body.RoleKeys, auditRequest(c))
	if err != nil {
		accessError(c, err)
		return
	}
	// The new person sets their own password through the reset email.
	if err := h.auth.RequestPasswordReset(c.Request.Context(), body.Email, auditRequest(c)); err != nil {
		h.logger.ErrorContext(c.Request.Context(), "set-password email for new user failed", slog.String("error", err.Error()))
	}
	c.JSON(http.StatusCreated, gin.H{"id": id.String()})
}

func (h adminHandlers) setUserRoles(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var body struct {
		RoleKeys []string `json:"role_keys" binding:"max=20"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if err := h.svc.SetUserRoles(c.Request.Context(), principal(c), id, body.RoleKeys, auditRequest(c)); err != nil {
		accessError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h adminHandlers) disableUser(c *gin.Context) { h.setActive(c, false) }
func (h adminHandlers) enableUser(c *gin.Context)  { h.setActive(c, true) }

func (h adminHandlers) setActive(c *gin.Context, active bool) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.svc.SetUserActive(c.Request.Context(), principal(c), id, active, auditRequest(c)); err != nil {
		accessError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h adminHandlers) resetTwoStep(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.svc.ResetTwoStep(c.Request.Context(), principal(c), id, auditRequest(c)); err != nil {
		accessError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type adminRole struct {
	ID             string   `json:"id"`
	Key            string   `json:"key"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	IsSystem       bool     `json:"is_system"`
	PermissionKeys []string `json:"permission_keys"`
	UserCount      int64    `json:"user_count"`
}

func (h adminHandlers) listRoles(c *gin.Context) {
	rows, err := h.svc.ListRoles(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	out := make([]adminRole, 0, len(rows))
	for _, r := range rows {
		out = append(out, adminRole{ID: r.ID.String(), Key: r.Key, Name: r.Name, Description: r.Description, IsSystem: r.IsSystem, PermissionKeys: r.PermissionKeys, UserCount: r.UserCount})
	}
	c.JSON(http.StatusOK, gin.H{"roles": out})
}

func (h adminHandlers) createRole(c *gin.Context) {
	var body struct {
		Key            string   `json:"key" binding:"required"`
		Name           string   `json:"name" binding:"required,min=1,max=100"`
		Description    string   `json:"description" binding:"max=500"`
		PermissionKeys []string `json:"permission_keys" binding:"max=100"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if !roleKeyPattern.MatchString(body.Key) {
		writeError(c, http.StatusUnprocessableEntity, "invalid_role_key", "Use lowercase letters and underscores for the role key, for example exam_officer.")
		return
	}
	id, err := h.svc.CreateRole(c.Request.Context(), body.Key, body.Name, body.Description, body.PermissionKeys, auditRequest(c))
	if err != nil {
		accessError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id.String()})
}

func (h adminHandlers) setRolePermissions(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var body struct {
		PermissionKeys []string `json:"permission_keys" binding:"max=100"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if err := h.svc.SetRolePermissions(c.Request.Context(), id, body.PermissionKeys, auditRequest(c)); err != nil {
		accessError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h adminHandlers) deleteRole(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteRole(c.Request.Context(), id, auditRequest(c)); err != nil {
		accessError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h adminHandlers) listPermissions(c *gin.Context) {
	rows, err := h.svc.ListPermissions(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	type perm struct {
		Key         string `json:"key"`
		Module      string `json:"module"`
		Description string `json:"description"`
		Grantable   bool   `json:"grantable"`
	}
	out := make([]perm, 0, len(rows))
	for _, r := range rows {
		out = append(out, perm{Key: r.Key, Module: r.Module, Description: r.Description, Grantable: r.Key != access.SystemAdmin})
	}
	c.JSON(http.StatusOK, gin.H{"permissions": out})
}
