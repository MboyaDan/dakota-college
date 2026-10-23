package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/notify"
)

// Staff editing of notification templates (Stage 8b).

type notificationHandlers struct{ svc *notify.Service }

func (h notificationHandlers) list(c *gin.Context) {
	items, err := h.svc.ListTemplates(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h notificationHandlers) update(c *gin.Context) {
	var body struct {
		Subject string `json:"subject" binding:"max=1000"`
		Body    string `json:"body" binding:"max=20000"`
	}
	if !bindJSON(c, &body) {
		return
	}
	err := h.svc.UpdateTemplate(c.Request.Context(), c.Param("key"), c.Param("channel"), body.Subject, body.Body, auditRequest(c))
	var ve *content.ValidationError
	switch {
	case errors.As(err, &ve):
		writeValidation(c, ve.Problems)
	case errors.Is(err, notify.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", "That message template does not exist.")
	case err != nil:
		internalError(c, err)
	default:
		c.Status(http.StatusNoContent)
	}
}
