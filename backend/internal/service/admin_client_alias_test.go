package service

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
)

func TestAdminClientAliasReasonIsRequired(t *testing.T) {
	// The reason is validated before anything is read, so a bare service is enough.
	s := &AdminService{}
	for name, reason := range map[string]string{"empty": "", "blank": "  \t ", "too long": strings.Repeat("a", 301)} {
		_, err := s.ClientAlias(context.Background(), uuid.New(), reason)
		e, ok := apperror.As(err)
		if !ok || e.Code != apperror.CodeValidation {
			t.Fatalf("%s: error = %v; want validation", name, err)
		}
	}
}

func TestAdminClientAliasIsOffWithoutTheAliasStore(t *testing.T) {
	s := &AdminService{chat: &ChatService{}}
	_, err := s.ClientAlias(context.Background(), uuid.New(), "Review")
	e, ok := apperror.As(err)
	if !ok || e.Status != http.StatusServiceUnavailable {
		t.Fatalf("error = %v; want 503", err)
	}
}
