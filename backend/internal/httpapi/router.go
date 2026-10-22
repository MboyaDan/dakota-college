// Package httpapi wires HTTP routes and middleware for the Go API.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Pinger checks that a dependency is reachable. *pgxpool.Pool satisfies it.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps are the services the router needs.
type Deps struct {
	Logger         *slog.Logger
	DB             Pinger
	TrustedProxies []string
}

// NewRouter builds the Gin engine with all /api/v1 routes.
func NewRouter(d Deps) (*gin.Engine, error) {
	r := gin.New()
	if err := r.SetTrustedProxies(d.TrustedProxies); err != nil {
		return nil, err
	}
	r.HandleMethodNotAllowed = true
	r.Use(requestID(), accessLog(d.Logger), recoverJSON(d.Logger))

	r.NoRoute(func(c *gin.Context) {
		writeError(c, http.StatusNotFound, "not_found", "No such endpoint.")
	})
	r.NoMethod(func(c *gin.Context) {
		writeError(c, http.StatusMethodNotAllowed, "method_not_allowed", "This endpoint does not accept that method.")
	})

	v1 := r.Group("/api/v1")
	h := healthHandlers{db: d.DB, logger: d.Logger}
	v1.GET("/health", h.health)
	v1.GET("/ready", h.ready)

	return r, nil
}

// errorBody is the error shape for every endpoint (see openapi.yaml Error).
type errorBody struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id,omitempty"`
	} `json:"error"`
}

func writeError(c *gin.Context, status int, code, message string) {
	var b errorBody
	b.Error.Code = code
	b.Error.Message = message
	b.Error.RequestID = c.GetString(requestIDKey)
	c.AbortWithStatusJSON(status, b)
}
