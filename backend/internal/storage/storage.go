// Package storage stores user-uploaded objects in Cloudflare R2 through its
// S3-compatible API, with an in-memory driver for development and tests and
// a disabled driver for deployments without object storage. Three buckets
// are used: public avatars, public advisor images (R2_ADVISOR_IMAGE) and
// private chat images (R2_CHAT_IMAGES_PRIVATE), which are never served
// directly but streamed by the API to authorised participants.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
)

// ObjectStore uploads, reads and deletes objects. For private buckets the
// returned URL is meaningless and must not be exposed.
type ObjectStore interface {
	Put(ctx context.Context, key, contentType string, body []byte, cacheControl string) (publicURL string, err error)
	Get(ctx context.Context, key string, maxBytes int64) (body []byte, contentType string, err error)
	Delete(ctx context.Context, key string) error
}

// New builds the configured avatar store (storage.r2_bucket).
func New(cfg config.StorageConfig) (ObjectStore, error) {
	return NewBucket(cfg, cfg.R2Bucket, cfg.PublicBaseURL)
}

// NewBucket builds a store for one bucket with the configured driver. With
// the r2 driver an empty bucket name yields a Disabled store (the feature
// is turned off rather than writing to the wrong bucket).
func NewBucket(cfg config.StorageConfig, bucket, publicBaseURL string) (ObjectStore, error) {
	switch cfg.Driver {
	case "r2":
		if bucket == "" {
			return Disabled{}, nil
		}
		return NewR2(cfg, bucket, publicBaseURL)
	case "memory":
		return NewMemory(publicBaseURL), nil
	case "disabled":
		return Disabled{}, nil
	}
	return nil, fmt.Errorf("storage: unknown driver %q", cfg.Driver)
}

// ErrNotFound is returned by Get for missing objects.
var ErrNotFound = errors.New("storage: object not found")

// ErrTooLarge is returned by Get when an object exceeds maxBytes.
var ErrTooLarge = errors.New("storage: object too large")

// ErrDisabled is returned by uploads when object storage is not configured.
var ErrDisabled = errors.New("storage: uploads are disabled")

// Disabled rejects uploads. Deletes succeed because nothing was ever stored.
type Disabled struct{}

// Put implements ObjectStore.
func (Disabled) Put(context.Context, string, string, []byte, string) (string, error) {
	return "", ErrDisabled
}

// Get implements ObjectStore.
func (Disabled) Get(context.Context, string, int64) ([]byte, string, error) {
	return nil, "", ErrDisabled
}

// Delete implements ObjectStore.
func (Disabled) Delete(context.Context, string) error { return nil }

// R2 is a Cloudflare R2 bucket accessed with scoped API-token credentials.
type R2 struct {
	client  *s3.Client
	bucket  string
	baseURL string
}

// NewR2 returns a store for one R2 bucket.
func NewR2(cfg config.StorageConfig, bucket, publicBaseURL string) (*R2, error) {
	client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(cfg.R2Endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.R2AccessKeyID, cfg.R2SecretKey, ""),
		UsePathStyle: true,
	})
	return &R2{client: client, bucket: bucket, baseURL: publicBaseURL}, nil
}

// Get implements ObjectStore.
func (r *R2) Get(ctx context.Context, key string, maxBytes int64) ([]byte, string, error) {
	if err := validateKey(key); err != nil {
		return nil, "", err
	}
	out, err := r.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(r.bucket), Key: aws.String(key)})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, "", ErrNotFound
		}
		return nil, "", fmt.Errorf("storage: r2 get: %w", err)
	}
	defer out.Body.Close()
	if out.ContentLength != nil && maxBytes > 0 && *out.ContentLength > maxBytes {
		return nil, "", ErrTooLarge
	}
	limit := maxBytes
	if limit <= 0 {
		limit = 32 << 20
	}
	body, err := io.ReadAll(io.LimitReader(out.Body, limit+1))
	if err != nil {
		return nil, "", fmt.Errorf("storage: r2 read: %w", err)
	}
	if int64(len(body)) > limit {
		return nil, "", ErrTooLarge
	}
	return body, aws.ToString(out.ContentType), nil
}

// Put implements ObjectStore.
func (r *R2) Put(ctx context.Context, key, contentType string, body []byte, cacheControl string) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(r.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(body),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(int64(len(body))),
		CacheControl:  aws.String(cacheControl),
	})
	if err != nil {
		return "", fmt.Errorf("storage: r2 put: %w", err)
	}
	return r.baseURL + "/" + key, nil
}

// Delete implements ObjectStore.
func (r *R2) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if _, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(r.bucket), Key: aws.String(key)}); err != nil {
		return fmt.Errorf("storage: r2 delete: %w", err)
	}
	return nil
}

// validateKey rejects traversal and unexpected characters in object keys.
func validateKey(key string) error {
	if key == "" || len(key) > 512 || strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
		return fmt.Errorf("storage: invalid key")
	}
	for _, r := range key {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("/-_.", r)) {
			return fmt.Errorf("storage: invalid key")
		}
	}
	return nil
}

// Memory keeps objects in process memory.
type Memory struct {
	mu      sync.Mutex
	baseURL string
	Objects map[string][]byte
	Types   map[string]string
}

// NewMemory returns an in-memory store.
func NewMemory(baseURL string) *Memory {
	return &Memory{baseURL: baseURL, Objects: map[string][]byte{}, Types: map[string]string{}}
}

// Put implements ObjectStore.
func (m *Memory) Put(_ context.Context, key, contentType string, body []byte, _ string) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Objects[key] = append([]byte(nil), body...)
	m.Types[key] = contentType
	return m.baseURL + "/" + key, nil
}

// Delete implements ObjectStore.
func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Objects, key)
	delete(m.Types, key)
	return nil
}

// Get implements ObjectStore.
func (m *Memory) Get(_ context.Context, key string, maxBytes int64) ([]byte, string, error) {
	if err := validateKey(key); err != nil {
		return nil, "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.Objects[key]
	if !ok {
		return nil, "", ErrNotFound
	}
	if maxBytes > 0 && int64(len(b)) > maxBytes {
		return nil, "", ErrTooLarge
	}
	return append([]byte(nil), b...), m.Types[key], nil
}

// Lookup returns a stored object (tests).
func (m *Memory) Lookup(key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.Objects[key]
	return b, ok
}

// Keys lists stored object keys (tests).
func (m *Memory) Keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.Objects))
	for k := range m.Objects {
		out = append(out, k)
	}
	return out
}
