package service

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

func TestNormalizePhone(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		ok       bool
	}{
		{"", "", true},
		{"   ", "", true},
		{"+14155552671", "+14155552671", true},
		{"+44 20 7946-0958", "+442079460958", true},
		{" +33 (6) 12.34.56.78 ", "+33612345678", true},
		{"+216 66 669 696", "+21666669696", true},
		{"0044 20 7946 0958", "", false},
		{"4155552671", "", false},
		{"+0123456789", "", false},
		{"+12345", "", false},                // too short
		{"+1234567890123456", "", false},     // 16 digits
		{"+1415555abcd", "", false},          // letters
		{"+1 415 555 2671 ext 2", "", false}, // extensions are not E.164
		{"++14155552671", "", false},
	} {
		got, ok := NormalizePhone(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("NormalizePhone(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestMaskPhone(t *testing.T) {
	if got := MaskPhone("+14155552671"); got != "+•••••••••71" {
		t.Errorf("mask: %q", got)
	}
	if strings.Contains(MaskPhone("+442079460958"), "2079") {
		t.Error("mask leaks digits")
	}
	for _, bad := range []string{"", "+1", "14155552671"} {
		if got := MaskPhone(bad); got != "" {
			t.Errorf("MaskPhone(%q) = %q", bad, got)
		}
	}
}

func TestPhoneSealIsBoundToTheAdvisor(t *testing.T) {
	sealer, err := security.NewSealer([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	a, b := uuid.New(), uuid.New()
	sealed, err := sealer.Seal("+14155552671", phoneAD(a))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, "4155552671") {
		t.Fatal("phone stored in clear")
	}
	if got, err := sealer.Open(sealed, phoneAD(a)); err != nil || got != "+14155552671" {
		t.Fatalf("open own: %q %v", got, err)
	}
	if _, err := sealer.Open(sealed, phoneAD(b)); err == nil {
		t.Fatal("a phone copied to another advisor opened")
	}
}

func TestProfileChangesDeduplicatesAndBoundsSkills(t *testing.T) {
	skills := []string{" Tarot ", "tarot", "TAROT", "Astrology", "  Aura   reading "}
	upd, fields := profileChanges(PsychicProfileUpdate{Skills: &skills})
	if len(fields) > 0 {
		t.Fatalf("fields: %v", fields)
	}
	got := upd["skills"].(domain.StringList)
	if strings.Join(got, "|") != "Tarot|Astrology|Aura reading" {
		t.Fatalf("skills: %q", got)
	}

	// Duplicates do not count towards the limit of eight.
	many := []string{"a", "b", "c", "d", "e", "f", "g", "h", "A", "B"}
	if _, fields := profileChanges(PsychicProfileUpdate{Skills: &many}); len(fields) > 0 {
		t.Fatalf("deduplicated list rejected: %v", fields)
	}
	nine := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
	if _, fields := profileChanges(PsychicProfileUpdate{Skills: &nine}); fields["skills"] != "Use at most 8 skills." {
		t.Fatalf("nine skills: %v", fields)
	}
	long := []string{strings.Repeat("é", 41)}
	if _, fields := profileChanges(PsychicProfileUpdate{Skills: &long}); fields["skills"] == "" {
		t.Fatal("41-character skill accepted")
	}
	blank := []string{"Tarot", "   "}
	if _, fields := profileChanges(PsychicProfileUpdate{Skills: &blank}); fields["skills"] == "" {
		t.Fatal("blank skill accepted")
	}
	ok40 := []string{strings.Repeat("é", 40)}
	if _, fields := profileChanges(PsychicProfileUpdate{Skills: &ok40}); len(fields) > 0 {
		t.Fatalf("40-character skill rejected: %v", fields)
	}
}

func TestProfileChangesTextLimits(t *testing.T) {
	headline := strings.Repeat("h", 201)
	philosophy := strings.Repeat("p", 601)
	about := []string{"one", "two", "three", "four", "five", "six", "seven"}
	_, fields := profileChanges(PsychicProfileUpdate{Headline: &headline, Philosophy: &philosophy, About: &about})
	if fields["headline"] == "" || fields["philosophy"] == "" || fields["about"] == "" {
		t.Fatalf("limits not enforced: %v", fields)
	}
	paragraph := []string{strings.Repeat("x", 2001)}
	if _, fields := profileChanges(PsychicProfileUpdate{About: &paragraph}); fields["about"] == "" {
		t.Fatal("2001-character paragraph accepted")
	}
	// Control characters are stripped and blank paragraphs dropped.
	text := []string{"First​ paragraph.", "   ", "Second\x00 one."}
	upd, fields := profileChanges(PsychicProfileUpdate{About: &text})
	if len(fields) > 0 {
		t.Fatalf("fields: %v", fields)
	}
	if got := upd["about"].(domain.StringList); strings.Join(got, "|") != "First paragraph.|Second one." {
		t.Fatalf("about: %q", got)
	}
}

func TestValidCustomService(t *testing.T) {
	name, desc, price, days := "  Soulmate   sketch ", " A portrait of your match. ", int64(2000), 8
	n, d, p, dd, fields := validCustomService(CustomServiceInput{Name: &name, Description: &desc, PriceCredits: &price, DeliveryDays: &days})
	if len(fields) > 0 || n != "Soulmate sketch" || d != "A portrait of your match." || p != 2000 || dd != 8 {
		t.Fatalf("valid input: %q %q %d %d %v", n, d, p, dd, fields)
	}

	_, _, _, _, fields = validCustomService(CustomServiceInput{})
	for _, f := range []string{"name", "price_credits", "delivery_days"} {
		if fields[f] != "This field is required." {
			t.Errorf("%s: %q", f, fields[f])
		}
	}
	if _, ok := fields["description"]; ok {
		t.Error("description must be optional")
	}

	for _, tc := range []struct {
		field string
		in    CustomServiceInput
	}{
		{"name", CustomServiceInput{Name: ptr("   "), PriceCredits: ptr(int64(100)), DeliveryDays: ptr(3)}},
		{"name", CustomServiceInput{Name: ptr(strings.Repeat("n", 81)), PriceCredits: ptr(int64(100)), DeliveryDays: ptr(3)}},
		{"price_credits", CustomServiceInput{Name: ptr("x"), PriceCredits: ptr(int64(9)), DeliveryDays: ptr(3)}},
		{"price_credits", CustomServiceInput{Name: ptr("x"), PriceCredits: ptr(int64(50_001)), DeliveryDays: ptr(3)}},
		{"delivery_days", CustomServiceInput{Name: ptr("x"), PriceCredits: ptr(int64(100)), DeliveryDays: ptr(0)}},
		{"delivery_days", CustomServiceInput{Name: ptr("x"), PriceCredits: ptr(int64(100)), DeliveryDays: ptr(61)}},
		{"description", CustomServiceInput{Name: ptr("x"), PriceCredits: ptr(int64(100)), DeliveryDays: ptr(3),
			Description: ptr(strings.Repeat("d", 1001))}},
	} {
		if _, _, _, _, fields := validCustomService(tc.in); fields[tc.field] == "" {
			t.Errorf("%s accepted: %+v", tc.field, fields)
		}
	}
	// The bounds themselves are accepted.
	for _, in := range []CustomServiceInput{
		{Name: ptr(strings.Repeat("n", 80)), PriceCredits: ptr(int64(10)), DeliveryDays: ptr(1)},
		{Name: ptr("x"), PriceCredits: ptr(int64(50_000)), DeliveryDays: ptr(60), Description: ptr(strings.Repeat("d", 1000))},
	} {
		if _, _, _, _, fields := validCustomService(in); len(fields) > 0 {
			t.Errorf("bounds rejected: %v", fields)
		}
	}
}

func TestPhotoViewsNeverExposeTheStorageKey(t *testing.T) {
	sub := &domain.PhotoSubmission{ID: uuid.New(), PsychicID: uuid.New(), Status: domain.PhotoSubmissionPending,
		ObjectKey: "advisor-photos/x/secret.jpg", ContentType: "image/jpeg", Bytes: 10, Width: 300, Height: 300}
	v := photoView(sub)
	if !v.HasImage {
		t.Fatal("pending image not previewable")
	}
	av := adminPhotoView(sub, AdminPhotoPsychic{})
	if !av.HasImage {
		t.Fatal("admin preview not available")
	}
	sub.Status = domain.PhotoSubmissionApproved
	if photoView(sub).HasImage || adminPhotoView(sub, AdminPhotoPsychic{}).HasImage {
		t.Fatal("approved submissions are served from the public photo, not the private copy")
	}
}
