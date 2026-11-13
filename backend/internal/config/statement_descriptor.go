package config

import (
	"regexp"
	"unicode"
)

// revisions 5.1: statement descriptor (§25.3).

// DefaultStatementDescriptor is what customers see on their card statement
// (STARYIELD_PAYMENTS_STATEMENT_DESCRIPTOR). It must equal the descriptor the
// English Refund Policy §11 states ("Your statement may display purchases as
// ..."); a test keeps both in line.
const DefaultStatementDescriptor = "STARYIELD MEDIA LLC"

// Card networks accept 5 to 22 characters of this alphabet on a statement.
const (
	StatementDescriptorMin = 5
	StatementDescriptorMax = 22
)

var statementDescriptorChars = regexp.MustCompile(`^[A-Za-z0-9 .,-]+$`)

// ValidStatementDescriptor reports whether s may be sent to a card processor:
// 5 to 22 characters of letters, digits, space, dot, comma and hyphen, with at
// least one letter.
func ValidStatementDescriptor(s string) bool {
	if n := len([]rune(s)); n < StatementDescriptorMin || n > StatementDescriptorMax {
		return false
	}
	if !statementDescriptorChars.MatchString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// validateStatementDescriptor checks STARYIELD_PAYMENTS_STATEMENT_DESCRIPTOR.
// Production refuses an empty value; elsewhere an empty value sends none.
func (c *Config) validateStatementDescriptor(add func(string, ...any)) {
	d := c.Payments.StatementDescriptor
	switch {
	case d == "" && c.IsProduction():
		add("%s must be set in production (5-22 characters, e.g. %s)", EnvVar("payments.statement_descriptor"), DefaultStatementDescriptor)
	case d != "" && !ValidStatementDescriptor(d):
		add("%s must be %d-%d characters of letters, digits, space, . , - with at least one letter", EnvVar("payments.statement_descriptor"),
			StatementDescriptorMin, StatementDescriptorMax)
	}
}
