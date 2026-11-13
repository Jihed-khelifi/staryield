package mail

// revisions 5.1: company and customer-service details (§25.4).
//
// The merchant of record as card schemes and payment processors must see it on
// every customer touch point: the footer of every e-mail carries the legal
// name, the postal address, the support address and the support hours. The
// web app keeps the same facts in frontend/src/config/company.ts; a test
// (company_test.go) parses that file and fails when the two drift apart.

const (
	// CompanyLegalName is the legal name of the merchant.
	CompanyLegalName = "Staryield Media LLC"
	// CompanyAddress is the postal address, on one line.
	CompanyAddress = "30 N Gould St, Ste N, Sheridan, WY 82801, United States"
	// DefaultSupportEmail is the support address when STARYIELD_MAIL_SUPPORT_INBOX
	// is empty; the configured value wins.
	DefaultSupportEmail = "support@staryield.net"
	// SupportHours is the English source of the support hours sentence
	// (translated in the footer).
	SupportHours = "Support is available 24/7. We reply within 1–2 days."
)
