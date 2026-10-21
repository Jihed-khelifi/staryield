package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// scanned are the packages whose string literals can reach users (error
// messages, validation messages, realtime error frames).
var scanned = []string{"../apperror", "../api", "../api/handler", "../api/middleware", "../realtime", "../service", "../security"}

// sentence matches user-facing copy: starts with a capital letter, has a
// space, ends like a sentence. Log lines are lowercase and excluded.
var sentence = regexp.MustCompile(`^[A-Z0-9%][^\n]* [^\n]*[.!?…]$`)

// skipCalls never produce user-facing text.
var skipCalls = map[string]bool{
	"Errorf": true, "New": false, "Error": true, "Warn": true, "Info": true, "Debug": true, "Fatal": true, "Panic": true,
	"Fatalf": true, "Logf": true, "Printf": true, "Println": true, "warnErr": true, "Check": true, "Track": true, "String": true,
}

// collectMessages returns every user-facing message literal (and "%s not
// found." resources) in the scanned packages.
func collectMessages(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	fset := token.NewFileSet()
	for _, dir := range scanned {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			var walk func(n ast.Node, skip bool)
			walk = func(n ast.Node, skip bool) {
				ast.Inspect(n, func(node ast.Node) bool {
					switch x := node.(type) {
					case *ast.CallExpr:
						fn := ""
						pkg := ""
						switch f := x.Fun.(type) {
						case *ast.SelectorExpr:
							fn = f.Sel.Name
							if id, ok := f.X.(*ast.Ident); ok {
								pkg = id.Name
							}
						case *ast.Ident:
							fn = f.Name
						}
						if pkg == "errors" || pkg == "zap" || pkg == "fmt" && fn != "Sprintf" || skipCalls[fn] {
							return false // log/error text, not user-facing
						}
						// Resource names: apperror.NotFound("X"), mapRepoErr(err, "X").
						if (pkg == "apperror" && fn == "NotFound") || fn == "mapRepoErr" {
							arg := x.Args[len(x.Args)-1]
							if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
								v, _ := strconv.Unquote(lit.Value)
								out[v+" not found."] = fset.Position(lit.Pos()).String()
							}
						}
					case *ast.BasicLit:
						if x.Kind != token.STRING || skip {
							return true
						}
						v, err := strconv.Unquote(x.Value)
						if err == nil && sentence.MatchString(v) && !strings.Contains(v, "://") {
							out[v] = fset.Position(x.Pos()).String()
						}
					}
					return true
				})
			}
			walk(f, false)
		}
	}
	return out
}

// TestEveryServerMessageIsTranslated fails when a user-facing message has
// no French or Spanish translation.
func TestEveryServerMessageIsTranslated(t *testing.T) {
	msgs := collectMessages(t)
	if len(msgs) < 100 {
		t.Fatalf("scanner found only %d messages; is it broken?", len(msgs))
	}
	var missing []string
	for msg, where := range msgs {
		for _, loc := range []string{"fr", "es"} {
			if !Has(loc, msg) {
				missing = append(missing, loc+": "+strconv.Quote(msg)+"  ("+where+")")
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		if os.Getenv("I18N_DUMP") != "" {
			keys := make([]string, 0, len(msgs))
			for m := range msgs {
				if !Has("fr", m) {
					keys = append(keys, m)
				}
			}
			sort.Strings(keys)
			_ = os.WriteFile(os.Getenv("I18N_DUMP"), []byte(strings.Join(keys, "\n")), 0o600)
		}
		t.Fatalf("%d untranslated server messages:\n%s", len(missing), strings.Join(missing, "\n"))
	}
}

func TestCataloguesAreConsistent(t *testing.T) {
	verbs := regexp.MustCompile(`%[ds]`)
	for key, fv := range fr {
		ev, ok := es[key]
		if !ok {
			t.Errorf("es missing %q", key)
			continue
		}
		want := verbs.FindAllString(key, -1)
		for loc, v := range map[string]string{"fr": fv, "es": ev} {
			if got := verbs.FindAllString(v, -1); strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("%s: placeholders differ for %q: %q", loc, key, v)
			}
			if strings.TrimSpace(v) == "" {
				t.Errorf("%s: empty translation for %q", loc, key)
			}
		}
	}
	for key := range es {
		if _, ok := fr[key]; !ok {
			t.Errorf("fr missing %q", key)
		}
	}
}

func TestTranslate(t *testing.T) {
	fr["Test message."] = "Message de test."
	fr["Use between %d and %d characters."] = "Utilisez entre %d et %d caractères."
	fr["%s not found."] = "%s introuvable."
	fr["Session"] = "Séance"
	defer func() {
		delete(fr, "Test message.")
		delete(fr, "Use between %d and %d characters.")
		delete(fr, "%s not found.")
		delete(fr, "Session")
	}()
	templates = buildTemplates()
	defer func() { templates = buildTemplates() }()

	if got := T("fr-CA", "Test message."); got != "Message de test." {
		t.Errorf("exact: %q", got)
	}
	if got := T("FR", "Use between 1 and 80 characters."); got != "Utilisez entre 1 et 80 caractères." {
		t.Errorf("template: %q", got)
	}
	if got := T("fr", "Session not found."); got != "Séance introuvable." {
		t.Errorf("nested: %q", got)
	}
	if got := T("de", "Test message."); got != "Test message." {
		t.Errorf("unsupported locale: %q", got)
	}
	if got := T("fr", "Unknown text."); got != "Unknown text." {
		t.Errorf("unknown text: %q", got)
	}
	details := Details("fr", map[string]any{"fields": map[string]any{"name": "Test message."}, "max": 5})
	if details["fields"].(map[string]any)["name"] != "Message de test." || details["max"] != 5 {
		t.Errorf("details: %v", details)
	}
	if Normalize("es-MX") != "es" || Normalize("") != "en" || Normalize("pt") != "en" {
		t.Error("normalize")
	}
}
