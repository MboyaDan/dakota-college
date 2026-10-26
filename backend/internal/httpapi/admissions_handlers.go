package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/admissions"
	"github.com/MboyaDan/dakota-college/backend/internal/application"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
)

// Staff admissions workflow (Stage 10a). Permissions are checked by the
// router; the application's status is checked again under its row lock.

type admissionsHandlers struct{ svc *admissions.Service }

func admissionsError(c *gin.Context, err error) {
	var ve *content.ValidationError
	switch {
	case errors.As(err, &ve):
		writeValidation(c, ve.Problems)
	case errors.Is(err, admissions.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", "That application does not exist or has not been submitted.")
	case errors.Is(err, application.ErrIllegalTransition):
		writeError(c, http.StatusConflict, "not_allowed", "That is not possible at the application's current status. Reload to see the latest.")
	case errors.Is(err, admissions.ErrUnchanged):
		writeError(c, http.StatusConflict, "unchanged", "The application already has that value. Reload to see the latest.")
	case errors.Is(err, admissions.ErrNotAssignable):
		writeError(c, http.StatusUnprocessableEntity, "not_assignable", "That person cannot review applications. Choose someone from the list.")
	case errors.Is(err, admissions.ErrStorageOff):
		writeError(c, http.StatusServiceUnavailable, "storage_unavailable", "Offer letters cannot be stored right now. Please try again later.")
	case errors.Is(err, admissions.ErrNotApplicantAccount):
		writeError(c, http.StatusConflict, "account_not_applicant",
			"The applicant's account is already a student account, so it cannot be enrolled again. Contact the ICT administrator.")
	default:
		internalError(c, err)
	}
}

func staffApplicationID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusNotFound, "not_found", "That application does not exist or has not been submitted.")
		return uuid.Nil, false
	}
	return id, true
}

// optionalUUID reads an optional id from the query string.
func optionalUUID(c *gin.Context, name, message string) (*uuid.UUID, bool) {
	v := c.Query(name)
	if v == "" {
		return nil, true
	}
	id, err := uuid.Parse(v)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", message)
		return nil, false
	}
	return &id, true
}

func (h admissionsHandlers) list(c *gin.Context) {
	f := admissions.Filter{Status: c.Query("status"), Search: c.Query("q"), Page: 1}
	if f.Status != "" && !admissions.IsStatus(f.Status) {
		writeError(c, http.StatusBadRequest, "invalid_request", "Choose a status from the list.")
		return
	}
	var ok bool
	if f.ProgrammeID, ok = optionalUUID(c, "programme", "Choose a programme from the list."); !ok {
		return
	}
	if f.IntakeID, ok = optionalUUID(c, "intake", "Choose an intake from the list."); !ok {
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
			writeError(c, http.StatusBadRequest, "invalid_request", "Choose who the applications are assigned to.")
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
		admissionsError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h admissionsHandlers) counts(c *gin.Context) {
	counts, err := h.svc.Counts(c.Request.Context(), principal(c).UserID)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, counts)
}

func (h admissionsHandlers) assignees(c *gin.Context) {
	people, err := h.svc.Assignees(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": people})
}

func (h admissionsHandlers) report(c *gin.Context) {
	r, err := h.svc.Report(c.Request.Context(), c.Query("from"), c.Query("to"))
	if err != nil {
		admissionsError(c, err)
		return
	}
	c.JSON(http.StatusOK, r)
}

func (h admissionsHandlers) get(c *gin.Context) {
	if id, ok := staffApplicationID(c); ok {
		h.respond(c, id, nil)
	}
}

// respond answers with the application as it is now, so the screen shows
// the result of a change without another request.
func (h admissionsHandlers) respond(c *gin.Context, id uuid.UUID, err error) {
	if err != nil {
		admissionsError(c, err)
		return
	}
	d, err := h.svc.Get(c.Request.Context(), principal(c), id)
	if err != nil {
		admissionsError(c, err)
		return
	}
	c.JSON(http.StatusOK, d)
}

// act binds the body into T and runs do.
func act[T any](h admissionsHandlers, c *gin.Context, do func(id uuid.UUID, body T) error) {
	id, ok := staffApplicationID(c)
	if !ok {
		return
	}
	var body T
	if !bindJSON(c, &body) {
		return
	}
	h.respond(c, id, do(id, body))
}

func (h admissionsHandlers) assign(c *gin.Context) {
	act(h, c, func(id uuid.UUID, b struct {
		// Null clears the owner.
		AssigneeID *uuid.UUID `json:"assignee_id"`
	}) error {
		return h.svc.Assign(c.Request.Context(), principal(c), id, b.AssigneeID, auditRequest(c))
	})
}

func (h admissionsHandlers) addNote(c *gin.Context) {
	act(h, c, func(id uuid.UUID, b struct {
		Body string `json:"body" binding:"max=20000"`
	}) error {
		return h.svc.AddNote(c.Request.Context(), principal(c), id, b.Body, auditRequest(c))
	})
}

func (h admissionsHandlers) requestInfo(c *gin.Context) {
	act(h, c, func(id uuid.UUID, b struct {
		Message string `json:"message" binding:"max=20000"`
	}) error {
		return h.svc.RequestInfo(c.Request.Context(), principal(c), id, b.Message, auditRequest(c))
	})
}

func (h admissionsHandlers) resumeReview(c *gin.Context) {
	act(h, c, func(id uuid.UUID, b struct {
		Reason string `json:"reason" binding:"max=20000"`
	}) error {
		return h.svc.ResumeReview(c.Request.Context(), principal(c), id, b.Reason, auditRequest(c))
	})
}

func (h admissionsHandlers) reject(c *gin.Context) {
	act(h, c, func(id uuid.UUID, b struct {
		Reason string `json:"reason" binding:"max=20000"`
	}) error {
		return h.svc.Reject(c.Request.Context(), principal(c), id, b.Reason, auditRequest(c))
	})
}

func (h admissionsHandlers) offer(c *gin.Context) {
	act(h, c, func(id uuid.UUID, b struct {
		Conditions string `json:"conditions" binding:"max=20000"`
		RespondBy  string `json:"respond_by" binding:"max=20"`
	}) error {
		return h.svc.MakeOffer(c.Request.Context(), principal(c), id, admissions.OfferInput{Conditions: b.Conditions, RespondBy: b.RespondBy}, auditRequest(c))
	})
}

func (h admissionsHandlers) offerLetterLink(c *gin.Context) {
	id, ok := staffApplicationID(c)
	if !ok {
		return
	}
	link, err := h.svc.OfferLetterLink(c.Request.Context(), principal(c), id, auditRequest(c))
	if err != nil {
		admissionsError(c, err)
		return
	}
	c.JSON(http.StatusOK, link)
}

func (h admissionsHandlers) enrol(c *gin.Context) {
	id, ok := staffApplicationID(c)
	if !ok {
		return
	}
	_, err := h.svc.Enrol(c.Request.Context(), principal(c), id, auditRequest(c))
	h.respond(c, id, err)
}
