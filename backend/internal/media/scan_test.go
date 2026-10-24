package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"
)

// fakeClamd speaks enough of the clamd INSTREAM protocol to test the client:
// it reassembles the chunks and reports a "virus" when the data contains it.
func fakeClamd(t *testing.T, reply func(data []byte) string) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				cmd := make([]byte, len("zINSTREAM\x00"))
				if _, err := io.ReadFull(conn, cmd); err != nil || string(cmd) != "zINSTREAM\x00" {
					return
				}
				var data bytes.Buffer
				for {
					var size [4]byte
					if _, err := io.ReadFull(conn, size[:]); err != nil {
						return
					}
					n := binary.BigEndian.Uint32(size[:])
					if n == 0 {
						break
					}
					if _, err := io.CopyN(&data, conn, int64(n)); err != nil {
						return
					}
				}
				conn.Write([]byte(reply(data.Bytes()) + "\x00"))
			}()
		}
	}()
	return l.Addr().String()
}

func TestClamAVScan(t *testing.T) {
	addr := fakeClamd(t, func(data []byte) string {
		if bytes.Contains(data, []byte("TEST-SIGNATURE")) {
			return "stream: Test.Signature FOUND"
		}
		if len(data) == 0 {
			return "INSTREAM size limit exceeded. ERROR"
		}
		return "stream: OK"
	})
	c := ClamAV{Addr: addr}
	ctx := context.Background()

	// Larger than one chunk, to exercise chunking.
	clean := bytes.Repeat([]byte("a"), 3*clamChunk+17)
	if s, err := c.Scan(ctx, clean); err != nil || s != ScanClean {
		t.Fatalf("clean: %q %v", s, err)
	}
	infected := append(bytes.Repeat([]byte("b"), clamChunk), []byte("TEST-SIGNATURE")...)
	if s, err := c.Scan(ctx, infected); err != nil || s != ScanInfected {
		t.Fatalf("infected: %q %v", s, err)
	}
	if _, err := c.Scan(ctx, nil); err == nil || !strings.Contains(err.Error(), "unexpected reply") {
		t.Fatalf("error reply: %v", err)
	}
	if _, err := (ClamAV{Addr: "127.0.0.1:1"}).Scan(ctx, clean); err == nil {
		t.Fatal("no error when clamd is unreachable")
	}
}
