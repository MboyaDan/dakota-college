package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
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

// S3Config describes an S3-compatible store. Endpoint is either host[:port]
// (MinIO, R2; the scheme comes from UseSSL) or a full URL that may include a
// path, such as Supabase's https://<ref>.storage.supabase.co/storage/v1/s3.
type S3Config struct {
	Endpoint, Region, AccessKey, SecretKey string
	UseSSL                                 bool
}

// EndpointURL returns the endpoint as a URL, adding the scheme when Endpoint
// is a bare host.
func (c S3Config) EndpointURL() string {
	if strings.Contains(c.Endpoint, "://") {
		return strings.TrimRight(c.Endpoint, "/")
	}
	scheme := "http"
	if c.UseSSL {
		scheme = "https"
	}
	return scheme + "://" + strings.TrimRight(c.Endpoint, "/")
}

type S3 struct{ client *s3.Client }

func NewS3(c S3Config) (*S3, error) {
	if c.Endpoint == "" {
		return nil, errors.New("S3 client: endpoint is empty")
	}
	client := s3.New(s3.Options{
		Region:       c.Region,
		BaseEndpoint: aws.String(c.EndpointURL()),
		Credentials:  credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, ""),
		// MinIO and Supabase address buckets by path, not by subdomain.
		UsePathStyle: true,
		// Only send and check checksums when the API requires them: S3
		// lookalikes do not all accept the newer checksum trailers.
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	return &S3{client: client}, nil
}

func (s *S3) Put(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket), Key: aws.String(key),
		Body: bytes.NewReader(data), ContentLength: aws.Int64(int64(len(data))),
		ContentType: aws.String(contentType),
		// Variants never change: a new upload gets a new key.
		CacheControl: aws.String("public, max-age=31536000, immutable"),
	})
	return err
}

func (s *S3) PutPrivate(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket), Key: aws.String(key),
		Body: bytes.NewReader(data), ContentLength: aws.Int64(int64(len(data))),
		ContentType: aws.String(contentType), CacheControl: aws.String("private, no-store"),
	})
	return err
}

func (s *S3) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrObjectNotFound
		}
		return nil, err
	}
	return out.Body, nil
}

func (s *S3) Delete(ctx context.Context, bucket, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	return err
}

// Check confirms both buckets can be reached with these credentials. Used by
// the setup check before a hosted deploy; it writes nothing.
func (s *S3) Check(ctx context.Context, buckets ...string) error {
	for _, b := range buckets {
		if _, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(b)}); err != nil {
			return fmt.Errorf("bucket %s: %w", b, err)
		}
	}
	return nil
}

// EnsureBuckets creates the buckets if missing and lets anyone read the
// public one. Used in development and test only; hosted buckets are set up
// by hand (docs/deployment.md).
func (s *S3) EnsureBuckets(ctx context.Context, public, private string) error {
	for _, b := range []string{public, private} {
		_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(b)})
		if err == nil {
			continue
		}
		if !isNotFound(err) {
			return err
		}
		if _, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(b)}); err != nil {
			return err
		}
	}
	policy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::` + public + `/*"]}]}`
	_, err := s.client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{Bucket: aws.String(public), Policy: aws.String(policy)})
	return err
}

// isNotFound reports a missing object or bucket, whichever way the store
// says it (typed S3 errors, an error code, or a bare 404 on HEAD).
func isNotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NoSuchBucket", "NotFound":
			return true
		}
	}
	var respErr interface{ HTTPStatusCode() int }
	return errors.As(err, &respErr) && respErr.HTTPStatusCode() == http.StatusNotFound
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
