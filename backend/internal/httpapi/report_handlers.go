package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/reports"
)

// Dashboards and count-only CSV exports (Stage 16b).

type reportHandlers struct{ svc *reports.Service }

func reportError(c *gin.Context, err error) {
	var ve *content.ValidationError
	switch {
	case errors.As(err, &ve):
		writeValidation(c, ve.Problems)
	case errors.Is(err, reports.ErrForbidden):
		writeError(c, http.StatusForbidden, "forbidden", "You do not have permission to see the data in this export.")
	case errors.Is(err, reports.ErrUnknownExport):
		writeError(c, http.StatusNotFound, "not_found", "That export does not exist.")
	default:
		internalError(c, err)
	}
}

func (h reportHandlers) dashboard(c *gin.Context) {
	period, err := h.svc.ParsePeriod(c.Query("from"), c.Query("to"))
	if err != nil {
		reportError(c, err)
		return
	}
	d, err := h.svc.Dashboard(c.Request.Context(), principal(c), period)
	if err != nil {
		reportError(c, err)
		return
	}
	c.JSON(http.StatusOK, d)
}

func (h reportHandlers) export(c *gin.Context) {
	period, err := h.svc.ParsePeriod(c.Query("from"), c.Query("to"))
	if err != nil {
		reportError(c, err)
		return
	}
	f, err := h.svc.Export(c.Request.Context(), principal(c), auditRequest(c), c.Param("kind"), period)
	if err != nil {
		reportError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+f.Name+`"`)
	c.Header("Cache-Control", "no-store")
	c.Header("X-Export-Rows", strconv.Itoa(f.Rows))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", f.Data)
}
