package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/dakota-college/backend/internal/auth"
)

type authHandlers struct {
	svc    *auth.Service
	cookie sessionCookie
	logger *slog.Logger
}

type sessionUser struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Kind     string `json:"kind"`
}

type sessionResponse struct {
	User        sessionUser `json:"user"`
	Permissions []string    `json:"permissions"`
	CSRFToken   string      `json:"csrf_token"`
	ExpiresAt   *time.Time  `json:"expires_at,omitempty"`
}

func toSessionResponse(p *auth.Principal) sessionResponse {
	return sessionResponse{
		User:        sessionUser{ID: p.UserID.String(), Email: p.Email, FullName: p.FullName, Kind: p.Kind},
		Permissions: p.Permissions,
		CSRFToken:   p.CSRFToken,
	}
}

func bindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Check the form and try again: some details are missing or in the wrong format.")
		return false
	}
	return true
}

func internalError(c *gin.Context, err error) {
	_ = c.Error(err)
	writeError(c, http.StatusInternalServerError, "internal_error", "Something went wrong on our side. Please try again.")
}

func (h authHandlers) login(c *gin.Context) {
	var body struct {
		Email    string `json:"email" binding:"required,max=254"`
		Password string `json:"password" binding:"required"`
	}
	if !bindJSON(c, &body) {
		return
	}
	res, err := h.svc.Login(c.Request.Context(), body.Email, body.Password, auditRequest(c))
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(c, http.StatusUnauthorized, "invalid_credentials", "The email or password is not correct. Check them and try again.")
		return
	case errors.Is(err, auth.ErrTooManyAttempts):
		writeError(c, http.StatusTooManyRequests, "too_many_attempts", "Too many sign-in attempts. Wait 15 minutes, or reset your password.")
		return
	case err != nil:
		internalError(c, err)
		return
	}
	h.cookie.set(c, res.Token, res.ExpiresAt)
	out := toSessionResponse(res.Principal)
	out.ExpiresAt = &res.ExpiresAt
	c.JSON(http.StatusOK, out)
}

func (h authHandlers) session(c *gin.Context) {
	c.JSON(http.StatusOK, toSessionResponse(principal(c)))
}

func (h authHandlers) logout(c *gin.Context) {
	if err := h.svc.Logout(c.Request.Context(), principal(c), auditRequest(c)); err != nil {
		internalError(c, err)
		return
	}
	h.cookie.clear(c)
	c.Status(http.StatusNoContent)
}

func (h authHandlers) requestReset(c *gin.Context) {
	var body struct {
		Email string `json:"email" binding:"required,email,max=254"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if err := h.svc.RequestPasswordReset(c.Request.Context(), body.Email, auditRequest(c)); err != nil {
		internalError(c, err)
		return
	}
	// Same answer whether or not the account exists.
	c.JSON(http.StatusAccepted, gin.H{"message": "If an account uses this email, we have sent a link to reset the password. It works for one hour."})
}

func (h authHandlers) confirmReset(c *gin.Context) {
	var body struct {
		Token    string `json:"token" binding:"required,max=128"`
		Password string `json:"password" binding:"required"`
	}
	if !bindJSON(c, &body) {
		return
	}
	err := h.svc.ConfirmPasswordReset(c.Request.Context(), body.Token, body.Password, auditRequest(c))
	switch {
	case errors.Is(err, auth.ErrWeakPassword):
		writeError(c, http.StatusUnprocessableEntity, "weak_password", "Choose a password of at least 12 characters. A short sentence is easy to remember.")
	case errors.Is(err, auth.ErrInvalidResetToken):
		writeError(c, http.StatusBadRequest, "invalid_reset_link", "This reset link has expired or was already used. Ask for a new one.")
	case err != nil:
		internalError(c, err)
	default:
		c.Status(http.StatusNoContent)
	}
}
