package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/enquiry"
)

// Staff enquiry inbox (Stage 8a). Permissions are checked by the router:
// enquiries.view to read, enquiries.manage to change.

type enquiryHandlers struct{ svc *enquiry.Service }

func enquiryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, enquiry.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", "That enquiry does not exist.")
	case errors.Is(err, enquiry.ErrNotAssignable):
		writeError(c, http.StatusUnprocessableEntity, "not_assignable", "That person cannot see the enquiry inbox. Choose someone from the list.")
	case errors.Is(err, enquiry.ErrUnchanged):
		writeError(c, http.StatusConflict, "unchanged", "The enquiry already has that value. Reload to see the latest.")
	case errors.Is(err, enquiry.ErrInvalidStatus):
		writeError(c, http.StatusUnprocessableEntity, "invalid_status", "Choose open, waiting or closed.")
	case errors.Is(err, enquiry.ErrNoteEmpty):
		writeError(c, http.StatusUnprocessableEntity, "note_empty", "Write the note before saving it.")
	case errors.Is(err, enquiry.ErrNoteTooLong):
		writeError(c, http.StatusUnprocessableEntity, "note_too_long", "Keep the note under 5,000 characters.")
	default:
		internalError(c, err)
	}
}

func enquiryID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusNotFound, "not_found", "That enquiry does not exist.")
		return uuid.Nil, false
	}
	return id, true
}

func (h enquiryHandlers) list(c *gin.Context) {
	f := enquiry.InboxFilter{Office: c.Query("office"), Status: c.Query("status"), Search: c.Query("q"), Page: 1}
	switch f.Office {
	case "", "admissions", "finance", "general":
	default:
		writeError(c, http.StatusBadRequest, "invalid_request", "Choose the admissions, finance or general office.")
		return
	}
	switch f.Status {
	case "", "new", "open", "waiting", "closed":
	default:
		writeError(c, http.StatusBadRequest, "invalid_request", "Choose new, open, waiting or closed.")
		return
	}
	switch a := c.Query("assigned"); a {
	case "":
	case "me":
		id := principal(c).UserID
		f.AssignedTo = &id
	case "none":
		f.Unassigned = true
	default:
		id, err := uuid.Parse(a)
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", "Choose who the enquiries are assigned to.")
			return
		}
		f.AssignedTo = &id
	}
	if v := c.Query("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 10000 {
			writeError(c, http.StatusBadRequest, "invalid_request", "The page number must be a whole number from 1.")
			return
		}
		f.Page = n
	}
	page, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		enquiryError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h enquiryHandlers) counts(c *gin.Context) {
	counts, err := h.svc.Counts(c.Request.Context(), principal(c).UserID)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, counts)
}

func (h enquiryHandlers) assignees(c *gin.Context) {
	people, err := h.svc.Assignees(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": people})
}

func (h enquiryHandlers) get(c *gin.Context) {
	if id, ok := enquiryID(c); ok {
		h.respond(c, id, http.StatusOK)
	}
}

// respond answers with the enquiry as it is now, so the screen shows the
// result of a change without another request.
func (h enquiryHandlers) respond(c *gin.Context, id uuid.UUID, status int) {
	d, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		enquiryError(c, err)
		return
	}
	c.JSON(status, d)
}

func (h enquiryHandlers) assign(c *gin.Context) {
	id, ok := enquiryID(c)
	if !ok {
		return
	}
	var body struct {
		// Null clears the owner.
		AssigneeID *uuid.UUID `json:"assignee_id"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if err := h.svc.Assign(c.Request.Context(), id, body.AssigneeID, auditRequest(c)); err != nil {
		enquiryError(c, err)
		return
	}
	h.respond(c, id, http.StatusOK)
}

func (h enquiryHandlers) setStatus(c *gin.Context) {
	id, ok := enquiryID(c)
	if !ok {
		return
	}
	var body struct {
		Status string `json:"status" binding:"required"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if err := h.svc.SetStatus(c.Request.Context(), id, body.Status, auditRequest(c)); err != nil {
		enquiryError(c, err)
		return
	}
	h.respond(c, id, http.StatusOK)
}

func (h enquiryHandlers) addNote(c *gin.Context) {
	id, ok := enquiryID(c)
	if !ok {
		return
	}
	var body struct {
		Body string `json:"body" binding:"max=20000"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if _, err := h.svc.AddNote(c.Request.Context(), id, principal(c).UserID, body.Body, auditRequest(c)); err != nil {
		enquiryError(c, err)
		return
	}
	h.respond(c, id, http.StatusCreated)
}
