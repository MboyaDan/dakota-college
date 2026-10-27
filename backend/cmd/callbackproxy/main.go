// Command callbackproxy lets Safaricom's sandbox reach a developer's API
// through a tunnel (ngrok) without exposing the rest of it. It forwards
// only POST /api/v1/payments/c2b/{secret}/{confirmation,validation} to the
// local API and answers 404 to everything else, so the staff portal, the
// seeded test accounts and every other route stay off the internet.
//
// Development and sandbox only: it refuses to start when APP_ENV is
// production or staging, or when MPESA_ENV is not sandbox.
//
//	CALLBACK_PROXY_ADDR (default 127.0.0.1:8181)  where ngrok points
//	CALLBACK_PROXY_UPSTREAM (default http://127.0.0.1:$API_PORT)
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"time"
)

var allowed = regexp.MustCompile(`^/api/v1/payments/c2b/[A-Za-z0-9_-]{32,128}/(confirmation|validation)$`)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if env := os.Getenv("APP_ENV"); env == "production" || env == "staging" {
		logger.Error("callbackproxy is for development only", slog.String("APP_ENV", env))
		os.Exit(1)
	}
	if os.Getenv("MPESA_ENV") != "sandbox" {
		logger.Error("callbackproxy needs MPESA_ENV=sandbox")
		os.Exit(1)
	}
	addr := getenv("CALLBACK_PROXY_ADDR", "127.0.0.1:8181")
	upstream, err := url.Parse(getenv("CALLBACK_PROXY_UPSTREAM", "http://127.0.0.1:"+getenv("API_PORT", "8180")))
	if err != nil {
		logger.Error("bad CALLBACK_PROXY_UPSTREAM", slog.String("error", err.Error()))
		os.Exit(1)
	}
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !allowed.MatchString(r.URL.Path) || r.URL.RawQuery != "" {
			// Not logged with the path: it may hold a guess at the secret.
			logger.Warn("refused", slog.String("method", r.Method), slog.String("remote", r.Header.Get("X-Forwarded-For")))
			http.NotFound(w, r)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		// The tunnel is the only caller; nothing else from the request is trusted.
		r.Header.Del("Cookie")
		r.Header.Del("Authorization")
		r.Header.Del("Origin")
		logger.Info("forwarded callback", slog.String("kind", path(r.URL.Path)))
		proxy.ServeHTTP(w, r)
	})
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second}
	fmt.Printf("Forwarding only M-Pesa callbacks from http://%s to %s\nStart the tunnel with: ngrok http %s\n", addr, upstream, addr)
	if err := srv.ListenAndServe(); err != nil {
		logger.Error("stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func path(p string) string {
	if m := allowed.FindStringSubmatch(p); m != nil {
		return m[1]
	}
	return ""
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
