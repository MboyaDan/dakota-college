package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/privacy"
)

// Retention schedule and data subject requests (Stage 17c).

type privacyHandlers struct{ svc *privacy.Service }

func privacyError(c *gin.Context, err error) {
	var ve *content.ValidationError
	switch {
	case errors.As(err, &ve):
		writeValidation(c, ve.Problems)
	case errors.Is(err, privacy.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", "That record does not exist.")
	case errors.Is(err, privacy.ErrClosed):
		writeError(c, http.StatusConflict, "request_closed", "This request is already closed.")
	case errors.Is(err, privacy.ErrIdentityNotChecked):
		writeError(c, http.StatusConflict, "identity_not_checked", "Record how you checked who is asking before you export or erase anything.")
	case errors.Is(err, privacy.ErrWrongKind):
		writeError(c, http.StatusConflict, "wrong_request_kind", "Export data for access and portability requests, and erase records for erasure requests.")
	case errors.Is(err, privacy.ErrNotThisPerson):
		writeError(c, http.StatusConflict, "not_this_person", "That record does not carry the email address or phone number in this request.")
	case errors.Is(err, privacy.ErrIsStudent):
		writeError(c, http.StatusConflict, "is_student", "This person became a student. Student records are kept; correct them instead, or refuse the erasure with the reason.")
	case errors.Is(err, privacy.ErrInProgress):
		writeError(c, http.StatusConflict, "application_in_progress", "An application is still being decided. Withdraw or decide it first, then erase.")
	case errors.Is(err, privacy.ErrAlreadyErased):
		writeError(c, http.StatusConflict, "already_erased", "This applicant was already erased.")
	case errors.Is(err, privacy.ErrBusy):
		writeError(c, http.StatusConflict, "retention_busy", "A retention run is in progress. Try again in a minute.")
	default:
		internalError(c, err)
	}
}

func (h privacyHandlers) retention(c *gin.Context) {
	o, err := h.svc.Overview(c.Request.Context())
	if err != nil {
		privacyError(c, err)
		return
	}
	c.JSON(http.StatusOK, o)
}

// retentionReport counts what the schedule would remove now. It never
// erases: applying the schedule is for the daily run or the command line.
func (h privacyHandlers) retentionReport(c *gin.Context) {
	res, err := h.svc.Run(c.Request.Context(), privacy.ModeReport)
	if err != nil {
		privacyError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h privacyHandlers) list(c *gin.Context) {
	page, ok := pageParam(c)
	if !ok {
		return
	}
	l, err := h.svc.List(c.Request.Context(), c.Query("status"), page)
	if err != nil {
		privacyError(c, err)
		return
	}
	c.JSON(http.StatusOK, l)
}

func (h privacyHandlers) create(c *gin.Context) {
	var in privacy.RequestInput
	if !bindJSON(c, &in) {
		return
	}
	r, err := h.svc.Create(c.Request.Context(), principal(c), auditRequest(c), in)
	if err != nil {
		privacyError(c, err)
		return
	}
	c.JSON(http.StatusCreated, r)
}

func dataRequestID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusNotFound, "not_found", "That record does not exist.")
		return uuid.Nil, false
	}
	return id, true
}

func (h privacyHandlers) get(c *gin.Context) {
	id, ok := dataRequestID(c)
	if !ok {
		return
	}
	r, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		privacyError(c, err)
		return
	}
	c.JSON(http.StatusOK, r)
}

// after answers a change with the request as it now stands.
func (h privacyHandlers) after(c *gin.Context, err error) {
	if err != nil {
		privacyError(c, err)
		return
	}
	h.get(c)
}

func (h privacyHandlers) identity(c *gin.Context) {
	id, ok := dataRequestID(c)
	if !ok {
		return
	}
	var body struct {
		Method string `json:"method"`
	}
	if !bindJSON(c, &body) {
		return
	}
	h.after(c, h.svc.CheckIdentity(c.Request.Context(), principal(c), auditRequest(c), id, body.Method))
}

func (h privacyHandlers) note(c *gin.Context) {
	id, ok := dataRequestID(c)
	if !ok {
		return
	}
	var body struct {
		Note string `json:"note"`
	}
	if !bindJSON(c, &body) {
		return
	}
	h.after(c, h.svc.AddNote(c.Request.Context(), principal(c), auditRequest(c), id, body.Note))
}

func (h privacyHandlers) erase(c *gin.Context) {
	id, ok := dataRequestID(c)
	if !ok {
		return
	}
	var body struct {
		RecordType string `json:"record_type"`
		RecordID   string `json:"record_id"`
	}
	if !bindJSON(c, &body) {
		return
	}
	rid, err := uuid.Parse(body.RecordID)
	if err != nil {
		writeValidation(c, content.Problems{{Field: "record_id", Message: "Choose a record from the list."}})
		return
	}
	h.after(c, h.svc.Erase(c.Request.Context(), principal(c), auditRequest(c), id, body.RecordType, rid))
}

func (h privacyHandlers) close(c *gin.Context) {
	id, ok := dataRequestID(c)
	if !ok {
		return
	}
	var body struct {
		Status  string `json:"status"`
		Outcome string `json:"outcome"`
	}
	if !bindJSON(c, &body) {
		return
	}
	h.after(c, h.svc.Close(c.Request.Context(), principal(c), auditRequest(c), id, body.Status, body.Outcome))
}

func (h privacyHandlers) export(c *gin.Context) {
	id, ok := dataRequestID(c)
	if !ok {
		return
	}
	f, err := h.svc.Export(c.Request.Context(), principal(c), auditRequest(c), id)
	if err != nil {
		privacyError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+f.Name+`"`)
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/json; charset=utf-8", f.Data)
}
