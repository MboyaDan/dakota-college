package httpapi

import (
	"errors"
	"io"
	"net/http"
	"net/netip"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/mpesa"
)

// M-Pesa C2B (Stage 15a). Safaricom calls the two public callback routes;
// they answer 404 unless the path holds the configured secret (and the
// caller is on the allow list, if one is set). Finance reads with
// finance.view and settles unmatched payments with finance.payment.confirm;
// registering the URLs needs finance.mpesa.setup.

type mpesaHandlers struct {
	svc *mpesa.Service
}

const callbackBodyLimit = 64 << 10

func mpesaError(c *gin.Context, err error) {
	var ve *content.ValidationError
	var de *mpesa.ErrDaraja
	switch {
	case errors.As(err, &ve):
		writeValidation(c, ve.Problems)
	case errors.Is(err, mpesa.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", "That does not exist.")
	case errors.Is(err, mpesa.ErrDecided):
		writeError(c, http.StatusConflict, "already_settled", "This payment was already settled. Reload to see it.")
	case errors.Is(err, mpesa.ErrSandboxOnly):
		writeError(c, http.StatusConflict, "sandbox_only", "Test payments can only be sent in the sandbox.")
	case errors.As(err, &de):
		writeError(c, http.StatusBadGateway, "mpesa_refused", de.Error())
	default:
		internalError(c, err)
	}
}

func (h mpesaHandlers) callback(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			writeError(c, http.StatusRequestEntityTooLarge, "too_large", "Too large.")
			return
		}
		var ip *netip.Addr
		if a, err := netip.ParseAddr(c.ClientIP()); err == nil {
			ip = &a
		}
		err = h.svc.Callback(c.Request.Context(), kind, c.Param("secret"), ip, body)
		switch {
		case errors.Is(err, mpesa.ErrUnauthorised):
			writeError(c, http.StatusNotFound, "not_found", "No such endpoint.")
		case errors.Is(err, mpesa.ErrThrottled):
			writeError(c, http.StatusTooManyRequests, "too_many_requests", "Too many requests.")
		case err != nil:
			// Not stored: answer an error so the failure is visible on Safaricom's side.
			internalError(c, err)
		case kind == "validation":
			// Every payment is accepted; one that matches no student waits for Finance (D-171).
			c.JSON(http.StatusOK, gin.H{"ResultCode": "0", "ResultDesc": "Accepted"})
		default:
			c.JSON(http.StatusOK, gin.H{"ResultCode": "0", "ResultDesc": "Success"})
		}
	}
}

func (h mpesaHandlers) status(c *gin.Context) {
	s, err := h.svc.Status(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h mpesaHandlers) register(c *gin.Context) {
	msg, err := h.svc.Register(c.Request.Context(), auditRequest(c))
	if err != nil {
		mpesaError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": msg})
}

func (h mpesaHandlers) simulate(c *gin.Context) {
	var body struct {
		AmountCents int64  `json:"amount_cents"`
		Msisdn      string `json:"msisdn" binding:"max=20"`
		Account     string `json:"account" binding:"max=40"`
	}
	if !bindJSON(c, &body) {
		return
	}
	msg, err := h.svc.Simulate(c.Request.Context(), body.AmountCents, body.Msisdn, body.Account)
	if err != nil {
		mpesaError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": msg})
}

func (h mpesaHandlers) reprocess(c *gin.Context) {
	n, err := h.svc.Reprocess(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"processed": n})
}

func queryDay(c *gin.Context) (*time.Time, bool) {
	v := c.Query("day")
	if v == "" {
		return nil, true
	}
	d, err := time.Parse(time.DateOnly, v)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Enter the day as YYYY-MM-DD.")
		return nil, false
	}
	return &d, true
}

func (h mpesaHandlers) list(c *gin.Context) {
	status, ok := queryChoice(c, "status", "matched", "unmatched", "assigned", "refund", "rejected")
	if !ok {
		return
	}
	day, ok := queryDay(c)
	if !ok {
		return
	}
	items, total, err := h.svc.List(c.Request.Context(), mpesa.Filter{Status: status, Day: day, Query: c.Query("q"), Page: queryPage(c)})
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page_size": 50})
}

func (h mpesaHandlers) assign(c *gin.Context) {
	id, ok := recordID(c, "id")
	var body struct {
		StudentID uuid.UUID `json:"student_id" binding:"required"`
		Note      string    `json:"note" binding:"max=2000"`
	}
	if !ok || !bindJSON(c, &body) {
		return
	}
	if err := h.svc.Assign(c.Request.Context(), principal(c), id, body.StudentID, body.Note, auditRequest(c)); err != nil {
		mpesaError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h mpesaHandlers) refund(c *gin.Context) {
	id, ok := recordID(c, "id")
	var body struct {
		Note string `json:"note" binding:"max=2000"`
	}
	if !ok || !bindJSON(c, &body) {
		return
	}
	if err := h.svc.Refund(c.Request.Context(), principal(c), id, body.Note, auditRequest(c)); err != nil {
		mpesaError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h mpesaHandlers) reconcile(c *gin.Context) {
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
