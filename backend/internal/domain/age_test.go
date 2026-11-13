package domain

import (
	"testing"
	"time"
)

// revisions 5.0: 18+ age gate (§24.6).
func TestAgeStatusOf(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name string
		role Role
		user *User
		want AgeStatus
	}{
		{"client without a date", RoleClient, &User{}, AgeBirthDateRequired},
		{"nil user", RoleClient, nil, AgeBirthDateRequired},
		{"confirmed client", RoleClient, &User{AdultConfirmedAt: &now}, AgeAdult},
		{"restricted client", RoleClient, &User{AgeRestrictedAt: &now}, AgeRestricted},
		{"restriction wins over confirmation", RoleClient, &User{AdultConfirmedAt: &now, AgeRestrictedAt: &now}, AgeRestricted},
		{"advisor", RolePsychic, &User{}, AgeAdult},
		{"advisor with a stale restriction", RolePsychic, &User{AgeRestrictedAt: &now}, AgeAdult},
		{"admin", RoleAdmin, &User{}, AgeAdult},
	} {
		if got := AgeStatusOf(tc.role, tc.user); got != tc.want {
			t.Errorf("%s: %s, want %s", tc.name, got, tc.want)
		}
	}
	u := &User{Role: RoleClient, AdultConfirmedAt: &now}
	if u.AgeStatus() != AgeAdult {
		t.Error("User.AgeStatus")
	}
	for _, s := range []AgeStatus{AgeAdult, AgeBirthDateRequired, AgeRestricted} {
		if !s.Valid() {
			t.Errorf("%s must be valid", s)
		}
	}
	if AgeStatus("").Valid() || AgeStatus("x").Valid() {
		t.Error("unknown statuses are invalid")
	}
}
