// Package storage stores user-uploaded objects (avatars) in Cloudflare R2
// through its S3-compatible API, with an in-memory driver for development
// and tests and a disabled driver for deployments without object storage.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
)

// ObjectStore uploads and deletes public objects.
type ObjectStore interface {
	Put(ctx context.Context, key, contentType string, body []byte, cacheControl string) (publicURL string, err error)
	Delete(ctx context.Context, key string) error
}

// New builds the configured store.
func New(cfg config.StorageConfig) (ObjectStore, error) {
	switch cfg.Driver {
	case "r2":
		return NewR2(cfg)
	case "memory":
		return NewMemory(cfg.PublicBaseURL), nil
	case "disabled":
		return Disabled{}, nil
	}
	return nil, fmt.Errorf("storage: unknown driver %q", cfg.Driver)
}

// ErrDisabled is returned by uploads when object storage is not configured.
var ErrDisabled = errors.New("storage: uploads are disabled")

// Disabled rejects uploads. Deletes succeed because nothing was ever stored.
type Disabled struct{}

// Put implements ObjectStore.
func (Disabled) Put(context.Context, string, string, []byte, string) (string, error) {
	return "", ErrDisabled
}

// Delete implements ObjectStore.
func (Disabled) Delete(context.Context, string) error { return nil }

// R2 is a Cloudflare R2 bucket accessed with scoped API-token credentials.
type R2 struct {
	client  *s3.Client
	bucket  string
	baseURL string
}

// NewR2 returns an R2 store.
func NewR2(cfg config.StorageConfig) (*R2, error) {
	client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(cfg.R2Endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.R2AccessKeyID, cfg.R2SecretKey, ""),
		UsePathStyle: true,
	})
	return &R2{client: client, bucket: cfg.R2Bucket, baseURL: cfg.PublicBaseURL}, nil
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

// Get returns a stored object (tests).
func (m *Memory) Get(key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.Objects[key]
	return b, ok
}
