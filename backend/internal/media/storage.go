package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Storage writes and reads objects. Production uses Supabase Storage through
// its S3 endpoint; local development uses MinIO (brief 12.1).
type Storage interface {
	// Put stores a public, never-changing object (website images).
	Put(ctx context.Context, bucket, key string, data []byte, contentType string) error
	// PutPrivate stores a private object that no cache may keep.
	PutPrivate(ctx context.Context, bucket, key string, data []byte, contentType string) error
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, bucket, key string) error
}

// ErrObjectNotFound means the object is not in storage.
var ErrObjectNotFound = errors.New("object not found")

type S3Config struct {
	Endpoint, Region, AccessKey, SecretKey string
	UseSSL                                 bool
}

type S3 struct{ client *minio.Client }

func NewS3(c S3Config) (*S3, error) {
	client, err := minio.New(c.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(c.AccessKey, c.SecretKey, ""),
		Secure: c.UseSSL,
		Region: c.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("S3 client: %w", err)
	}
	return &S3{client: client}, nil
}

func (s *S3) Put(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
		// Variants never change: a new upload gets a new key.
		CacheControl: "public, max-age=31536000, immutable",
	})
	return err
}

func (s *S3) PutPrivate(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType, CacheControl: "private, no-store",
	})
	return err
}

func (s *S3) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	// GetObject is lazy: Stat surfaces a missing object now, not mid-response.
	if _, err := obj.Stat(); err != nil {
		obj.Close()
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, ErrObjectNotFound
		}
		return nil, err
	}
	return obj, nil
}

func (s *S3) Delete(ctx context.Context, bucket, key string) error {
	return s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
}

// EnsureBuckets creates the buckets if missing and lets anyone read the
// public one. Used in development and test only; production buckets are set
// up by hand (Stage 19 runbook).
func (s *S3) EnsureBuckets(ctx context.Context, public, private string) error {
	for _, b := range []string{public, private} {
		exists, err := s.client.BucketExists(ctx, b)
		if err != nil {
			return err
		}
		if !exists {
			if err := s.client.MakeBucket(ctx, b, minio.MakeBucketOptions{}); err != nil {
				return err
			}
		}
	}
	policy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::` + public + `/*"]}]}`
	return s.client.SetBucketPolicy(ctx, public, policy)
}

// Memory keeps objects in memory, for tests.
type Memory struct {
	mu      sync.Mutex
	Objects map[string][]byte
}

func (m *Memory) Put(_ context.Context, bucket, key string, data []byte, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Objects == nil {
		m.Objects = map[string][]byte{}
	}
	m.Objects[bucket+"/"+key] = append([]byte(nil), data...)
	return nil
}

func (m *Memory) PutPrivate(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	return m.Put(ctx, bucket, key, data, contentType)
}

func (m *Memory) Get(_ context.Context, bucket, key string) (io.ReadCloser, error) {
	b, ok := m.Object(bucket, key)
	if !ok {
		return nil, ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (m *Memory) Delete(_ context.Context, bucket, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Objects, bucket+"/"+key)
	return nil
}

// Keys lists the stored objects as bucket/key, for tests.
func (m *Memory) Keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.Objects))
	for k := range m.Objects {
		out = append(out, k)
	}
	return out
}

// Object returns a stored object, for tests.
func (m *Memory) Object(bucket, key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.Objects[bucket+"/"+key]
	return b, ok
}

// PublicURL joins the public base URL and an object key.
func PublicURL(base, key string) string { return strings.TrimRight(base, "/") + "/" + key }
