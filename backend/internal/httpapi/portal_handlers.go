package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/academics"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/portal"
)

// The student portal (Stage 12a): sign-in by student number, and the
// student's own pages (student accounts only); plus the staff side of
// notices, service requests and profile change requests.

type portalHandlers struct {
	svc       *portal.Service
	academics *academics.Service
	auth      *auth.Service
	cookie    sessionCookie
}

func portalError(c *gin.Context, err error) {
	var ve *content.ValidationError
	switch {
	case errors.As(err, &ve):
		writeValidation(c, ve.Problems)
	case errors.Is(err, portal.ErrNoStudent):
		writeError(c, http.StatusForbidden, "forbidden", "This account has no student record. Contact the registrar's office.")
	case errors.Is(err, portal.ErrNotFound), errors.Is(err, academics.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", "That does not exist, or it is not yours.")
	case errors.Is(err, portal.ErrNotActive):
		writeError(c, http.StatusConflict, "not_active", "Only active students can register for units. Contact the registrar's office.")
	case errors.Is(err, portal.ErrClosed):
		writeError(c, http.StatusConflict, "registration_closed", "Registration for that unit is not open to you now.")
	case errors.Is(err, portal.ErrPending):
		writeError(c, http.StatusConflict, "change_pending", "You already have a change waiting for the registrar. Wait for an answer first.")
	case errors.Is(err, portal.ErrUnchanged), errors.Is(err, academics.ErrUnchanged):
		writeError(c, http.StatusConflict, "unchanged", "Nothing changed. Reload to see the latest.")
	case errors.Is(err, portal.ErrTooMany):
		writeError(c, http.StatusConflict, "too_many_requests_open", "You have 5 requests open. Wait for one to be done before asking for more.")
	case errors.Is(err, portal.ErrCollegeAll):
		writeError(c, http.StatusForbidden, "college_wide", "Only the registrar's office can post to the whole college.")
	case errors.Is(err, academics.ErrOutsideScope):
		writeError(c, http.StatusForbidden, "outside_your_schools", "You can only post to the schools, cohorts and classes you work with.")
	default:
		internalError(c, err)
	}
}

// ---------- sign-in by student number (public) ----------

func studentNumber(v string) string { return strings.ToUpper(strings.TrimSpace(v)) }

func (h portalHandlers) signIn(c *gin.Context) {
	var body struct {
		StudentNumber string `json:"student_number" binding:"required,max=40"`
		Password      string `json:"password" binding:"required"`
	}
	if !bindJSON(c, &body) {
		return
	}
	// An unknown number goes through the same checks as a known one (rate
	// limit, dummy password check), so the answer and timing say nothing.
	email, err := h.svc.LoginEmail(c.Request.Context(), studentNumber(body.StudentNumber))
	if err != nil {
		internalError(c, err)
		return
	}
	res, err := h.auth.Login(c.Request.Context(), email, body.Password, auditRequest(c))
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(c, http.StatusUnauthorized, "invalid_credentials", "The student number or password is not correct. Check them and try again.")
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

func (h portalHandlers) passwordReset(c *gin.Context) {
	var body struct {
		StudentNumber string `json:"student_number" binding:"required,max=40"`
	}
	if !bindJSON(c, &body) {
		return
	}
	email, err := h.svc.LoginEmail(c.Request.Context(), studentNumber(body.StudentNumber))
	if err != nil {
		internalError(c, err)
		return
	}
	if err := h.auth.RequestPasswordReset(c.Request.Context(), email, auditRequest(c)); err != nil {
		internalError(c, err)
		return
	}
	// Always the same answer, whether or not the number exists.
	c.JSON(http.StatusAccepted, gin.H{"message": "If that student number is ours, we have emailed a link to the address on record."})
}

// ---------- the student's own pages ----------

func me(c *gin.Context) uuid.UUID { return principal(c).UserID }

func reply[T any](c *gin.Context, v T, err error) {
	if err != nil {
		portalError(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h portalHandlers) dashboard(c *gin.Context) {
	d, err := h.svc.Dashboard(c.Request.Context(), me(c))
	reply(c, d, err)
}

func (h portalHandlers) profile(c *gin.Context) {
	p, err := h.svc.Profile(c.Request.Context(), me(c))
	reply(c, p, err)
}

// afterProfile answers with the profile as it is now.
func (h portalHandlers) afterProfile(c *gin.Context, err error) {
	if err != nil {
		portalError(c, err)
		return
	}
	h.profile(c)
}

func (h portalHandlers) setPhone(c *gin.Context) {
	var body struct {
		Phone string `json:"phone" binding:"max=60"`
	}
	if bindJSON(c, &body) {
		h.afterProfile(c, h.svc.SetPhone(c.Request.Context(), me(c), body.Phone, auditRequest(c)))
	}
}

func (h portalHandlers) requestChange(c *gin.Context) {
	var body portal.ChangeInput
	if bindJSON(c, &body) {
		h.afterProfile(c, h.svc.RequestChange(c.Request.Context(), me(c), body, auditRequest(c)))
	}
}

func (h portalHandlers) addGuardian(c *gin.Context) {
	var body academics.GuardianInput
	if bindJSON(c, &body) {
		h.afterProfile(c, h.svc.AddGuardian(c.Request.Context(), me(c), body, auditRequest(c)))
	}
}

func (h portalHandlers) updateGuardian(c *gin.Context) {
	gid, ok := recordID(c, "item")
	var body academics.GuardianInput
	if ok && bindJSON(c, &body) {
		h.afterProfile(c, h.svc.UpdateGuardian(c.Request.Context(), me(c), gid, body, auditRequest(c)))
	}
}

func (h portalHandlers) removeGuardian(c *gin.Context) {
	if gid, ok := recordID(c, "item"); ok {
		h.afterProfile(c, h.svc.RemoveGuardian(c.Request.Context(), me(c), gid, auditRequest(c)))
	}
}

func (h portalHandlers) registration(c *gin.Context) {
	r, err := h.svc.Registration(c.Request.Context(), me(c))
	reply(c, r, err)
}

func (h portalHandlers) afterRegistration(c *gin.Context, err error) {
	if err != nil {
		portalError(c, err)
		return
	}
	h.registration(c)
}

func (h portalHandlers) register(c *gin.Context) {
	if id, ok := recordID(c, "id"); ok {
		h.afterRegistration(c, h.svc.Register(c.Request.Context(), me(c), id, auditRequest(c)))
	}
}

func (h portalHandlers) drop(c *gin.Context) {
	if id, ok := recordID(c, "id"); ok {
		h.afterRegistration(c, h.svc.Drop(c.Request.Context(), me(c), id, auditRequest(c)))
	}
}

func (h portalHandlers) timetable(c *gin.Context) {
	t, err := h.svc.Timetable(c.Request.Context(), me(c))
	if err != nil {
		portalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": t})
}

func (h portalHandlers) notices(c *gin.Context) {
	n, err := h.svc.Notices(c.Request.Context(), me(c), 50)
	if err != nil {
		portalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": n})
}

func (h portalHandlers) requests(c *gin.Context) {
	r, err := h.svc.Requests(c.Request.Context(), me(c))
	if err != nil {
		portalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": r})
}

func (h portalHandlers) createRequest(c *gin.Context) {
	var body struct {
		Kind    string `json:"kind" binding:"max=40"`
		Details string `json:"details" binding:"max=20000"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if err := h.svc.CreateRequest(c.Request.Context(), me(c), body.Kind, body.Details, auditRequest(c)); err != nil {
		portalError(c, err)
		return
	}
	h.requests(c)
}

// ---------- staff: notices ----------

func (h portalHandlers) scope(c *gin.Context) (academics.Scope, bool) {
	sc, err := h.academics.ScopeFor(c.Request.Context(), principal(c))
	if err != nil {
		internalError(c, err)
		return sc, false
	}
	return sc, true
}

func (h portalHandlers) listNotices(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	n, err := h.svc.ListNotices(c.Request.Context(), sc, nil)
	if err != nil {
		portalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": n})
}

func (h portalHandlers) postNotice(c *gin.Context) {
	sc, ok := h.scope(c)
	var body portal.NoticeInput
	if !ok || !bindJSON(c, &body) {
		return
	}
	if _, err := h.svc.PostNotice(c.Request.Context(), principal(c), sc, body, auditRequest(c)); err != nil {
		portalError(c, err)
		return
	}
	h.listNotices(c)
}

func (h portalHandlers) withdrawNotice(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.WithdrawNotice(c.Request.Context(), principal(c), sc, true, id, auditRequest(c)); err != nil {
		portalError(c, err)
		return
	}
	h.listNotices(c)
}

func (h portalHandlers) classNotices(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	n, err := h.svc.ClassNotices(c.Request.Context(), principal(c), id)
	if err != nil {
		portalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": n})
}

func (h portalHandlers) postClassNotice(c *gin.Context) {
	id, ok := recordID(c, "id")
	var body portal.NoticeInput
	if !ok || !bindJSON(c, &body) {
		return
	}
	if _, err := h.svc.PostClassNotice(c.Request.Context(), principal(c), id, body, auditRequest(c)); err != nil {
		portalError(c, err)
		return
	}
	h.classNotices(c)
}

// withdrawClassNotice: trainers withdraw only notices they posted; the
// answer is the class's notices (which checks they still teach it).
func (h portalHandlers) withdrawClassNotice(c *gin.Context) {
	nid, ok := recordID(c, "item")
	if !ok {
		return
	}
	if err := h.svc.WithdrawNotice(c.Request.Context(), principal(c), academics.Scope{}, false, nid, auditRequest(c)); err != nil {
		portalError(c, err)
		return
	}
	h.classNotices(c)
}

// ---------- staff: service and change requests ----------

func (h portalHandlers) listServiceRequests(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	status := c.Query("status")
	switch status {
	case "", "open", "in_progress", "done", "declined":
	default:
		writeError(c, http.StatusBadRequest, "invalid_request", "Choose a status from the list.")
		return
	}
	r, err := h.svc.ListServiceRequests(c.Request.Context(), sc, status)
	if err != nil {
		portalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": r})
}

func (h portalHandlers) updateServiceRequest(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	id, ok := recordID(c, "id")
	var body struct {
		Status   string `json:"status" binding:"max=20"`
		Response string `json:"response" binding:"max=20000"`
	}
	if !ok || !bindJSON(c, &body) {
		return
	}
	if err := h.svc.UpdateServiceRequest(c.Request.Context(), principal(c), sc, id, body.Status, body.Response, auditRequest(c)); err != nil {
		portalError(c, err)
		return
	}
	h.listServiceRequests(c)
}

func (h portalHandlers) listChangeRequests(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	status := c.Query("status")
	switch status {
	case "", "pending", "approved", "declined":
	default:
		writeError(c, http.StatusBadRequest, "invalid_request", "Choose a status from the list.")
		return
	}
	r, err := h.svc.ListChangeRequests(c.Request.Context(), sc, status)
	if err != nil {
		portalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": r})
}

func (h portalHandlers) decideChange(approve bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		sc, ok := h.scope(c)
		if !ok {
			return
		}
		id, ok := recordID(c, "id")
		var body struct {
			Decision string `json:"decision" binding:"max=20000"`
		}
		if !ok || !bindJSON(c, &body) {
			return
		}
		if err := h.svc.DecideChange(c.Request.Context(), principal(c), sc, id, approve, body.Decision, auditRequest(c)); err != nil {
			portalError(c, err)
			return
		}
		h.listChangeRequests(c)
	}
}
