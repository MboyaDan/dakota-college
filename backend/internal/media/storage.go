package media

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Storage writes objects. Production uses Supabase Storage through its S3
// endpoint; local development uses MinIO (brief 12.1).
type Storage interface {
	Put(ctx context.Context, bucket, key string, data []byte, contentType string) error
}

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

func (m *Memory) Get(bucket, key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.Objects[bucket+"/"+key]
	return b, ok
}

// PublicURL joins the public base URL and an object key.
func PublicURL(base, key string) string { return strings.TrimRight(base, "/") + "/" + key }
