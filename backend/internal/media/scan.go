package media

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// Scan statuses, as stored in files.scan_status.
const (
	ScanNotScanned = "not_scanned"
	ScanClean      = "clean"
	ScanInfected   = "infected"
)

// Scanner checks uploads for malware (brief 8 and 13: "if available").
// Without a scanner, files are stored as "not_scanned" and nobody claims
// otherwise (D-19, R-15).
type Scanner interface {
	Scan(ctx context.Context, data []byte) (status string, err error)
}

// NoScanner is used when no virus scanner is configured.
type NoScanner struct{}

func (NoScanner) Scan(context.Context, []byte) (string, error) { return ScanNotScanned, nil }

// ClamAV scans with a clamd daemon over TCP (INSTREAM command).
type ClamAV struct {
	Addr    string
	Timeout time.Duration
}

const clamChunk = 64 << 10

func (c ClamAV) Scan(ctx context.Context, data []byte) (string, error) {
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", c.Addr)
	if err != nil {
		return "", fmt.Errorf("clamd: %w", err)
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}

	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return "", fmt.Errorf("clamd: %w", err)
	}
	var size [4]byte
	for off := 0; off < len(data); off += clamChunk {
		chunk := data[off:min(off+clamChunk, len(data))]
		binary.BigEndian.PutUint32(size[:], uint32(len(chunk)))
		if _, err := conn.Write(size[:]); err != nil {
			return "", fmt.Errorf("clamd: %w", err)
		}
		if _, err := conn.Write(chunk); err != nil {
			return "", fmt.Errorf("clamd: %w", err)
		}
	}
	binary.BigEndian.PutUint32(size[:], 0)
	if _, err := conn.Write(size[:]); err != nil {
		return "", fmt.Errorf("clamd: %w", err)
	}

	reply, err := bufio.NewReader(conn).ReadString(0)
	if err != nil && !errors.Is(err, net.ErrClosed) && reply == "" {
		return "", fmt.Errorf("clamd: %w", err)
	}
	return parseClamReply(strings.TrimRight(reply, "\x00\n"))
}

// parseClamReply reads "stream: OK", "stream: <name> FOUND" or an error.
func parseClamReply(reply string) (string, error) {
	switch {
	case strings.HasSuffix(reply, ": OK"):
		return ScanClean, nil
	case strings.HasSuffix(reply, " FOUND"):
		return ScanInfected, nil
	default:
		return "", fmt.Errorf("clamd: unexpected reply %q", reply)
	}
}
