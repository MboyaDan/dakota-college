package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/media"
)

type contentHandlers struct{ svc *content.Service }

func writeValidation(c *gin.Context, problems content.Problems) {
	var b errorBody
	b.Error.Code = "validation_failed"
	b.Error.Message = "Some fields need attention. Fix them and save again."
	b.Error.RequestID = c.GetString(requestIDKey)
	b.Error.Fields = problems
	c.AbortWithStatusJSON(http.StatusUnprocessableEntity, b)
}

func contentError(c *gin.Context, err error) {
	var ve *content.ValidationError
	switch {
	case errors.As(err, &ve):
		writeValidation(c, ve.Problems)
	case errors.Is(err, content.ErrNotFound), errors.Is(err, content.ErrUnknownKind):
		writeError(c, http.StatusNotFound, "not_found", "That content does not exist. It may have been deleted.")
	case errors.Is(err, content.ErrConflict):
		writeError(c, http.StatusConflict, "edit_conflict", "Someone else saved this since you opened it. Reload to see their changes, then make yours again.")
	case errors.Is(err, content.ErrNeedsPublish):
		writeError(c, http.StatusForbidden, "needs_publisher", "Only a publisher (Registrar or Principal) can change this.")
	case errors.Is(err, content.ErrNotDeletable):
		writeError(c, http.StatusConflict, "not_deletable", "Published content cannot be deleted. Archive it instead, so the history is kept.")
	case errors.Is(err, content.ErrInUse):
		writeError(c, http.StatusConflict, "in_use", "Other content or applications refer to this. Remove those links first.")
	case errors.Is(err, content.ErrDuplicateSlug):
		writeError(c, http.StatusConflict, "duplicate_slug", "That web address (slug) is already used. Choose a different one.")
	case errors.Is(err, content.ErrNoWorkflow), errors.Is(err, content.ErrWrongStatus):
		writeError(c, http.StatusConflict, "wrong_status", "This action does not apply to the content in its current state.")
	default:
		internalError(c, err)
	}
}

func contentID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusNotFound, "not_found", "That content does not exist. It may have been deleted.")
		return uuid.Nil, false
	}
	return id, true
}

func (h contentHandlers) list(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context(), c.Param("kind"))
	if err != nil {
		contentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type documentBody struct {
	Document    json.RawMessage `json:"document" binding:"required"`
	BaseVersion string          `json:"base_version"`
}

func (h contentHandlers) create(c *gin.Context) {
	var body documentBody
	if !bindJSON(c, &body) {
		return
	}
	id, err := h.svc.Create(c.Request.Context(), principal(c), c.Param("kind"), body.Document, auditRequest(c))
	if err != nil {
		contentError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id.String()})
}

func (h contentHandlers) get(c *gin.Context) {
	id, ok := contentID(c)
	if !ok {
		return
	}
	item, err := h.svc.Get(c.Request.Context(), c.Param("kind"), id)
	if err != nil {
		contentError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h contentHandlers) update(c *gin.Context) {
	id, ok := contentID(c)
	if !ok {
		return
	}
	var body documentBody
	if !bindJSON(c, &body) {
		return
	}
	item, err := h.svc.Update(c.Request.Context(), principal(c), c.Param("kind"), id, body.Document, body.BaseVersion, auditRequest(c))
	if err != nil {
		contentError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h contentHandlers) remove(c *gin.Context) {
	id, ok := contentID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), principal(c), c.Param("kind"), id, auditRequest(c)); err != nil {
		contentError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h contentHandlers) discardPending(c *gin.Context) {
	id, ok := contentID(c)
	if !ok {
		return
	}
	if err := h.svc.DiscardPending(c.Request.Context(), c.Param("kind"), id, auditRequest(c)); err != nil {
		contentError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h contentHandlers) publish(c *gin.Context) {
	id, ok := contentID(c)
	if !ok {
		return
	}
	var body struct {
		BaseVersion string `json:"base_version" binding:"required"`
	}
	if !bindJSON(c, &body) {
		return
	}
	item, err := h.svc.Publish(c.Request.Context(), principal(c), c.Param("kind"), id, body.BaseVersion, auditRequest(c))
	if err != nil {
		contentError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

type takeDownFunc func(ctx context.Context, actor *auth.Principal, kind string, id uuid.UUID, req audit.Request) (*content.Item, error)

func (h contentHandlers) unpublish(c *gin.Context) { h.takeDown(c, h.svc.Unpublish) }
func (h contentHandlers) archive(c *gin.Context)   { h.takeDown(c, h.svc.Archive) }

func (h contentHandlers) takeDown(c *gin.Context, fn takeDownFunc) {
	id, ok := contentID(c)
	if !ok {
		return
	}
	item, err := fn(c.Request.Context(), principal(c), c.Param("kind"), id, auditRequest(c))
	if err != nil {
		contentError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h contentHandlers) listSettings(c *gin.Context) {
	settings, err := h.svc.ListSettings(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

func (h contentHandlers) updateSettings(c *gin.Context) {
	var body struct {
		Values map[string]string `json:"values" binding:"required"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if err := h.svc.UpdateSettings(c.Request.Context(), principal(c), body.Values, auditRequest(c)); err != nil {
		contentError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type imageHandlers struct{ images *media.Images }

func (h imageHandlers) upload(c *gin.Context) {
	if h.images == nil {
		writeError(c, http.StatusServiceUnavailable, "storage_unavailable", "Image uploads are not set up on this server yet. Ask the ICT administrator.")
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Choose an image file to upload.")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, media.MaxUploadBytes+1))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "The upload did not finish. Try again.")
		return
	}
	img, err := h.images.Upload(c.Request.Context(), principal(c).UserID, header.Filename, data, auditRequest(c))
	switch {
	case errors.Is(err, media.ErrUnsupported), errors.Is(err, media.ErrTooBig), errors.Is(err, media.ErrCorrupt):
		writeError(c, http.StatusUnprocessableEntity, "invalid_image", capitalise(err.Error())+".")
	case err != nil:
		internalError(c, err)
	default:
		c.JSON(http.StatusCreated, img)
	}
}

func (h imageHandlers) get(c *gin.Context) {
	if h.images == nil {
		writeError(c, http.StatusServiceUnavailable, "storage_unavailable", "Image storage is not set up on this server yet.")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusNotFound, "not_found", "That image does not exist.")
		return
	}
	img, err := h.images.Get(c.Request.Context(), id)
	if errors.Is(err, media.ErrNotFound) {
		writeError(c, http.StatusNotFound, "not_found", "That image does not exist.")
		return
	}
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, img)
}

func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
