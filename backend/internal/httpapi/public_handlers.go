package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/enquiry"
	"github.com/MboyaDan/dakota-college/backend/internal/media"
)

// Public website endpoints (Stage 6). No session: they only read published
// content or accept a visitor's enquiry.

type publicHandlers struct {
	content   *content.Service
	images    *media.Images
	enquiries *enquiry.Service
}

// site answers the public snapshot. The website caches it and revalidates
// (D-69); a short shared cache here only absorbs bursts.
func (h publicHandlers) site(c *gin.Context) {
	site, err := h.content.Site(c.Request.Context(), h.images)
	if err != nil {
		internalError(c, err)
		return
	}
	c.Header("Cache-Control", "public, max-age=60")
	c.JSON(http.StatusOK, site)
}

func (h publicHandlers) createEnquiry(c *gin.Context) {
	var body struct {
		FullName  string `json:"full_name" binding:"max=500"`
		Email     string `json:"email" binding:"max=500"`
		Phone     string `json:"phone" binding:"max=100"`
		QueryType string `json:"query_type" binding:"max=50"`
		Programme string `json:"programme" binding:"max=200"`
		Message   string `json:"message" binding:"max=20000"`
		Website   string `json:"website"`
		ElapsedMS int64  `json:"elapsed_ms"`
	}
	if !bindJSON(c, &body) {
		return
	}
	err := h.enquiries.Submit(c.Request.Context(), enquiry.Submission{
		FullName: body.FullName, Email: body.Email, Phone: body.Phone, QueryType: body.QueryType,
		ProgrammeSlug: body.Programme, Message: body.Message, Website: body.Website,
		Elapsed: time.Duration(body.ElapsedMS) * time.Millisecond,
	}, auditRequest(c))
	var ve *content.ValidationError
	switch {
	case err == nil:
		c.JSON(http.StatusAccepted, gin.H{"status": "received"})
	case errors.As(err, &ve):
		b := errorBody{}
		b.Error.Code = "validation_failed"
		b.Error.Message = "Some details need attention. Fix them and send again."
		b.Error.RequestID = c.GetString(requestIDKey)
		b.Error.Fields = ve.Problems
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, b)
	case errors.Is(err, enquiry.ErrTooMany):
		writeError(c, http.StatusTooManyRequests, "too_many_enquiries",
			"You have sent several messages in the last hour. We will answer those. If it is urgent, please call or WhatsApp us.")
	case errors.Is(err, enquiry.ErrBusy):
		writeError(c, http.StatusTooManyRequests, "enquiries_busy",
			"We are receiving a lot of messages right now. Please try again in an hour, or call or WhatsApp us.")
	default:
		internalError(c, err)
	}
}
