package httpapi

import (
	"errors"
	"net/http"
	"slices"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/notify"
)

// Staff editing of notification templates (Stage 8b) and the log of sent
// messages with resend (Stage 16a).

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

func (h notificationHandlers) sent(c *gin.Context) {
	f := notify.SentFilter{Status: c.Query("status"), Channel: c.Query("channel"), Query: c.Query("q"), Page: 1}
	if f.Status != "" && !slices.Contains(notify.Statuses, f.Status) {
		writeError(c, http.StatusBadRequest, "invalid_request", "Choose queued, sending, sent, failed or skipped.")
		return
	}
	if f.Channel != "" && f.Channel != "email" && f.Channel != "sms" {
		writeError(c, http.StatusBadRequest, "invalid_request", "Choose email or sms.")
		return
	}
	if v := c.Query("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 10000 {
			writeError(c, http.StatusBadRequest, "invalid_request", "The page number must be a whole number from 1.")
			return
		}
		f.Page = n
	}
	page, err := h.svc.ListSent(c.Request.Context(), f)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h notificationHandlers) counts(c *gin.Context) {
	counts, err := h.svc.SentCounts(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, counts)
}

func (h notificationHandlers) resend(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusNotFound, "not_found", "That message does not exist.")
		return
	}
	err = h.svc.Resend(c.Request.Context(), id, auditRequest(c))
	switch {
	case errors.Is(err, notify.ErrNotResendable):
		writeError(c, http.StatusConflict, "not_resendable", "Only a failed message whose text is still kept can be sent again. Messages with an expired link cannot.")
	case err != nil:
		internalError(c, err)
	default:
		c.Status(http.StatusNoContent)
	}
}
