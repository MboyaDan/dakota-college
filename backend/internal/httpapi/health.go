package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const readyTimeout = 2 * time.Second

type healthHandlers struct {
	db     Pinger
	logger *slog.Logger
}

// health is the liveness probe: the process is up and serving HTTP. It never
// touches dependencies, so a slow database cannot get the API restarted.
func (h healthHandlers) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ready is the readiness probe: the API can serve requests that need the
// database. Failure detail is logged, never returned to the caller.
func (h healthHandlers) ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), readyTimeout)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		h.logger.WarnContext(ctx, "readiness check failed",
			slog.String("check", "database"),
			slog.String("error", err.Error()),
			slog.String("request_id", c.GetString(requestIDKey)),
		)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
			"checks": gin.H{"database": "unavailable"},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"checks": gin.H{"database": "ok"},
	})
}
