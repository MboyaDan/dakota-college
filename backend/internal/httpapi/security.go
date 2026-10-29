package httpapi

import (
	"crypto/subtle"
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

// stepCookie holds a sign-in challenge between the password and the code
// (Stage 17b). It lives only as long as the challenge.
func (s sessionCookie) stepName() string {
	if s.secure {
		return "__Host-dtc_signin_step"
	}
	return "dtc_signin_step"
}

func (s sessionCookie) setStep(c *gin.Context, token string, expires time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: s.stepName(), Value: token, Path: "/", Expires: expires, MaxAge: int(time.Until(expires).Seconds()),
		HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteStrictMode,
	})
}

func (s sessionCookie) clearStep(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: s.stepName(), Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteStrictMode,
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

// securityHeaders sets the headers every API response carries. Responses are
// JSON, PDFs and CSVs that must never be sniffed, framed or cached by a shared
// cache; handlers that serve cacheable public data overwrite Cache-Control,
// and file handlers add a stricter policy of their own. HSTS is sent only
// when the API runs behind HTTPS (secure cookies), never on local http.
func securityHeaders(secure bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "frame-ancestors 'none'")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Cache-Control", "no-store")
		if secure {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		c.Next()
	}
}

const (
	clientIPKey       = "client_ip"
	clientIPHeader    = "X-Dakota-Client-IP"
	clientIPKeyHeader = "X-Dakota-Client-IP-Key"
)

// trustedClientIP believes the visitor address the frontend passes on, but
// only when the request also carries the shared secret. The frontend copies
// the address from a header its hosting platform sets and overwrites, so the
// value cannot come from the visitor (R-23). Without the secret, or with a
// malformed address, the API falls back to the TCP peer as before.
func trustedClientIP(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret != "" {
			key := c.GetHeader(clientIPKeyHeader)
			if subtle.ConstantTimeCompare([]byte(key), []byte(secret)) == 1 {
				if ip, err := netip.ParseAddr(c.GetHeader(clientIPHeader)); err == nil {
					c.Set(clientIPKey, ip.Unmap().String())
				}
			}
		}
		c.Next()
	}
}

// clientIP is the visitor's address: the verified one from the frontend when
// present, otherwise what Gin derives from the connection and trusted proxies.
func clientIP(c *gin.Context) string {
	if ip := c.GetString(clientIPKey); ip != "" {
		return ip
	}
	return c.ClientIP()
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
	if ip, err := netip.ParseAddr(clientIP(c)); err == nil {
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
