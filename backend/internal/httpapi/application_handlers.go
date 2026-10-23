package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/application"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
)

// The applicant's own application form (Stage 9a). Every route is for
// applicant sessions only, and an application of someone else answers 404.

type applicationHandlers struct{ svc *application.Service }

func applicationError(c *gin.Context, err error) {
	var ve *content.ValidationError
	switch {
	case errors.As(err, &ve):
		writeValidation(c, ve.Problems)
	case errors.Is(err, application.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", "We could not find that application. Open it from your account page.")
	case errors.Is(err, application.ErrNotEditable):
		writeError(c, http.StatusConflict, "not_editable", "This application can no longer be changed. Reload the page to see its status.")
	case errors.Is(err, application.ErrClosed):
		writeError(c, http.StatusConflict, "applications_closed", "This programme is not taking applications now. Contact the admissions office.")
	case errors.Is(err, application.ErrDuplicate):
		writeError(c, http.StatusConflict, "duplicate_application", "You already have an application for that intake. Open it from your account page.")
	default:
		internalError(c, err)
	}
}

func (h applicationHandlers) start(c *gin.Context) {
	var body struct {
		Programme string     `json:"programme" binding:"max=200"`
		IntakeID  *uuid.UUID `json:"intake_id"`
	}
	if !bindJSON(c, &body) {
		return
	}
	res, err := h.svc.Start(c.Request.Context(), principal(c), body.Programme, body.IntakeID, auditRequest(c))
	if err != nil {
		applicationError(c, err)
		return
	}
	status := http.StatusCreated
	if res.Existing {
		status = http.StatusOK
	}
	c.JSON(status, gin.H{"id": res.ID, "existing": res.Existing})
}

func applicationID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusNotFound, "not_found", "We could not find that application. Open it from your account page.")
		return uuid.Nil, false
	}
	return id, true
}

// respond sends the whole application after a change, so the form can show
// what is still missing.
func (h applicationHandlers) respond(c *gin.Context, id uuid.UUID, err error) {
	if err != nil {
		applicationError(c, err)
		return
	}
	d, err := h.svc.Get(c.Request.Context(), principal(c).UserID, id)
	if err != nil {
		applicationError(c, err)
		return
	}
	c.JSON(http.StatusOK, d)
}

func (h applicationHandlers) get(c *gin.Context) {
	if id, ok := applicationID(c); ok {
		h.respond(c, id, nil)
	}
}

// saveStep binds the body into T and calls save.
func saveStep[T any](h applicationHandlers, c *gin.Context, save func(id uuid.UUID, body T) error) {
	id, ok := applicationID(c)
	if !ok {
		return
	}
	var body T
	if !bindJSON(c, &body) {
		return
	}
	h.respond(c, id, save(id, body))
}

func (h applicationHandlers) saveIntake(c *gin.Context) {
	saveStep(h, c, func(id uuid.UUID, b struct {
		IntakeID uuid.UUID `json:"intake_id" binding:"required"`
	}) error {
		return h.svc.SaveIntake(c.Request.Context(), principal(c).UserID, id, b.IntakeID)
	})
}

func (h applicationHandlers) savePersonal(c *gin.Context) {
	saveStep(h, c, func(id uuid.UUID, b application.Personal) error {
		return h.svc.SavePersonal(c.Request.Context(), principal(c).UserID, id, b)
	})
}

func (h applicationHandlers) saveGuardian(c *gin.Context) {
	saveStep(h, c, func(id uuid.UUID, b application.Guardian) error {
		return h.svc.SaveGuardian(c.Request.Context(), principal(c).UserID, id, b, auditRequest(c))
	})
}

func (h applicationHandlers) saveEducation(c *gin.Context) {
	saveStep(h, c, func(id uuid.UUID, b struct {
		Items []application.Education `json:"items" binding:"max=20"`
	}) error {
		return h.svc.SaveEducation(c.Request.Context(), principal(c).UserID, id, b.Items)
	})
}

func (h applicationHandlers) saveSponsor(c *gin.Context) {
	saveStep(h, c, func(id uuid.UUID, b application.Sponsor) error {
		return h.svc.SaveSponsor(c.Request.Context(), principal(c).UserID, id, b)
	})
}
