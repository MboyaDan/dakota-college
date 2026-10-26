package httpapi

import (
	"errors"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/academics"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/results"
)

// Results (Stage 13a, AC-09): trainers enter and submit marks for their own
// classes (academics.marks.enter); Heads of School approve or return
// (academics.results.approve); the registrar releases
// (academics.results.release). Students see released results only.

type resultsHandlers struct {
	svc       *results.Service
	academics *academics.Service
}

func resultsError(c *gin.Context, err error) {
	var ve *content.ValidationError
	switch {
	case errors.As(err, &ve):
		writeValidation(c, ve.Problems)
	case errors.Is(err, results.ErrNotFound), errors.Is(err, academics.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", "That does not exist, or it is not yours to see.")
	case errors.Is(err, results.ErrWaiting):
		writeError(c, http.StatusConflict, "waiting_for_approval", "These results are waiting for the Head of School. Ask them to return the sheet if something needs changing.")
	case errors.Is(err, results.ErrWrongStatus):
		writeError(c, http.StatusConflict, "not_allowed", "That is not possible at the sheet's current status. Reload to see the latest.")
	case errors.Is(err, results.ErrNoStudent):
		writeError(c, http.StatusForbidden, "forbidden", "This account has no student record. Contact the registrar's office.")
	case errors.Is(err, results.ErrStorageOff):
		writeError(c, http.StatusServiceUnavailable, "storage_unavailable", "Transcripts cannot be stored right now. Please try again later.")
	default:
		internalError(c, err)
	}
}

func (h resultsHandlers) scope(c *gin.Context) (academics.Scope, bool) {
	sc, err := h.academics.ScopeFor(c.Request.Context(), principal(c))
	if err != nil {
		internalError(c, err)
		return sc, false
	}
	return sc, true
}

// ---------- schemes ----------

func (h resultsHandlers) scheme(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	s, err := h.svc.SchemeFor(c.Request.Context(), id)
	if err != nil {
		resultsError(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h resultsHandlers) setScheme(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	id, ok := recordID(c, "id")
	var body results.Scheme
	if !ok || !bindJSON(c, &body) {
		return
	}
	if err := h.svc.SetScheme(c.Request.Context(), principal(c), sc, id, body, auditRequest(c)); err != nil {
		resultsError(c, err)
		return
	}
	h.scheme(c)
}

// ---------- trainers ----------

func (h resultsHandlers) trainerSheet(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	s, err := h.svc.TrainerSheet(c.Request.Context(), principal(c).UserID, id)
	if err != nil {
		resultsError(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h resultsHandlers) enterMarks(c *gin.Context) {
	id, ok := recordID(c, "id")
	var body struct {
		Entries []results.Entry `json:"entries" binding:"max=2000"`
		Reason  string          `json:"reason" binding:"max=2000"`
	}
	if !ok || !bindJSON(c, &body) {
		return
	}
	if err := h.svc.EnterMarks(c.Request.Context(), principal(c).UserID, id, body.Entries, body.Reason, auditRequest(c)); err != nil {
		resultsError(c, err)
		return
	}
	h.trainerSheet(c)
}

func (h resultsHandlers) submit(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Submit(c.Request.Context(), principal(c).UserID, id, auditRequest(c)); err != nil {
		resultsError(c, err)
		return
	}
	h.trainerSheet(c)
}

// ---------- approval and release ----------

func (h resultsHandlers) list(defaultStatus string, allowed ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		sc, ok := h.scope(c)
		if !ok {
			return
		}
		status := c.DefaultQuery("status", defaultStatus)
		valid := false
		for _, a := range allowed {
			valid = valid || a == status
		}
		if !valid {
			writeError(c, http.StatusBadRequest, "invalid_request", "Choose a status from the list.")
			return
		}
		items, err := h.svc.ListSheets(c.Request.Context(), sc, []string{status})
		if err != nil {
			internalError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	}
}

func (h resultsHandlers) get(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	s, err := h.svc.GetSheet(c.Request.Context(), sc, id)
	if err != nil {
		resultsError(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h resultsHandlers) decide(do func(c *gin.Context, sc academics.Scope, id uuid.UUID, reason string) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		sc, ok := h.scope(c)
		if !ok {
			return
		}
		id, ok := recordID(c, "id")
		var body struct {
			Reason string `json:"reason" binding:"max=2000"`
		}
		if !ok || !bindJSON(c, &body) {
			return
		}
		if err := do(c, sc, id, body.Reason); err != nil {
			resultsError(c, err)
			return
		}
		h.get(c)
	}
}

func (h resultsHandlers) approve(c *gin.Context, sc academics.Scope, id uuid.UUID, _ string) error {
	return h.svc.Approve(c.Request.Context(), principal(c), sc, id, auditRequest(c))
}

func (h resultsHandlers) ret(c *gin.Context, sc academics.Scope, id uuid.UUID, reason string) error {
	return h.svc.Return(c.Request.Context(), principal(c), sc, id, reason, auditRequest(c))
}

func (h resultsHandlers) release(c *gin.Context, sc academics.Scope, id uuid.UUID, _ string) error {
	return h.svc.Release(c.Request.Context(), principal(c), sc, id, auditRequest(c))
}

// ---------- transcripts (staff) ----------

func (h resultsHandlers) staffStudentResults(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	t, err := h.svc.StaffTranscripts(c.Request.Context(), sc, id)
	if err != nil {
		resultsError(c, err)
		return
	}
	r, err := h.svc.StaffResults(c.Request.Context(), sc, id)
	if err != nil {
		resultsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"transcripts": t, "results": r})
}

func (h resultsHandlers) issueTranscript(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	id, ok := recordID(c, "id")
	var body struct {
		ServiceRequestID *uuid.UUID `json:"service_request_id"`
	}
	if !ok || !bindJSON(c, &body) {
		return
	}
	if _, err := h.svc.IssueTranscript(c.Request.Context(), principal(c), sc, id, body.ServiceRequestID, auditRequest(c)); err != nil {
		resultsError(c, err)
		return
	}
	h.staffStudentResults(c)
}

func (h resultsHandlers) staffTranscriptLink(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	item, ok := recordID(c, "item")
	if !ok {
		return
	}
	l, err := h.svc.StaffTranscriptLink(c.Request.Context(), principal(c), sc, id, item, auditRequest(c))
	if err != nil {
		resultsError(c, err)
		return
	}
	c.JSON(http.StatusOK, l)
}

// ---------- students ----------

func (h resultsHandlers) studentResults(c *gin.Context) {
	r, err := h.svc.StudentResults(c.Request.Context(), me(c))
	if err != nil {
		resultsError(c, err)
		return
	}
	t, err := h.svc.StudentTranscripts(c.Request.Context(), me(c))
	if err != nil {
		resultsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": r, "transcripts": t})
}

func (h resultsHandlers) slip(c *gin.Context) {
	pdf, name, err := h.svc.ResultsSlip(c.Request.Context(), me(c))
	if err != nil {
		resultsError(c, err)
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "application/pdf", pdf)
}

func (h resultsHandlers) studentTranscriptLink(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	l, err := h.svc.StudentTranscriptLink(c.Request.Context(), me(c), id)
	if err != nil {
		resultsError(c, err)
		return
	}
	c.JSON(http.StatusOK, l)
}
