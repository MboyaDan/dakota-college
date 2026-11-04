package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"
)

func TestS3EndpointURL(t *testing.T) {
	for _, tc := range []struct {
		cfg  S3Config
		want string
	}{
		{S3Config{Endpoint: "localhost:9000"}, "http://localhost:9000"},
		{S3Config{Endpoint: "abc.r2.cloudflarestorage.com", UseSSL: true}, "https://abc.r2.cloudflarestorage.com"},
		{S3Config{Endpoint: "https://ref.storage.supabase.co/storage/v1/s3/"}, "https://ref.storage.supabase.co/storage/v1/s3"},
		// A full URL keeps its own scheme whatever UseSSL says.
		{S3Config{Endpoint: "http://minio:9000", UseSSL: true}, "http://minio:9000"},
	} {
		if got := tc.cfg.EndpointURL(); got != tc.want {
			t.Errorf("EndpointURL(%q, ssl=%v) = %q, want %q", tc.cfg.Endpoint, tc.cfg.UseSSL, got, tc.want)
		}
	}
}

// TestS3Live runs the real client against an S3 store when TEST_S3_ENDPOINT
// is set (local MinIO from docker compose, or a hosted store), using
// throwaway buckets it creates.
func TestS3Live(t *testing.T) {
	endpoint := os.Getenv("TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("TEST_S3_ENDPOINT not set")
	}
	cfg := S3Config{
		Endpoint:  endpoint,
		Region:    envOr("TEST_S3_REGION", "us-east-1"),
		AccessKey: os.Getenv("TEST_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("TEST_S3_SECRET_KEY"),
		UseSSL:    os.Getenv("TEST_S3_USE_SSL") == "true",
	}
	s, err := NewS3(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pub, priv := "test-public-s3live", "test-private-s3live"
	if err := s.EnsureBuckets(ctx, pub, priv); err != nil {
		t.Fatalf("EnsureBuckets: %v", err)
	}
	// A second call finds the buckets and changes nothing.
	if err := s.EnsureBuckets(ctx, pub, priv); err != nil {
		t.Fatalf("EnsureBuckets again: %v", err)
	}
	if err := s.Check(ctx, pub, priv); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if err := s.Check(ctx, "no-such-bucket-s3live"); err == nil {
		t.Fatal("Check of a missing bucket succeeded")
	}

	data := bytes.Repeat([]byte("dakota "), 200_000) // 1.4 MB
	if err := s.PutPrivate(ctx, priv, "a/doc.pdf", data, "application/pdf"); err != nil {
		t.Fatalf("PutPrivate: %v", err)
	}
	if err := s.Put(ctx, pub, "img/x.webp", []byte("img"), "image/webp"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	rc, err := s.Get(ctx, priv, "a/doc.pdf")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(got, data) {
		t.Fatalf("Get returned %d bytes, want %d", len(got), len(data))
	}
	if _, err := s.Get(ctx, priv, "a/missing.pdf"); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("Get missing: err = %v, want ErrObjectNotFound", err)
	}
	for _, k := range [][2]string{{priv, "a/doc.pdf"}, {pub, "img/x.webp"}} {
		if err := s.Delete(ctx, k[0], k[1]); err != nil {
			t.Fatalf("Delete %s: %v", k[1], err)
		}
	}
	if _, err := s.Get(ctx, priv, "a/doc.pdf"); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("Get after delete: err = %v", err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
