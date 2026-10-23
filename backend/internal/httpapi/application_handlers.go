package httpapi

import (
	"errors"
	"io"
	"mime"
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

// ---------- documents, submission and files (Stage 9b) ----------

func documentID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("doc"))
	if err != nil {
		writeError(c, http.StatusNotFound, "not_found", "That document is not on this application.")
		return uuid.Nil, false
	}
	return id, true
}

func documentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, application.ErrStorageOff):
		writeError(c, http.StatusServiceUnavailable, "uploads_unavailable", "Uploading documents is not available right now. Please try again later.")
	case errors.Is(err, application.ErrScanFailed):
		writeError(c, http.StatusServiceUnavailable, "scan_unavailable", "We could not check the file for viruses just now. Please try again in a few minutes.")
	case errors.Is(err, application.ErrRejectedFile):
		writeError(c, http.StatusUnprocessableEntity, "file_rejected", "This file could not be accepted. Scan or photograph the document again and upload the new file.")
	default:
		applicationError(c, err)
	}
}

func (h applicationHandlers) uploadDocument(c *gin.Context) {
	id, ok := applicationID(c)
	if !ok {
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		writeValidation(c, content.Problems{{Field: "file", Message: "Choose a file to upload."}})
		return
	}
	if fh.Size > application.MaxDocumentBytes {
		writeValidation(c, content.Problems{{Field: "file", Message: "The file is larger than 10 MB. Take a smaller photo or save the PDF at a lower quality."}})
		return
	}
	f, err := fh.Open()
	if err != nil {
		internalError(c, err)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, application.MaxDocumentBytes+1))
	if err != nil {
		internalError(c, err)
		return
	}
	doc, err := h.svc.UploadDocument(c.Request.Context(), principal(c), id, c.PostForm("kind"), fh.Filename, data, auditRequest(c))
	if err != nil {
		documentError(c, err)
		return
	}
	c.JSON(http.StatusCreated, doc)
}

func (h applicationHandlers) removeDocument(c *gin.Context) {
	id, ok := applicationID(c)
	if !ok {
		return
	}
	docID, ok := documentID(c)
	if !ok {
		return
	}
	h.respond(c, id, h.svc.RemoveDocument(c.Request.Context(), principal(c).UserID, id, docID, auditRequest(c)))
}

func (h applicationHandlers) documentLink(c *gin.Context) {
	id, ok := applicationID(c)
	if !ok {
		return
	}
	docID, ok := documentID(c)
	if !ok {
		return
	}
	link, err := h.svc.DocumentLink(c.Request.Context(), principal(c).UserID, id, docID)
	if err != nil {
		documentError(c, err)
		return
	}
	c.JSON(http.StatusOK, link)
}

func (h applicationHandlers) staffDocumentLink(c *gin.Context) {
	id, ok := applicationID(c)
	if !ok {
		return
	}
	docID, ok := documentID(c)
	if !ok {
		return
	}
	link, err := h.svc.StaffDocumentLink(c.Request.Context(), principal(c), id, docID, auditRequest(c))
	if err != nil {
		documentError(c, err)
		return
	}
	c.JSON(http.StatusOK, link)
}

func (h applicationHandlers) submit(c *gin.Context) {
	id, ok := applicationID(c)
	if !ok {
		return
	}
	var body struct {
		Declaration bool `json:"declaration"`
	}
	if !bindJSON(c, &body) {
		return
	}
	res, err := h.svc.Submit(c.Request.Context(), principal(c), id, c.GetHeader("Idempotency-Key"), body.Declaration, auditRequest(c))
	switch {
	case errors.Is(err, application.ErrKeyReused):
		writeError(c, http.StatusUnprocessableEntity, "idempotency_key_reused", "Reload the page and submit again.")
	case errors.Is(err, application.ErrAlreadySubmitted):
		writeError(c, http.StatusConflict, "already_submitted", "This application was already submitted. Reload the page to see its status.")
	case err != nil:
		applicationError(c, err)
	default:
		if res.Replayed {
			c.Header("Idempotent-Replayed", "true")
		}
		c.Data(res.Status, "application/json; charset=utf-8", res.Body)
	}
}

// openFile sends a private file through a short-lived link (AC-05). The
// link works only for the signed-in account it was issued to. PDFs are
// downloaded rather than shown, and nothing may run inside the response.
func (h applicationHandlers) openFile(c *gin.Context) {
	f, err := h.svc.OpenLink(c.Request.Context(), principal(c).UserID, c.Param("token"))
	if errors.Is(err, application.ErrLinkInvalid) {
		writeError(c, http.StatusNotFound, "link_expired", "This link has expired. Go back and open the document again.")
		return
	}
	if err != nil {
		internalError(c, err)
		return
	}
	defer f.Body.Close()
	disposition := "inline"
	if f.ContentType == "application/pdf" {
		disposition = "attachment"
	}
	c.Header("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": f.Name}))
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; sandbox")
	c.DataFromReader(http.StatusOK, f.Size, f.ContentType, f.Body, nil)
}
