package httpapi

import (
	"errors"
	"io"
	"net/http"
	"net/netip"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/kcb"
)

// KCB Buni IPN (Stage 15b). KCB calls the public notification route; it
// answers 404 unless the path holds the configured secret (and the caller
// is on the allow list, if set). Finance reads with finance.view and
// settles unmatched notifications with finance.payment.confirm.

type kcbHandlers struct {
	svc *kcb.Service
}

func kcbError(c *gin.Context, err error) {
	var ve *content.ValidationError
	switch {
	case errors.As(err, &ve):
		writeValidation(c, ve.Problems)
	case errors.Is(err, kcb.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", "That does not exist.")
	case errors.Is(err, kcb.ErrDecided):
		writeError(c, http.StatusConflict, "already_settled", "This payment was already settled. Reload to see it.")
	default:
		internalError(c, err)
	}
}

func (h kcbHandlers) notification(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		writeError(c, http.StatusRequestEntityTooLarge, "too_large", "Too large.")
		return
	}
	var ip *netip.Addr
	if a, err := netip.ParseAddr(clientIP(c)); err == nil {
		ip = &a
	}
	answer, err := h.svc.Receive(c.Request.Context(), c.Param("secret"), ip, c.GetHeader("Signature"), body)
	switch {
	case errors.Is(err, kcb.ErrUnauthorised):
		writeError(c, http.StatusNotFound, "not_found", "No such endpoint.")
	case errors.Is(err, kcb.ErrThrottled):
		writeError(c, http.StatusTooManyRequests, "too_many_requests", "Too many requests.")
	case err != nil:
		// Not stored: answer an error so KCB sends it again.
		internalError(c, err)
	default:
		c.JSON(http.StatusOK, answer)
	}
}

func (h kcbHandlers) status(c *gin.Context) {
	s, err := h.svc.Status(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h kcbHandlers) list(c *gin.Context) {
	status, ok := queryChoice(c, "status", "matched", "unmatched", "assigned", "refund", "rejected")
	if !ok {
		return
	}
	day, ok := queryDay(c)
	if !ok {
		return
	}
	items, total, err := h.svc.List(c.Request.Context(), kcb.Filter{Status: status, Day: day, Query: c.Query("q"), Page: queryPage(c)})
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page_size": 50})
}

func (h kcbHandlers) assign(c *gin.Context) {
	id, ok := recordID(c, "id")
	var body struct {
		StudentID uuid.UUID `json:"student_id" binding:"required"`
		Note      string    `json:"note" binding:"max=2000"`
	}
	if !ok || !bindJSON(c, &body) {
		return
	}
	if err := h.svc.Assign(c.Request.Context(), principal(c), id, body.StudentID, body.Note, auditRequest(c)); err != nil {
		kcbError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h kcbHandlers) refund(c *gin.Context) {
	id, ok := recordID(c, "id")
	var body struct {
		Note string `json:"note" binding:"max=2000"`
	}
	if !ok || !bindJSON(c, &body) {
		return
	}
	if err := h.svc.Refund(c.Request.Context(), principal(c), id, body.Note, auditRequest(c)); err != nil {
		kcbError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h kcbHandlers) reconcile(c *gin.Context) {
	day, ok := queryDay(c)
	if !ok {
		return
	}
	if day == nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Choose a day.")
		return
	}
	r, err := h.svc.Reconcile(c.Request.Context(), *day)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, r)
}

func (h kcbHandlers) reprocess(c *gin.Context) {
	n, err := h.svc.Reprocess(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"processed": n})
}
