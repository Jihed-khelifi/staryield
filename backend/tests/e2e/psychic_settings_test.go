package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// Advisor settings (M4): contact details and profile text, the photo
// submitted for review (private until approved) and the custom offering.
//
// The suite shares one database and runs files in name order. These tests
// create many advisors and move their fake clocks forward (audit rows in
// the "future"), so the file is named to run after the tests that read the
// first page of global lists (directory, audit log, mail outbox).

const (
	settingsPath = "/api/v1/psychic/settings"
	photoPath    = "/api/v1/psychic/photo"
	servicePath  = "/api/v1/psychic/service"
	reviewsPath  = "/api/v1/admin/photo-submissions"
)

// uploadPhoto posts a multipart photo like the settings page does.
func (c *client) uploadPhoto(data []byte) response {
	c.e.t.Helper()
	body, ct := multipartFile(c.e.t, "photo", "portrait.png", data)
	c.ensureCSRF()
	return c.raw(http.MethodPost, photoPath, body, map[string]string{"Content-Type": ct, "X-CSRF-Token": c.csrf})
}

func (c *client) mustUploadPhoto(data []byte) service.PhotoStateView {
	c.e.t.Helper()
	r := c.uploadPhoto(data)
	if r.status != http.StatusCreated {
		c.e.t.Fatalf("upload photo: %d %s", r.status, r.body)
	}
	var out service.PhotoStateView
	r.decode(c.e.t, &out)
	return out
}

// privateKeys lists the private-bucket keys of an advisor's photos.
func (e *env) privateKeys(psychicID string) []string {
	var out []string
	for _, k := range e.chat.Keys() {
		if strings.HasPrefix(k, domain.PhotoObjectPrefix+psychicID+"/") {
			out = append(out, k)
		}
	}
	return out
}

func (e *env) publicKeys(psychicID string) []string {
	var out []string
	for _, k := range e.advisors.Keys() {
		if strings.HasPrefix(k, "advisors/"+psychicID+"/") {
			out = append(out, k)
		}
	}
	return out
}

func publicProfile(t *testing.T, c *client, slug string) service.PsychicView {
	t.Helper()
	var out service.PsychicView
	c.must(http.StatusOK, http.MethodGet, "/api/v1/psychics/"+slug, nil, &out)
	return out
}

func TestAdvisorSettingsRoundTrip(t *testing.T) {
	e := newEnv(t)
	advisor := e.createPsychic(30)
	visitor := e.client()

	var s service.AdvisorSettingsView
	advisor.must(http.StatusOK, http.MethodGet, settingsPath, nil, &s)
	if s.Version != 1 || s.Phone != "" || s.PhoneMasked != "" || s.Modalities == nil || s.About == nil {
		t.Fatalf("defaults: %+v", s)
	}

	// Validation: every bad field is reported at once, unknown fields too.
	r := advisor.call(http.MethodPatch, settingsPath, map[string]any{
		"phone": "555-0100", "modalities": []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"},
		"headline": strings.Repeat("h", 201), "about": []string{strings.Repeat("x", 2001)},
	})
	var verr struct {
		Error struct {
			Code    string `json:"code"`
			Details struct {
				Fields map[string]string `json:"fields"`
			} `json:"details"`
		} `json:"error"`
	}
	r.decode(t, &verr)
	if r.status != http.StatusUnprocessableEntity || len(verr.Error.Details.Fields) != 4 || verr.Error.Details.Fields["phone"] == "" ||
		verr.Error.Details.Fields["modalities"] == "" || verr.Error.Details.Fields["headline"] == "" || verr.Error.Details.Fields["about"] == "" {
		t.Fatalf("validation: %d %s", r.status, r.body)
	}
	if r := advisor.call(http.MethodPatch, settingsPath, map[string]any{"modalities": []string{strings.Repeat("m", 41)}}); r.errCode(t) != "validation_failed" {
		t.Fatalf("41-char modality: %s", r.body)
	}
	if r := advisor.call(http.MethodPatch, settingsPath, map[string]any{"skills": []string{"Tarot"}}); r.errCode(t) != "validation_failed" {
		t.Fatalf("unknown field accepted: %s", r.body)
	}
	// Messages follow the caller's language.
	advisor.locale = "fr"
	if r := advisor.call(http.MethodPatch, settingsPath, map[string]any{"phone": "12"}); !strings.Contains(string(r.body), "format international") {
		t.Fatalf("french message: %s", r.body)
	}
	advisor.locale = ""

	// A valid save: separators are stripped, modalities de-duplicated.
	advisor.must(http.StatusOK, http.MethodPatch, settingsPath, map[string]any{
		"expected_version": 1, "phone": " +44 20 7946-0958 ", "about": []string{"Reads palms and auras.", "  ", "Twenty years of practice."},
		"modalities": []string{"Tarot", " tarot ", "Astrology", "Aura   reading"},
	}, &s)
	if s.Version != 2 || s.Phone != "+442079460958" || s.PhoneMasked != "+••••••••••58" ||
		strings.Join(s.Modalities, "|") != "Tarot|Astrology|Aura reading" || len(s.About) != 2 {
		t.Fatalf("saved: %+v", s)
	}
	// Sealed at rest, bound to the advisor.
	var sealed string
	e.app.DB.Table("psychics").Where("user_id = ?", advisor.user.ID).Pluck("phone_sealed", &sealed)
	if !strings.HasPrefix(sealed, "v1:") || strings.Contains(sealed, "2079460958") {
		t.Fatalf("phone not sealed: %q", sealed)
	}

	// A stale page cannot overwrite the newer save.
	r = advisor.call(http.MethodPatch, settingsPath, map[string]any{"expected_version": 1, "headline": "Stale"})
	if r.status != http.StatusConflict || r.errCode(t) != "version_conflict" || !strings.Contains(string(r.body), `"current_version":2`) {
		t.Fatalf("stale save: %d %s", r.status, r.body)
	}
	// Saving identical values changes nothing (no version bump).
	advisor.must(http.StatusOK, http.MethodPatch, settingsPath, map[string]any{"expected_version": 2, "phone": "+442079460958"}, &s)
	if s.Version != 2 {
		t.Fatalf("no-op save bumped the version: %d", s.Version)
	}
	// PATCH /psychic/profile shares the validation and the version.
	advisor.must(http.StatusOK, http.MethodPatch, "/api/v1/psychic/profile", map[string]any{"headline": "Clear answers"}, nil)
	advisor.must(http.StatusOK, http.MethodGet, settingsPath, nil, &s)
	if s.Version != 3 || s.Headline != "Clear answers" {
		t.Fatalf("profile save: %+v", s)
	}

	// The public profile and directory show the modalities, never the phone.
	profile := publicProfile(t, visitor, advisor.slug())
	if strings.Join(profile.Skills, "|") != "Tarot|Astrology|Aura reading" || len(profile.About) != 2 {
		t.Fatalf("public profile: %+v", profile)
	}
	raw := visitor.call(http.MethodGet, "/api/v1/psychics/"+advisor.slug(), nil)
	if bytes.Contains(raw.body, []byte("2079460958")) || bytes.Contains(raw.body, []byte("phone")) {
		t.Fatalf("public profile leaks the phone: %s", raw.body)
	}
	var dir directoryPage
	visitor.must(http.StatusOK, http.MethodGet, "/api/v1/psychics?q="+url.QueryEscape(advisor.user.Psychic.Name), nil, &dir)
	if len(dir.Items) != 1 || !strings.Contains(string(mustJSON(t, dir.Items[0]["skills"])), "Aura reading") {
		t.Fatalf("directory not refreshed: %+v", dir.Items)
	}

	// Clearing the phone removes the sealed value.
	advisor.must(http.StatusOK, http.MethodPatch, settingsPath, map[string]any{"phone": ""}, &s)
	e.app.DB.Table("psychics").Where("user_id = ?", advisor.user.ID).Pluck("phone_sealed", &sealed)
	if s.Phone != "" || sealed != "" {
		t.Fatalf("phone not cleared: %+v %q", s, sealed)
	}

	// Only advisors reach these routes; each advisor only sees their own.
	client := e.registerClient()
	if r := client.call(http.MethodGet, settingsPath, nil); r.status != http.StatusForbidden {
		t.Fatalf("client read settings: %d", r.status)
	}
	if r := client.call(http.MethodPatch, settingsPath, map[string]any{"phone": "+14155552671"}); r.status != http.StatusForbidden {
		t.Fatalf("client wrote settings: %d", r.status)
	}
	if r := visitor.call(http.MethodGet, settingsPath, nil); r.status != http.StatusUnauthorized {
		t.Fatalf("anonymous read settings: %d", r.status)
	}
	other := e.createPsychic(30)
	advisor.must(http.StatusOK, http.MethodPatch, settingsPath, map[string]any{"phone": "+14155552671"}, nil)
	var theirs service.AdvisorSettingsView
	other.must(http.StatusOK, http.MethodGet, settingsPath, nil, &theirs)
	if theirs.Phone != "" || theirs.Version != 1 {
		t.Fatalf("another advisor sees foreign settings: %+v", theirs)
	}
	// CSRF stays on.
	advisor.csrf = "forged"
	if r := advisor.raw(http.MethodPatch, settingsPath, strings.NewReader(`{"phone":""}`), map[string]string{"Content-Type": "application/json",
		"X-CSRF-Token": "forged"}); r.errCode(t) != "csrf_failed" {
		t.Fatalf("csrf: %d %s", r.status, r.body)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestAdvisorPhotoReviewFlow(t *testing.T) {
	e := newEnv(t)
	advisor := e.createPsychic(30)
	id := advisor.user.ID.String()
	admin := e.createAdmin()
	visitor := e.client()

	var state service.PhotoStateView
	advisor.must(http.StatusOK, http.MethodGet, photoPath, nil, &state)
	if state.Submission != nil || state.ImageURL != "" || state.Limits.MinSide != 256 || state.Limits.MaxBytes != 5<<20 {
		t.Fatalf("initial state: %+v", state)
	}

	// Upload: pending, private, never public.
	state = advisor.mustUploadPhoto(pngBytes(t, 900, 600))
	if state.Submission == nil {
		t.Fatalf("no submission: %+v", state)
	}
	// A copy: later decodes into state reuse the pointed-to struct.
	first := *state.Submission
	sub := &first
	if sub.Status != "pending" || !sub.HasImage || sub.Width != 900 || sub.Height != 600 {
		t.Fatalf("submission: %+v", state)
	}
	if keys := e.privateKeys(id); len(keys) != 1 {
		t.Fatalf("private objects: %v", keys)
	}
	if keys := e.publicKeys(id); len(keys) != 0 {
		t.Fatalf("pending photo published: %v", keys)
	}
	if p := publicProfile(t, visitor, advisor.slug()); p.ImageURL != "" {
		t.Fatalf("pending photo public: %s", p.ImageURL)
	}
	for _, r := range []response{advisor.call(http.MethodGet, photoPath, nil), visitor.call(http.MethodGet, "/api/v1/psychics/"+advisor.slug(), nil)} {
		if bytes.Contains(r.body, []byte("advisor-photos")) {
			t.Fatalf("storage key exposed: %s", r.body)
		}
	}
	var status string
	e.app.DB.Table("psychics").Where("user_id = ?", advisor.user.ID).Pluck("photo_review_status", &status)
	if status != "pending" {
		t.Fatalf("photo_review_status: %s", status)
	}

	// The owner can preview it, privately; nobody else can.
	r := advisor.call(http.MethodGet, photoPath+"/image", nil)
	if r.status != http.StatusOK || r.header.Get("Content-Type") != "image/jpeg" || !strings.Contains(r.header.Get("Cache-Control"), "no-store") {
		t.Fatalf("owner preview: %d %v", r.status, r.header)
	}
	other := e.createPsychic(30)
	if r := other.call(http.MethodGet, photoPath+"/image", nil); r.status != http.StatusNotFound {
		t.Fatalf("other advisor preview: %d", r.status)
	}
	buyer := e.registerClient()
	for _, path := range []string{photoPath, photoPath + "/image"} {
		if r := buyer.call(http.MethodGet, path, nil); r.status != http.StatusForbidden {
			t.Fatalf("client %s: %d", path, r.status)
		}
	}
	if r := visitor.call(http.MethodGet, photoPath+"/image", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("anonymous preview: %d", r.status)
	}
	// The site's accounts never reach the review console.
	for _, c := range []*client{advisor, buyer} {
		if r := c.call(http.MethodGet, reviewsPath+"/"+sub.ID.String()+"/image", nil); r.status == http.StatusOK {
			t.Fatalf("console preview reached from the site: %s", r.body)
		}
	}

	// The console needs a fresh step-up for the list, the preview and decisions.
	e.clock.Advance(11 * time.Minute)
	for _, rq := range []struct{ method, path string }{
		{http.MethodGet, reviewsPath + "?status=pending"},
		{http.MethodGet, reviewsPath + "/" + sub.ID.String() + "/image"},
		{http.MethodPost, reviewsPath + "/" + sub.ID.String() + "/approve"},
	} {
		if r := admin.call(rq.method, rq.path, nil); r.errCode(t) != "step_up_required" {
			t.Fatalf("%s %s without step-up: %d %s", rq.method, rq.path, r.status, r.body)
		}
	}
	admin.stepUp()
	var page service.Page[service.AdminPhotoSubmissionView]
	admin.must(http.StatusOK, http.MethodGet, reviewsPath+"?status=pending", nil, &page)
	found := false
	for _, it := range page.Items {
		if it.ID == sub.ID {
			found = it.Psychic.Slug == advisor.slug() && it.HasImage && it.Status == "pending"
		}
	}
	if !found {
		t.Fatalf("pending queue: %+v", page.Items)
	}
	if r := admin.call(http.MethodGet, reviewsPath+"?status=withdrawn", nil); r.errCode(t) != "validation_failed" {
		t.Fatalf("status filter: %s", r.body)
	}
	r = admin.call(http.MethodGet, reviewsPath+"/"+sub.ID.String()+"/image", nil)
	if r.status != http.StatusOK || !strings.HasPrefix(r.header.Get("Content-Type"), "image/") || !strings.Contains(r.header.Get("Cache-Control"), "no-store") {
		t.Fatalf("admin preview: %d %v", r.status, r.header)
	}

	// Reject: a reason is required; the advisor sees it and is notified.
	for _, reason := range []string{"", "  ", strings.Repeat("r", 501)} {
		if r := admin.call(http.MethodPost, reviewsPath+"/"+sub.ID.String()+"/reject", map[string]any{"reason": reason}); r.errCode(t) != "validation_failed" {
			t.Fatalf("reason %q: %s", reason[:min(len(reason), 5)], r.body)
		}
	}
	var reviewed service.AdminPhotoSubmissionView
	admin.must(http.StatusOK, http.MethodPost, reviewsPath+"/"+sub.ID.String()+"/reject", map[string]any{"reason": "Your face is not visible."}, &reviewed)
	if reviewed.Status != "rejected" || reviewed.Reason != "Your face is not visible." || reviewed.ReviewedBy == nil || *reviewed.ReviewedBy != admin.adminID {
		t.Fatalf("rejected: %+v", reviewed)
	}
	advisor.must(http.StatusOK, http.MethodGet, photoPath, nil, &state)
	if state.Submission == nil || state.Submission.Status != "rejected" || state.Submission.Reason != "Your face is not visible." || !state.Submission.HasImage {
		t.Fatalf("advisor after rejection: %+v", state.Submission)
	}
	n := waitNotification(t, advisor, service.NotifyPhotoRejected)
	if n.Data["reason"] != "Your face is not visible." || n.Link != "/profile/settings" {
		t.Fatalf("rejection notice: %+v", n)
	}
	if r := admin.call(http.MethodPost, reviewsPath+"/"+sub.ID.String()+"/approve", nil); r.errCode(t) != "invalid_state" {
		t.Fatalf("approve after reject: %d %s", r.status, r.body)
	}
	if p := publicProfile(t, visitor, advisor.slug()); p.ImageURL != "" {
		t.Fatalf("rejected photo public: %s", p.ImageURL)
	}

	// A new upload supersedes the pending one; withdrawing deletes it.
	replaced := advisor.mustUploadPhoto(pngBytes(t, 400, 400)).Submission
	second := advisor.mustUploadPhoto(pngBytes(t, 500, 500)).Submission
	if keys := e.privateKeys(id); len(keys) != 2 { // the rejected photo and the latest upload
		t.Fatalf("superseded image kept: %v", keys)
	}
	var withdrawn int64
	e.app.DB.Table("psychic_photo_submissions").Where("id = ? AND status = 'withdrawn' AND deleted_at IS NOT NULL AND object_key = ''", replaced.ID).Count(&withdrawn)
	if withdrawn != 1 {
		t.Fatal("superseded submission not withdrawn")
	}
	if r := admin.call(http.MethodPost, reviewsPath+"/"+replaced.ID.String()+"/approve", nil); r.status != http.StatusNotFound {
		t.Fatalf("approve a withdrawn photo: %d %s", r.status, r.body)
	}
	advisor.must(http.StatusNoContent, http.MethodDelete, photoPath, nil, nil)
	if r := advisor.call(http.MethodDelete, photoPath, nil); r.status != http.StatusNotFound {
		t.Fatalf("second withdraw: %d", r.status)
	}
	if keys := e.privateKeys(id); len(keys) != 1 {
		t.Fatalf("withdrawn image kept: %v", keys)
	}
	if r := admin.call(http.MethodGet, reviewsPath+"/"+second.ID.String()+"/image", nil); r.status != http.StatusNotFound {
		t.Fatalf("withdrawn image previewable: %d", r.status)
	}
	e.app.DB.Table("psychics").Where("user_id = ?", advisor.user.ID).Pluck("photo_review_status", &status)
	if status != "rejected" { // the latest live submission is the rejected one
		t.Fatalf("photo_review_status after withdraw: %s", status)
	}

	// Approve: published to the public bucket, the old photo deleted, the
	// directory refreshed and the advisor notified.
	oldBody, oldCT := multipartFile(t, "image", "old.png", pngBytes(t, 300, 300))
	if r := admin.raw(http.MethodPost, "/api/v1/admin/psychics/"+id+"/image", oldBody, map[string]string{"Content-Type": oldCT, "X-CSRF-Token": csrfOf(admin)}); r.status != http.StatusOK {
		t.Fatalf("admin image: %d %s", r.status, r.body)
	}
	oldKeys := e.publicKeys(id)
	if len(oldKeys) != 1 {
		t.Fatalf("admin image keys: %v", oldKeys)
	}
	pending := advisor.mustUploadPhoto(pngBytes(t, 640, 640)).Submission
	admin.must(http.StatusOK, http.MethodPost, reviewsPath+"/"+pending.ID.String()+"/approve", nil, &reviewed)
	if reviewed.Status != "approved" || reviewed.HasImage || reviewed.Psychic.ImageURL == "" {
		t.Fatalf("approved: %+v", reviewed)
	}
	keys := e.publicKeys(id)
	if len(keys) != 1 || keys[0] == oldKeys[0] {
		t.Fatalf("public objects after approval: %v (old %v)", keys, oldKeys)
	}
	profile := publicProfile(t, visitor, advisor.slug())
	if profile.ImageURL != "https://advisors.test/"+keys[0] {
		t.Fatalf("profile image: %s", profile.ImageURL)
	}
	var dir directoryPage
	visitor.must(http.StatusOK, http.MethodGet, "/api/v1/psychics?q="+url.QueryEscape(advisor.user.Psychic.Name), nil, &dir)
	if len(dir.Items) != 1 || dir.Items[0]["image_url"] != profile.ImageURL {
		t.Fatalf("directory image: %+v", dir.Items)
	}
	for _, k := range e.privateKeys(id) {
		if strings.Contains(k, pending.ID.String()) {
			t.Fatal("approved image kept in the private bucket")
		}
	}
	waitNotification(t, advisor, service.NotifyPhotoApproved)
	advisor.must(http.StatusOK, http.MethodGet, photoPath, nil, &state)
	if state.ImageURL != profile.ImageURL || state.Submission == nil || state.Submission.Status != "approved" || state.Submission.HasImage {
		t.Fatalf("advisor after approval: %+v", state)
	}
	if r := advisor.call(http.MethodGet, photoPath+"/image", nil); r.status != http.StatusNotFound {
		t.Fatalf("approved photo streamed privately: %d", r.status)
	}
	e.app.DB.Table("psychics").Where("user_id = ?", advisor.user.ID).Pluck("photo_review_status", &status)
	if status != "approved" {
		t.Fatalf("photo_review_status after approval: %s", status)
	}

	// Every decision and preview is audited.
	for _, action := range []string{"photo_submission.viewed", "photo_submission.rejected", "photo_submission.approved", "psychic.photo_submitted", "psychic.photo_withdrawn"} {
		var audit service.Page[service.AuditView]
		admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/audit-logs?action="+action, nil, &audit)
		if len(audit.Items) == 0 {
			t.Fatalf("no %s audit entry", action)
		}
	}
	var audit service.Page[service.AuditView]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/audit-logs?action=photo_submission.rejected&target_type=photo_submission&target_id="+sub.ID.String(), nil, &audit)
	if len(audit.Items) != 1 || audit.Items[0].TargetID != sub.ID.String() || audit.Items[0].Metadata["reason"] != "Your face is not visible." {
		t.Fatalf("rejection audit: %+v", audit.Items)
	}
}

func TestAdvisorPhotoValidationAndLimits(t *testing.T) {
	e := newEnv(t)
	advisor := e.createPsychic(30)

	// Content is sniffed: a script named .png is refused.
	if r := advisor.uploadPhoto([]byte(strings.Repeat("<script>alert(1)</script>", 10))); r.status != http.StatusUnsupportedMediaType {
		t.Fatalf("non-image: %d %s", r.status, r.body)
	}
	if r := advisor.uploadPhoto(pngBytes(t, 200, 200)); r.errCode(t) != "validation_failed" || !strings.Contains(string(r.body), `"photo"`) {
		t.Fatalf("small photo: %d %s", r.status, r.body)
	}
	if r := advisor.uploadPhoto(bytes.Repeat([]byte{0x89}, 6<<20)); r.status != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized upload: %d", r.status)
	}
	body, ct := multipartFile(t, "image", "wrong-field.png", pngBytes(t, 300, 300))
	advisor.ensureCSRF()
	if r := advisor.raw(http.MethodPost, photoPath, body, map[string]string{"Content-Type": ct, "X-CSRF-Token": advisor.csrf}); r.errCode(t) != "validation_failed" {
		t.Fatalf("missing photo field: %d %s", r.status, r.body)
	}
	if len(e.privateKeys(advisor.user.ID.String())) != 0 {
		t.Fatal("refused uploads were stored")
	}

	// The daily cap counts withdrawn uploads too and holds in the database.
	img := pngBytes(t, 300, 300)
	for i := 0; i < 10; i++ {
		advisor.mustUploadPhoto(img)
		if i%2 == 0 {
			advisor.must(http.StatusNoContent, http.MethodDelete, photoPath, nil, nil)
		}
	}
	if r := advisor.uploadPhoto(img); r.status != http.StatusTooManyRequests || r.errCode(t) != "rate_limited" {
		t.Fatalf("daily cap: %d %s", r.status, r.body)
	}
	e.clock.Advance(25 * time.Hour)
	advisor.refresh()
	advisor.mustUploadPhoto(img)
	// One pending submission at a time.
	var pending int64
	e.app.DB.Table("psychic_photo_submissions").Where("psychic_id = ? AND status = 'pending' AND deleted_at IS NULL", advisor.user.ID).Count(&pending)
	if pending != 1 {
		t.Fatalf("pending submissions: %d", pending)
	}
}

func TestAdvisorPhotoUploadRateLimit(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.RateLimit.Scale = 1 })
	advisor := e.createPsychic(30)
	img := pngBytes(t, 300, 300)
	for i := 0; i < 3; i++ {
		advisor.mustUploadPhoto(img)
	}
	r := advisor.uploadPhoto(img)
	if r.status != http.StatusTooManyRequests || r.header.Get("Retry-After") == "" {
		t.Fatalf("burst not limited: %d %s", r.status, r.body)
	}
}

func TestAdvisorPhotoCleanup(t *testing.T) {
	e := newEnv(t)
	advisor := e.createPsychic(30)
	id := advisor.user.ID.String()
	admin := e.createAdmin()
	sub := advisor.mustUploadPhoto(pngBytes(t, 300, 300)).Submission
	admin.stepUp()
	admin.must(http.StatusOK, http.MethodPost, reviewsPath+"/"+sub.ID.String()+"/reject", map[string]any{"reason": "Blurry."}, nil)

	// Within the retention the advisor still sees the rejection.
	e.clock.Advance(6 * 24 * time.Hour)
	e.maintenance()
	advisor.refresh()
	var state service.PhotoStateView
	advisor.must(http.StatusOK, http.MethodGet, photoPath, nil, &state)
	if state.Submission == nil || state.Submission.Status != "rejected" || len(e.privateKeys(id)) != 1 {
		t.Fatalf("rejection cleaned too early: %+v", state.Submission)
	}
	// Then the image goes and the row leaves the advisor's page.
	e.clock.Advance(2 * 24 * time.Hour)
	e.maintenance()
	advisor.refresh()
	advisor.must(http.StatusOK, http.MethodGet, photoPath, nil, &state)
	if state.Submission != nil || len(e.privateKeys(id)) != 0 {
		t.Fatalf("rejected photo kept: %+v %v", state.Submission, e.privateKeys(id))
	}
	var status string
	e.app.DB.Table("psychics").Where("user_id = ?", advisor.user.ID).Pluck("photo_review_status", &status)
	if status != "approved" {
		t.Fatalf("photo_review_status after clean-up: %s", status)
	}
	var rows int64
	e.app.DB.Table("psychic_photo_submissions").Where("id = ?", sub.ID).Count(&rows)
	if rows != 1 {
		t.Fatal("row purged before the purge delay")
	}
	e.clock.Advance(91 * 24 * time.Hour)
	e.maintenance()
	e.app.DB.Table("psychic_photo_submissions").Where("id = ?", sub.ID).Count(&rows)
	if rows != 0 {
		t.Fatal("soft-deleted submission not purged")
	}
}

func TestAdvisorErasureRemovesPhotosAndPhone(t *testing.T) {
	e := newEnv(t)
	advisor := e.createPsychic(30)
	id := advisor.user.ID.String()
	admin := e.createAdmin()
	advisor.must(http.StatusOK, http.MethodPatch, settingsPath, map[string]any{"phone": "+14155552671"}, nil)
	approved := advisor.mustUploadPhoto(pngBytes(t, 300, 300)).Submission
	admin.stepUp()
	admin.must(http.StatusOK, http.MethodPost, reviewsPath+"/"+approved.ID.String()+"/approve", nil, nil)
	rejected := advisor.mustUploadPhoto(pngBytes(t, 320, 320)).Submission
	admin.must(http.StatusOK, http.MethodPost, reviewsPath+"/"+rejected.ID.String()+"/reject", map[string]any{"reason": "Too dark."}, nil)
	advisor.mustUploadPhoto(pngBytes(t, 340, 340)) // pending
	if len(e.privateKeys(id)) != 2 || len(e.publicKeys(id)) != 1 {
		t.Fatalf("before erasure: private %v public %v", e.privateKeys(id), e.publicKeys(id))
	}

	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/users/"+id, map[string]any{"reason": "GDPR erasure"}, nil)

	var rows int64
	e.app.DB.Table("psychic_photo_submissions").Where("psychic_id = ?", advisor.user.ID).Count(&rows)
	if rows != 0 {
		t.Fatalf("submissions survived erasure: %d", rows)
	}
	if keys := append(e.privateKeys(id), e.publicKeys(id)...); len(keys) != 0 {
		t.Fatalf("images survived erasure: %v", keys)
	}
	var p domain.Psychic
	e.app.DB.Where("user_id = ?", advisor.user.ID).First(&p)
	if p.PhoneSealed != "" || p.ImageURL != "" || p.ImageKey != "" || p.Listed {
		t.Fatalf("advisor row after erasure: phone=%q image=%q listed=%v", p.PhoneSealed, p.ImageURL, p.Listed)
	}
}

func TestAdvisorCustomOffering(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	advisor := e.createPsychic(30)
	admin := e.createAdmin()
	visitor := e.client()

	var state service.CustomServiceState
	advisor.must(http.StatusOK, http.MethodGet, servicePath, nil, &state)
	if state.Service != nil || state.Limits.MinPrice != 10 || state.Limits.MaxPrice != 50000 || state.Limits.MaxDays != 60 {
		t.Fatalf("initial: %+v", state)
	}
	if r := advisor.call(http.MethodDelete, servicePath, nil); r.status != http.StatusNotFound {
		t.Fatalf("delete none: %d", r.status)
	}

	// An admin-curated service exists alongside and is never touched.
	var standard service.ServiceView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/psychics/"+advisor.user.ID.String()+"/services",
		map[string]any{"name": "Schedule a Reading", "subtitle": "30 Min Live Session", "price_credits": 500}, &standard)

	// Validation.
	for _, body := range []map[string]any{
		{},
		{"name": "x", "price_credits": 9, "delivery_days": 3},
		{"name": "x", "price_credits": 50001, "delivery_days": 3},
		{"name": "x", "price_credits": 100, "delivery_days": 0},
		{"name": "x", "price_credits": 100, "delivery_days": 61},
		{"name": strings.Repeat("n", 81), "price_credits": 100, "delivery_days": 3},
		{"name": "x", "price_credits": 100, "delivery_days": 3, "description": strings.Repeat("d", 1001)},
		{"name": "x", "price_credits": 100, "delivery_days": 3, "kind": "standard"},
		{"name": "x", "price_credits": "100", "delivery_days": 3},
	} {
		if r := advisor.call(http.MethodPut, servicePath, body); r.errCode(t) != "validation_failed" {
			t.Fatalf("PUT %v: %d %s", body, r.status, r.body)
		}
	}

	// Create, then replace in place.
	r := advisor.call(http.MethodPut, servicePath, map[string]any{"name": " Soulmate   sketch ", "price_credits": 2000, "delivery_days": 8})
	if r.status != http.StatusCreated {
		t.Fatalf("create: %d %s", r.status, r.body)
	}
	r.decode(t, &state)
	created := *state.Service
	if created.Name != "Soulmate sketch" || created.PriceCredits != 2000 || created.DeliveryDays != 8 || !created.Active {
		t.Fatalf("created: %+v", created)
	}
	advisor.must(http.StatusOK, http.MethodPut, servicePath, map[string]any{"name": "Soulmate sketch", "price_credits": 2400,
		"delivery_days": 30, "description": "A pencil portrait of your future partner."}, &state)
	if state.Service.ID != created.ID || state.Service.PriceCredits != 2400 || state.Service.DeliveryDays != 30 {
		t.Fatalf("replaced: %+v", state.Service)
	}
	var count int64
	e.app.DB.Table("psychic_services").Where("psychic_id = ? AND kind = 'custom' AND deleted_at IS NULL", advisor.user.ID).Count(&count)
	if count != 1 {
		t.Fatalf("custom offerings: %d", count)
	}
	var std domain.PsychicService
	e.app.DB.Where("id = ?", standard.ID).First(&std)
	if std.Name != "Schedule a Reading" || std.PriceCredits != 500 || std.Kind != domain.ServiceStandard {
		t.Fatalf("standard service changed: %+v", std)
	}

	// Public profile lists it after the curated services, with its kind.
	profile := publicProfile(t, visitor, advisor.slug())
	if len(profile.Services) != 2 || profile.Services[0].ID != standard.ID || profile.Services[0].Kind != "standard" ||
		profile.Services[1].ID != created.ID || profile.Services[1].Kind != "custom" || profile.Services[1].DeliveryDays != 30 {
		t.Fatalf("public services: %+v", profile.Services)
	}

	// Another advisor's offering is separate (no IDs to probe); clients
	// cannot reach the route.
	other := e.createPsychic(30)
	other.must(http.StatusOK, http.MethodGet, servicePath, nil, &state)
	if state.Service != nil {
		t.Fatalf("other advisor sees a foreign offering: %+v", state.Service)
	}
	if r := other.call(http.MethodDelete, servicePath, nil); r.status != http.StatusNotFound {
		t.Fatalf("other advisor deleted: %d", r.status)
	}
	client := e.registerClient()
	if r := client.call(http.MethodPut, servicePath, map[string]any{"name": "x", "price_credits": 100, "delivery_days": 3}); r.status != http.StatusForbidden {
		t.Fatalf("client PUT: %d", r.status)
	}

	// Orderable through the custom-request flow; the delivery deadline
	// honours the advertised 30 days.
	client.buyCredits(250)
	client.buyCredits(1500)
	client.buyCredits(1500)
	var order service.CustomRequestView
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/custom-requests", map[string]any{"service_id": created.ID,
		"note": "Please draw them.", "idempotency_key": uniq("order-key-")}, &order)
	if order.PriceCredits != 2400 || order.ServiceName != "Soulmate sketch" {
		t.Fatalf("order: %+v", order)
	}
	var accepted service.CustomRequestView
	advisor.must(http.StatusOK, http.MethodPost, "/api/v1/custom-requests/"+order.ID.String()+"/accept", nil, &accepted)
	if accepted.DeliverBy == nil || accepted.DeliverBy.Sub(e.clock.Now()) < 29*24*time.Hour {
		t.Fatalf("deliver_by: %v (now %v)", accepted.DeliverBy, e.clock.Now())
	}

	// Removing it hides it, keeps the order and blocks new orders.
	advisor.must(http.StatusNoContent, http.MethodDelete, servicePath, nil, nil)
	if p := publicProfile(t, visitor, advisor.slug()); len(p.Services) != 1 || p.Services[0].ID != standard.ID {
		t.Fatalf("services after delete: %+v", p.Services)
	}
	if r := client.call(http.MethodPost, "/api/v1/custom-requests", map[string]any{"service_id": created.ID, "idempotency_key": uniq("order-key-")}); r.status != http.StatusNotFound {
		t.Fatalf("ordered a removed offering: %d %s", r.status, r.body)
	}
	var still service.CustomRequestView
	client.must(http.StatusOK, http.MethodGet, "/api/v1/custom-requests/"+order.ID.String(), nil, &still)
	if still.ServiceName != "Soulmate sketch" || still.Status != "accepted" {
		t.Fatalf("order after removal: %+v", still)
	}
	// A new offering can be created afterwards.
	r = advisor.call(http.MethodPut, servicePath, map[string]any{"name": "Aura photo", "price_credits": 900, "delivery_days": 5})
	if r.status != http.StatusCreated {
		t.Fatalf("recreate: %d %s", r.status, r.body)
	}
	var audit service.Page[service.AuditView]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/audit-logs?action=psychic.custom_service_deleted&target_type=psychic_service&target_id="+created.ID.String(), nil, &audit)
	if len(audit.Items) != 1 || audit.Items[0].TargetID != created.ID.String() {
		t.Fatalf("delete audit: %+v", audit.Items)
	}
	// The order e-mails go out now, not in a later test sharing the outbox.
	e.flushMail()
}
