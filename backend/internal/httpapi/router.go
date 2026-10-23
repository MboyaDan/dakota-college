// Package httpapi wires HTTP routes and middleware for the Go API.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/dakota-college/backend/internal/access"
	"github.com/MboyaDan/dakota-college/backend/internal/applicant"
	"github.com/MboyaDan/dakota-college/backend/internal/application"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/enquiry"
	"github.com/MboyaDan/dakota-college/backend/internal/media"
	"github.com/MboyaDan/dakota-college/backend/internal/notify"
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
	Content        *content.Service
	// Images may be nil when object storage is not configured; uploads then
	// answer 503.
	Images     *media.Images
	Enquiries  *enquiry.Service
	Applicants *applicant.Service
	// Applications is the applicant's own application form.
	Applications *application.Service
	Notify       *notify.Service
}

const defaultBodyLimit = 1 << 20 // 1 MB for JSON endpoints

// Route describes one endpoint and how it is protected. The route table is
// what AC-07 tests walk: every route is public or names a permission.
type Route struct {
	Method string
	Path   string
	// Public routes need no session. Otherwise a session is required, and
	// Permission (if set) must be held, and Kind (if set) is the only kind
	// of account allowed (applicant routes).
	Public     bool
	Permission string
	Kind       string
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
	e.Use(requestID(), accessLog(d.Logger), recoverJSON(d.Logger), checkOrigin(d.AllowedOrigins))

	e.NoRoute(func(c *gin.Context) {
		writeError(c, http.StatusNotFound, "not_found", "No such endpoint.")
	})
	e.NoMethod(func(c *gin.Context) {
		writeError(c, http.StatusMethodNotAllowed, "method_not_allowed", "This endpoint does not accept that method.")
	})

	rt := &Router{Engine: e}
	v1 := e.Group("/api/v1")
	cookie := sessionCookie{secure: d.CookieSecure}

	// addWithLimit mounts a handler with its protection and body size limit.
	// Authenticated routes get the session check, CSRF check (for unsafe
	// methods) and permission check.
	mount := func(r Route, bodyLimit int64, h gin.HandlerFunc) {
		path := r.Path
		r.Path = "/api/v1" + path
		rt.Routes = append(rt.Routes, r)
		chain := []gin.HandlerFunc{limitBody(bodyLimit)}
		if !r.Public {
			chain = append(chain, requireSession(d.Auth, cookie), requireCSRF(cookie))
			if r.Permission != "" {
				chain = append(chain, requirePermission(r.Permission, d.Logger))
			}
			if r.Kind != "" {
				chain = append(chain, requireKind(r.Kind))
			}
		}
		v1.Handle(r.Method, path, append(chain, h)...)
	}
	addWithLimit := func(bodyLimit int64, method, path string, public bool, permission string, h gin.HandlerFunc) {
		mount(Route{Method: method, Path: path, Public: public, Permission: permission}, bodyLimit, h)
	}
	add := func(method, path string, public bool, permission string, h gin.HandlerFunc) {
		addWithLimit(defaultBodyLimit, method, path, public, permission, h)
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

	if d.Content != nil && d.Auth != nil {
		ch := contentHandlers{svc: d.Content}
		add(http.MethodGet, "/staff/content/:kind", false, content.PermRead, ch.list)
		add(http.MethodPost, "/staff/content/:kind", false, content.PermWrite, ch.create)
		add(http.MethodGet, "/staff/content/:kind/:id", false, content.PermRead, ch.get)
		add(http.MethodPut, "/staff/content/:kind/:id", false, content.PermWrite, ch.update)
		add(http.MethodDelete, "/staff/content/:kind/:id", false, content.PermWrite, ch.remove)
		add(http.MethodDelete, "/staff/content/:kind/:id/pending", false, content.PermWrite, ch.discardPending)
		add(http.MethodPost, "/staff/content/:kind/:id/publish", false, content.PermPublish, ch.publish)
		add(http.MethodPost, "/staff/content/:kind/:id/unpublish", false, content.PermPublish, ch.unpublish)
		add(http.MethodPost, "/staff/content/:kind/:id/archive", false, content.PermPublish, ch.archive)
		add(http.MethodGet, "/staff/settings", false, content.PermRead, ch.listSettings)
		add(http.MethodPut, "/staff/settings", false, content.PermPublish, ch.updateSettings)

		ih := imageHandlers{images: d.Images}
		addWithLimit(media.MaxUploadBytes+(1<<20), http.MethodPost, "/staff/images", false, content.PermWrite, ih.upload)
		add(http.MethodGet, "/staff/images/:id", false, content.PermRead, ih.get)
	}

	if d.Enquiries != nil && d.Auth != nil {
		eh := enquiryHandlers{svc: d.Enquiries}
		add(http.MethodGet, "/staff/enquiries", false, enquiry.PermView, eh.list)
		add(http.MethodGet, "/staff/enquiries/counts", false, enquiry.PermView, eh.counts)
		add(http.MethodGet, "/staff/enquiries/assignees", false, enquiry.PermManage, eh.assignees)
		add(http.MethodGet, "/staff/enquiries/:id", false, enquiry.PermView, eh.get)
		add(http.MethodPost, "/staff/enquiries/:id/assign", false, enquiry.PermManage, eh.assign)
		add(http.MethodPost, "/staff/enquiries/:id/status", false, enquiry.PermManage, eh.setStatus)
		add(http.MethodPost, "/staff/enquiries/:id/notes", false, enquiry.PermManage, eh.addNote)
	}

	if d.Notify != nil && d.Auth != nil {
		nh := notificationHandlers{svc: d.Notify}
		add(http.MethodGet, "/staff/notification-templates", false, notify.PermManage, nh.list)
		add(http.MethodPut, "/staff/notification-templates/:key/:channel", false, notify.PermManage, nh.update)
	}

	if d.Applicants != nil && d.Auth != nil {
		ah := applicantHandlers{svc: d.Applicants, cookie: cookie}
		add(http.MethodPost, "/applicant/sign-in-link", true, "", ah.requestLink)
		add(http.MethodPost, "/applicant/resume", true, "", ah.resume)
		add(http.MethodPost, "/applicant/sign-in", true, "", ah.signIn)
		mount(Route{Method: http.MethodGet, Path: "/applicant/applications", Kind: "applicant"}, defaultBodyLimit, ah.applications)
	}

	if d.Applications != nil && d.Auth != nil {
		fh := applicationHandlers{svc: d.Applications}
		own := func(method, path string, h gin.HandlerFunc) {
			mount(Route{Method: method, Path: path, Kind: "applicant"}, defaultBodyLimit, h)
		}
		own(http.MethodPost, "/applicant/applications", fh.start)
		own(http.MethodGet, "/applicant/applications/:id", fh.get)
		own(http.MethodPut, "/applicant/applications/:id/intake", fh.saveIntake)
		own(http.MethodPut, "/applicant/applications/:id/personal", fh.savePersonal)
		own(http.MethodPut, "/applicant/applications/:id/guardian", fh.saveGuardian)
		own(http.MethodPut, "/applicant/applications/:id/education", fh.saveEducation)
		own(http.MethodPut, "/applicant/applications/:id/sponsor", fh.saveSponsor)
		mount(Route{Method: http.MethodPost, Path: "/applicant/applications/:id/documents", Kind: "applicant"},
			application.DocumentUploadsLimit, fh.uploadDocument)
		own(http.MethodDelete, "/applicant/applications/:id/documents/:doc", fh.removeDocument)
		own(http.MethodPost, "/applicant/applications/:id/documents/:doc/link", fh.documentLink)
		own(http.MethodPost, "/applicant/applications/:id/submit", fh.submit)
		add(http.MethodPost, "/staff/applications/:id/documents/:doc/link", false, application.PermView, fh.staffDocumentLink)
		// Any signed-in account; the link itself says whose it is.
		add(http.MethodGet, "/files/:token", false, "", fh.openFile)
	}

	if d.Content != nil && d.Enquiries != nil {
		ph := publicHandlers{content: d.Content, images: d.Images, enquiries: d.Enquiries}
		add(http.MethodGet, "/public/site", true, "", ph.site)
		add(http.MethodPost, "/public/enquiries", true, "", ph.createEnquiry)
		add(http.MethodGet, "/public/news", true, "", ph.newsList)
		add(http.MethodGet, "/public/news/:slug", true, "", ph.newsArticle)
		add(http.MethodGet, "/public/events", true, "", ph.events)
		add(http.MethodGet, "/public/events/:slug", true, "", ph.event)
	}

	return rt, nil
}

// errorBody is the error shape for every endpoint (see openapi.yaml Error).
// Fields lists per-field problems for validation errors.
type errorBody struct {
	Error struct {
		Code      string                 `json:"code"`
		Message   string                 `json:"message"`
		RequestID string                 `json:"request_id,omitempty"`
		Fields    []content.FieldProblem `json:"fields,omitempty"`
	} `json:"error"`
}

func writeError(c *gin.Context, status int, code, message string) {
	var b errorBody
	b.Error.Code = code
	b.Error.Message = message
	b.Error.RequestID = c.GetString(requestIDKey)
	c.AbortWithStatusJSON(status, b)
}
