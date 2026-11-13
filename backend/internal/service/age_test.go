package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 5.0: 18+ age gate (§24.6).

func TestUnderMinimumAge(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"2015-01-01", true},
		{"2008-11-13", true},  // turns 18 tomorrow
		{"2008-11-12", false}, // turns 18 today
		{"1990-05-17", false},
		{"", false},
		{"not a date", false},
		{"2030-01-01", false}, // future: a validation error, not a restriction
		{"1800-01-01", false}, // implausible: same
		{"2000-02-30", false},
	} {
		if got := UnderMinimumAge(tc.in, now, 18); got != tc.want {
			t.Errorf("UnderMinimumAge(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	if UnderMinimumAge("2015-01-01", now, 0) {
		t.Error("a minimum age of 0 never restricts")
	}
}

func TestStatusEntryCarriesAge(t *testing.T) {
	for _, age := range []domain.AgeStatus{domain.AgeAdult, domain.AgeBirthDateRequired, domain.AgeRestricted} {
		e := statusEntry{Status: domain.UserActive, Role: domain.RoleClient, TokenVersion: 3, Age: age}
		got, ok := decodeStatus(e.encode())
		if !ok || got != e {
			t.Fatalf("%s round trip: %+v %v", age, got, ok)
		}
	}
	// Entries cached before the age gate (three parts) are a miss, so the
	// status is reloaded from the database with its age.
	if _, ok := decodeStatus("active|client|3"); ok {
		t.Fatal("a legacy cache entry must be treated as a miss")
	}
	if _, ok := decodeStatus("active|client|3|bogus"); ok {
		t.Fatal("an unknown age must be treated as a miss")
	}
}

func ageOf(t *testing.T, f *identityFixture) domain.AgeStatus {
	t.Helper()
	u := f.db.users[f.user.ID]
	return u.AgeStatus()
}

func errCode(err error) apperror.Code {
	if e, ok := apperror.As(err); ok {
		return e.Code
	}
	return ""
}

func TestBirthDateSavesDriveTheAgeStatus(t *testing.T) {
	ctx := context.Background()
	s := func(v string) *string { return &v }
	f := newIdentityFixture(t)
	if got := ageOf(t, f); got != domain.AgeBirthDateRequired {
		t.Fatalf("a new client without a date: %s", got)
	}

	// An adult date confirms (and the cache is dropped through Auth).
	if _, err := f.svc.UpdateSettings(ctx, f.user.ID, SettingsPatch{Profile: &ProfileSettingsPatch{BirthDate: s("1990-05-17")}}); err != nil {
		t.Fatal(err)
	}
	if got := ageOf(t, f); got != domain.AgeAdult {
		t.Fatalf("after an adult date: %s", got)
	}
	if f.db.users[f.user.ID].AdultConfirmedAt == nil {
		t.Fatal("adult_confirmed_at not set")
	}
}

func TestUnderAgeDateRestrictsAndSticks(t *testing.T) {
	ctx := context.Background()
	s := func(v string) *string { return &v }
	for name, save := range map[string]func(f *identityFixture, date string) error{
		"settings": func(f *identityFixture, d string) error {
			_, err := f.svc.UpdateSettings(ctx, f.user.ID, SettingsPatch{Profile: &ProfileSettingsPatch{BirthDate: s(d)}})
			return err
		},
		"profile alias": func(f *identityFixture, d string) error {
			_, err := f.svc.UpdateProfile(ctx, f.user.ID, ProfileUpdate{BirthDate: s(d)})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newIdentityFixture(t)
			err := save(f, "2015-01-01")
			if e, ok := apperror.As(err); !ok || e.Status != http.StatusUnprocessableEntity || e.Code != apperror.CodeValidation {
				t.Fatalf("under-age date: %v", err)
			}
			if got := ageOf(t, f); got != domain.AgeRestricted {
				t.Fatalf("the refusal must persist the restriction: %s", got)
			}
			if f.db.profiles[f.user.ID].BirthDate != nil {
				t.Fatal("the under-age date must not be saved")
			}
			// No retry with another date.
			err = save(f, "1990-05-17")
			if errCode(err) != apperror.CodeAgeRestricted {
				t.Fatalf("retry with an adult date: %v", err)
			}
			if got := ageOf(t, f); got != domain.AgeRestricted {
				t.Fatalf("still restricted: %s", got)
			}
		})
	}
}

func TestRestrictedAccountMayResendTheDateOnFile(t *testing.T) {
	ctx := context.Background()
	s := func(v string) *string { return &v }
	f := newIdentityFixture(t)
	if _, err := f.svc.UpdateSettings(ctx, f.user.ID, SettingsPatch{Profile: &ProfileSettingsPatch{BirthDate: s("1990-05-17")}}); err != nil {
		t.Fatal(err)
	}
	// An admin restricts the account; the settings form still re-sends the date.
	u := f.db.users[f.user.ID]
	now := f.clock.Now()
	u.AgeRestrictedAt = &now
	f.db.users[f.user.ID] = u
	if _, err := f.svc.UpdateSettings(ctx, f.user.ID, SettingsPatch{Profile: &ProfileSettingsPatch{BirthDate: s("1990-05-17"), Nickname: s("Moon")}}); err != nil {
		t.Fatalf("re-sending the unchanged date: %v", err)
	}
	if f.db.profiles[f.user.ID].Nickname != "Moon" {
		t.Fatal("other fields still save")
	}
	if _, err := f.svc.UpdateSettings(ctx, f.user.ID, SettingsPatch{Profile: &ProfileSettingsPatch{BirthDate: s("1985-01-01")}}); errCode(err) != apperror.CodeAgeRestricted {
		t.Fatalf("changing the date: %v", err)
	}
	if _, err := f.svc.UpdateSettings(ctx, f.user.ID, SettingsPatch{Profile: &ProfileSettingsPatch{BirthDate: s("")}}); errCode(err) != apperror.CodeAgeRestricted {
		t.Fatalf("clearing the date is a change too: %v", err)
	}
	if got := ageOf(t, f); got != domain.AgeRestricted {
		t.Fatalf("status: %s", got)
	}
}

func TestAdvisorsAreNeverRestrictedByADate(t *testing.T) {
	ctx := context.Background()
	f := newIdentityFixture(t)
	adv := f.addUser(t, "advisor@example.com", domain.RolePsychic)
	d := "2015-01-01"
	_, err := f.svc.UpdateSettings(ctx, adv.ID, SettingsPatch{Profile: &ProfileSettingsPatch{BirthDate: &d}})
	if errCode(err) != apperror.CodeValidation {
		t.Fatalf("an invalid date is still refused: %v", err)
	}
	if u := f.db.users[adv.ID]; u.AgeRestrictedAt != nil || u.AgeStatus() != domain.AgeAdult {
		t.Fatalf("advisor status: %+v", u)
	}
}
