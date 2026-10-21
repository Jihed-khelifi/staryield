package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

func solid(w, h int, c color.Color) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func codeOf(err error) apperror.Code {
	if e, ok := apperror.As(err); ok {
		return e.Code
	}
	return ""
}

func TestProcessImageSanitisesAndResizes(t *testing.T) {
	policy := ImagePolicy{MaxBytes: 2 << 20, MaxPixels: 40_000_000, MaxSide: 100, KeepTransparency: true, AllowGIF: true}

	// Opaque images become JPEG and are downscaled preserving the ratio.
	out, err := ProcessImage(encodePNG(t, solid(400, 200, color.RGBA{10, 20, 30, 255})), policy)
	if err != nil {
		t.Fatal(err)
	}
	if out.ContentType != "image/jpeg" || out.Width != 100 || out.Height != 50 || out.Extension() != ".jpg" {
		t.Fatalf("opaque: %+v", out)
	}
	if _, err := jpeg.Decode(bytes.NewReader(out.Data)); err != nil {
		t.Fatalf("output is not a JPEG: %v", err)
	}

	// Transparency is kept as PNG when allowed.
	out, err = ProcessImage(encodePNG(t, solid(50, 80, color.NRGBA{200, 0, 0, 100})), policy)
	if err != nil {
		t.Fatal(err)
	}
	if out.ContentType != "image/png" || out.Width != 50 || out.Height != 80 {
		t.Fatalf("transparent: %+v", out)
	}
	// ...and flattened to JPEG otherwise.
	noAlpha := policy
	noAlpha.KeepTransparency = false
	if out, err = ProcessImage(encodePNG(t, solid(50, 80, color.NRGBA{200, 0, 0, 100})), noAlpha); err != nil || out.ContentType != "image/jpeg" {
		t.Fatalf("flatten: %+v %v", out, err)
	}

	// GIFs keep their first frame only when allowed.
	var g bytes.Buffer
	pal := image.NewPaletted(image.Rect(0, 0, 30, 30), []color.Color{color.Black, color.White})
	if err := gif.EncodeAll(&g, &gif.GIF{Image: []*image.Paletted{pal, pal}, Delay: []int{10, 10}}); err != nil {
		t.Fatal(err)
	}
	if out, err = ProcessImage(g.Bytes(), policy); err != nil || out.Width != 30 {
		t.Fatalf("gif: %+v %v", out, err)
	}
	noGIF := policy
	noGIF.AllowGIF = false
	if _, err := ProcessImage(g.Bytes(), noGIF); codeOf(err) != apperror.CodeUnsupportedMedia {
		t.Fatalf("gif refused: %v", err)
	}

	// Refusals: not an image, too large, too many pixels, too small.
	if _, err := ProcessImage([]byte(strings.Repeat("<?php system($_GET['x']); ?>", 5)), policy); codeOf(err) != apperror.CodeUnsupportedMedia {
		t.Fatalf("script: %v", err)
	}
	if _, err := ProcessImage(make([]byte, 3<<20), policy); codeOf(err) != apperror.CodePayloadTooLarge {
		t.Fatalf("size: %v", err)
	}
	huge := policy
	huge.MaxPixels = 1000
	if _, err := ProcessImage(encodePNG(t, solid(100, 100, color.White)), huge); codeOf(err) != apperror.CodeValidation {
		t.Fatalf("pixels: %v", err)
	}
	if _, err := ProcessImage([]byte("\x89PNG"), policy); codeOf(err) != apperror.CodeValidation {
		t.Fatalf("tiny: %v", err)
	}
	// A truncated PNG header with valid magic is rejected, not decoded.
	bad := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 100)...)
	if _, err := ProcessImage(bad, policy); codeOf(err) != apperror.CodeValidation {
		t.Fatalf("corrupt: %v", err)
	}
	if w, h := fit(10, 3000, 100); w != 1 || h != 100 {
		t.Fatalf("fit tall: %d %d", w, h)
	}
}

func newGrowthForTest() *GrowthService {
	return NewGrowthService(nil, nil, nil, clock.Real{}, GrowthSettings{FrontendURL: "https://staryield.net",
		PublicBaseURL: "https://staryield.net/go", AllowedHosts: []string{"blog.staryield.net"}, Currency: "USD"})
}

func TestLinkDestinationsAndURLs(t *testing.T) {
	g := newGrowthForTest()
	for raw, want := range map[string]string{
		"/en/psychics":                    "/en/psychics",
		"/fr/psychics/theo?x=1#about":     "/fr/psychics/theo?x=1#about",
		"https://staryield.net/es":        "https://staryield.net/es",
		"https://blog.staryield.net/moon": "https://blog.staryield.net/moon",
		"https://evil.example/":           "",
		"http://staryield.net/":           "",
		"//evil.example/x":                "",
		"javascript:alert(1)":             "",
		"https://user:pw@staryield.net/":  "",
		"/en/psy chics":                   "",
		"/en\\x":                          "",
		"":                                "",
	} {
		got, ok := g.NormalizeDestination(raw)
		if (want == "") == ok || got != want {
			t.Errorf("NormalizeDestination(%q) = %q,%v want %q", raw, got, ok, want)
		}
	}
	l := &domain.ShortLink{Destination: "/en/psychics?ref=bio", UTMSource: "instagram", UTMMedium: "social", UTMCampaign: "spring", UTMContent: "story 1"}
	if got := g.FullURL(l); got != "https://staryield.net/en/psychics?ref=bio&utm_campaign=spring&utm_content=story+1&utm_medium=social&utm_source=instagram" {
		t.Fatalf("full url: %s", got)
	}
	if g.ShortURL("abc") != "https://staryield.net/go/abc" {
		t.Fatal("short url")
	}
	for _, code := range []string{"ab", "a b", "abc/def", strings.Repeat("a", 33), "<x>"} {
		if ValidLinkCode(code) {
			t.Errorf("code %q accepted", code)
		}
	}
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c := randomCode()
		if !ValidLinkCode(c) || len(c) != 7 || seen[c] {
			t.Fatalf("random code %q", c)
		}
		seen[c] = true
	}
}

func TestDeviceClassificationAndAttributionCleaning(t *testing.T) {
	for ua, want := range map[string]string{
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) Mobile/15E148 Instagram 350": "mobile",
		"Mozilla/5.0 (Linux; Android 15; Pixel 9) Mobile Safari/537.36":                      "mobile",
		"Mozilla/5.0 (Linux; Android 14; SM-X910) Safari/537.36":                             "tablet",
		"Mozilla/5.0 (iPad; CPU OS 18_0 like Mac OS X)":                                      "tablet",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 15_0) Safari/605.1.15":                       "desktop",
		"facebookexternalhit/1.1":                                                            "bot",
		"Twitterbot/1.0":                                                                     "bot",
		"WhatsApp/2.24.1 A":                                                                  "bot",
		"curl/8.7.1":                                                                         "bot",
		"":                                                                                   "bot",
		"Mozilla/5.0 (Linux; Android 15) [FBAN/FBIOS;FBAV/480.0] Mobile":                     "mobile",
	} {
		if got := ClassifyDevice(ua); got != want {
			t.Errorf("ClassifyDevice(%q) = %s, want %s", ua, got, want)
		}
	}
	if referrerHost("https://L.Instagram.com/path?u=secret") != "l.instagram.com" || referrerHost("not a url") != "" {
		t.Fatal("referrer host")
	}
	if cleanUTM(" instagram ") != "instagram" || cleanUTM("<script>") != "" || cleanUTM(strings.Repeat("a", 101)) != "" {
		t.Fatal("clean utm")
	}
}

func TestWorkingHoursLevels(t *testing.T) {
	h := workingHours([]repository.HourCell{
		{Weekday: 1, Hour: 20, Count: 9}, {Weekday: 5, Hour: 21, Count: 9}, {Weekday: 6, Hour: 22, Count: 5},
		{Weekday: 2, Hour: 9, Count: 3}, {Weekday: 3, Hour: 10, Count: 1}, {Weekday: 7, Hour: 11, Count: 1},
		{Weekday: 9, Hour: 3, Count: 4}, // invalid weekday ignored
	})
	if len(h.Grid) != 7 || len(h.Grid[0]) != 24 || h.Grid[0][20] != 9 || h.Grid[6][11] != 1 {
		t.Fatalf("grid: %v", h.Grid)
	}
	if h.Levels[0][20] != "peak" || h.Levels[4][21] != "peak" || h.Levels[5][22] != "medium" || h.Levels[1][9] != "medium" ||
		h.Levels[2][10] != "low" || h.Levels[0][0] != "none" {
		t.Fatalf("levels: mon20=%s fri21=%s sat22=%s tue9=%s wed10=%s", h.Levels[0][20], h.Levels[4][21], h.Levels[5][22], h.Levels[1][9], h.Levels[2][10])
	}
	if len(h.Best) != 3 || h.Best[0].Weekday != 0 || h.Best[0].Hour != 20 || h.Best[1].Weekday != 4 {
		t.Fatalf("best: %+v", h.Best)
	}
	empty := workingHours(nil)
	if len(empty.Best) != 0 || empty.Levels[3][3] != "none" {
		t.Fatal("empty grid")
	}
}

func TestSuccessScore(t *testing.T) {
	// A new advisor is scored on rating alone.
	s := computeScore(&repository.PsychicSessionStats{}, &repository.PsychicClientStats{}, 4.5, 10)
	if s.Value != 90 || s.Grade != "excellent" || s.Components.Acceptance != 0 {
		t.Fatalf("new advisor: %+v", s)
	}
	s = computeScore(&repository.PsychicSessionStats{Accepted: 8, Declined: 1, Expired: 1, Completed: 8, Interrupted: 2, AvgResponseSeconds: 45},
		&repository.PsychicClientStats{Unique: 4, Returning: 1}, 4.0, 20)
	// acceptance 80, rating 80, retention 25, completion 75, responsiveness 50
	// (80×25 + 80×30 + 25×15 + 75×15 + 50×15) / 100 = 66.5 → 67
	if s.Value != 67 {
		t.Fatalf("score %d, want 67 (%+v)", s.Value, s.Components)
	}
	if s.Grade != "fair" || s.Components.Responsiveness != 50 {
		t.Fatalf("grade: %+v", s)
	}
	if computeScore(&repository.PsychicSessionStats{}, &repository.PsychicClientStats{}, 0, 0).Value != 0 {
		t.Fatal("no data")
	}
}

func TestPayoutDestinationHelpers(t *testing.T) {
	if maskDestination("paypal", "luna@example.com") != "l•••@example.com" || maskDestination("bank_transfer", "FR76 3000 6000 0112 3456 7890 189") != "•••• 0189" {
		t.Fatal("mask")
	}
	if maskDestination("bank_transfer", "123") != "••••" {
		t.Fatal("short mask")
	}
	if _, msg := validDestination("paypal", "Luna <luna@example.com>"); msg == "" {
		t.Fatal("display-name email accepted")
	}
	if d, msg := validDestination("bank_transfer", "  FR76 3000 6000  "); msg != "" || d != "FR76 3000 6000" {
		t.Fatalf("iban: %q %q", d, msg)
	}
	if _, msg := validDestination("bank_transfer", "12"); msg == "" {
		t.Fatal("short iban accepted")
	}
}

func TestTimezoneAndLinksAndNames(t *testing.T) {
	for tz, ok := range map[string]bool{"": true, "UTC": true, "Europe/Paris": true, "America/Argentina/Buenos_Aires": true,
		"Local": false, "../../etc/passwd": false, "Not/AZone": false, "Europe/Paris; DROP": false} {
		_, _, err := ParseTimezone(tz)
		if (err == nil) != ok {
			t.Errorf("ParseTimezone(%q) err=%v", tz, err)
		}
	}
	for link, ok := range map[string]bool{"": true, "/psychics": true, "/chatroom?reader=theo&review=abc": true,
		"https://evil.example": false, "//evil.example": false, "/x\\y": false, "javascript:alert(1)": false} {
		if ValidAppLink(link) != ok {
			t.Errorf("ValidAppLink(%q)", link)
		}
	}
	fields := map[string]string{}
	m := cleanLocalized(map[string]string{"en": " Hello ", "fr": "", "de": "Hallo"}, "title", 10, fields)
	if m["en"] != "Hello" || m["fr"] != nil || fields["title.de"] == "" {
		t.Fatalf("localized: %v %v", m, fields)
	}
	if firstName("", "Luna Moon") != "Luna" || firstName("Star", "Luna") != "Star" || firstName("", "") != "Client" {
		t.Fatal("first name")
	}
	birth := time.Date(1990, 5, 17, 0, 0, 0, 0, time.UTC)
	if ageOn(birth, time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC)) != 35 || ageOn(birth, time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)) != 36 {
		t.Fatal("age")
	}
	a, b := uuid.New(), uuid.New()
	if got := intersect([]uuid.UUID{a, b}, []uuid.UUID{b}); len(got) != 1 || got[0] != b {
		t.Fatal("intersect")
	}
}

func TestRecoveryCodesAreSingleUse(t *testing.T) {
	codes, hashes, err := newRecoverySet()
	if err != nil || len(codes) != 10 || len(hashes) != 10 {
		t.Fatal(err)
	}
	mfa := &domain.AdminMFA{RecoveryHashes: hashes}
	if !consumeRecovery(mfa, strings.ToUpper(strings.ReplaceAll(codes[3], "-", " "))) || len(mfa.RecoveryHashes) != 9 {
		t.Fatal("valid code rejected")
	}
	if consumeRecovery(mfa, codes[3]) || consumeRecovery(mfa, "zzzzz-zzzzz") || consumeRecovery(mfa, "") {
		t.Fatal("reused or unknown code accepted")
	}
	for _, h := range mfa.RecoveryHashes {
		if h == security.HashToken("recovery:"+codes[3]) {
			t.Fatal("consumed hash still stored")
		}
	}
}

// fakeNotifications records inserted notifications.
type fakeNotifications struct {
	repository.NotificationRepository
	mu   sync.Mutex
	rows []domain.Notification
	fail bool
	slow chan struct{}
}

func (f *fakeNotifications) InsertBatch(_ context.Context, rows []domain.Notification) error {
	if f.slow != nil {
		<-f.slow
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return errors.New("db down")
	}
	f.rows = append(f.rows, rows...)
	return nil
}

func (f *fakeNotifications) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.rows)
}

type recordingPublisher struct {
	NoopPublisher
	mu    sync.Mutex
	users []uuid.UUID
}

func (p *recordingPublisher) Publish(_ context.Context, _ string, _ any, ids ...uuid.UUID) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.users = append(p.users, ids...)
	return nil
}

func TestDispatcherDeliversAsynchronouslyAndNeverBlocks(t *testing.T) {
	repo := &fakeNotifications{}
	pub := &recordingPublisher{}
	d := NewDispatcher(repo, pub, zap.NewNop(), clock.Real{}, DispatcherConfig{BufferSize: 10, BatchSize: 3, FlushInterval: 10 * time.Millisecond})
	d.Start()
	u1, u2 := uuid.New(), uuid.New()
	d.Notify(context.Background(), NotificationSpec{Kind: NotifyPurchase, Data: map[string]any{"credits": 10}}, u1, u2, u1, uuid.Nil)
	d.Notify(context.Background(), NotificationSpec{}, u1) // no kind: ignored
	deadline := time.Now().Add(2 * time.Second)
	for repo.count() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if repo.count() != 2 {
		t.Fatalf("delivered %d, want 2 (deduplicated, nil skipped)", repo.count())
	}
	d.Stop(context.Background())
	pub.mu.Lock()
	if len(pub.users) != 2 {
		t.Fatalf("published to %v", pub.users)
	}
	pub.mu.Unlock()

	// A stuck database fills the buffer; Notify still returns immediately
	// and drops the overflow.
	stuck := &fakeNotifications{slow: make(chan struct{})}
	d2 := NewDispatcher(stuck, &recordingPublisher{}, zap.NewNop(), clock.Real{}, DispatcherConfig{BufferSize: 2, BatchSize: 1, FlushInterval: time.Hour})
	d2.Start()
	start := time.Now()
	for i := 0; i < 50; i++ {
		d2.Notify(context.Background(), NotificationSpec{Kind: NotifyBroadcast}, uuid.New())
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatal("Notify blocked the caller")
	}
	if _, dropped := d2.Stats(); dropped == 0 {
		t.Fatal("overflow not dropped")
	}
	close(stuck.slow)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	d2.Stop(ctx)
}
