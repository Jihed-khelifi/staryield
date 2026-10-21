package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/seed"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
	"go.uber.org/zap"
)

// jpegWithExif builds a JPEG carrying an EXIF APP1 segment with a GPS tag
// marker, as phone cameras produce.
func jpegWithExif(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 3), uint8(y * 5), 200, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	raw := buf.Bytes()
	payload := append([]byte("Exif\x00\x00"), []byte("GPSLatitude=48.8566;GPSLongitude=2.3522;SecretCameraSerial")...)
	seg := []byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte((len(payload) + 2) & 0xff)}
	out := append([]byte{}, raw[:2]...) // SOI
	out = append(out, seg...)
	out = append(out, payload...)
	return append(out, raw[2:]...)
}

func imageForm(t *testing.T, data []byte, fields map[string]string) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	fw, _ := w.CreateFormFile("image", "photo.jpg")
	_, _ = fw.Write(data)
	_ = w.Close()
	return &buf, w.FormDataContentType()
}

func (c *client) sendImage(sessionID string, data []byte, fields map[string]string) response {
	c.e.t.Helper()
	body, ct := imageForm(c.e.t, data, fields)
	return c.raw(http.MethodPost, "/api/v1/chat/sessions/"+sessionID+"/attachments", body, map[string]string{"Content-Type": ct, "X-CSRF-Token": csrfOf(c)})
}

func TestChatImagesArePrivateAndSanitised(t *testing.T) {
	e := newEnv(t)
	cl, psychic, cws, pws, s := startedSession(t, e, 10, 250)
	sid := s.ID.String()

	// Multi-line text keeps its formatting.
	text := "Line one\n\n  indented line   with spaces\nhttps://staryield.net/en ✨"
	var sent struct {
		Message service.MessageView `json:"message"`
	}
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions/"+sid+"/messages", map[string]any{"body": text, "client_msg_id": uniq("m-")}, &sent)
	if sent.Message.Body != text {
		t.Fatalf("formatting changed: %q", sent.Message.Body)
	}

	// Image upload with caption: EXIF/GPS removed, both participants notified.
	r := cl.sendImage(sid, jpegWithExif(t, 64, 48), map[string]string{"caption": "My palm\nleft hand", "client_msg_id": "img-123456"})
	if r.status != http.StatusCreated {
		t.Fatalf("upload: %d %s", r.status, r.body)
	}
	var up struct {
		Message service.MessageView `json:"message"`
	}
	r.decode(t, &up)
	m := up.Message
	if m.Kind != "image" || m.Body != "My palm\nleft hand" || len(m.Attachments) != 1 || m.Attachments[0].Width != 64 || m.Attachments[0].ContentType != "image/jpeg" {
		t.Fatalf("image message: %+v", m)
	}
	att := m.Attachments[0].ID.String()
	for _, w := range []*ws{cws, pws} {
		ev := w.waitMatch(realtime.EventMessageCreated, att, 5*time.Second)
		if !strings.Contains(string(ev.Data), `"attachments"`) {
			t.Fatalf("event without attachments: %s", ev.Data)
		}
	}
	// Retrying with the same client id returns the same message (no new object).
	keys := len(e.chat.Keys())
	r2 := cl.sendImage(sid, jpegWithExif(t, 64, 48), map[string]string{"client_msg_id": "img-123456"})
	var again struct {
		Message service.MessageView `json:"message"`
	}
	r2.decode(t, &again)
	if again.Message.ID != m.ID || len(e.chat.Keys()) != keys {
		t.Fatalf("idempotent retry stored a new image: %+v", again.Message)
	}

	// Sender and receiver can open it; the stored bytes have no EXIF.
	for _, who := range []*client{cl, psychic} {
		res := who.raw(http.MethodGet, "/api/v1/chat/attachments/"+att, nil, nil)
		if res.status != http.StatusOK || res.header.Get("Content-Type") != "image/jpeg" {
			t.Fatalf("participant fetch: %d %s", res.status, res.header.Get("Content-Type"))
		}
		if bytes.Contains(res.body, []byte("GPSLatitude")) || bytes.Contains(res.body, []byte("Exif")) || bytes.Contains(res.body, []byte("SecretCameraSerial")) {
			t.Fatal("metadata survived re-encoding")
		}
		if !strings.Contains(res.header.Get("Cache-Control"), "private") {
			t.Fatalf("cache control: %s", res.header.Get("Cache-Control"))
		}
	}
	// Nobody else can: another client, another advisor, an admin's web session.
	stranger := e.registerClient()
	if res := stranger.raw(http.MethodGet, "/api/v1/chat/attachments/"+att, nil, nil); res.status != http.StatusNotFound {
		t.Fatalf("stranger fetch: %d", res.status)
	}
	other := e.createPsychic(10)
	if res := other.raw(http.MethodGet, "/api/v1/chat/attachments/"+att, nil, nil); res.status != http.StatusNotFound {
		t.Fatalf("other advisor fetch: %d", res.status)
	}
	_, adminEmail := e.insertAdmin()
	adminWeb := e.login(adminEmail, strongPassword)
	if res := adminWeb.raw(http.MethodGet, "/api/v1/chat/attachments/"+att, nil, nil); res.status != http.StatusNotFound {
		t.Fatalf("admin fetch: %d", res.status)
	}
	anon := e.client()
	if res := anon.raw(http.MethodGet, "/api/v1/chat/attachments/"+att, nil, nil); res.status != http.StatusUnauthorized {
		t.Fatalf("anonymous fetch: %d", res.status)
	}

	// Only images are accepted.
	if r := cl.sendImage(sid, []byte(strings.Repeat("%PDF-1.7 not an image ", 20)), nil); r.status != http.StatusUnsupportedMediaType {
		t.Fatalf("pdf upload: %d %s", r.status, r.body)
	}
	if r := cl.sendImage(sid, pngBytes(t, 20000, 1), nil); r.errCode(t) != "validation_failed" {
		t.Fatalf("oversized dimensions: %d %s", r.status, r.body)
	}

	// History includes attachment metadata; images can be bookmarked.
	var page service.Page[service.MessageView]
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/chat/conversations/"+s.ConversationID.String()+"/messages", nil, &page)
	found := false
	for _, msg := range page.Items {
		if msg.ID == m.ID && len(msg.Attachments) == 1 {
			found = true
		}
	}
	if !found {
		t.Fatal("history lacks image metadata")
	}
	cl.must(http.StatusNoContent, http.MethodPut, "/api/v1/chat/saved/"+m.ID.String(), nil, nil)

	// After the session ends, uploads are refused.
	cl.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+sid+"/end", nil, nil)
	if r := cl.sendImage(sid, pngBytes(t, 32, 32), nil); r.errCode(t) != "invalid_state" {
		t.Fatalf("upload after end: %d %s", r.status, r.body)
	}
}

func TestPsychicSeesClientProfileAndPresence(t *testing.T) {
	e := newEnv(t)
	client, psychic, cws, pws, s := startedSession(t, e, 10, 250)
	client.must(http.StatusOK, http.MethodPatch, "/api/v1/me/profile", map[string]any{"nickname": "Luna", "birth_time": "14:30",
		"birth_place_id": "paris-fr"}, nil)

	var prof service.ClientProfileView
	r := psychic.must(http.StatusOK, http.MethodGet, "/api/v1/chat/conversations/"+s.ConversationID.String()+"/client", nil, &prof)
	if prof.Name != "Luna" || !prof.Online || prof.CountryCode != "FR" || prof.Age == nil || *prof.Age < 30 || prof.Astro == nil ||
		prof.Astro.SunSign != "Taurus" || prof.Astro.MoonSign == "" || prof.Astro.RisingSign == "" || prof.Horoscope == nil || prof.History.Sessions != 1 {
		t.Fatalf("client profile: %+v astro=%+v", prof, prof.Astro)
	}
	// Never sensitive: no email, balance or birth place/time.
	for _, banned := range []string{client.user.Email, "balance", "wallet", "birth_place", "14:30", "free_minutes"} {
		if strings.Contains(string(r.body), banned) {
			t.Fatalf("client profile leaks %q: %s", banned, r.body)
		}
	}
	// Clients and other advisors cannot read it.
	if r := client.call(http.MethodGet, "/api/v1/chat/conversations/"+s.ConversationID.String()+"/client", nil); r.status != http.StatusForbidden {
		t.Fatalf("client reading card: %d", r.status)
	}
	other := e.createPsychic(10)
	if r := other.call(http.MethodGet, "/api/v1/chat/conversations/"+s.ConversationID.String()+"/client", nil); r.status != http.StatusNotFound {
		t.Fatalf("other advisor reading card: %d", r.status)
	}

	// The inbox shows the client online; disconnecting is pushed to the advisor.
	var inbox struct {
		Items []service.ConversationView `json:"items"`
	}
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/chat/conversations", nil, &inbox)
	if len(inbox.Items) != 1 || inbox.Items[0].Counterpart.Status != "online" {
		t.Fatalf("inbox presence: %+v", inbox.Items)
	}
	cws.conn.Close()
	ev := pws.waitFor(service.EventPresenceUpdated, 5*time.Second)
	var p struct {
		UserID string `json:"user_id"`
		Online bool   `json:"online"`
	}
	_ = json.Unmarshal(ev.Data, &p)
	if p.UserID != client.user.ID.String() || p.Online {
		t.Fatalf("presence event: %s", ev.Data)
	}
}

func TestClientRatesAdvisorAfterReading(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	client, psychic, cws, pws, s := startedSession(t, e, 10, 250)
	sid := s.ID.String()

	// Not before the reading ends.
	if r := client.call(http.MethodPost, "/api/v1/chat/sessions/"+sid+"/review", map[string]any{"rating": 5}); r.errCode(t) != "invalid_state" {
		t.Fatalf("review during session: %s", r.body)
	}
	var sum service.SessionSummary
	client.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+sid+"/end", nil, &sum)
	cws.drain()
	pws.drain()
	client.must(http.StatusOK, http.MethodGet, "/api/v1/chat/sessions/"+sid+"/summary", nil, &sum)
	if !sum.CanReview || sum.Review != nil {
		t.Fatalf("summary review state: %+v", sum)
	}
	// The client is invited to rate (notification).
	waitNotification(t, client, service.NotifyReviewPrompt)

	var before service.PsychicView
	e.client().must(http.StatusOK, http.MethodGet, "/api/v1/psychics/"+psychic.slug(), nil, &before)

	if r := client.call(http.MethodPost, "/api/v1/chat/sessions/"+sid+"/review", map[string]any{"rating": 6}); r.errCode(t) != "validation_failed" {
		t.Fatalf("rating 6: %s", r.body)
	}
	if r := psychic.call(http.MethodPost, "/api/v1/chat/sessions/"+sid+"/review", map[string]any{"rating": 5}); r.status != http.StatusForbidden {
		t.Fatalf("advisor rating self: %d", r.status)
	}
	stranger := e.registerClient()
	if r := stranger.call(http.MethodPost, "/api/v1/chat/sessions/"+sid+"/review", map[string]any{"rating": 1}); r.status != http.StatusNotFound {
		t.Fatalf("stranger review: %d", r.status)
	}
	var rv service.ReviewView
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions/"+sid+"/review", map[string]any{"rating": 4, "comment": "Warm and insightful.\nThank you!"}, &rv)
	if rv.Rating != 4 || rv.Author.Name == "" || strings.Contains(rv.Author.Name, "@") {
		t.Fatalf("review: %+v", rv)
	}
	if r := client.call(http.MethodPost, "/api/v1/chat/sessions/"+sid+"/review", map[string]any{"rating": 5}); r.status != http.StatusConflict {
		t.Fatalf("second review: %d", r.status)
	}
	client.must(http.StatusOK, http.MethodGet, "/api/v1/chat/sessions/"+sid+"/summary", nil, &sum)
	if sum.CanReview || sum.Review == nil || sum.Review.Rating != 4 {
		t.Fatalf("summary after review: %+v", sum)
	}

	var after service.PsychicView
	e.client().must(http.StatusOK, http.MethodGet, "/api/v1/psychics/"+psychic.slug(), nil, &after)
	if after.ReviewsCount != before.ReviewsCount+1 {
		t.Fatalf("review count %d -> %d", before.ReviewsCount, after.ReviewsCount)
	}
	// New advisors start at 5.0 with 0 reviews: one 4-star review → 4.0.
	if before.ReviewsCount == 0 && after.Rating != 4 {
		t.Fatalf("rating: %v", after.Rating)
	}
	var pub service.Page[service.ReviewView]
	e.client().must(http.StatusOK, http.MethodGet, "/api/v1/psychics/"+psychic.slug()+"/reviews", nil, &pub)
	if len(pub.Items) != 1 || pub.Items[0].Comment != "Warm and insightful.\nThank you!" || pub.Items[0].SessionID != nil {
		t.Fatalf("public reviews: %+v", pub.Items)
	}
	waitNotification(t, psychic, service.NotifyReviewReceived)

	// Moderation hides it and recomputes the rating.
	var page service.Page[service.AdminReviewView]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/reviews?psychic_id="+after.ID.String(), nil, &page)
	if len(page.Items) != 1 || page.Items[0].Psychic.Slug != psychic.slug() {
		t.Fatalf("admin reviews: %+v", page.Items)
	}
	var mod service.AdminReviewView
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/reviews/"+rv.ID.String(), map[string]any{"status": "hidden", "note": "Contains contact details"}, &mod)
	if mod.Status != "hidden" || mod.ModerationNote == "" {
		t.Fatalf("moderated: %+v", mod)
	}
	e.client().must(http.StatusOK, http.MethodGet, "/api/v1/psychics/"+psychic.slug()+"/reviews", nil, &pub)
	if len(pub.Items) != 0 {
		t.Fatal("hidden review still public")
	}
	var restored service.PsychicView
	e.client().must(http.StatusOK, http.MethodGet, "/api/v1/psychics/"+psychic.slug(), nil, &restored)
	if restored.ReviewsCount != before.ReviewsCount || restored.Rating != before.Rating {
		t.Fatalf("rating not restored: %+v vs %+v", restored.Rating, before.Rating)
	}
}

// Seeded advisors carry a historical average (rating = rating_sum /
// reviews_count): a new review nudges it instead of resetting it.
func TestSeededAdvisorRatingSurvivesNewReview(t *testing.T) {
	e := newEnv(t)
	if err := seed.Run(context.Background(), e.app.DB, security.NewPasswordHasher(1024, 1, 1),
		seed.Options{PsychicPassword: strongPassword}, zap.NewNop()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var before service.PsychicView
	e.client().must(http.StatusOK, http.MethodGet, "/api/v1/psychics/theo", nil, &before)
	if before.ReviewsCount == 0 || before.Rating < 4 {
		t.Fatalf("seeded advisor: rating %v from %d reviews", before.Rating, before.ReviewsCount)
	}

	psychic := e.login("theo@advisors.staryield.net", strongPassword)
	client, _, cws, pws, s := startedSessionWith(t, e, psychic, 250)
	sid := s.ID.String()
	client.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+sid+"/end", nil, nil)
	cws.drain()
	pws.drain()
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions/"+sid+"/review", map[string]any{"rating": 1}, nil)

	var after service.PsychicView
	e.client().must(http.StatusOK, http.MethodGet, "/api/v1/psychics/theo", nil, &after)
	n := float64(before.ReviewsCount)
	want := math.Round((before.Rating*n+1)/(n+1)*10) / 10
	if after.ReviewsCount != before.ReviewsCount+1 || math.Abs(after.Rating-want) > 0.11 {
		t.Fatalf("rating %v (%d reviews) -> %v (%d reviews), want about %v",
			before.Rating, before.ReviewsCount, after.Rating, after.ReviewsCount, want)
	}
}

// waitNotification polls until the user has a notification of kind.
func waitNotification(t *testing.T, c *client, kind string) service.NotificationView {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var page service.Page[service.NotificationView]
		c.must(http.StatusOK, http.MethodGet, "/api/v1/me/notifications", nil, &page)
		for _, n := range page.Items {
			if n.Kind == kind {
				return n
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("no %s notification", kind)
	return service.NotificationView{}
}
