package httpapi

import (
	"errors"
	"mime"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/finance"
)

// Finance (Stage 14a, brief 9, AC-10, AC-11). Reading needs finance.view;
// structures and invoices finance.invoice; recording payments
// finance.payment.record; confirming or rejecting them
// finance.payment.confirm; adjustments finance.adjust. Payment and
// adjustment writes take an Idempotency-Key header (NN-04). Students see
// only their own fees.

type financeHandlers struct {
	svc *finance.Service
}

func financeError(c *gin.Context, err error) {
	var ve *content.ValidationError
	switch {
	case errors.As(err, &ve):
		writeValidation(c, ve.Problems)
	case errors.Is(err, finance.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", "That does not exist, or it is not yours to see.")
	case errors.Is(err, finance.ErrNotDraft):
		writeError(c, http.StatusConflict, "not_draft", "This invoice is no longer a draft, so it cannot change. Reload to see it; correct an issued invoice with an adjustment.")
	case errors.Is(err, finance.ErrDecided):
		writeError(c, http.StatusConflict, "already_decided", "This payment was already confirmed or rejected. Reload to see it.")
	case errors.Is(err, finance.ErrKeyReused):
		writeError(c, http.StatusUnprocessableEntity, "idempotency_key_reused", "Reload the page and try again.")
	case errors.Is(err, finance.ErrNoStudent):
		writeError(c, http.StatusForbidden, "forbidden", "This account has no student record. Contact the registrar's office.")
	default:
		internalError(c, err)
	}
}

// answer sends a keyed reply: the first answer, or its stored copy.
func answer(c *gin.Context, a *finance.Answer, err error) {
	if err != nil {
		financeError(c, err)
		return
	}
	if a.Replayed {
		c.Header("Idempotent-Replayed", "true")
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", a.Body)
}

func sendPDF(c *gin.Context, pdf []byte, name string, err error) {
	if err != nil {
		financeError(c, err)
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "application/pdf", pdf)
}

// queryID reads an optional id from the query string.
func queryID(c *gin.Context, name string) (*uuid.UUID, bool) {
	v := c.Query(name)
	if v == "" {
		return nil, true
	}
	id, err := uuid.Parse(v)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Choose from the list.")
		return nil, false
	}
	return &id, true
}

// queryChoice reads an optional value that must be one of allowed.
func queryChoice(c *gin.Context, name string, allowed ...string) (*string, bool) {
	v := c.Query(name)
	if v == "" {
		return nil, true
	}
	for _, a := range allowed {
		if v == a {
			return &v, true
		}
	}
	writeError(c, http.StatusBadRequest, "invalid_request", "Choose from the list.")
	return nil, false
}

func queryPage(c *gin.Context) int {
	n, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || n < 1 || n > 10000 {
		return 1
	}
	return n
}

func (h financeHandlers) summary(c *gin.Context) {
	s, err := h.svc.Summary(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

// ---------- fee structures ----------

func (h financeHandlers) programmes(c *gin.Context) {
	items, err := h.svc.Programmes(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h financeHandlers) listStructures(c *gin.Context) {
	items, err := h.svc.ListStructures(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h financeHandlers) createStructure(c *gin.Context) {
	var body struct {
		ProgrammeID   uuid.UUID  `json:"programme_id" binding:"required"`
		IntakeID      *uuid.UUID `json:"intake_id"`
		FromPublished bool       `json:"from_published"`
	}
	if !bindJSON(c, &body) {
		return
	}
	s, err := h.svc.CreateStructure(c.Request.Context(), principal(c), body.ProgrammeID, body.IntakeID, body.FromPublished, auditRequest(c))
	if err != nil {
		financeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, s)
}

func (h financeHandlers) getStructure(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	s, err := h.svc.GetStructure(c.Request.Context(), id)
	if err != nil {
		financeError(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h financeHandlers) updateStructure(c *gin.Context) {
	id, ok := recordID(c, "id")
	var body finance.StructureInput
	if !ok || !bindJSON(c, &body) {
		return
	}
	s, err := h.svc.UpdateStructure(c.Request.Context(), principal(c), id, body, auditRequest(c))
	if err != nil {
		financeError(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h financeHandlers) deleteStructure(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteStructure(c.Request.Context(), id, auditRequest(c)); err != nil {
		financeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------- invoices ----------

func (h financeHandlers) listInvoices(c *gin.Context) {
	status, ok := queryChoice(c, "status", "draft", "issued", "void")
	if !ok {
		return
	}
	term, ok := queryID(c, "term_id")
	if !ok {
		return
	}
	cohort, ok := queryID(c, "cohort_id")
	if !ok {
		return
	}
	items, total, err := h.svc.ListInvoices(c.Request.Context(), finance.InvoiceFilter{Status: status, TermID: term, Cohort: cohort, Query: c.Query("q"), Page: queryPage(c)})
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page_size": 50})
}

func (h financeHandlers) getInvoice(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	inv, err := h.svc.GetInvoice(c.Request.Context(), id)
	if err != nil {
		financeError(c, err)
		return
	}
	c.JSON(http.StatusOK, inv)
}

func (h financeHandlers) updateInvoice(c *gin.Context) {
	id, ok := recordID(c, "id")
	var body finance.DraftInput
	if !ok || !bindJSON(c, &body) {
		return
	}
	inv, err := h.svc.UpdateDraft(c.Request.Context(), id, body, auditRequest(c))
	if err != nil {
		financeError(c, err)
		return
	}
	c.JSON(http.StatusOK, inv)
}

func (h financeHandlers) fillInvoice(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	inv, err := h.svc.Fill(c.Request.Context(), id, auditRequest(c))
	if err != nil {
		financeError(c, err)
		return
	}
	c.JSON(http.StatusOK, inv)
}

func (h financeHandlers) issueInvoice(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	if _, err := h.svc.Issue(c.Request.Context(), principal(c), []uuid.UUID{id}, auditRequest(c)); err != nil {
		financeError(c, err)
		return
	}
	h.getInvoice(c)
}

func (h financeHandlers) issueInvoices(c *gin.Context) {
	var body struct {
		IDs []uuid.UUID `json:"ids" binding:"required,max=500"`
	}
	if !bindJSON(c, &body) {
		return
	}
	numbers, err := h.svc.Issue(c.Request.Context(), principal(c), body.IDs, auditRequest(c))
	if err != nil {
		financeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"issued": len(numbers), "numbers": numbers})
}

func (h financeHandlers) voidInvoice(c *gin.Context) {
	id, ok := recordID(c, "id")
	var body struct {
		Reason string `json:"reason" binding:"max=2000"`
	}
	if !ok || !bindJSON(c, &body) {
		return
	}
	inv, err := h.svc.Void(c.Request.Context(), id, body.Reason, auditRequest(c))
	if err != nil {
		financeError(c, err)
		return
	}
	c.JSON(http.StatusOK, inv)
}

func (h financeHandlers) invoicePDF(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	pdf, name, err := h.svc.InvoicePDF(c.Request.Context(), id, nil)
	sendPDF(c, pdf, name, err)
}

func (h financeHandlers) bill(c *gin.Context) {
	var body struct {
		CohortID uuid.UUID `json:"cohort_id" binding:"required"`
		TermID   uuid.UUID `json:"term_id" binding:"required"`
	}
	if !bindJSON(c, &body) {
		return
	}
	r, err := h.svc.Bill(c.Request.Context(), principal(c), body.CohortID, body.TermID, auditRequest(c))
	if err != nil {
		financeError(c, err)
		return
	}
	c.JSON(http.StatusOK, r)
}

// ---------- payments ----------

func (h financeHandlers) listPayments(c *gin.Context) {
	status, ok := queryChoice(c, "status", "pending", "confirmed", "rejected")
	if !ok {
		return
	}
	method, ok := queryChoice(c, "method", "cash", "bank", "mpesa", "sponsor")
	if !ok {
		return
	}
	items, total, err := h.svc.ListPayments(c.Request.Context(), finance.PaymentFilter{Status: status, Method: method, Query: c.Query("q"), Page: queryPage(c)})
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page_size": 50})
}

func (h financeHandlers) recordPayment(c *gin.Context) {
	var body finance.PaymentInput
	if !bindJSON(c, &body) {
		return
	}
	a, err := h.svc.RecordPayment(c.Request.Context(), principal(c), c.GetHeader("Idempotency-Key"), body, auditRequest(c))
	answer(c, a, err)
}

func (h financeHandlers) getPayment(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	p, err := h.svc.GetPayment(c.Request.Context(), id)
	if err != nil {
		financeError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h financeHandlers) confirmPayment(c *gin.Context) {
	id, ok := recordID(c, "id")
	var body struct {
		Note string `json:"note" binding:"max=2000"`
	}
	if !ok || !bindJSON(c, &body) {
		return
	}
	a, err := h.svc.ConfirmPayment(c.Request.Context(), principal(c), c.GetHeader("Idempotency-Key"), id, body.Note, auditRequest(c))
	answer(c, a, err)
}

func (h financeHandlers) rejectPayment(c *gin.Context) {
	id, ok := recordID(c, "id")
	var body struct {
		Reason string `json:"reason" binding:"max=2000"`
	}
	if !ok || !bindJSON(c, &body) {
		return
	}
	a, err := h.svc.RejectPayment(c.Request.Context(), principal(c), c.GetHeader("Idempotency-Key"), id, body.Reason, auditRequest(c))
	answer(c, a, err)
}

func (h financeHandlers) receipt(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	pdf, name, err := h.svc.ReceiptPDF(c.Request.Context(), id, nil)
	sendPDF(c, pdf, name, err)
}

// ---------- accounts and reports ----------

func (h financeHandlers) arrears(c *gin.Context) {
	cohort, ok := queryID(c, "cohort_id")
	if !ok {
		return
	}
	items, err := h.svc.Arrears(c.Request.Context(), cohort, c.Query("overdue") == "1")
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h financeHandlers) account(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	a, err := h.svc.Account(c.Request.Context(), id)
	if err != nil {
		financeError(c, err)
		return
	}
	c.JSON(http.StatusOK, a)
}

func (h financeHandlers) statement(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	pdf, name, err := h.svc.StatementPDF(c.Request.Context(), id, nil)
	sendPDF(c, pdf, name, err)
}

func (h financeHandlers) createInvoice(c *gin.Context) {
	id, ok := recordID(c, "id")
	var body struct {
		Kind   string     `json:"kind" binding:"required"`
		TermID *uuid.UUID `json:"term_id"`
	}
	if !ok || !bindJSON(c, &body) {
		return
	}
	inv, err := h.svc.CreateInvoice(c.Request.Context(), principal(c), id, body.Kind, body.TermID, auditRequest(c))
	if err != nil {
		financeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, inv)
}

func (h financeHandlers) adjust(c *gin.Context) {
	id, ok := recordID(c, "id")
	var body finance.AdjustmentInput
	if !ok || !bindJSON(c, &body) {
		return
	}
	a, err := h.svc.Adjust(c.Request.Context(), principal(c), c.GetHeader("Idempotency-Key"), id, body, auditRequest(c))
	answer(c, a, err)
}

// ---------- students ----------

func (h financeHandlers) myFees(c *gin.Context) {
	f, err := h.svc.MyFees(c.Request.Context(), me(c))
	if err != nil {
		financeError(c, err)
		return
	}
	c.JSON(http.StatusOK, f)
}

func (h financeHandlers) myStatement(c *gin.Context) {
	user := me(c)
	pdf, name, err := h.svc.StatementPDF(c.Request.Context(), uuid.Nil, &user)
	sendPDF(c, pdf, name, err)
}

func (h financeHandlers) myInvoice(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	user := me(c)
	pdf, name, err := h.svc.InvoicePDF(c.Request.Context(), id, &user)
	sendPDF(c, pdf, name, err)
}

func (h financeHandlers) myReceipt(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	user := me(c)
	pdf, name, err := h.svc.ReceiptPDF(c.Request.Context(), id, &user)
	sendPDF(c, pdf, name, err)
}
