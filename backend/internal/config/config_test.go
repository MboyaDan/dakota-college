package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func lookupFrom(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func TestFromLookupDefaults(t *testing.T) {
	cfg, err := FromLookup(lookupFrom(map[string]string{
		"APP_ENV":      "development",
		"DATABASE_URL": "postgres://u:p@localhost:5432/db",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 15s", cfg.ShutdownTimeout)
	}
	if len(cfg.TrustedProxies) != 0 {
		t.Errorf("TrustedProxies = %v, want none", cfg.TrustedProxies)
	}
	if !cfg.Session.CookieSecure || cfg.Session.TTL != 12*time.Hour || cfg.Session.IdleTimeout != 30*time.Minute {
		t.Errorf("session defaults = %+v", cfg.Session)
	}
	if strings.Join(cfg.AllowedOrigins, ",") != "http://localhost:3000" || cfg.AppBaseURL != "http://localhost:3000" {
		t.Errorf("origin defaults = %v %q", cfg.AllowedOrigins, cfg.AppBaseURL)
	}
	if cfg.IsProduction() {
		t.Error("development config reports production")
	}
}

func TestFromLookupOverrides(t *testing.T) {
	cfg, err := FromLookup(lookupFrom(map[string]string{
		"APP_ENV":          "production",
		"DATABASE_URL":     "postgres://x",
		"APP_BASE_URL":     "https://dakota.example/",
		"ALLOWED_ORIGINS":  "https://dakota.example",
		"HTTP_ADDR":        ":9000",
		"LOG_LEVEL":        "debug",
		"SHUTDOWN_TIMEOUT": "5s",
		"TRUSTED_PROXIES":  " 10.0.0.0/8, ,192.168.1.1 ",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.IsProduction() || cfg.HTTPAddr != ":9000" || cfg.LogLevel != slog.LevelDebug || cfg.ShutdownTimeout != 5*time.Second {
		t.Errorf("overrides not applied: %+v", cfg)
	}
	if got := strings.Join(cfg.TrustedProxies, "|"); got != "10.0.0.0/8|192.168.1.1" {
		t.Errorf("TrustedProxies = %q", got)
	}
}

func TestFromLookupErrors(t *testing.T) {
	prod := func(extra map[string]string) map[string]string {
		m := map[string]string{"APP_ENV": "production", "DATABASE_URL": "x", "APP_BASE_URL": "https://dakota.example", "ALLOWED_ORIGINS": "https://dakota.example"}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"insecure cookie in production", prod(map[string]string{"SESSION_COOKIE_SECURE": "false"}), "SESSION_COOKIE_SECURE must be true"},
		{"http base URL in production", prod(map[string]string{"APP_BASE_URL": "http://dakota.example"}), "APP_BASE_URL must use https"},
		{"http origin in production", prod(map[string]string{"ALLOWED_ORIGINS": "http://dakota.example"}), "ALLOWED_ORIGINS must use https"},
		{"origin with path", map[string]string{"APP_ENV": "test", "DATABASE_URL": "x", "ALLOWED_ORIGINS": "http://a.test/app"}, "not an origin"},
		{"idle longer than ttl", map[string]string{"APP_ENV": "test", "DATABASE_URL": "x", "SESSION_TTL": "10m", "SESSION_IDLE_TIMEOUT": "1h"}, "SESSION_IDLE_TIMEOUT"},
		{"bad smtp port", map[string]string{"APP_ENV": "test", "DATABASE_URL": "x", "SMTP_PORT": "abc"}, "SMTP_PORT"},
		{"missing APP_ENV", map[string]string{"DATABASE_URL": "x"}, "APP_ENV is required"},
		{"unknown APP_ENV", map[string]string{"APP_ENV": "prod", "DATABASE_URL": "x"}, `APP_ENV "prod"`},
		{"missing DATABASE_URL", map[string]string{"APP_ENV": "test"}, "DATABASE_URL is required"},
		{"blank DATABASE_URL", map[string]string{"APP_ENV": "test", "DATABASE_URL": "  "}, "DATABASE_URL is required"},
		{"bad LOG_LEVEL", map[string]string{"APP_ENV": "test", "DATABASE_URL": "x", "LOG_LEVEL": "loud"}, "LOG_LEVEL"},
		{"bad SHUTDOWN_TIMEOUT", map[string]string{"APP_ENV": "test", "DATABASE_URL": "x", "SHUTDOWN_TIMEOUT": "soon"}, "SHUTDOWN_TIMEOUT"},
		{"zero SHUTDOWN_TIMEOUT", map[string]string{"APP_ENV": "test", "DATABASE_URL": "x", "SHUTDOWN_TIMEOUT": "0s"}, "must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FromLookup(lookupFrom(tt.env))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestFromLookupReportsAllErrors(t *testing.T) {
	_, err := FromLookup(lookupFrom(map[string]string{}))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"APP_ENV", "DATABASE_URL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}
