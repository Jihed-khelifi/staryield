package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

const testPassword = "Str0ng!Passw0rd#"

// identityFixture is an AccountService over in-memory repositories.
type identityFixture struct {
	db      *memDB
	clock   *clock.Fake
	mail    *memMail
	svc     *AccountService
	consent *ConsentService
	hasher  *security.PasswordHasher
	user    domain.User
}

func testPlaces(id string) (PlaceInfo, bool) {
	if id == "paris-fr" {
		return PlaceInfo{ID: id, Name: "Paris", Country: "France", CountryCode: "fr", Latitude: 48.85, Longitude: 2.35, TimeZone: "Europe/Paris"}, true
	}
	return PlaceInfo{}, false
}

func newIdentityFixture(t *testing.T) *identityFixture {
	t.Helper()
	db := newMemDB()
	clk := clock.NewFake(time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC))
	hasher := security.NewPasswordHasher(1024, 1, 1)
	sealer, err := security.NewSealer(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := security.NewTokenIssuer(strings.Repeat("s", 32), "staryield", "staryield", 15*time.Minute, clk.Now)
	if err != nil {
		t.Fatal(err)
	}
	tx := memTx{db: db}
	users := memUsers{db: db}
	mail := &memMail{}
	auth := NewAuthService(tx, users, memTokens{db: db}, nil, nil, hasher, issuer, cache.NewMemory(clk.Now), realtime.NewMemoryTickets(clk.Now),
		mail, NoopPublisher{}, nil, clk, AuthSettings{RefreshTTL: 24 * time.Hour})
	consent := NewConsentService(tx, memConsents{db: db}, memAudit{db: db}, clk, "secret")
	svc := NewAccountService(AccountDeps{Tx: tx, Users: users, Accounts: memAccounts{db: db}, Tokens: memTokens{db: db},
		Actions: memActions{db: db}, Psychics: memPsychics{db: db}, Audit: memAudit{db: db}, Auth: auth, Consents: consent,
		Mail: mail, Hasher: hasher, Sealer: sealer, Places: testPlaces, Clock: clk,
		Settings: AccountSettings{FrontendURL: "https://staryield.test", MinimumAge: 18}})
	f := &identityFixture{db: db, clock: clk, mail: mail, svc: svc, consent: consent, hasher: hasher}
	f.user = f.addUser(t, "moon@example.com", domain.RoleClient)
	return f
}

func (f *identityFixture) addUser(t *testing.T, email string, role domain.Role) domain.User {
	t.Helper()
	hash, err := f.hasher.Hash(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	now := f.clock.Now()
	u := domain.User{ID: domain.NewID(), Email: email, PasswordHash: hash, Role: role, Status: domain.UserActive, DisplayName: "Moon",
		Locale: "en", TermsAcceptedAt: now, CreatedAt: now, UpdatedAt: now}
	f.db.users[u.ID] = u
	p := newProfile(u.ID, nil, now)
	p.NotifyEmail, p.HouseSystem = true, domain.HousePlacidus
	f.db.profiles[u.ID] = memProfile{Profile: *p, version: 1}
	return u
}

func TestDescribeUserAgent(t *testing.T) {
	for ua, want := range map[string]DeviceInfo{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36": {
			Label: "Chrome on macOS", Type: "desktop", Browser: "Chrome", OS: "macOS"},
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1": {
			Label: "Safari on iPhone", Type: "mobile", Browser: "Safari", OS: "iPhone"},
		"Mozilla/5.0 (iPad; CPU OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1": {
			Label: "Safari on iPad", Type: "tablet", Browser: "Safari", OS: "iPad"},
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:131.0) Gecko/20100101 Firefox/131.0": {
			Label: "Firefox on Windows", Type: "desktop", Browser: "Firefox", OS: "Windows"},
		"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Mobile Safari/537.36": {
			Label: "Chrome on Android", Type: "mobile", Browser: "Chrome", OS: "Android"},
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)": {Type: "bot"},
		"": {Label: "Unknown device", Type: "unknown"},
	} {
		got := DescribeUserAgent(ua)
		if want.Type == "bot" {
			if got.Type != "bot" || got.Label == "" {
				t.Errorf("bot: %+v", got)
			}
			continue
		}
		if got != want {
			t.Errorf("DescribeUserAgent(%.40q) = %+v, want %+v", ua, got, want)
		}
	}
	long := DescribeUserAgent(strings.Repeat("Mozilla/5.0 é", 100))
	if len(long.Label) > 80 || len(long.Browser) > 40 || len(long.OS) > 40 {
		t.Fatalf("labels must be bounded: %+v", long)
	}
}

func TestSettingsValidationMatrix(t *testing.T) {
	ctx := context.Background()
	s := func(v string) *string { return &v }
	cases := []struct {
		name  string
		patch SettingsPatch
		field string // "" = valid
	}{
		{"empty name", SettingsPatch{Account: &AccountSettingsPatch{DisplayName: s("  ")}}, "account.display_name"},
		{"long name", SettingsPatch{Account: &AccountSettingsPatch{DisplayName: s(strings.Repeat("a", 81))}}, "account.display_name"},
		{"future birth", SettingsPatch{Profile: &ProfileSettingsPatch{BirthDate: s("2030-01-01")}}, "profile.birth_date"},
		{"impossible date", SettingsPatch{Profile: &ProfileSettingsPatch{BirthDate: s("1990-02-30")}}, "profile.birth_date"},
		{"under age", SettingsPatch{Profile: &ProfileSettingsPatch{BirthDate: s("2015-01-01")}}, "profile.birth_date"},
		{"bad time", SettingsPatch{Profile: &ProfileSettingsPatch{BirthTime: s("25:00")}}, "profile.birth_time"},
		{"short time", SettingsPatch{Profile: &ProfileSettingsPatch{BirthTime: s("7:45")}}, "profile.birth_time"},
		{"unknown place", SettingsPatch{Profile: &ProfileSettingsPatch{BirthPlaceID: s("atlantis")}}, "profile.birth_place_id"},
		{"bad geonames id", SettingsPatch{Profile: &ProfileSettingsPatch{BirthPlaceID: s("gn:abc")}}, "profile.birth_place_id"},
		{"long label", SettingsPatch{Profile: &ProfileSettingsPatch{BirthPlaceLabel: s(strings.Repeat("x", 121))}}, "profile.birth_place_label"},
		{"house system", SettingsPatch{Profile: &ProfileSettingsPatch{HouseSystem: s("koch")}}, "profile.house_system"},
		{"mood", SettingsPatch{Profile: &ProfileSettingsPatch{Mood: s("Doomed")}}, "profile.mood"},
		{"theme", SettingsPatch{Appearance: &AppearancePatch{Theme: s("Dark")}}, "appearance.theme"},
		{"locale", SettingsPatch{Appearance: &AppearancePatch{Locale: s("de")}}, "appearance.locale"},
		{"zone", SettingsPatch{Appearance: &AppearancePatch{TimeZone: s("Mars/Olympus")}}, "appearance.time_zone"},
		{"local zone", SettingsPatch{Appearance: &AppearancePatch{TimeZone: s("Local")}}, "appearance.time_zone"},
		{"zone traversal", SettingsPatch{Appearance: &AppearancePatch{TimeZone: s("../../etc/passwd")}}, "appearance.time_zone"},
		{"essential off", SettingsPatch{Privacy: &PrivacyPatch{Consents: map[string]bool{"essential": false}}}, "privacy.consents.essential"},
		{"unknown consent", SettingsPatch{Privacy: &PrivacyPatch{Consents: map[string]bool{"tracking": true}}}, "privacy.consents.tracking"},
		{"valid zone", SettingsPatch{Appearance: &AppearancePatch{TimeZone: s("America/Argentina/Buenos_Aires")}}, ""},
		{"clear zone", SettingsPatch{Appearance: &AppearancePatch{TimeZone: s("")}}, ""},
		{"upper locale", SettingsPatch{Appearance: &AppearancePatch{Locale: s("FR")}}, ""},
		{"known place", SettingsPatch{Profile: &ProfileSettingsPatch{BirthPlaceID: s("paris-fr")}}, ""},
		{"future geonames id", SettingsPatch{Profile: &ProfileSettingsPatch{BirthPlaceID: s("gn:2988507"), BirthPlaceLabel: s("Paris, Île-de-France")}}, ""},
		{"whole sign", SettingsPatch{Profile: &ProfileSettingsPatch{HouseSystem: s("whole_sign")}}, ""},
		{"consents", SettingsPatch{Privacy: &PrivacyPatch{Consents: map[string]bool{"essential": true, "analytics": true}}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newIdentityFixture(t)
			before := f.db.clone()
			_, err := f.svc.UpdateSettings(ctx, f.user.ID, tc.patch)
			if tc.field == "" {
				if err != nil {
					t.Fatalf("valid patch rejected: %v", err)
				}
				return
			}
			fields := fieldsOf(t, err)
			if _, ok := fields[tc.field]; !ok || len(fields) != 1 {
				t.Fatalf("fields %v, want only %s", fields, tc.field)
			}
			after := f.db.users[f.user.ID]
			if tc.name == "under age" { // revisions 5.0: the refusal persists the restriction (§24.6)
				if after.AgeRestrictedAt == nil {
					t.Fatal("an under-age date must restrict the account")
				}
				after.AgeRestrictedAt = nil
			}
			if f.db.profiles[f.user.ID].version != before.profiles[f.user.ID].version || after != before.users[f.user.ID] {
				t.Fatal("a rejected patch must not write anything")
			}
		})
	}

	// Every problem is reported at once, and nothing is written.
	f := newIdentityFixture(t)
	_, err := f.svc.UpdateSettings(ctx, f.user.ID, SettingsPatch{Account: &AccountSettingsPatch{DisplayName: s("Valid Name")},
		Profile: &ProfileSettingsPatch{BirthTime: s("99:99")}, Appearance: &AppearancePatch{Theme: s("Neon"), TimeZone: s("Nowhere/City")}})
	if fields := fieldsOf(t, err); len(fields) != 3 {
		t.Fatalf("all problems reported: %v", fields)
	}
	if f.db.users[f.user.ID].DisplayName != "Moon" {
		t.Fatal("valid fields of a rejected patch were written")
	}
}

func TestSettingsSaveAndRoundTrip(t *testing.T) {
	ctx := context.Background()
	f := newIdentityFixture(t)
	s := func(v string) *string { return &v }
	b := func(v bool) *bool { return &v }
	v, err := f.svc.Settings(ctx, f.user.ID)
	if err != nil || v.Version != 1 || v.Profile.HouseSystem != "placidus" || !v.Notifications.Email || v.Privacy.Consents["personalization"] != true ||
		v.Privacy.Consents["analytics"] {
		t.Fatalf("initial settings: %+v %v", v, err)
	}
	one := 1
	v, err = f.svc.UpdateSettings(ctx, f.user.ID, SettingsPatch{ExpectedVersion: &one,
		Account:       &AccountSettingsPatch{DisplayName: s(" Luna ")},
		Profile:       &ProfileSettingsPatch{BirthDate: s("1990-05-17"), BirthTime: s("07:45"), BirthPlaceID: s("paris-fr"), HouseSystem: s("equal")},
		Appearance:    &AppearancePatch{Theme: s("Light Cream"), Locale: s("fr"), TimeZone: s("Europe/Paris")},
		Notifications: &NotificationsPatch{Email: b(false), Push: b(true), Marketing: b(true)},
		Privacy:       &PrivacyPatch{ShareBirthDetailsWithAdvisors: b(true), Consents: map[string]bool{"analytics": true, "personalization": false}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.Version != 2 || v.Account.DisplayName != "Luna" || v.Appearance.Locale != "fr" || v.Appearance.TimeZone != "Europe/Paris" ||
		v.Appearance.Theme != "Light Cream" || v.Profile.BirthDate != "1990-05-17" || v.Profile.BirthPlace != "Paris, France" ||
		v.Profile.BirthTimezone != "Europe/Paris" || v.Profile.CountryCode != "FR" || v.Profile.HouseSystem != "equal" ||
		v.Notifications.Email || !v.Notifications.Push || !v.Notifications.Marketing || !v.Privacy.ShareBirthDetailsWithAdvisors ||
		!v.Privacy.Consents["analytics"] || v.Privacy.Consents["personalization"] {
		t.Fatalf("saved settings: %+v", v)
	}
	if f.db.users[f.user.ID].Locale != "fr" {
		t.Fatal("locale must persist to users.locale")
	}
	if len(f.db.events) != 2 {
		t.Fatalf("consent history: %d events", len(f.db.events))
	}
	// An unresolvable GeoNames id is kept with the client's label.
	v, err = f.svc.UpdateSettings(ctx, f.user.ID, SettingsPatch{Profile: &ProfileSettingsPatch{BirthPlaceID: s("gn:3117735"), BirthPlaceLabel: s("Madrid, Spain")}})
	if err != nil || v.Profile.BirthPlaceID != "gn:3117735" || v.Profile.BirthPlace != "Madrid, Spain" || v.Profile.BirthLatitude != nil ||
		v.Profile.BirthTimezone != "" {
		t.Fatalf("geonames id: %+v %v", v.Profile, err)
	}
	// Clearing the birthplace clears everything derived from it.
	v, _ = f.svc.UpdateSettings(ctx, f.user.ID, SettingsPatch{Profile: &ProfileSettingsPatch{BirthPlaceID: s("")}})
	if v.Profile.BirthPlaceID != "" || v.Profile.BirthPlace != "" || v.Profile.CountryCode != "" {
		t.Fatalf("cleared place: %+v", v.Profile)
	}
}

func TestSettingsVersionConflict(t *testing.T) {
	ctx := context.Background()
	f := newIdentityFixture(t)
	name := func(v string) SettingsPatch { return SettingsPatch{Account: &AccountSettingsPatch{DisplayName: &v}} }
	one, two := 1, 2
	p := name("Tab A")
	p.ExpectedVersion = &one
	if v, err := f.svc.UpdateSettings(ctx, f.user.ID, p); err != nil || v.Version != 2 {
		t.Fatalf("first save: %v", err)
	}
	p = name("Tab B")
	p.ExpectedVersion = &one // stale
	_, err := f.svc.UpdateSettings(ctx, f.user.ID, p)
	ae, ok := apperror.As(err)
	if !ok || ae.Code != apperror.CodeVersionConflict || ae.Status != http.StatusConflict || ae.Details["current_version"] != 2 {
		t.Fatalf("stale version: %v", err)
	}
	if f.db.users[f.user.ID].DisplayName != "Tab A" {
		t.Fatal("a conflicting save must not write")
	}
	// Without expected_version the last write wins.
	if v, err := f.svc.UpdateSettings(ctx, f.user.ID, name("Tab C")); err != nil || v.Version != 3 {
		t.Fatalf("unversioned save: %v", err)
	}
	// An empty patch with a stale version still reports the conflict.
	if _, err := f.svc.UpdateSettings(ctx, f.user.ID, SettingsPatch{ExpectedVersion: &two}); codeOf(err) != apperror.CodeVersionConflict {
		t.Fatalf("empty stale patch: %v", err)
	}
}

// A failure late in the transaction (the consent write) rolls back the
// user and profile updates made before it.
func TestSettingsAtomicity(t *testing.T) {
	ctx := context.Background()
	f := newIdentityFixture(t)
	f.db.failConsentUpsert = true
	name, tz := "Changed", "Europe/Paris"
	_, err := f.svc.UpdateSettings(ctx, f.user.ID, SettingsPatch{Account: &AccountSettingsPatch{DisplayName: &name},
		Appearance: &AppearancePatch{TimeZone: &tz}, Privacy: &PrivacyPatch{Consents: map[string]bool{"analytics": true}}})
	if err == nil {
		t.Fatal("expected the save to fail")
	}
	if u := f.db.users[f.user.ID]; u.DisplayName != "Moon" {
		t.Fatalf("user update not rolled back: %q", u.DisplayName)
	}
	if p := f.db.profiles[f.user.ID]; p.Timezone != "" || p.version != 1 {
		t.Fatalf("profile update not rolled back: %+v", p)
	}
	if len(f.db.events) != 0 || len(f.db.consents) != 0 {
		t.Fatal("consent rows left behind")
	}
}

func TestProfileAliasKeepsLegacyContract(t *testing.T) {
	ctx := context.Background()
	f := newIdentityFixture(t)
	f.svc.d.UserSvc = &UserService{users: memUsers{db: f.db}, wallets: memWallets{}}
	long, locale := strings.Repeat("x", 61), "de"
	_, err := f.svc.UpdateProfile(ctx, f.user.ID, ProfileUpdate{Location: &long})
	if fields := fieldsOf(t, err); fields["location"] == nil {
		t.Fatalf("alias must keep flat field names: %v", fields)
	}
	if _, err := f.svc.UpdateProfile(ctx, f.user.ID, ProfileUpdate{Locale: &locale}); err != nil {
		t.Fatalf("lenient locale: %v", err)
	}
	if f.db.users[f.user.ID].Locale != "en" || f.db.profiles[f.user.ID].version != 2 {
		t.Fatalf("alias save: locale %q version %d", f.db.users[f.user.ID].Locale, f.db.profiles[f.user.ID].version)
	}
}

func TestEmailChangeTokenLifecycle(t *testing.T) {
	ctx := context.Background()
	f := newIdentityFixture(t)
	uid := f.user.ID
	actions := memActions{db: f.db}

	if err := f.svc.RequestEmailChange(ctx, uid, EmailChangeInput{Email: "new@example.com", Password: "wrong"}); fieldsOf(t, err)["password"] == nil {
		t.Fatal("wrong password accepted")
	}
	if err := f.svc.RequestEmailChange(ctx, uid, EmailChangeInput{Email: "MOON@example.com", Password: testPassword}); fieldsOf(t, err)["email"] == nil {
		t.Fatal("same address accepted")
	}

	// First request, then a second one: only the latest confirmation works.
	if err := f.svc.RequestEmailChange(ctx, uid, EmailChangeInput{Email: "first@example.com", Password: testPassword}); err != nil {
		t.Fatal(err)
	}
	firstConfirm := linkToken(f.mail.byTemplate("email_change_confirm")[0])
	if err := f.svc.RequestEmailChange(ctx, uid, EmailChangeInput{Email: "new@example.com", Password: testPassword}); err != nil {
		t.Fatal(err)
	}
	confirms, notices := f.mail.byTemplate("email_change_confirm"), f.mail.byTemplate("email_change_notice")
	if len(confirms) != 2 || confirms[1].To != "new@example.com" || len(notices) != 2 || notices[1].To != "moon@example.com" ||
		confirms[1].Category != domain.EmailSecurity || !strings.Contains(confirms[1].Content.Action.URL, "/en/confirm-email?token=") {
		t.Fatalf("mails: %+v", f.mail.sent)
	}
	for _, tk := range f.db.actions {
		if tk.PayloadEnc == "" || strings.Contains(tk.PayloadEnc, "example.com") {
			t.Fatal("token payloads must be sealed")
		}
	}
	if v, _ := f.svc.Settings(ctx, uid); v.Account.PendingEmail != "n***@example.com" {
		t.Fatalf("pending email: %q", v.Account.PendingEmail)
	}
	if _, err := f.svc.ConfirmEmailChange(ctx, firstConfirm); codeOf(err) != apperror.CodeInvalidToken {
		t.Fatalf("superseded token: %v", err)
	}

	confirm := linkToken(confirms[1])
	res, err := f.svc.ConfirmEmailChange(ctx, confirm)
	if err != nil || res.Status != "confirmed" {
		t.Fatalf("confirm: %v", err)
	}
	u := f.db.users[uid]
	if u.Email != "new@example.com" || u.EmailVerifiedAt == nil || u.TokenVersion != 1 || f.db.revoked[uid] != 1 {
		t.Fatalf("after confirm: %+v revoked=%d", u, f.db.revoked[uid])
	}
	if _, err := f.svc.ConfirmEmailChange(ctx, confirm); codeOf(err) != apperror.CodeInvalidToken {
		t.Fatalf("confirmation links are single use: %v", err)
	}
	changed := f.mail.byTemplate("email_changed")
	if len(changed) != 1 || changed[0].To != "moon@example.com" {
		t.Fatalf("restore notice: %+v", changed)
	}

	// The previous owner restores the address: sessions end, a reset link
	// is mailed.
	res, err = f.svc.CancelEmailChange(ctx, linkToken(changed[0]))
	if err != nil || res.Status != "restored" {
		t.Fatalf("restore: %v", err)
	}
	if u := f.db.users[uid]; u.Email != "moon@example.com" || u.TokenVersion != 2 || f.db.revoked[uid] != 2 || len(f.db.resets) != 1 {
		t.Fatalf("after restore: %+v", u)
	}
	if restored := f.mail.byTemplate("email_restored"); len(restored) != 1 || !strings.Contains(restored[0].Content.Action.URL, "/reset-password?token=") {
		t.Fatalf("reset mail: %+v", restored)
	}

	// Expiry: a link used after its lifetime fails.
	if err := f.svc.RequestEmailChange(ctx, uid, EmailChangeInput{Email: "late@example.com", Password: testPassword}); err != nil {
		t.Fatal(err)
	}
	late := linkToken(f.mail.byTemplate("email_change_confirm")[2])
	f.clock.Advance(31 * time.Minute)
	if _, err := f.svc.ConfirmEmailChange(ctx, late); codeOf(err) != apperror.CodeInvalidToken {
		t.Fatalf("expired link: %v", err)
	}
	// Cancelling before confirmation just kills the pending change.
	if err := f.svc.RequestEmailChange(ctx, uid, EmailChangeInput{Email: "other@example.com", Password: testPassword}); err != nil {
		t.Fatal(err)
	}
	notice := f.mail.byTemplate("email_change_notice")
	res, err = f.svc.CancelEmailChange(ctx, linkToken(notice[len(notice)-1]))
	if err != nil || res.Status != "cancelled" || len(actions.open(uid, domain.ActionEmailChange)) != 0 {
		t.Fatalf("cancel: %v %+v", err, res)
	}
	for _, bad := range []string{"", strings.Repeat("x", 200), "unknown"} {
		if _, err := f.svc.ConfirmEmailChange(ctx, bad); codeOf(err) != apperror.CodeInvalidToken {
			t.Fatalf("bad token %q: %v", bad, err)
		}
	}
}

// An address that already has an account gets a notice; the response is
// the same and no token is created (no enumeration).
func TestEmailChangeToTakenAddress(t *testing.T) {
	ctx := context.Background()
	f := newIdentityFixture(t)
	other := f.addUser(t, "taken@example.com", domain.RoleClient)
	if err := f.svc.RequestEmailChange(ctx, f.user.ID, EmailChangeInput{Email: "Taken@Example.com", Password: testPassword}); err != nil {
		t.Fatalf("taken address must look like success: %v", err)
	}
	attempts := f.mail.byTemplate("email_change_attempt")
	if len(attempts) != 1 || attempts[0].To != "taken@example.com" || *attempts[0].UserID != other.ID || len(f.mail.byTemplate("email_change_confirm")) != 0 {
		t.Fatalf("mails: %+v", f.mail.sent)
	}
	if len(f.db.actions) != 0 {
		t.Fatal("no token for a taken address")
	}
}

func TestCloseValidation(t *testing.T) {
	ctx := context.Background()
	f := newIdentityFixture(t)
	_, err := f.svc.Close(ctx, f.user.ID, CloseInput{Password: "nope", Confirmation: "delete", Reason: strings.Repeat("r", 501)})
	fields := fieldsOf(t, err)
	if fields["password"] == nil || fields["confirmation"] == nil || fields["reason"] == nil {
		t.Fatalf("close validation: %v", fields)
	}
	admin := f.addUser(t, "admin@example.com", domain.RoleAdmin)
	if _, err := f.svc.Close(ctx, admin.ID, CloseInput{Password: testPassword, Confirmation: "DELETE"}); codeOf(err) != apperror.CodeForbidden {
		t.Fatalf("admins cannot self-close: %v", err)
	}
	if err := f.svc.Deactivate(ctx, admin.ID, PasswordInput{Password: testPassword}); codeOf(err) != apperror.CodeForbidden {
		t.Fatalf("admins cannot self-deactivate: %v", err)
	}
	v, err := f.svc.Close(ctx, f.user.ID, CloseInput{Password: testPassword, Confirmation: "DELETE", Reason: "Moving on"})
	if err != nil || !v.DeletionScheduledAt.Equal(f.clock.Now().Add(30*24*time.Hour)) {
		t.Fatalf("close: %+v %v", v, err)
	}
	u := f.db.users[f.user.ID]
	if u.Status != domain.UserDeactivated || u.DeactivatedBy != domain.DeactivatedByClosure || u.ClosureScheduledAt == nil || f.db.revoked[u.ID] != 1 {
		t.Fatalf("closing account: %+v", u)
	}
	keep := f.mail.byTemplate("closure_scheduled")
	if len(keep) != 1 || !strings.Contains(keep[0].Content.Action.URL, "/en/keep-account?token=") {
		t.Fatalf("closure mail: %+v", keep)
	}
	// The e-mailed link keeps the account (single use).
	tok := linkToken(keep[0])
	if err := f.svc.CancelClosure(ctx, tok); err != nil {
		t.Fatal(err)
	}
	if u := f.db.users[f.user.ID]; u.Status != domain.UserActive || u.DeactivatedBy != "" || u.ClosureScheduledAt != nil {
		t.Fatalf("kept account: %+v", u)
	}
	if err := f.svc.CancelClosure(ctx, tok); codeOf(err) != apperror.CodeInvalidToken {
		t.Fatalf("keep link reuse: %v", err)
	}
}

func TestConsentPolicyAndChoices(t *testing.T) {
	ctx := context.Background()
	f := newIdentityFixture(t)
	pol := f.consent.Policy()
	want := map[string][2]any{"essential": {true, "contract"}, "analytics": {false, "consent"},
		"marketing_attribution": {false, "consent"}, "personalization": {true, "legitimate_interest"}}
	if pol.Version != ConsentPolicyVersion || len(pol.Categories) != 4 || !pol.HonorGPC {
		t.Fatalf("policy: %+v", pol)
	}
	for _, c := range pol.Categories {
		w := want[c.Category]
		if c.Default != w[0] || c.LegalBasis != w[1] || c.Required != (c.Category == "essential") {
			t.Errorf("category %+v", c)
		}
	}
	// Defaults apply until the user decides.
	if ok, _ := f.consent.NoteSharing(ctx, f.user.ID); !ok {
		t.Fatal("personalization defaults to allowed")
	}
	v, err := f.consent.Set(ctx, f.user.ID, ConsentInput{Consents: map[string]bool{"personalization": false, "analytics": false}, Source: "banner"})
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := f.consent.NoteSharing(ctx, f.user.ID); ok {
		t.Fatal("withdrawn personalization must stop note sharing")
	}
	if len(f.db.events) != 2 || len(v.Items) != 4 || v.Items[0].Category != "essential" || !v.Items[0].Granted {
		t.Fatalf("view: %+v events %d", v, len(f.db.events))
	}
	// Re-sending the same choices records nothing new.
	if _, err := f.consent.Set(ctx, f.user.ID, ConsentInput{Consents: map[string]bool{"personalization": false}}); err != nil || len(f.db.events) != 2 {
		t.Fatalf("unchanged choice recorded again: %v %d", err, len(f.db.events))
	}
	for _, bad := range []ConsentInput{{}, {Consents: map[string]bool{"essential": false}}, {Consents: map[string]bool{"x": true}},
		{Consents: map[string]bool{"analytics": true}, Source: "import"}} {
		if _, err := f.consent.Set(ctx, f.user.ID, bad); codeOf(err) != apperror.CodeValidation {
			t.Errorf("invalid input %+v accepted: %v", bad, err)
		}
	}
	// Audit trail of the change.
	if len(f.db.audit) != 1 || f.db.audit[0].Action != "consent.updated" {
		t.Fatalf("audit: %+v", f.db.audit)
	}
}

func TestExportArchive(t *testing.T) {
	uid := uuid.New()
	at := time.Date(2026, 11, 5, 9, 30, 0, 0, time.UTC)
	img := bytes.Repeat([]byte{0xFF, 0xD8, 0xFF}, 100)
	b := &ExportBundle{UserID: uid, Locale: "fr", GeneratedAt: at,
		Documents: []ExportDocument{{Path: "account.json", Data: map[string]any{"email": "moon@example.com"}},
			{Path: "chat/messages.json", Data: []map[string]any{{"from": "you", "body": "Hello"}}}},
		Blobs:   []ExportBlob{{Path: "chat/attachments/a.jpg", Data: img}},
		Omitted: []string{"Only the first 5 chat messages are included."}}
	var buf bytes.Buffer
	if err := WriteExportArchive(&buf, b); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	var order []string
	for _, f := range zr.File {
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = data
		order = append(order, f.Name)
		if !f.Modified.Equal(at) {
			t.Errorf("%s: modified %v", f.Name, f.Modified)
		}
	}
	if strings.Join(order, ",") != "README.txt,account.json,chat/messages.json,chat/attachments/a.jpg,manifest.json" {
		t.Fatalf("archive layout: %v", order)
	}
	if !bytes.Equal(files["chat/attachments/a.jpg"], img) {
		t.Fatal("blob altered")
	}
	readme := string(files["README.txt"])
	if !strings.Contains(readme, "Cette archive contient") || !strings.Contains(readme, "notes que les conseillers") ||
		!strings.Contains(readme, "Seuls les 5 premiers messages de chat sont inclus.") {
		t.Fatalf("localized readme:\n%s", readme)
	}
	var m exportManifest
	if err := json.Unmarshal(files["manifest.json"], &m); err != nil {
		t.Fatal(err)
	}
	if m.Format != ExportFormat || m.UserID != uid || len(m.Files) != 4 || len(m.Excluded) != 2 || len(m.Omitted) != 1 {
		t.Fatalf("manifest: %+v", m)
	}
	for _, mf := range m.Files {
		sum := sha256.Sum256(files[mf.Path])
		if hex.EncodeToString(sum[:]) != mf.SHA256 || len(files[mf.Path]) != mf.Bytes {
			t.Errorf("manifest entry %s does not match", mf.Path)
		}
	}
	var account map[string]any
	if err := json.Unmarshal(files["account.json"], &account); err != nil || account["email"] != "moon@example.com" {
		t.Fatalf("account.json: %v %v", account, err)
	}
	for _, bad := range []*ExportBundle{
		{Documents: []ExportDocument{{Path: "../escape.json", Data: 1}}},
		{Documents: []ExportDocument{{Path: "/abs.json", Data: 1}}},
		{Documents: []ExportDocument{{Path: "a.json", Data: 1}, {Path: "a.json", Data: 2}}},
		{Documents: []ExportDocument{{Path: "manifest.json", Data: 1}}},
		{Documents: []ExportDocument{{Path: "notes.txt", Data: 1}}},
		{Blobs: []ExportBlob{{Path: "chat/attachments/x y.jpg", Data: img}}},
	} {
		if err := WriteExportArchive(io.Discard, bad); err == nil {
			t.Errorf("unsafe archive accepted: %+v %+v", bad.Documents, bad.Blobs)
		}
	}
}

func TestAccountEraser(t *testing.T) {
	ctx := context.Background()
	f := newIdentityFixture(t)
	calls := &eraseCalls{exportKeys: []string{"exports/u/j/x.zip"}}
	avatars, exports := storage.NewMemory(""), storage.NewMemory("")
	_, _ = avatars.Put(ctx, "avatars/u/a.jpg", "image/jpeg", []byte("img"), "")
	_, _ = exports.Put(ctx, "exports/u/j/x.zip", "application/zip", []byte("zip"), "")
	mp := f.db.profiles[f.user.ID]
	mp.AvatarKey, mp.Nickname, mp.Timezone, mp.ShareBirthDetailsWithAdvisors = "avatars/u/a.jpg", "Luna", "Europe/Paris", true
	f.db.profiles[f.user.ID] = mp
	_ = memActions{db: f.db}.Create(ctx, &domain.ActionToken{ID: uuid.New(), UserID: f.user.ID, Purpose: domain.ActionReactivate,
		TokenHash: strings.Repeat("a", 64), ExpiresAt: f.clock.Now().Add(time.Hour)})
	f.db.consents[consentKey(f.user.ID, domain.ConsentAnalytics)] = domain.Consent{UserID: f.user.ID, Category: domain.ConsentAnalytics, Granted: true}
	deps := EraserDeps{Tx: memTx{db: f.db}, Users: memUsers{db: f.db}, Tokens: memTokens{db: f.db}, Psychics: memPsychics{db: f.db},
		Hasher: f.hasher, Clock: f.clock, Push: memPush{calls: calls}, Actions: memActions{db: f.db}, Exports: memExports{calls: calls},
		Sessions: memSessions{calls: calls}, Consents: memConsents{db: f.db}, AvatarStore: avatars, ExportStore: exports, Mail: f.mail}

	// A failing billing hook stops the erasure before anything changes.
	blocked := NewAccountEraser(deps)
	blocked.SetBilling(failingBilling{err: errors.New("provider down")})
	if _, err := blocked.Erase(ctx, f.user.ID, EraseRequest{Reason: "test"}); codeOf(err) != apperror.CodeUnavailable {
		t.Fatalf("billing failure: %v", err)
	}
	if f.db.users[f.user.ID].Status != domain.UserActive {
		t.Fatal("erased despite the billing failure")
	}
	// The closure worker only erases accounts whose date has passed.
	e := NewAccountEraser(deps)
	if _, err := e.Erase(ctx, f.user.ID, EraseRequest{OnlyClosureDue: true}); !errors.Is(err, ErrNotErasable) {
		t.Fatalf("not due: %v", err)
	}

	before, err := e.Erase(ctx, f.user.ID, EraseRequest{Reason: "self_closure", Notify: true})
	if err != nil || before.Email != "moon@example.com" {
		t.Fatalf("erase: %v", err)
	}
	u, p := f.db.users[f.user.ID], f.db.profiles[f.user.ID]
	if u.Status != domain.UserDeleted || u.Email != "deleted+"+u.ID.String()+"@deleted.invalid" || u.DisplayName != "Deleted user" ||
		u.TokenVersion != 1 || u.DeletedAt == nil || u.PasswordHash == f.user.PasswordHash || f.db.revoked[u.ID] != 1 {
		t.Fatalf("user not anonymised: %+v", u)
	}
	if p.Nickname != "" || p.AvatarKey != "" || p.Timezone != "" || p.ShareBirthDetailsWithAdvisors || p.BirthDate != nil || p.NotifyEmail {
		t.Fatalf("profile not scrubbed: %+v", p.Profile)
	}
	if calls.push != 1 || calls.exports != 1 || calls.sessions != 1 || len(f.db.actions) != 0 || len(f.db.consents) != 0 {
		t.Fatalf("extras: %+v actions=%d consents=%d", calls, len(f.db.actions), len(f.db.consents))
	}
	if _, ok := avatars.Lookup("avatars/u/a.jpg"); ok {
		t.Fatal("avatar object kept")
	}
	if _, ok := exports.Lookup("exports/u/j/x.zip"); ok {
		t.Fatal("export object kept")
	}
	closed := f.mail.byTemplate("account_closed")
	if len(closed) != 1 || closed[0].To != "moon@example.com" || closed[0].Category != domain.EmailSecurity {
		t.Fatalf("closing mail: %+v", closed)
	}
	if _, err := e.Erase(ctx, f.user.ID, EraseRequest{Reason: "again"}); !errors.Is(err, ErrNotErasable) {
		t.Fatalf("second erase: %v", err)
	}
}

// Signing a device out kills its access token at once (cache marker read
// together with the status); without the cache the database answers.
func TestAuthenticateChecksDeviceRevocation(t *testing.T) {
	ctx := context.Background()
	f := newIdentityFixture(t)
	auth := f.svc.d.Auth
	devices := &memDevices{revoked: map[uuid.UUID]bool{}, touched: map[uuid.UUID]time.Time{}}
	auth.WithIdentity(devices, memActions{db: f.db})
	issue := func(fam uuid.UUID) string {
		tok, _, err := auth.issuer.Issue(f.user.ID, string(domain.RoleClient), fam, f.db.users[f.user.ID].TokenVersion)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	famA, famB := uuid.New(), uuid.New()
	tokA, tokB := issue(famA), issue(famB)
	p, err := auth.Authenticate(ctx, tokA)
	if err != nil || p.FamilyID != famA || p.UserID != f.user.ID {
		t.Fatalf("authenticate: %+v %v", p, err)
	}
	if _, ok := devices.touched[famA]; !ok {
		t.Fatal("first request of the minute records activity")
	}
	delete(devices.touched, famA)
	if _, err := auth.Authenticate(ctx, tokA); err != nil || len(devices.touched) != 0 {
		t.Fatalf("activity recorded twice within a minute: %v", err)
	}

	auth.afterFamilyRevoked(ctx, f.user.ID, famA)
	if _, err := auth.Authenticate(ctx, tokA); codeOf(err) != apperror.CodeSessionExpired {
		t.Fatalf("revoked device: %v", err)
	}
	if _, err := auth.Authenticate(ctx, tokB); err != nil {
		t.Fatalf("other device: %v", err)
	}

	// Cache unreachable: the device row decides.
	auth.kv = downCache{}
	devices.revoked[famB] = true
	if _, err := auth.Authenticate(ctx, tokB); codeOf(err) != apperror.CodeSessionExpired {
		t.Fatalf("database fallback (revoked): %v", err)
	}
	devices.revoked[famB] = false
	if _, err := auth.Authenticate(ctx, tokB); err != nil {
		t.Fatalf("database fallback (live): %v", err)
	}
	// If the marker cannot be written, every access token is invalidated
	// instead (other devices refresh).
	auth.afterFamilyRevoked(ctx, f.user.ID, famB)
	if f.db.users[f.user.ID].TokenVersion != 1 {
		t.Fatal("token_version not bumped when the revocation marker failed")
	}
	if _, err := auth.Authenticate(ctx, issue(uuid.New())); err != nil {
		t.Fatalf("fresh token after the bump: %v", err)
	}
}
