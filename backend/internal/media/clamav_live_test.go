package media

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

// TestClamAVLive scans against a real clamd (the Compose `scan` profile
// locally, a service container in CI). It skips unless TEST_CLAMAV_ADDR is
// set, so ordinary runs need no 2 GB scanner.
func TestClamAVLive(t *testing.T) {
	addr := os.Getenv("TEST_CLAMAV_ADDR")
	if addr == "" {
		t.Skip("TEST_CLAMAV_ADDR not set")
	}
	s := ClamAV{Addr: addr}
	ctx := context.Background()

	// The EICAR anti-virus test file, assembled at run time so this source
	// file itself is not flagged by scanners on developer machines.
	eicar := []byte(strings.Join([]string{`X5O!P%@AP[4\PZX54(P^)7CC)7}$`, `EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`}, ""))
	if status, err := s.Scan(ctx, eicar); err != nil || status != ScanInfected {
		t.Errorf("EICAR: %q %v, want infected", status, err)
	}

	pdf := append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte("% clean filler line\n"), 10_000)...) // several INSTREAM chunks
	if status, err := s.Scan(ctx, pdf); err != nil || status != ScanClean {
		t.Errorf("clean file: %q %v, want clean", status, err)
	}

}
