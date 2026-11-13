package domain

// AgeStatus is where an account stands on the 18+ gate (revisions 5.0, §24.6).
type AgeStatus string

// Age statuses.
const (
	// AgeAdult: the account may use every feature (a date of birth that
	// meets the minimum age was saved, or the role is vetted by Staryield).
	AgeAdult AgeStatus = "adult"
	// AgeBirthDateRequired: no usable date of birth yet; client actions are
	// refused until one is entered.
	AgeBirthDateRequired AgeStatus = "birth_date_required"
	// AgeRestricted: an under-age date of birth was entered, or an admin
	// restricted the account.
	AgeRestricted AgeStatus = "restricted"
)

// Valid reports whether s is a known status.
func (s AgeStatus) Valid() bool {
	return s == AgeAdult || s == AgeBirthDateRequired || s == AgeRestricted
}

// AgeStatusOf derives the status. Advisors and administrators are vetted by
// Staryield and always "adult"; for clients restriction wins over
// confirmation.
func AgeStatusOf(role Role, u *User) AgeStatus {
	if role != RoleClient {
		return AgeAdult
	}
	switch {
	case u == nil:
		return AgeBirthDateRequired
	case u.AgeRestrictedAt != nil:
		return AgeRestricted
	case u.AdultConfirmedAt != nil:
		return AgeAdult
	}
	return AgeBirthDateRequired
}

// AgeStatus is the account's status on the 18+ gate.
func (u *User) AgeStatus() AgeStatus { return AgeStatusOf(u.Role, u) }
