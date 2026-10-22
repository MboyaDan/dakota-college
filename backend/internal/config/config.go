// Package config loads API settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
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

	for _, p := range strings.Split(get("TRUSTED_PROXIES", ""), ",") {
		if p = strings.TrimSpace(p); p != "" {
			cfg.TrustedProxies = append(cfg.TrustedProxies, p)
		}
	}

	return cfg, errors.Join(errs...)
}

// IsProduction reports whether the API runs against production data.
func (c Config) IsProduction() bool { return c.Env == EnvProduction }
