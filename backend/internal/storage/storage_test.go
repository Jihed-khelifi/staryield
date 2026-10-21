package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
)

func TestValidateKey(t *testing.T) {
	for key, ok := range map[string]bool{
		"avatars/u/abc.jpg": true, "../etc/passwd": false, "/abs": false, "a b": false, "a\x00b": false, "": false, "x?y": false,
	} {
		if (validateKey(key) == nil) != ok {
			t.Errorf("%q", key)
		}
	}
}

func TestMemoryStore(t *testing.T) {
	m := NewMemory("https://cdn.test")
	url, err := m.Put(context.Background(), "avatars/a.jpg", "image/jpeg", []byte("x"), "public")
	if err != nil || url != "https://cdn.test/avatars/a.jpg" {
		t.Fatalf("put: %s %v", url, err)
	}
	if _, ok := m.Lookup("avatars/a.jpg"); !ok {
		t.Fatal("stored object missing")
	}
	_ = m.Delete(context.Background(), "avatars/a.jpg")
	if _, ok := m.Lookup("avatars/a.jpg"); ok {
		t.Fatal("delete failed")
	}
	if _, err := m.Put(context.Background(), "../x", "", nil, ""); err == nil {
		t.Fatal("traversal key accepted")
	}
}

func TestDisabledStore(t *testing.T) {
	store, err := New(config.StorageConfig{Driver: "disabled"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(context.Background(), "avatars/u/a.jpg", "image/jpeg", []byte("x"), ""); !errors.Is(err, ErrDisabled) {
		t.Fatalf("put: %v", err)
	}
	if err := store.Delete(context.Background(), "avatars/u/a.jpg"); err != nil {
		t.Fatalf("delete must be a no-op: %v", err)
	}
}

func TestMemoryGetLimitsAndMissing(t *testing.T) {
	m := NewMemory("")
	ctx := context.Background()
	if _, err := m.Put(ctx, "chat/c/a.png", "image/png", []byte("12345"), "private"); err != nil {
		t.Fatal(err)
	}
	body, ct, err := m.Get(ctx, "chat/c/a.png", 10)
	if err != nil || string(body) != "12345" || ct != "image/png" {
		t.Fatalf("get: %q %q %v", body, ct, err)
	}
	if _, _, err := m.Get(ctx, "chat/c/a.png", 4); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("limit: %v", err)
	}
	if _, _, err := m.Get(ctx, "chat/c/missing.png", 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if _, _, err := m.Get(ctx, "../secret", 0); err == nil {
		t.Fatal("traversal key read")
	}
}

func TestBucketSelection(t *testing.T) {
	// The r2 driver with no bucket configured disables that store instead of
	// writing into another bucket.
	store, err := NewBucket(config.StorageConfig{Driver: "r2"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.(Disabled); !ok {
		t.Fatalf("expected Disabled, got %T", store)
	}
	if _, _, err := store.Get(context.Background(), "a/b.jpg", 0); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled get: %v", err)
	}
	mem, err := NewBucket(config.StorageConfig{Driver: "memory"}, "advisors", "https://advisors.test")
	if err != nil {
		t.Fatal(err)
	}
	url, _ := mem.Put(context.Background(), "advisors/x.jpg", "image/jpeg", []byte("x"), "")
	if url != "https://advisors.test/advisors/x.jpg" {
		t.Fatalf("url %q", url)
	}
	if _, err := NewBucket(config.StorageConfig{Driver: "ftp"}, "x", ""); err == nil {
		t.Fatal("unknown driver accepted")
	}
}
