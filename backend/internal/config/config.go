// Package config loads API settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Env is the deployment environment. It gates behaviour such as the seed
// command, which refuses to run in production.
type Env string

const (
	EnvDevelopment Env = "development"
	EnvTest        Env = "test"
	EnvStaging     Env = "staging"
	EnvProduction  Env = "production"
)

// Config holds every setting the API reads at start-up.
type Config struct {
	Env             Env
	HTTPAddr        string
	DatabaseURL     string
	LogLevel        slog.Level
	ShutdownTimeout time.Duration
	// TrustedProxies lists proxy IPs or CIDRs whose X-Forwarded-For headers
	// are believed. Empty means the client IP is the TCP peer.
	TrustedProxies []string

	// AllowedOrigins are the browser origins allowed to make state-changing
	// requests (the frontend). Requests from any other Origin are refused.
	AllowedOrigins []string
	// AppBaseURL is the public frontend URL, used in links inside emails.
	AppBaseURL string

	Session SessionConfig
	SMTP    SMTPConfig
	S3      S3Config
}

// S3Config is S3-compatible object storage: MinIO locally, Supabase Storage
// in production. Uploads are disabled when Endpoint or AccessKey is empty.
type S3Config struct {
	Endpoint      string
	Region        string
	AccessKey     string
	SecretKey     string
	UseSSL        bool
	PublicBucket  string
	PrivateBucket string
	// PublicBaseURL is where browsers fetch public bucket objects.
	PublicBaseURL string
}

func (c S3Config) Enabled() bool { return c.Endpoint != "" && c.AccessKey != "" }

// SessionConfig controls the session cookie (ADR-04).
type SessionConfig struct {
	// CookieSecure must be true outside local development. When true the
	// cookie uses the __Host- prefix.
	CookieSecure bool
	// TTL is the absolute session lifetime; IdleTimeout ends a session after
	// that long without a request.
	TTL         time.Duration
	IdleTimeout time.Duration
}

// SMTPConfig is the outgoing mail server. Locally this is Mailpit. From is a
// bare address; the display name is added when sending.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// Load reads configuration from the process environment.
func Load() (Config, error) {
	return FromLookup(os.LookupEnv)
}

// FromLookup reads configuration through lookup, so tests can supply values
// without touching the real environment.
func FromLookup(lookup func(string) (string, bool)) (Config, error) {
	get := func(key, def string) string {
		if v, ok := lookup(key); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
		return def
	}

	var errs []error
	cfg := Config{
		HTTPAddr:    get("HTTP_ADDR", ":8080"),
		DatabaseURL: get("DATABASE_URL", ""),
	}

	switch env := Env(get("APP_ENV", "")); env {
	case EnvDevelopment, EnvTest, EnvStaging, EnvProduction:
		cfg.Env = env
	case "":
		errs = append(errs, errors.New("APP_ENV is required (development, test, staging or production)"))
	default:
		errs = append(errs, fmt.Errorf("APP_ENV %q is not one of development, test, staging, production", env))
	}

	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}

	if err := cfg.LogLevel.UnmarshalText([]byte(get("LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL: %w", err))
	}

	timeout, err := time.ParseDuration(get("SHUTDOWN_TIMEOUT", "15s"))
	switch {
	case err != nil:
		errs = append(errs, fmt.Errorf("SHUTDOWN_TIMEOUT: %w", err))
	case timeout <= 0:
		errs = append(errs, errors.New("SHUTDOWN_TIMEOUT must be positive"))
	default:
		cfg.ShutdownTimeout = timeout
	}

	cfg.TrustedProxies = splitList(get("TRUSTED_PROXIES", ""))

	cfg.AllowedOrigins = splitList(get("ALLOWED_ORIGINS", "http://localhost:3000"))
	for _, o := range cfg.AllowedOrigins {
		if u, err := url.Parse(o); err != nil || u.Scheme == "" || u.Host == "" || (u.Path != "" && u.Path != "/") {
			errs = append(errs, fmt.Errorf("ALLOWED_ORIGINS: %q is not an origin like https://example.com", o))
		}
	}

	cfg.AppBaseURL = strings.TrimRight(get("APP_BASE_URL", "http://localhost:3000"), "/")
	if u, err := url.Parse(cfg.AppBaseURL); err != nil || u.Scheme == "" || u.Host == "" {
		errs = append(errs, fmt.Errorf("APP_BASE_URL %q is not a URL", cfg.AppBaseURL))
	}

	cfg.Session.CookieSecure, err = strconv.ParseBool(get("SESSION_COOKIE_SECURE", "true"))
	if err != nil {
		errs = append(errs, errors.New("SESSION_COOKIE_SECURE must be true or false"))
	}
	cfg.Session.TTL = duration(get("SESSION_TTL", "12h"), "SESSION_TTL", &errs)
	cfg.Session.IdleTimeout = duration(get("SESSION_IDLE_TIMEOUT", "30m"), "SESSION_IDLE_TIMEOUT", &errs)
	if cfg.Session.IdleTimeout > cfg.Session.TTL {
		errs = append(errs, errors.New("SESSION_IDLE_TIMEOUT must not be longer than SESSION_TTL"))
	}

	cfg.SMTP = SMTPConfig{
		Host:     get("SMTP_HOST", "localhost"),
		Username: get("SMTP_USERNAME", ""),
		Password: get("SMTP_PASSWORD", ""),
		From:     get("SMTP_FROM", "no-reply@example.invalid"),
	}
	if _, err := mail.ParseAddress(cfg.SMTP.From); err != nil || strings.ContainsAny(cfg.SMTP.From, "<> ") {
		errs = append(errs, fmt.Errorf("SMTP_FROM must be a plain email address like no-reply@college.example"))
	}
	if cfg.SMTP.Port, err = strconv.Atoi(get("SMTP_PORT", "1025")); err != nil || cfg.SMTP.Port <= 0 {
		errs = append(errs, errors.New("SMTP_PORT must be a positive number"))
	}

	cfg.S3 = S3Config{
		Endpoint:      get("S3_ENDPOINT", ""),
		Region:        get("S3_REGION", "us-east-1"),
		AccessKey:     get("S3_ACCESS_KEY", ""),
		SecretKey:     get("S3_SECRET_KEY", ""),
		PublicBucket:  get("S3_PUBLIC_BUCKET", "dakota-public"),
		PrivateBucket: get("S3_PRIVATE_BUCKET", "dakota-private"),
		PublicBaseURL: strings.TrimRight(get("S3_PUBLIC_BASE_URL", ""), "/"),
	}
	if cfg.S3.UseSSL, err = strconv.ParseBool(get("S3_USE_SSL", "true")); err != nil {
		errs = append(errs, errors.New("S3_USE_SSL must be true or false"))
	}
	if cfg.S3.Enabled() && cfg.S3.PublicBaseURL == "" {
		errs = append(errs, errors.New("S3_PUBLIC_BASE_URL is required when S3_ENDPOINT is set"))
	}

	// Production must not run with development-only settings.
	if cfg.Env == EnvProduction || cfg.Env == EnvStaging {
		if !cfg.Session.CookieSecure {
			errs = append(errs, fmt.Errorf("SESSION_COOKIE_SECURE must be true when APP_ENV=%s", cfg.Env))
		}
		if !strings.HasPrefix(cfg.AppBaseURL, "https://") {
			errs = append(errs, fmt.Errorf("APP_BASE_URL must use https when APP_ENV=%s", cfg.Env))
		}
		for _, o := range cfg.AllowedOrigins {
			if !strings.HasPrefix(o, "https://") {
				errs = append(errs, fmt.Errorf("ALLOWED_ORIGINS must use https when APP_ENV=%s (got %q)", cfg.Env, o))
			}
		}
	}

	return cfg, errors.Join(errs...)
}

// IsProduction reports whether the API runs against production data.
func (c Config) IsProduction() bool { return c.Env == EnvProduction }

func splitList(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func duration(v, name string, errs *[]error) time.Duration {
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		*errs = append(*errs, fmt.Errorf("%s must be a positive duration like 30m", name))
		return 0
	}
	return d
}
