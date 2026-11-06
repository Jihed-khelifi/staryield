package service

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

func TestSupportReferenceFormat(t *testing.T) {
	re := regexp.MustCompile(`^SY-[A-HJ-NP-Z2-9]{6}$`)
	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		ref, err := newSupportReference()
		if err != nil {
			t.Fatal(err)
		}
		if !re.MatchString(ref) {
			t.Fatalf("bad reference %q", ref)
		}
		seen[ref] = true
	}
	if len(seen) < 495 {
		t.Fatalf("references repeat too often: %d unique of 500", len(seen))
	}
}

func TestValidateTicket(t *testing.T) {
	ok := TicketInput{Name: "  Luna  ", Email: " Luna@Example.com ", Topic: "Privacy", Subject: "Delete my data", Message: "Please erase my account and chats.\u0007", Consent: true}
	got, err := validateTicket(ok)
	if err != nil {
		t.Fatalf("valid ticket rejected: %v", err)
	}
	if got.name != "Luna" || got.email != "luna@example.com" || got.topic != domain.SupportPrivacy || got.message != "Please erase my account and chats." {
		t.Fatalf("normalised: %+v", got)
	}
	// Empty topic falls back to general.
	if got, err := validateTicket(TicketInput{Email: "a@b.co", Subject: "Hi there", Message: "Something happened here.", Consent: true}); err != nil || got.topic != domain.SupportGeneral {
		t.Fatalf("default topic: %+v %v", got, err)
	}

	bad := TicketInput{Name: strings.Repeat("n", 81), Email: "not-an-email", Topic: "refunds", Subject: "Hi", Message: "too short", Consent: false}
	_, err = validateTicket(bad)
	e, isApp := apperror.As(err)
	if !isApp || e.Code != apperror.CodeValidation {
		t.Fatalf("want validation error, got %v", err)
	}
	fields, _ := e.Details["fields"].(map[string]any)
	for _, f := range []string{"name", "email", "topic", "subject", "message", "consent"} {
		if msg, _ := fields[f].(string); msg == "" {
			t.Errorf("missing field error %q in %v", f, fields)
		}
	}
	// Length counts characters, not bytes.
	if _, err := validateTicket(TicketInput{Email: "a@b.co", Subject: "Été", Message: strings.Repeat("é", 5000), Consent: true}); err != nil {
		t.Fatalf("5000 accented characters should be accepted: %v", err)
	}
	if _, err := validateTicket(TicketInput{Email: "a@b.co", Subject: "Été", Message: strings.Repeat("é", 5001), Consent: true}); err == nil {
		t.Fatal("5001 characters accepted")
	}
}
