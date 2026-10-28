// Package config loads API settings from environment variables.
package config

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
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
	// MessageKey encrypts queued messages while they wait to be sent
	// (32 bytes, MESSAGE_KEY in base64). Development and test fall back to a
	// fixed key, reported by MessageKeyIsDevelopment; staging and production
	// must set one.
	MessageKey              []byte
	MessageKeyIsDevelopment bool
	S3                      S3Config
	// ClamAVAddr is a clamd host:port for scanning uploads. Empty means no
	// scanner: files are stored as "not_scanned" (D-19).
	ClamAVAddr string
	// Mpesa is Safaricom Daraja C2B (Stage 15a). Off unless MPESA_ENABLED.
	Mpesa MpesaConfig
	// KcbIPN is KCB Buni Instant Payment Notifications (Stage 15b). Off
	// unless KCB_IPN_ENABLED.
	KcbIPN KcbIPNConfig
}

// KcbIPNConfig is the college's KCB account as seen by Buni IPN. Nothing is
// built in: every value comes from the environment.
type KcbIPNConfig struct {
	Enabled bool
	// Secret is part of the notification path (KCB is told the full URL).
	Secret     string
	AllowedIPs []string
	// CreditAccount, Till and OrgShortCode, when set, must match what KCB
	// sends, so a notification for another account is never credited.
	CreditAccount string
	Till          string
	OrgShortCode  string
	// AutoCreditMaxCents, if above zero, is the largest notification matched
	// to a student by itself.
	AutoCreditMaxCents int64
}

// MpesaConfig is everything about the college's M-Pesa paybill. Nothing is
// built in: every value comes from the environment, so the college can
// change shortcode, keys or URLs without a code change.
type MpesaConfig struct {
	Enabled bool
	// Environment is sandbox or production; it picks the default API host.
	Environment    string
	APIBaseURL     string
	ConsumerKey    string
	ConsumerSecret string
	// ShortCode is the paybill (or till) number payments go to.
	ShortCode string
	// ResponseType is what Safaricom does when our validation URL does not
	// answer: Completed (accept the payment) or Cancelled.
	ResponseType string
	// CallbackBaseURL is the API's public HTTPS address Safaricom calls.
	CallbackBaseURL string
	// CallbackSecret is part of the callback path, so only Safaricom (who is
	// told the URL) can reach it. At least 32 characters.
	CallbackSecret string
	// AllowedIPs, if set, are the only addresses callbacks are accepted from.
	// Required in production.
	AllowedIPs []string
	// AutoCreditMaxCents, if above zero, is the largest payment credited to
	// a student by itself; larger ones wait for Finance to check them.
	AutoCreditMaxCents int64
	RegisterPath       string
	SimulatePath       string
}

var mpesaHosts = map[string]string{
	"sandbox":    "https://sandbox.safaricom.co.ke",
	"production": "https://api.safaricom.co.ke",
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

const messageKeySize = 32

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

	if v := get("MESSAGE_KEY", ""); v != "" {
		key, err := base64.StdEncoding.DecodeString(v)
		if err != nil || len(key) != messageKeySize {
			errs = append(errs, errors.New("MESSAGE_KEY must be 32 random bytes in base64 (openssl rand -base64 32)"))
		}
		cfg.MessageKey = key
	} else if cfg.Env == EnvDevelopment || cfg.Env == EnvTest {
		key := sha256.Sum256([]byte("dakota-college development message key"))
		cfg.MessageKey, cfg.MessageKeyIsDevelopment = key[:], true
	} else {
		errs = append(errs, fmt.Errorf("MESSAGE_KEY is required when APP_ENV=%s (openssl rand -base64 32)", cfg.Env))
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

	cfg.ClamAVAddr = get("CLAMAV_ADDR", "")

	if cfg.Mpesa.Enabled, err = strconv.ParseBool(get("MPESA_ENABLED", "false")); err != nil {
		errs = append(errs, errors.New("MPESA_ENABLED must be true or false"))
	}
	if cfg.Mpesa.Enabled {
		m := &cfg.Mpesa
		m.Environment = get("MPESA_ENV", "sandbox")
		m.APIBaseURL = strings.TrimRight(get("MPESA_API_BASE_URL", mpesaHosts[m.Environment]), "/")
		m.ConsumerKey = get("MPESA_CONSUMER_KEY", "")
		m.ConsumerSecret = get("MPESA_CONSUMER_SECRET", "")
		m.ShortCode = get("MPESA_SHORTCODE", "")
		m.ResponseType = get("MPESA_C2B_RESPONSE_TYPE", "Completed")
		m.CallbackBaseURL = strings.TrimRight(get("MPESA_CALLBACK_BASE_URL", ""), "/")
		m.CallbackSecret = get("MPESA_CALLBACK_SECRET", "")
		m.AllowedIPs = splitList(get("MPESA_ALLOWED_IPS", ""))
		if v := get("MPESA_AUTO_CREDIT_MAX_KES", ""); v != "" {
			kes, err := strconv.ParseInt(v, 10, 64)
			if err != nil || kes < 1 || kes > 10_000_000 {
				errs = append(errs, errors.New("MPESA_AUTO_CREDIT_MAX_KES must be whole shillings from 1 to 10,000,000, or empty for no limit"))
			}
			m.AutoCreditMaxCents = kes * 100
		}
		m.RegisterPath = get("MPESA_REGISTER_URL_PATH", "/mpesa/c2b/v2/registerurl")
		m.SimulatePath = get("MPESA_SIMULATE_PATH", "/mpesa/c2b/v2/simulate")
		errs = append(errs, m.check(cfg.Env)...)
	}

	if cfg.KcbIPN.Enabled, err = strconv.ParseBool(get("KCB_IPN_ENABLED", "false")); err != nil {
		errs = append(errs, errors.New("KCB_IPN_ENABLED must be true or false"))
	}
	if cfg.KcbIPN.Enabled {
		k := &cfg.KcbIPN
		k.Secret = get("KCB_IPN_SECRET", "")
		k.AllowedIPs = splitList(get("KCB_IPN_ALLOWED_IPS", ""))
		k.CreditAccount = get("KCB_CREDIT_ACCOUNT", "")
		k.Till = get("KCB_TILL_NUMBER", "")
		k.OrgShortCode = get("KCB_ORG_SHORTCODE", "")
		if v := get("KCB_AUTO_CREDIT_MAX_KES", ""); v != "" {
			kes, err := strconv.ParseInt(v, 10, 64)
			if err != nil || kes < 1 || kes > 10_000_000 {
				errs = append(errs, errors.New("KCB_AUTO_CREDIT_MAX_KES must be whole shillings from 1 to 10,000,000, or empty for no limit"))
			}
			k.AutoCreditMaxCents = kes * 100
		}
		if len(k.Secret) < 32 || len(k.Secret) > 128 || strings.Trim(k.Secret, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-") != "" {
			errs = append(errs, errors.New("KCB_IPN_SECRET must be 32 to 128 letters, digits, - or _ (openssl rand -hex 32)"))
		}
		for _, ip := range k.AllowedIPs {
			if !validIPOrCIDR(ip) {
				errs = append(errs, fmt.Errorf("KCB_IPN_ALLOWED_IPS: %q is not an IP address or CIDR", ip))
			}
		}
		if cfg.Env == EnvProduction {
			if k.CreditAccount == "" {
				errs = append(errs, errors.New("KCB_CREDIT_ACCOUNT is required when APP_ENV=production: the college account notifications must be for"))
			}
			if len(k.AllowedIPs) == 0 {
				errs = append(errs, errors.New("KCB_IPN_ALLOWED_IPS is required when APP_ENV=production: the addresses KCB sends from"))
			}
		}
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

func (m MpesaConfig) check(env Env) []error {
	var errs []error
	if _, ok := mpesaHosts[m.Environment]; !ok {
		errs = append(errs, fmt.Errorf("MPESA_ENV %q is not sandbox or production", m.Environment))
	}
	if env == EnvProduction && m.Environment != "production" {
		errs = append(errs, errors.New("MPESA_ENV must be production when APP_ENV=production"))
	}
	if u, err := url.Parse(m.APIBaseURL); err != nil || u.Scheme != "https" || u.Host == "" {
		errs = append(errs, errors.New("MPESA_API_BASE_URL must be an https URL"))
	}
	if m.ConsumerKey == "" || m.ConsumerSecret == "" {
		errs = append(errs, errors.New("MPESA_CONSUMER_KEY and MPESA_CONSUMER_SECRET are required when MPESA_ENABLED=true"))
	}
	if !isDigits(m.ShortCode, 5, 8) {
		errs = append(errs, errors.New("MPESA_SHORTCODE must be the 5 to 8 digit paybill or till number"))
	}
	if m.ResponseType != "Completed" && m.ResponseType != "Cancelled" {
		errs = append(errs, errors.New("MPESA_C2B_RESPONSE_TYPE must be Completed or Cancelled"))
	}
	u, err := url.Parse(m.CallbackBaseURL)
	switch {
	case err != nil || u.Host == "" || (u.Scheme != "https" && env != EnvDevelopment && env != EnvTest):
		errs = append(errs, errors.New("MPESA_CALLBACK_BASE_URL must be the API's public https address"))
	case strings.Contains(strings.ToLower(m.CallbackBaseURL), "mpesa") || strings.Contains(strings.ToLower(m.CallbackBaseURL), "safaricom"):
		// Safaricom refuses callback URLs containing these words.
		errs = append(errs, errors.New(`MPESA_CALLBACK_BASE_URL must not contain "mpesa" or "safaricom"`))
	}
	if len(m.CallbackSecret) < 32 || strings.ContainsAny(m.CallbackSecret, "/?#% ") {
		errs = append(errs, errors.New("MPESA_CALLBACK_SECRET must be at least 32 characters, letters, digits, - or _"))
	}
	if env == EnvProduction && len(m.AllowedIPs) == 0 {
		errs = append(errs, errors.New("MPESA_ALLOWED_IPS is required when APP_ENV=production: list the addresses Safaricom calls from"))
	}
	for _, ip := range m.AllowedIPs {
		if !validIPOrCIDR(ip) {
			errs = append(errs, fmt.Errorf("MPESA_ALLOWED_IPS: %q is not an IP address or CIDR", ip))
		}
	}
	return errs
}

func isDigits(s string, min, max int) bool {
	if len(s) < min || len(s) > max {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func validIPOrCIDR(s string) bool {
	if _, _, err := net.ParseCIDR(s); err == nil {
		return true
	}
	return net.ParseIP(s) != nil
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
