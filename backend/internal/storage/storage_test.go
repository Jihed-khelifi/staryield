package storage

import (
	"context"
	"testing"
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
	if _, ok := m.Get("avatars/a.jpg"); !ok {
		t.Fatal("stored object missing")
	}
	_ = m.Delete(context.Background(), "avatars/a.jpg")
	if _, ok := m.Get("avatars/a.jpg"); ok {
		t.Fatal("delete failed")
	}
	if _, err := m.Put(context.Background(), "../x", "", nil, ""); err == nil {
		t.Fatal("traversal key accepted")
	}
}
