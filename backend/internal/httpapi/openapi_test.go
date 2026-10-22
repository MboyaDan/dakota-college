package httpapi

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// openapi.yaml is the contract (CLAUDE.md). This fails when a route exists in
// code but not in the file, or the other way round.
func TestOpenAPIMatchesRoutes(t *testing.T) {
	raw, err := os.ReadFile("../../openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	pathLine := regexp.MustCompile(`^  (/api/v1/\S+):\s*$`)
	methodLine := regexp.MustCompile(`^    (get|post|put|patch|delete):\s*$`)

	var documented []string
	var current string
	for _, line := range strings.Split(string(raw), "\n") {
		if m := pathLine.FindStringSubmatch(line); m != nil {
			current = strings.NewReplacer("{", ":", "}", "").Replace(m[1])
			continue
		}
		if !strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "  /") {
			current = ""
		}
		if m := methodLine.FindStringSubmatch(line); m != nil && current != "" {
			documented = append(documented, strings.ToUpper(m[1])+" "+current)
		}
	}

	full := newHarness(t)
	var routed []string
	for _, r := range full.router.Routes {
		routed = append(routed, r.Method+" "+r.Path)
	}
	slices.Sort(documented)
	slices.Sort(routed)
	if !slices.Equal(documented, routed) {
		t.Fatalf("openapi.yaml and router differ\nopenapi: %v\nrouter:  %v", documented, routed)
	}
}
