package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	requestIDKey    = "request_id"
	requestIDHeader = "X-Request-ID"
)

// validRequestID limits caller-supplied IDs so they are safe to log and echo.
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// requestID reuses a well-formed X-Request-ID from the caller (the frontend
// forwards it) or generates one, and echoes it on the response.
func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if !validRequestID.MatchString(id) {
			id = newRequestID()
		}
		c.Set(requestIDKey, id)
		c.Header(requestIDHeader, id)
		c.Next()
	}
}

func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error
	return hex.EncodeToString(b[:])
}

// accessLog writes one structured line per request. It logs the path only,
// never the query string, which may carry tokens.
func accessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		level := slog.LevelInfo
		switch {
		case status >= 500:
			level = slog.LevelError
		case status >= 400:
			level = slog.LevelWarn
		}
		logger.LogAttrs(c.Request.Context(), level, "http request",
			slog.String("request_id", c.GetString(requestIDKey)),
			slog.String("method", c.Request.Method),
			slog.String("path", loggedPath(c)),
			slog.Int("status", status),
			slog.Duration("duration", time.Since(start)),
			slog.String("client_ip", clientIP(c)),
		)
	}
}

// recoverJSON turns a panic into a logged 500 with the standard error body.
func recoverJSON(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				logger.ErrorContext(c.Request.Context(), "panic recovered",
					slog.Any("panic", rec),
					slog.String("request_id", c.GetString(requestIDKey)),
				)
				writeError(c, http.StatusInternalServerError, "internal_error", "Something went wrong on our side. Please try again.")
			}
		}()
		c.Next()
	}
}

// loggedPath is the request path for the access log. Paths that carry a
// secret (the M-Pesa callback secret, file link tokens) are logged as their
// route pattern, so the secret never reaches the logs.
func loggedPath(c *gin.Context) string {
	if route := c.FullPath(); strings.Contains(route, ":secret") || strings.Contains(route, ":token") {
		return route
	}
	return c.Request.URL.Path
}
