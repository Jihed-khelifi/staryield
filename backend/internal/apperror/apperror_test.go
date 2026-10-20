package apperror

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestFromAndIs(t *testing.T) {
	if From(nil) != nil {
		t.Fatal("nil")
	}
	wrapped := fmt.Errorf("layer: %w", NotFound("Session"))
	if e := From(wrapped); e.Code != CodeNotFound || e.Status != http.StatusNotFound {
		t.Fatalf("wrapped: %+v", e)
	}
	if !errors.Is(wrapped, ErrNotFound) {
		t.Fatal("errors.Is by code")
	}
	if e := From(context.DeadlineExceeded); e.Code != CodeTimeout {
		t.Fatalf("deadline: %+v", e)
	}
	if e := From(errors.New("raw")); e.Code != CodeInternal || e.Unwrap() == nil {
		t.Fatalf("internal: %+v", e)
	}
	base := InsufficientCredits(40, 10)
	d := base.WithDetails(map[string]any{"x": 1})
	if base.Details["x"] != nil || d.Details["required"] != int64(40) || d.Details["x"] != 1 {
		t.Fatal("WithDetails must copy")
	}
}
