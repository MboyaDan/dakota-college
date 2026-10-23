package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"slices"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
)

const (
	principalKey  = "principal"
	sessionTokKey = "session_token"
	csrfHeader    = "X-CSRF-Token"
)

// sessionCookie names and writes the session cookie: HttpOnly, SameSite=Lax,
// and Secure with the __Host- prefix outside local development (ADR-04).
type sessionCookie struct{ secure bool }

func (s sessionCookie) name() string {
	if s.secure {
		return "__Host-dtc_session"
	}
	return "dtc_session"
}

func (s sessionCookie) set(c *gin.Context, token string, expires time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: s.name(), Value: token, Path: "/", Expires: expires, MaxAge: int(time.Until(expires).Seconds()),
		HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode,
	})
}

func (s sessionCookie) clear(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: s.name(), Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode,
	})
}

func unsafeMethod(m string) bool {
	return m != http.MethodGet && m != http.MethodHead && m != http.MethodOptions
}

// checkOrigin refuses state-changing requests sent by a browser from another
// site. Requests without an Origin header (curl, server-side calls) pass; the
// CSRF token still protects every authenticated change.
func checkOrigin(allowed []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if unsafeMethod(c.Request.Method) {
			if c.GetHeader("Sec-Fetch-Site") == "cross-site" {
				writeError(c, http.StatusForbidden, "cross_site_request", "This request came from another website and was blocked.")
				return
			}
			if o := c.GetHeader("Origin"); o != "" && !slices.Contains(allowed, o) {
				writeError(c, http.StatusForbidden, "cross_site_request", "This request came from another website and was blocked.")
				return
			}
		}
		c.Next()
	}
}

// limitBody caps request bodies so a large upload cannot exhaust memory on
// JSON endpoints. File uploads (Stage 9) get their own limit.
func limitBody(max int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		c.Next()
	}
}

func requireSession(svc *auth.Service, cookie sessionCookie) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(cookie.name())
		if err != nil || token == "" {
			writeError(c, http.StatusUnauthorized, "unauthenticated", "Please sign in.")
			return
		}
		p, err := svc.Authenticate(c.Request.Context(), token)
		if errors.Is(err, auth.ErrUnauthenticated) {
			cookie.clear(c)
			writeError(c, http.StatusUnauthorized, "unauthenticated", "Your session has ended. Please sign in again.")
			return
		}
		if err != nil {
			c.Error(err)
			writeError(c, http.StatusInternalServerError, "internal_error", "Something went wrong on our side. Please try again.")
			return
		}
		c.Set(principalKey, p)
		c.Set(sessionTokKey, token)
		c.Next()
	}
}

// requireCSRF checks X-CSRF-Token on unsafe methods for cookie-authenticated
// requests. The token comes from the login or session response.
func requireCSRF(cookie sessionCookie) gin.HandlerFunc {
	return func(c *gin.Context) {
		if unsafeMethod(c.Request.Method) && !auth.ValidCSRF(c.GetString(sessionTokKey), c.GetHeader(csrfHeader)) {
			writeError(c, http.StatusForbidden, "csrf_failed", "Your session security check failed. Reload the page and try again.")
			return
		}
		c.Next()
	}
}

// requirePermission enforces a permission key (never a role name) on the
// backend, whatever the frontend shows (NN-02, AC-07).
func requirePermission(permission string, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		if p == nil || !p.Has(permission) {
			logger.WarnContext(c.Request.Context(), "permission denied",
				slog.String("permission", permission), slog.String("path", c.FullPath()),
				slog.String("request_id", c.GetString(requestIDKey)))
			writeError(c, http.StatusForbidden, "forbidden", "You do not have permission to do this.")
			return
		}
		c.Next()
	}
}

// requireKind admits only one kind of account, so applicant routes are
// closed to staff sessions (and the other way round by permissions).
func requireKind(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if p := principal(c); p == nil || p.Kind != kind {
			writeError(c, http.StatusForbidden, "forbidden", "You do not have permission to do this.")
			return
		}
		c.Next()
	}
}

func principal(c *gin.Context) *auth.Principal {
	if v, ok := c.Get(principalKey); ok {
		return v.(*auth.Principal)
	}
	return nil
}

// auditRequest captures who and where for audit entries.
func auditRequest(c *gin.Context) audit.Request {
	r := audit.Request{RequestID: c.GetString(requestIDKey), UserAgent: truncate(c.Request.UserAgent(), 512)}
	if ip, err := netip.ParseAddr(c.ClientIP()); err == nil {
		r.IP = &ip
	}
	if p := principal(c); p != nil {
		id := p.UserID
		r.ActorID = &id
	}
	return r
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
