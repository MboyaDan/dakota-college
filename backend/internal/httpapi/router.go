// Package httpapi wires HTTP routes and middleware for the Go API.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/dakota-college/backend/internal/access"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
)

// Pinger checks that a dependency is reachable. *pgxpool.Pool satisfies it.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps are the services the router needs. Auth and Access may be nil in unit
// tests that only exercise the probes; their routes are then not mounted.
type Deps struct {
	Logger         *slog.Logger
	DB             Pinger
	TrustedProxies []string
	AllowedOrigins []string
	CookieSecure   bool
	Auth           *auth.Service
	Access         *access.Service
}

// Route describes one endpoint and how it is protected. The route table is
// what AC-07 tests walk: every route is public or names a permission.
type Route struct {
	Method string
	Path   string
	// Public routes need no session. Otherwise a session is required, and
	// Permission (if set) must be held.
	Public     bool
	Permission string
}

type Router struct {
	Engine *gin.Engine
	Routes []Route
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) { r.Engine.ServeHTTP(w, req) }

// NewRouter builds the Gin engine with all /api/v1 routes.
func NewRouter(d Deps) (*Router, error) {
	e := gin.New()
	if err := e.SetTrustedProxies(d.TrustedProxies); err != nil {
		return nil, err
	}
	e.HandleMethodNotAllowed = true
	e.Use(requestID(), accessLog(d.Logger), recoverJSON(d.Logger), limitBody(1<<20), checkOrigin(d.AllowedOrigins))

	e.NoRoute(func(c *gin.Context) {
		writeError(c, http.StatusNotFound, "not_found", "No such endpoint.")
	})
	e.NoMethod(func(c *gin.Context) {
		writeError(c, http.StatusMethodNotAllowed, "method_not_allowed", "This endpoint does not accept that method.")
	})

	rt := &Router{Engine: e}
	v1 := e.Group("/api/v1")
	cookie := sessionCookie{secure: d.CookieSecure}

	// add mounts a handler with its protection. Authenticated routes get the
	// session check, CSRF check (for unsafe methods) and permission check.
	add := func(method, path string, public bool, permission string, h gin.HandlerFunc) {
		rt.Routes = append(rt.Routes, Route{Method: method, Path: "/api/v1" + path, Public: public, Permission: permission})
		chain := []gin.HandlerFunc{}
		if !public {
			chain = append(chain, requireSession(d.Auth, cookie), requireCSRF(cookie))
			if permission != "" {
				chain = append(chain, requirePermission(permission, d.Logger))
			}
		}
		v1.Handle(method, path, append(chain, h)...)
	}

	h := healthHandlers{db: d.DB, logger: d.Logger}
	add(http.MethodGet, "/health", true, "", h.health)
	add(http.MethodGet, "/ready", true, "", h.ready)

	if d.Auth != nil {
		a := authHandlers{svc: d.Auth, cookie: cookie, logger: d.Logger}
		add(http.MethodPost, "/auth/login", true, "", a.login)
		add(http.MethodPost, "/auth/password-reset", true, "", a.requestReset)
		add(http.MethodPost, "/auth/password-reset/confirm", true, "", a.confirmReset)
		add(http.MethodGet, "/auth/session", false, "", a.session)
		add(http.MethodPost, "/auth/logout", false, "", a.logout)
	}

	if d.Access != nil && d.Auth != nil {
		ad := adminHandlers{svc: d.Access, auth: d.Auth, logger: d.Logger}
		add(http.MethodGet, "/admin/users", false, "users.manage", ad.listUsers)
		add(http.MethodPost, "/admin/users", false, "users.manage", ad.createUser)
		add(http.MethodPut, "/admin/users/:id/roles", false, "users.manage", ad.setUserRoles)
		add(http.MethodPost, "/admin/users/:id/disable", false, "users.manage", ad.disableUser)
		add(http.MethodPost, "/admin/users/:id/enable", false, "users.manage", ad.enableUser)
		add(http.MethodGet, "/admin/roles", false, "roles.manage", ad.listRoles)
		add(http.MethodPost, "/admin/roles", false, "roles.manage", ad.createRole)
		add(http.MethodPut, "/admin/roles/:id/permissions", false, "roles.manage", ad.setRolePermissions)
		add(http.MethodDelete, "/admin/roles/:id", false, "roles.manage", ad.deleteRole)
		add(http.MethodGet, "/admin/permissions", false, "roles.manage", ad.listPermissions)
	}

	return rt, nil
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
