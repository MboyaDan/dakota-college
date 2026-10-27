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

func TestMpesaConfig(t *testing.T) {
	base := func(extra map[string]string) map[string]string {
		m := map[string]string{
			"APP_ENV": "staging", "DATABASE_URL": "postgres://u:p@localhost:5432/db", "SESSION_COOKIE_SECURE": "true",
			"APP_BASE_URL": "https://staging.college.example", "ALLOWED_ORIGINS": "https://staging.college.example",
			"MPESA_ENABLED": "true", "MPESA_CONSUMER_KEY": "k", "MPESA_CONSUMER_SECRET": "s", "MPESA_SHORTCODE": "600999",
			"MPESA_CALLBACK_BASE_URL": "https://api.college.example", "MPESA_CALLBACK_SECRET": strings.Repeat("a", 40),
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}

	off, err := FromLookup(lookupFrom(map[string]string{"APP_ENV": "development", "DATABASE_URL": "postgres://x"}))
	if err != nil || off.Mpesa.Enabled {
		t.Fatalf("M-Pesa must be off by default: %+v %v", off.Mpesa, err)
	}

	cfg, err := FromLookup(lookupFrom(base(nil)))
	if err != nil {
		t.Fatalf("valid M-Pesa config refused: %v", err)
	}
	m := cfg.Mpesa
	if m.Environment != "sandbox" || m.APIBaseURL != "https://sandbox.safaricom.co.ke" || m.ResponseType != "Completed" ||
		m.RegisterPath != "/mpesa/c2b/v2/registerurl" {
		t.Errorf("defaults = %+v", m)
	}
	prod, err := FromLookup(lookupFrom(base(map[string]string{"APP_ENV": "production", "MPESA_ENV": "production",
		"MPESA_ALLOWED_IPS": "196.201.214.0/24", "MPESA_AUTO_CREDIT_MAX_KES": "200000"})))
	if err != nil || prod.Mpesa.AutoCreditMaxCents != 20_000_000 {
		t.Errorf("production with an allow list: %v, limit %d", err, prod.Mpesa.AutoCreditMaxCents)
	}
	if cfg, _ := FromLookup(lookupFrom(base(map[string]string{"MPESA_ENV": "production"}))); cfg.Mpesa.APIBaseURL != "https://api.safaricom.co.ke" {
		t.Errorf("production host = %q", cfg.Mpesa.APIBaseURL)
	}

	for name, extra := range map[string]map[string]string{
		"no keys":            {"MPESA_CONSUMER_KEY": ""},
		"short shortcode":    {"MPESA_SHORTCODE": "123"},
		"letters":            {"MPESA_SHORTCODE": "60099X"},
		"short secret":       {"MPESA_CALLBACK_SECRET": "short"},
		"secret with slash":  {"MPESA_CALLBACK_SECRET": strings.Repeat("a", 40) + "/x"},
		"http callback":      {"MPESA_CALLBACK_BASE_URL": "http://api.college.example"},
		"mpesa in callback":  {"MPESA_CALLBACK_BASE_URL": "https://mpesa.college.example"},
		"safaricom in path":  {"MPESA_CALLBACK_BASE_URL": "https://api.college.example/Safaricom"},
		"bad env":            {"MPESA_ENV": "live"},
		"bad response type":  {"MPESA_C2B_RESPONSE_TYPE": "Accept"},
		"bad allow list":     {"MPESA_ALLOWED_IPS": "196.201.214.0/24,not-an-ip"},
		"http api base":      {"MPESA_API_BASE_URL": "http://sandbox.safaricom.co.ke"},
		"sandbox in prod":    {"APP_ENV": "production"},
		"prod without IPs":   {"APP_ENV": "production", "MPESA_ENV": "production"},
		"bad credit limit":   {"MPESA_AUTO_CREDIT_MAX_KES": "12.50"},
		"not a bool enabled": {"MPESA_ENABLED": "yes please"},
	} {
		if _, err := FromLookup(lookupFrom(base(extra))); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestKcbIPNConfig(t *testing.T) {
	base := func(extra map[string]string) map[string]string {
		m := map[string]string{"APP_ENV": "development", "DATABASE_URL": "postgres://x",
			"KCB_IPN_ENABLED": "true", "KCB_IPN_SECRET": strings.Repeat("b", 64)}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	cfg, err := FromLookup(lookupFrom(base(map[string]string{"KCB_CREDIT_ACCOUNT": "1234567890", "KCB_AUTO_CREDIT_MAX_KES": "50000"})))
	if err != nil || !cfg.KcbIPN.Enabled || cfg.KcbIPN.CreditAccount != "1234567890" || cfg.KcbIPN.AutoCreditMaxCents != 5_000_000 {
		t.Fatalf("valid config: %+v %v", cfg.KcbIPN, err)
	}
	prod := map[string]string{"APP_ENV": "production", "SESSION_COOKIE_SECURE": "true", "APP_BASE_URL": "https://college.example",
		"ALLOWED_ORIGINS": "https://college.example"}
	for name, extra := range map[string]map[string]string{
		"short secret":      {"KCB_IPN_SECRET": "short"},
		"secret with slash": {"KCB_IPN_SECRET": strings.Repeat("b", 40) + "/"},
		"bad allow list":    {"KCB_IPN_ALLOWED_IPS": "nope"},
		"bad limit":         {"KCB_AUTO_CREDIT_MAX_KES": "0"},
		"prod no account":   prod,
	} {
		if _, err := FromLookup(lookupFrom(base(extra))); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
