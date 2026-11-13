package mail

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// revisions 5.1: the web footer and the e-mail footer print the same company
// facts (§25.4). frontend/src/config/company.ts is the web's single source; this
// test parses it so the two cannot drift apart.
func TestCompanyFactsEqualTheWebConfig(t *testing.T) {
	root := filepath.Join("..", "..", "..", "frontend")
	if _, err := os.Stat(root); err != nil {
		t.Skip("the frontend is not part of this checkout (backend-only build)")
	}
	b, err := os.ReadFile(filepath.Join(root, "src", "config", "company.ts"))
	if err != nil {
		t.Fatalf("frontend/src/config/company.ts must be readable: %v", err)
	}
	field := func(name string) string {
		m := regexp.MustCompile(name + `:\s*"([^"]+)"`).FindSubmatch(b)
		if m == nil {
			t.Fatalf("company.ts no longer defines %s: update this test together with the file", name)
		}
		return string(m[1])
	}
	if got, want := CompanyLegalName, field("legalName"); got != want {
		t.Errorf("legal name: e-mails print %q, the web prints %q", got, want)
	}
	if got, want := CompanyAddress, field("addressLine"); got != want {
		t.Errorf("address: e-mails print %q, the web prints %q", got, want)
	}
	if got, want := DefaultSupportEmail, field("supportEmail"); got != want {
		t.Errorf("support e-mail: e-mails default to %q, the web prints %q", got, want)
	}
}
