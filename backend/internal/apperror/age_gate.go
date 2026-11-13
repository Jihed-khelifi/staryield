package apperror

import "net/http"

// revisions 5.0: 18+ age gate (§24.6).
const (
	// CodeBirthDateRequired: enter a date of birth first (details.next).
	CodeBirthDateRequired Code = "birth_date_required"
	// CodeAgeRestricted: the account is restricted to adults.
	CodeAgeRestricted Code = "age_restricted"
)

// BirthDateNextPath is where the web app collects the date of birth.
const BirthDateNextPath = "/signup/birth"

// BirthDateRequired refuses a client action until a date of birth is on file.
func BirthDateRequired() *Error {
	return New(CodeBirthDateRequired, http.StatusForbidden, "Enter your date of birth to continue. Staryield is for adults 18 and over.").
		WithDetails(map[string]any{"next": BirthDateNextPath})
}

// AgeRestricted refuses an action of an account restricted to adults.
func AgeRestricted() *Error {
	return New(CodeAgeRestricted, http.StatusForbidden, "This account is restricted because Staryield is for adults 18 and over. Contact support if you entered the wrong date of birth.")
}
