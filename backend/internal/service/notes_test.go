package service

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

func TestCleanNoteBody(t *testing.T) {
	cases := []struct {
		name, in, want string
		ok             bool
	}{
		{"plain", "Lost her dog", "Lost her dog", true},
		{"trims", "  spaced out \n", "spaced out", true},
		{"keeps newlines", "line one\nline two", "line one\nline two", true},
		{"strips control characters", "bell\u0007 here", "bell here", true},
		{"empty", "", "", false},
		{"whitespace only", " \t\n ", "", false},
		{"max length counts runes", strings.Repeat("é", domain.NoteMaxChars), strings.Repeat("é", domain.NoteMaxChars), true},
		{"too long", strings.Repeat("a", domain.NoteMaxChars+1), "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cleanNoteBody(tc.in)
			if tc.ok {
				if err != nil || got != tc.want {
					t.Fatalf("cleanNoteBody(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
				}
				return
			}
			e, isApp := apperror.As(err)
			if !isApp || e.Code != apperror.CodeValidation {
				t.Fatalf("cleanNoteBody(%q) error = %v; want validation", tc.in, err)
			}
		})
	}
}

func TestNoteSealIsBoundToRowAndAuthor(t *testing.T) {
	sealer, err := security.NewSealer([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	id, author := uuid.New(), uuid.New()
	sealed, err := sealer.Seal("Struggles with sleep", noteAD(id, author))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, "sleep") {
		t.Fatal("sealed body leaks plaintext")
	}
	if got, err := sealer.Open(sealed, noteAD(id, author)); err != nil || got != "Struggles with sleep" {
		t.Fatalf("open: %q %v", got, err)
	}
	// Copied to another row, or attributed to another advisor: does not open.
	if _, err := sealer.Open(sealed, noteAD(uuid.New(), author)); err == nil {
		t.Fatal("ciphertext opened under another note id")
	}
	if _, err := sealer.Open(sealed, noteAD(id, uuid.New())); err == nil {
		t.Fatal("ciphertext opened under another author")
	}
}

func TestNotePageLimit(t *testing.T) {
	for in, want := range map[int]int{-1: notePageDefault, 0: notePageDefault, 3: 3, notePageMax: notePageMax, notePageMax + 1: notePageMax} {
		if got := pageLimit(in); got != want {
			t.Errorf("pageLimit(%d) = %d, want %d", in, got, want)
		}
	}
}

// Validation runs before any repository access, so nil repositories are safe.
func TestNoteServiceValidatesBeforeTouchingStorage(t *testing.T) {
	svc := NewNoteService(nil, nil, nil, nil, nil, nil, "")
	p := reqctx.Principal{UserID: uuid.New(), Role: domain.RolePsychic}
	ctx := context.Background()

	if _, err := svc.Create(ctx, p, uuid.New(), NoteInput{Body: "   "}); !noteErrIs(err, apperror.CodeValidation) {
		t.Fatalf("create blank: %v", err)
	}
	if _, err := svc.Update(ctx, p, uuid.New(), NotePatch{}); !noteErrIs(err, apperror.CodeBadRequest) {
		t.Fatalf("empty patch: %v", err)
	}
	long := strings.Repeat("x", domain.NoteMaxChars+1)
	if _, err := svc.Update(ctx, p, uuid.New(), NotePatch{Body: &long}); !noteErrIs(err, apperror.CodeValidation) {
		t.Fatalf("long patch: %v", err)
	}
}

func noteErrIs(err error, code apperror.Code) bool {
	e, ok := apperror.As(err)
	return ok && e.Code == code
}
