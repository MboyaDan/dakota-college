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
	var step *auth.SecondStepRequired
	switch {
	case errors.As(err, &step):
		// Right password, and the account uses two-step sign-in: no session
		// yet, only a short-lived challenge for the code.
		h.cookie.setStep(c, step.Token, step.ExpiresAt)
		c.JSON(http.StatusAccepted, gin.H{"second_step": "code", "expires_at": step.ExpiresAt})
		return
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

func (h authHandlers) loginCode(c *gin.Context) {
	var body struct {
		Code string `json:"code" binding:"required,max=64"`
	}
	if !bindJSON(c, &body) {
		return
	}
	token, _ := c.Cookie(h.cookie.stepName())
	res, err := h.svc.CompleteSecondStep(c.Request.Context(), token, body.Code, auditRequest(c))
	switch {
	case errors.Is(err, auth.ErrInvalidCode):
		writeError(c, http.StatusUnauthorized, "invalid_code", "That code is not right. Type the newest code from your app, or use one of your recovery codes.")
		return
	case errors.Is(err, auth.ErrChallengeExpired):
		h.cookie.clearStep(c)
		writeError(c, http.StatusUnauthorized, "sign_in_again", "This sign-in has expired. Enter your email and password again.")
		return
	case errors.Is(err, auth.ErrTooManyAttempts):
		h.cookie.clearStep(c)
		writeError(c, http.StatusTooManyRequests, "too_many_attempts", "Too many sign-in attempts. Wait 15 minutes, or ask an administrator to reset your two-step sign-in.")
		return
	case err != nil:
		internalError(c, err)
		return
	}
	h.cookie.clearStep(c)
	h.cookie.set(c, res.Token, res.ExpiresAt)
	out := toSessionResponse(res.Principal)
	out.ExpiresAt = &res.ExpiresAt
	c.JSON(http.StatusOK, out)
}

type twoStepStatus struct {
	Enabled           bool       `json:"enabled"`
	EnabledAt         *time.Time `json:"enabled_at"`
	RecoveryCodesLeft int64      `json:"recovery_codes_left"`
	Recommended       bool       `json:"recommended"`
}

func (h authHandlers) twoStepStatus(c *gin.Context) {
	st, err := h.svc.TwoStepStatus(c.Request.Context(), principal(c))
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, twoStepStatus{Enabled: st.Enabled, EnabledAt: st.EnabledAt, RecoveryCodesLeft: st.RecoveryCodesLeft, Recommended: st.Recommended})
}

// twoStepError answers the errors shared by the two-step settings endpoints.
func twoStepError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(c, http.StatusUnprocessableEntity, "wrong_password", "That password is not right. Type the password you sign in with.")
	case errors.Is(err, auth.ErrTooManyAttempts):
		writeError(c, http.StatusTooManyRequests, "too_many_attempts", "Too many attempts. Wait 15 minutes and try again.")
	case errors.Is(err, auth.ErrInvalidCode):
		writeError(c, http.StatusUnprocessableEntity, "invalid_code", "That code is not right. Check the time on your phone is set automatically, then type the newest code.")
	case errors.Is(err, auth.ErrTwoStepEnabled):
		writeError(c, http.StatusConflict, "two_step_already_on", "Two-step sign-in is already on. Turn it off first to move it to a new phone.")
	case errors.Is(err, auth.ErrTwoStepNotSetUp):
		writeError(c, http.StatusConflict, "two_step_not_set_up", "Two-step sign-in is not on. Start the setup again.")
	default:
		internalError(c, err)
	}
}

type passwordBody struct {
	Password string `json:"password" binding:"required"`
}

func (h authHandlers) twoStepSetup(c *gin.Context) {
	var body passwordBody
	if !bindJSON(c, &body) {
		return
	}
	setup, err := h.svc.BeginTwoStep(c.Request.Context(), principal(c), body.Password, auditRequest(c))
	if err != nil {
		twoStepError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"secret": setup.Secret, "otpauth_uri": setup.URI})
}

func (h authHandlers) twoStepConfirm(c *gin.Context) {
	var body struct {
		Code string `json:"code" binding:"required,max=64"`
	}
	if !bindJSON(c, &body) {
		return
	}
	codes, err := h.svc.ConfirmTwoStep(c.Request.Context(), principal(c), body.Code, auditRequest(c))
	if err != nil {
		twoStepError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"recovery_codes": codes})
}

func (h authHandlers) twoStepRecoveryCodes(c *gin.Context) {
	var body passwordBody
	if !bindJSON(c, &body) {
		return
	}
	codes, err := h.svc.RegenerateRecoveryCodes(c.Request.Context(), principal(c), body.Password, auditRequest(c))
	if err != nil {
		twoStepError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"recovery_codes": codes})
}

func (h authHandlers) twoStepDisable(c *gin.Context) {
	var body passwordBody
	if !bindJSON(c, &body) {
		return
	}
	if err := h.svc.DisableTwoStep(c.Request.Context(), principal(c), body.Password, auditRequest(c)); err != nil {
		twoStepError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
