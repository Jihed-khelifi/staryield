package mail

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"strings"
	"testing"

	"github.com/Jihed-khelifi/staryield/backend/internal/astro"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail/reportcopy"
)

// parityFixture is the file the web's report-parity test checks too.
type parityFixture struct {
	Fragments []struct {
		Kind  string `json:"kind"`
		Input struct {
			BirthDate   string `json:"birth_date"`
			BirthTime   string `json:"birth_time"`
			PlaceID     string `json:"place_id"`
			HouseSystem string `json:"house_system"`
		} `json:"input"`
		Fragment string `json:"fragment"`
	} `json:"fragments"`
	LifePathSteps []struct {
		BirthDate string   `json:"birth_date"`
		Steps     []string `json:"steps"`
	} `json:"life_path_steps"`
}

func loadParity(t *testing.T) parityFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/report_parity.json")
	if err != nil {
		t.Fatal(err)
	}
	var f parityFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Fragments) < 10 || len(f.LifePathSteps) < 10 {
		t.Fatalf("fixture too small: %d fragments, %d step lists", len(f.Fragments), len(f.LifePathSteps))
	}
	return f
}

// The URL fragment of the "View your report" link is the web's own encoding
// (src/lib/astro/fragment.ts): the fixture holds what the web produces.
func TestReportFragmentMatchesTheWeb(t *testing.T) {
	for _, c := range loadParity(t).Fragments {
		got := ReportFragment(c.Kind, c.Input.BirthDate, c.Input.BirthTime, c.Input.PlaceID, c.Input.HouseSystem)
		if got != c.Fragment {
			t.Errorf("%s %+v: fragment %q, the web writes %q", c.Kind, c.Input, got, c.Fragment)
		}
	}
}

// "How it adds up" prints the same lines as the web's result page.
func TestLifePathStepsMatchTheWeb(t *testing.T) {
	rc := reportcopy.Must()
	for _, c := range loadParity(t).LifePathSteps {
		var y, m, d int
		if _, err := fmtSscan(c.BirthDate, &y, &m, &d); err != nil {
			t.Fatal(err)
		}
		got := LifePathSteps(rc, "en", astro.LifePathReduction(y, m, d))
		if strings.Join(got, "|") != strings.Join(c.Steps, "|") {
			t.Errorf("%s: %v, the web prints %v", c.BirthDate, got, c.Steps)
		}
	}
}

func fmtSscan(date string, y, m, d *int) (int, error) {
	return fmt.Sscanf(date, "%d-%d-%d", y, m, d)
}

func htmlEscape(s string) string { return html.EscapeString(s) }

func sampleSpread() astro.TarotSpread {
	cards := make([]astro.TarotCard, 0, 12)
	slugs := []string{"the-sun", "the-high-priestess", "the-magician", "the-empress", "the-tower", "wheel-of-fortune", "the-world",
		"the-fool", "the-hanged-man", "judgement", "the-emperor", "strength"}
	for i := 12; i >= 1; i-- { // out of order on purpose: the report sorts by house
		card := astro.TarotCard{House: i, Card: slugs[i-1], Source: astro.CardSource{Kind: "cusp_sign", Sign: "Leo"}}
		switch i {
		case 1:
			card.Source = astro.CardSource{Kind: "planet", Body: "Sun", Sign: "Leo"}
		case 3:
			card.Source = astro.CardSource{Kind: "planet", Body: "Mercury", Sign: "Virgo"}
			card.Reversed = true
		case 5:
			card.Uncertain = true
		}
		cards = append(cards, card)
	}
	return astro.TarotSpread{HouseSystem: "placidus", Uncertain: true, Cards: cards}
}

func natalData(locale string) CalculatorReportData {
	return CalculatorReportData{Locale: locale, Kind: KindNatalChart, BirthDate: "1990-03-14", FrontendURL: "https://staryield.net",
		ViewPath: "/calculators/natal-chart#v=1&d=1990-03-14&t=08%3A45&p=gn%3A2988507", Disclaimer: "Offered for reflection and entertainment. Your choices remain your own.",
		Attribution: "Places: GeoNames (CC BY 4.0).",
		Natal: &NatalReport{Spread: sampleSpread(), TimeKnown: true, HouseSystem: "placidus", WallTimeStatus: "ambiguous",
			BirthTime: "08:45", PlaceLabel: "Paris, Île-de-France, France"}}
}

func renderReport(t *testing.T, d CalculatorReportData) (Content, Rendered) {
	t.Helper()
	c, err := CalculatorReport(d)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Render(c, Layout{Locale: d.Locale, FrontendURL: d.FrontendURL, Category: "account"})
	if err != nil {
		t.Fatal(err)
	}
	return c, r
}

func TestNatalReportCarriesEverySectionOfTheResult(t *testing.T) {
	c, r := renderReport(t, natalData("en"))
	if c.Heading != "Natal alignment calculated for:" || c.Paragraphs[0] != "14 / 03 / 1990" {
		t.Fatalf("headline %q %q", c.Heading, c.Paragraphs)
	}
	if c.Paragraphs[1] != "08:45 · Paris, Île-de-France, France" {
		t.Fatalf("details line %q", c.Paragraphs[1])
	}
	for _, want := range []string{
		"Your spread", "A 12-card spread exploring the current landscape", "Without an exact birth time, cards with a dashed frame may change",
		"Your cards and why they were drawn", "House 1 · Self", "The Sun", "Sun in Leo, in the first house", "House 3 · Communication", "Reversed",
		"Mercury in Virgo, in the third house · retrograde, so the card is reversed", "Cusp sign: Leo (no planet in this house)",
		"This card may change with your exact birth time.", "Houses: Placidus.", "That clock time happened twice there",
		"Get a better understanding of what this spread means", "Click here!", "Offered for reflection and entertainment.",
		"Places: GeoNames (CC BY 4.0).", "View your report", "Staryield Media LLC",
	} {
		if !strings.Contains(r.Text, want) {
			t.Errorf("text report lacks %q", want)
		}
		if !strings.Contains(r.HTML, strings.ReplaceAll(want, "·", "·")) && !strings.Contains(r.HTML, htmlEscape(want)) {
			t.Errorf("html report lacks %q", want)
		}
	}
	// Twelve cards, in house order, each with its picture from R2.
	if n := strings.Count(r.HTML, AssetsBaseURL+"/assets/tarot/"); n != 12 {
		t.Fatalf("%d card pictures, want 12", n)
	}
	if first, last := strings.Index(r.Text, "House 1 ·"), strings.Index(r.Text, "House 12 ·"); first < 0 || last < first {
		t.Fatalf("cards are not in house order: %d %d", first, last)
	}
	if !strings.Contains(r.HTML, `src="https://assets.staryield.net/assets/tarot/the-sun.png"`) {
		t.Fatal("tarot artwork must load from R2")
	}
	if !strings.Contains(r.Text, "https://staryield.net/en/calculators/natal-chart?utm_source=staryield&utm_medium=email&utm_campaign=calculator_report&utm_content=natal-chart#v=1&d=1990-03-14&t=08%3A45&p=gn%3A2988507") {
		t.Fatalf("the view link must reopen the result:\n%s", r.Text)
	}
	if !strings.Contains(r.Text, "https://staryield.net/en?utm_source=staryield&utm_medium=email&utm_campaign=calculator_report&utm_content=natal-chart#psychics") {
		t.Fatal("the call to action links the psychics section")
	}
	if strings.Contains(r.Text, "nsubscribe") || r.Headers["List-Unsubscribe"] != "" {
		t.Fatal("no unsubscribe link without a subscription")
	}
	if r.Subject != "Your Staryield natal chart report" {
		t.Fatalf("subject %q", r.Subject)
	}
}

func TestNatalReportWithoutTime(t *testing.T) {
	d := natalData("en")
	d.Natal.TimeKnown, d.Natal.BirthTime, d.Natal.WallTimeStatus, d.Natal.FallbackReason = false, "", "", true
	d.Natal.Spread.Uncertain = false
	c, r := renderReport(t, d)
	if c.Paragraphs[1] != "Birth time unknown · Paris, Île-de-France, France" {
		t.Fatalf("details %q", c.Paragraphs[1])
	}
	if !strings.Contains(r.Text, "Birth time unknown: houses are counted whole-sign from your Sun.") ||
		!strings.Contains(r.Text, "Placidus is undefined this close to the poles") {
		t.Fatal("whole-sign note missing")
	}
	if strings.Contains(r.Text, "Without an exact birth time") || strings.Contains(r.Text, "Houses: Placidus") {
		t.Fatal("known-time notes leaked in")
	}
}

func TestReportIsTranslated(t *testing.T) {
	for locale, wants := range map[string][]string{
		"fr": {"Votre tirage", "Votre rapport de thème natal Staryield", "Voir votre rapport", "Renversée", "Maison 1"},
		"es": {"Tu tirada", "Tu informe de carta natal de Staryield", "Ver tu informe", "Invertida", "Casa 1"},
	} {
		_, r := renderReport(t, natalData(locale))
		for _, want := range wants {
			if !strings.Contains(r.Text, want) && !strings.Contains(r.Subject, want) {
				t.Errorf("%s report lacks %q:\n%s", locale, want, r.Text)
			}
		}
		if strings.Contains(r.Text, "Your spread") || strings.Contains(r.Text, "Reversed") {
			t.Errorf("%s report still has English copy", locale)
		}
	}
}

func TestSunSignReport(t *testing.T) {
	d := CalculatorReportData{Locale: "en", Kind: KindAstrology, BirthDate: "1990-08-23", FrontendURL: "https://staryield.net",
		ViewPath: "/calculators/astrology#v=1&d=1990-08-23", Disclaimer: "Offered for reflection and entertainment. Your choices remain your own.",
		Sun: &SunReport{Sign: "Virgo", Uncertain: true, Alternatives: []string{"Leo", "Virgo"}, Element: "Earth", Modality: "Mutable"}}
	_, r := renderReport(t, d)
	for _, want := range []string{"Your zodiac", "You were born on a cusp: the Sun moved from Leo into Virgo that day.",
		"We show Virgo, the sign for your date at noon.", "According to your alignment, the sun resides in the sign of", "Virgo",
		"This highlighted sign reveals a precise, practical mind", "Virgo is ruled by Mercury and belongs to the Earth element (Mutable).",
		"Want to discover your planetary coordinates?", "Sign in / up", "https://staryield.net/en/signup", "Your Staryield astrology report"} {
		if !strings.Contains(r.Text, want) && r.Subject != want {
			t.Errorf("astrology report lacks %q", want)
		}
	}
	d.Locale = "fr"
	d.Sun.Uncertain = false
	_, fr := renderReport(t, d)
	if !strings.Contains(fr.Text, "Vierge") || strings.Contains(fr.Text, "born on a cusp") {
		t.Fatalf("fr astrology report:\n%s", fr.Text)
	}
}

func TestPathOfLifeReport(t *testing.T) {
	d := CalculatorReportData{Locale: "en", Kind: KindPathOfLife, BirthDate: "1990-11-29", FrontendURL: "https://staryield.net",
		ViewPath: "/calculators/path-of-life#v=1&d=1990-11-29", Disclaimer: "Offered for reflection and entertainment. Your choices remain your own.",
		Numbers: &NumbersReport{LifePath: astro.LifePathReduction(1990, 11, 29), PersonalYear: 5, PersonalMonth: 7, AsOf: "2026-11-04"}}
	_, r := renderReport(t, d)
	for _, want := range []string{"Your numbers", "Life Path: 5", "Personal year: 5", "Personal month: 7",
		"Personal cycles for November 4, 2026, your local date.", "Life path 5", "The freedom seeker", "Curious, adaptable and adventurous.",
		"How it adds up: Month: 11 · Day: 29 → 11 · Year: 1990 → 19 → 10 → 1 · Total: 23 → 5",
		"Get a deeper understanding of what a life path really means", "Click here!"} {
		if !strings.Contains(r.Text, want) {
			t.Errorf("path of life report lacks %q", want)
		}
	}
	d.Locale = "es"
	_, es := renderReport(t, d)
	if !strings.Contains(es.Text, "4 de noviembre de 2026") || !strings.Contains(es.Text, "Mes: 11") {
		t.Fatalf("es path of life report:\n%s", es.Text)
	}
	d.Locale = "fr"
	_, fr := renderReport(t, d)
	if !strings.Contains(fr.Text, "4 novembre 2026") {
		t.Fatalf("fr path of life report:\n%s", fr.Text)
	}
	// Unknown numbers fall back to the 9 portrait like the web.
	d.Numbers.LifePath.Number = 44
	if _, err := CalculatorReport(d); err != nil {
		t.Fatal(err)
	}
}

func TestUnsubscribeLinkOnlyWhenSubscribed(t *testing.T) {
	d := natalData("en")
	d.UnsubscribePath = "/unsubscribe?token=abc.def"
	c, r := renderReport(t, d)
	if len(c.Links) != 1 || c.Links[0].URL != "/unsubscribe?token=abc.def" {
		t.Fatalf("links %+v", c.Links)
	}
	if !strings.Contains(r.Text, "Unsubscribe from Staryield marketing e-mails: https://staryield.net/en/unsubscribe?token=abc.def") {
		t.Fatalf("text:\n%s", r.Text)
	}
	if r.Headers["List-Unsubscribe"] != "<https://staryield.net/en/unsubscribe?token=abc.def>" {
		t.Fatalf("header %q", r.Headers["List-Unsubscribe"])
	}
	if !strings.Contains(r.HTML, `href="https://staryield.net/en/unsubscribe?token=abc.def"`) {
		t.Fatal("html lacks the unsubscribe link")
	}
}

func TestReportRefusesAnEmptyResult(t *testing.T) {
	if _, err := CalculatorReport(CalculatorReportData{Locale: "en", Kind: KindNatalChart}); err == nil {
		t.Fatal("a report needs a result")
	}
}

func TestReportEscapesHostileValues(t *testing.T) {
	d := natalData("en")
	d.Natal.PlaceLabel = `<script>alert(1)</script>`
	d.Natal.Spread.Cards[0].Card = `"><img src=x onerror=alert(1)>`
	_, r := renderReport(t, d)
	if strings.Contains(r.HTML, "<script>") || strings.Contains(r.HTML, "<img src=x") {
		t.Fatalf("unescaped markup:\n%s", r.HTML)
	}
}

func TestLongDate(t *testing.T) {
	for locale, want := range map[string]string{"en": "December 31, 1999", "fr": "31 décembre 1999", "es": "31 de diciembre de 1999"} {
		if got := longDate(locale, "1999-12-31"); got != want {
			t.Errorf("%s: %q", locale, got)
		}
	}
	if longDate("en", "garbage") != "garbage" || dateParts("garbage") != "garbage" {
		t.Fatal("unparseable dates are echoed")
	}
}

// Every label the renderer asks for exists in the generated copy.
func TestRendererLabelsExistInTheCopy(t *testing.T) {
	rc := reportcopy.Must()
	for _, key := range []string{"headline", "birthTimeUnknown", "spreadTitle", "spreadIntro", "spreadUncertain", "cardsHeading", "houseNumber",
		"reversed", "cardPlanet", "cardRetrograde", "cardCusp", "cardMayChange", "housesLine", "housesUnknown", "housesFallback", "wallTimeSkipped",
		"wallTimeAmbiguous", "wallTimeLmt", "natalCta", "clickHere", "zodiacTitle", "cuspTitle", "cuspNote", "sunSignLead", "signFacts",
		"zodiacCtaTitle", "zodiacCtaBody", "signUp", "numbersTitle", "personalCycles", "lifePathTitle", "howItAddsUp", "month", "day", "year",
		"stepLine", "stepTotal", "lifePathCtaTitle", "lifePathCtaBody"} {
		for _, locale := range []string{"en", "fr", "es"} {
			if rc.Label(locale, key) == "" {
				t.Errorf("label %q is missing (%s)", key, locale)
			}
		}
		if rc.Label("fr", key) == rc.Label("en", key) && key != "clickHere" {
			t.Logf("label %q has the same text in French", key)
		}
	}
}

const utmNatal = "utm_source=staryield&utm_medium=email&utm_campaign=calculator_report&utm_content=natal"

func TestWithUTM(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"fragment only", "https://staryield.net/en#psychics", "https://staryield.net/en?" + utmNatal + "#psychics"},
		{"plain", "https://staryield.net/en/signup", "https://staryield.net/en/signup?" + utmNatal},
		{"existing query", "https://staryield.net/en/x?a=1", "https://staryield.net/en/x?a=1&" + utmNatal},
		{"existing query and fragment", "https://staryield.net/en/x?a=1#top", "https://staryield.net/en/x?a=1&" + utmNatal + "#top"},
		{"report fragment is byte-identical", "https://staryield.net/en/calculators/natal-chart#v=1&d=1990-03-14&t=08%3A45&p=gn%3A2988507&h=whole_sign",
			"https://staryield.net/en/calculators/natal-chart?" + utmNatal + "#v=1&d=1990-03-14&t=08%3A45&p=gn%3A2988507&h=whole_sign"},
		{"empty fragment marker", "https://staryield.net/en/x#", "https://staryield.net/en/x?" + utmNatal + "#"},
	} {
		if got := withUTM(tc.in, "natal"); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
	// The kind is escaped, and an unparsable URL is returned untouched.
	if got := withUTM("https://staryield.net/en", "a b&c"); !strings.Contains(got, "utm_content=a+b%26c") {
		t.Errorf("kind not escaped: %q", got)
	}
	if got := withUTM("http://[::1", "natal"); got != "http://[::1" {
		t.Errorf("unparsable URL changed: %q", got)
	}
}

// Review focus: the view, advisor and sign-up links carry the tags (before any
// fragment, the fragment unchanged); the unsubscribe link never does.
func TestReportLinksCarryUTM(t *testing.T) {
	utm := func(kind string) string {
		return "utm_source=staryield&utm_medium=email&utm_campaign=calculator_report&utm_content=" + kind
	}
	natal := natalData("en")
	natal.UnsubscribePath = "/unsubscribe?token=abc"
	c, r := renderReport(t, natal)
	if got, want := c.Action.URL, "https://staryield.net/en/calculators/natal-chart?"+utm("natal-chart")+"#v=1&d=1990-03-14&t=08%3A45&p=gn%3A2988507"; got != want {
		t.Errorf("view link %q, want %q", got, want)
	}
	if !strings.Contains(r.Text, "https://staryield.net/en?"+utm("natal-chart")+"#psychics") {
		t.Errorf("the advisor link lacks the tags:\n%s", r.Text)
	}
	if len(c.Links) != 1 || c.Links[0].URL != "/unsubscribe?token=abc" || strings.Contains(c.ListUnsubscribe, "utm_") {
		t.Errorf("unsubscribe must stay untagged: %+v %q", c.Links, c.ListUnsubscribe)
	}
	if strings.Contains(r.Headers["List-Unsubscribe"], "utm_") {
		t.Errorf("List-Unsubscribe is tagged: %q", r.Headers["List-Unsubscribe"])
	}
	for _, link := range regexpURLs(r.HTML) {
		if strings.Contains(link, "unsubscribe") && strings.Contains(link, "utm_") {
			t.Errorf("tagged unsubscribe link in the HTML: %q", link)
		}
	}

	sun := CalculatorReportData{Locale: "en", Kind: KindAstrology, BirthDate: "1990-08-23", FrontendURL: "https://staryield.net",
		ViewPath: "/calculators/astrology#v=1&d=1990-08-23", Sun: &SunReport{Sign: "Virgo", Element: "Earth", Modality: "Mutable"}}
	_, sr := renderReport(t, sun)
	for _, want := range []string{"https://staryield.net/en/signup?" + utm("astrology"), "https://staryield.net/en/calculators/astrology?" + utm("astrology") + "#v=1&d=1990-08-23"} {
		if !strings.Contains(sr.Text, want) {
			t.Errorf("sun report lacks %q:\n%s", want, sr.Text)
		}
	}

	life := CalculatorReportData{Locale: "fr", Kind: KindPathOfLife, BirthDate: "1990-11-29", FrontendURL: "https://staryield.net/",
		ViewPath: "/calculators/path-of-life#v=1&d=1990-11-29",
		Numbers:  &NumbersReport{LifePath: astro.LifePathReduction(1990, 11, 29), PersonalYear: 5, PersonalMonth: 7, AsOf: "2026-11-04"}}
	_, lr := renderReport(t, life)
	if !strings.Contains(lr.Text, "https://staryield.net/fr?"+utm("path-of-life")+"#psychics") {
		t.Errorf("numbers report lacks the advisor tags:\n%s", lr.Text)
	}
}

func regexpURLs(s string) []string {
	var out []string
	for _, part := range strings.Split(s, `href="`)[1:] {
		out = append(out, part[:strings.Index(part, `"`)])
	}
	return out
}
