package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/dakota-college/backend/internal/applicant"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
)

// Applicant sign-in by emailed link and the applicant's own applications
// (Stage 8b, D-96).

type applicantHandlers struct {
	svc    *applicant.Service
	cookie sessionCookie
}

// linkSent is the same answer whether or not anything was sent.
const linkSent = "If we can find your details, we have emailed you a link. It works once, for 30 minutes. Check your spam folder if it does not arrive in a few minutes."

func (h applicantHandlers) requestLink(c *gin.Context) {
	var body struct {
		Email    string `json:"email" binding:"max=500"`
		FullName string `json:"full_name" binding:"max=500"`
	}
	if !bindJSON(c, &body) {
		return
	}
	var p content.Problems
	p.Required("email", strings.TrimSpace(body.Email), 254)
	p.Email("email", strings.TrimSpace(body.Email))
	p.Optional("full_name", strings.TrimSpace(body.FullName), applicant.MaxNameLength)
	if len(p) > 0 {
		writeValidation(c, p)
		return
	}
	if err := h.svc.RequestLink(c.Request.Context(), body.Email, body.FullName, auditRequest(c)); err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message": linkSent})
}

func (h applicantHandlers) resume(c *gin.Context) {
	var body struct {
		Reference string `json:"reference" binding:"max=100"`
		Email     string `json:"email" binding:"max=500"`
	}
	if !bindJSON(c, &body) {
		return
	}
	var p content.Problems
	p.Required("reference", strings.TrimSpace(body.Reference), 40)
	p.Required("email", strings.TrimSpace(body.Email), 254)
	p.Email("email", strings.TrimSpace(body.Email))
	if len(p) > 0 {
		writeValidation(c, p)
		return
	}
	if err := h.svc.ResumeByReference(c.Request.Context(), body.Reference, body.Email, auditRequest(c)); err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message": linkSent})
}

func (h applicantHandlers) signIn(c *gin.Context) {
	var body struct {
		Token string `json:"token" binding:"required,max=128"`
	}
	if !bindJSON(c, &body) {
		return
	}
	res, err := h.svc.SignIn(c.Request.Context(), body.Token, auditRequest(c))
	switch {
	case errors.Is(err, applicant.ErrInvalidLink):
		writeError(c, http.StatusBadRequest, "invalid_sign_in_link", "This link has expired or was already used. Ask for a new one.")
		return
	case err != nil:
		internalError(c, err)
		return
	}
	h.cookie.set(c, res.Token, res.ExpiresAt)
	out := struct {
		sessionResponse
		ApplicationID *string `json:"application_id"`
	}{sessionResponse: toSessionResponse(res.Principal)}
	out.ExpiresAt = &res.ExpiresAt
	if res.ApplicationID != nil {
		id := res.ApplicationID.String()
		out.ApplicationID = &id
	}
	c.JSON(http.StatusOK, out)
}

func (h applicantHandlers) applications(c *gin.Context) {
	items, err := h.svc.Applications(c.Request.Context(), principal(c).UserID)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
