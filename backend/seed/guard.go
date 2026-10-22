// Package seed fills a development or test database with obviously fake
// sample data (NN-06). It never runs against staging or production.
package seed

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/MboyaDan/dakota-college/backend/internal/config"
)

// hostedSuffixes are database hosts that are never local. A second line of
// defence in case APP_ENV is set wrongly on a hosted environment.
var hostedSuffixes = []string{".supabase.co", ".supabase.com", ".render.com", ".neon.tech", ".rds.amazonaws.com"}

// Guard returns an error unless seeding is allowed: APP_ENV must be
// development or test, and the database must not be a hosted one.
func Guard(env config.Env, databaseURL string) error {
	if env != config.EnvDevelopment && env != config.EnvTest {
		return fmt.Errorf("seed refuses to run with APP_ENV=%s: sample data is for development and test only", env)
	}
	u, err := url.Parse(databaseURL)
	if err != nil || u.Hostname() == "" {
		return errors.New("seed refuses to run: DATABASE_URL is not a URL with a host")
	}
	host := strings.ToLower(u.Hostname())
	for _, suffix := range hostedSuffixes {
		if strings.HasSuffix(host, suffix) {
			return fmt.Errorf("seed refuses to run against hosted database host %q", host)
		}
	}
	return nil
}
