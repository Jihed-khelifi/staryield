package mail

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/astro"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail/reportcopy"
)

// revisions 6.0 (§26.4): the calculators' e-mailed report. It carries every
// section of the on-screen result, in the request's locale, from the copy the
// web app generates (reportcopy) so the page and the e-mail cannot drift.

// AssetsBaseURL is the public R2 host of the site's images (tarot artwork).
const AssetsBaseURL = "https://assets.staryield.net"

// Calculators (the route names of the web app).
const (
	KindNatalChart = "natal-chart"
	KindPathOfLife = "path-of-life"
	KindAstrology  = "astrology"
)

// NatalReport is the natal calculator's answer.
type NatalReport struct {
	Spread astro.TarotSpread
	// TimeKnown: the chart used an exact birth time and place.
	TimeKnown bool
	// HouseSystem is the house system the chart used ("placidus", "porphyry"…).
	HouseSystem    string
	FallbackReason bool
	WallTimeStatus string
	BirthTime      string
	PlaceLabel     string
}

// SunReport is the sun-sign calculator's answer.
type SunReport struct {
	Sign         string
	Uncertain    bool
	Alternatives []string
	Element      string
	Modality     string
	BirthTime    string
	PlaceLabel   string
}

// NumbersReport is the path of life calculator's answer.
type NumbersReport struct {
	LifePath      astro.Reduction
	PersonalYear  int
	PersonalMonth int
	// AsOf is the calendar day ("YYYY-MM-DD") the personal cycles are for.
	AsOf string
}

// CalculatorReportData is everything the report e-mail prints. Exactly one of
// Natal, Sun and Numbers is set, matching Kind.
type CalculatorReportData struct {
	Locale string
	Kind   string
	// BirthDate is "YYYY-MM-DD".
	BirthDate string
	// FrontendURL is the site origin (e.g. https://staryield.net).
	FrontendURL string
	// ViewPath is the locale-less app path that reopens the result, with the
	// input in the URL fragment ("/calculators/natal-chart#v=1&d=…").
	ViewPath string
	// UnsubscribePath is the locale-less path of the unsubscribe page with its
	// token ("/unsubscribe?token=…"); empty unless the address is subscribed
	// to marketing e-mail.
	UnsubscribePath string
	Disclaimer      string
	Attribution     string

	Natal   *NatalReport
	Sun     *SunReport
	Numbers *NumbersReport
}

// CalculatorReport builds the report e-mail (template calculator_report).
func CalculatorReport(d CalculatorReportData) (Content, error) {
	rc, err := reportcopy.Get()
	if err != nil {
		return Content{}, err
	}
	locale := localeOf(d.Locale)
	t := tr(locale)
	lbl := func(key string) string { return rc.Label(locale, key) }
	home := strings.TrimRight(d.FrontendURL, "/") + "/" + locale
	psychics := Action{Label: lbl("clickHere"), URL: home + "#psychics"}

	c := Content{
		Subject:   t(reportSubject(d.Kind)),
		Preheader: t("The report you asked for is ready to read."),
		Heading:   lbl("headline"),
		Paragraphs: []string{
			dateParts(d.BirthDate),
		},
	}
	if line := detailsLine(rc, locale, d); line != "" {
		c.Paragraphs = append(c.Paragraphs, line)
	}
	c.Paragraphs = append(c.Paragraphs, t("Here is the report you asked us to e-mail you."))

	switch {
	case d.Natal != nil:
		c.Sections = natalSections(rc, locale, d, psychics)
	case d.Sun != nil:
		c.Sections = sunSections(rc, locale, d, home)
	case d.Numbers != nil:
		c.Sections = numbersSections(rc, locale, d, psychics)
	default:
		return Content{}, fmt.Errorf("mail: calculator report without a result")
	}
	if foot := reportFootnote(rc, locale, d); foot != "" {
		c.Sections = append(c.Sections, Section{Note: foot})
	}
	if d.ViewPath != "" {
		c.Action = &Action{Label: t("View your report"), URL: AppURL(d.FrontendURL, locale, d.ViewPath)}
		c.Note = t("The link opens your result again and contains the details you entered. Keep it private.")
	}
	if d.UnsubscribePath != "" {
		c.Links = append(c.Links, Action{Label: t("Unsubscribe from Staryield marketing e-mails"), URL: d.UnsubscribePath})
		c.ListUnsubscribe = AppURL(d.FrontendURL, locale, d.UnsubscribePath)
	}
	return c, nil
}

func localeOf(l string) string {
	switch strings.ToLower(strings.TrimSpace(l)) {
	case "fr":
		return "fr"
	case "es":
		return "es"
	}
	return "en"
}

func reportSubject(kind string) string {
	switch kind {
	case KindNatalChart:
		return "Your Staryield natal chart report"
	case KindPathOfLife:
		return "Your Staryield path of life report"
	}
	return "Your Staryield astrology report"
}

// dateParts is "30 / 04 / 1945", the format of the result's date button.
func dateParts(date string) string {
	d, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return date
	}
	return fmt.Sprintf("%02d / %02d / %04d", d.Day(), int(d.Month()), d.Year())
}

// detailsLine is "08:45 · Paris, Île-de-France, France", under the date.
func detailsLine(rc *reportcopy.Copy, locale string, d CalculatorReportData) string {
	var parts []string
	switch {
	case d.Natal != nil:
		if d.Natal.TimeKnown && d.Natal.BirthTime != "" {
			parts = append(parts, d.Natal.BirthTime)
		} else {
			parts = append(parts, rc.Label(locale, "birthTimeUnknown"))
		}
		parts = append(parts, d.Natal.PlaceLabel)
	case d.Sun != nil:
		parts = append(parts, d.Sun.BirthTime, d.Sun.PlaceLabel)
	}
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " · ")
}

func natalSections(rc *reportcopy.Copy, locale string, d CalculatorReportData, psychics Action) []Section {
	n := d.Natal
	lbl := func(key string) string { return rc.Label(locale, key) }
	intro := Section{Title: lbl("spreadTitle"), Paragraphs: []string{lbl("spreadIntro")}}
	if n.Spread.Uncertain {
		intro.Paragraphs = append(intro.Paragraphs, lbl("spreadUncertain"))
	}
	cards := append([]astro.TarotCard(nil), n.Spread.Cards...)
	sort.SliceStable(cards, func(i, j int) bool { return cards[i].House < cards[j].House })
	items := make([]Item, 0, len(cards))
	for _, card := range cards {
		items = append(items, cardItem(rc, locale, card))
	}
	list := Section{Title: lbl("cardsHeading"), Items: items}

	var notes []string
	if n.TimeKnown && n.HouseSystem != "" {
		name := n.HouseSystem
		if en, ok := rc.HouseSystems[n.HouseSystem]; ok {
			name = rc.T(locale, en)
		}
		notes = append(notes, rc.LabelF(locale, "housesLine", name))
	} else {
		notes = append(notes, lbl("housesUnknown"))
	}
	if n.FallbackReason {
		notes[0] += " " + lbl("housesFallback")
	}
	switch n.WallTimeStatus {
	case "skipped":
		notes = append(notes, lbl("wallTimeSkipped"))
	case "ambiguous":
		notes = append(notes, lbl("wallTimeAmbiguous"))
	case "lmt":
		notes = append(notes, lbl("wallTimeLmt"))
	}
	facts := Section{Paragraphs: notes}
	cta := Section{Paragraphs: []string{lbl("natalCta")}, Action: &psychics}
	return []Section{intro, list, facts, cta}
}

// cardItem is one card of the spread: its picture, house, name and the reason
// it was drawn.
func cardItem(rc *reportcopy.Copy, locale string, card astro.TarotCard) Item {
	t := func(s string) string { return rc.T(locale, s) }
	it := Item{Title: card.Card}
	if name, ok := rc.Tarot[card.Card]; ok {
		it.Title = t(name)
		it.ImageURL = AssetsBaseURL + "/assets/tarot/" + card.Card + ".png"
		it.ImageAlt = it.Title
	}
	caption := rc.LabelF(locale, "houseNumber", fmt.Sprint(card.House))
	house := ""
	if card.House >= 1 && card.House <= len(rc.Houses) {
		h := rc.Houses[card.House-1]
		caption += " · " + t(h.Keyword)
		house = strings.ToLower(t(h.Label))
	}
	it.Caption = caption
	if card.Reversed {
		it.Badge = rc.Label(locale, "reversed")
	}
	if card.Source.Kind == "planet" && card.Source.Body != "" {
		it.Body = rc.LabelF(locale, "cardPlanet", t(card.Source.Body), t(card.Source.Sign), house)
		if card.Reversed {
			it.Body += " · " + rc.Label(locale, "cardRetrograde")
		}
	} else {
		it.Body = rc.LabelF(locale, "cardCusp", t(card.Source.Sign))
	}
	if card.Uncertain {
		it.Note = rc.Label(locale, "cardMayChange")
	}
	return it
}

func sunSections(rc *reportcopy.Copy, locale string, d CalculatorReportData, home string) []Section {
	s := d.Sun
	t := func(x string) string { return rc.T(locale, x) }
	lbl := func(key string) string { return rc.Label(locale, key) }
	head := Section{Title: lbl("zodiacTitle")}
	if s.Uncertain && len(s.Alternatives) > 0 {
		first, second := s.Alternatives[0], s.Sign
		if len(s.Alternatives) > 1 {
			second = s.Alternatives[1]
		}
		head.Paragraphs = append(head.Paragraphs, rc.LabelF(locale, "cuspTitle", t(first), t(second)), rc.LabelF(locale, "cuspNote", t(s.Sign)))
	}
	item := Item{Caption: lbl("sunSignLead"), Title: t(s.Sign)}
	if sign, ok := rc.SignByName(s.Sign); ok {
		element := s.Element
		if element == "" {
			element = sign.Element
		}
		item.Body = t(sign.Insight) + " " + rc.LabelF(locale, "signFacts", t(sign.Name), t(sign.Ruler), t(element), t(s.Modality))
	}
	head.Items = []Item{item}
	cta := Section{Title: lbl("zodiacCtaTitle"), Paragraphs: []string{lbl("zodiacCtaBody")},
		Action: &Action{Label: lbl("signUp"), URL: home + "/signup"}}
	return []Section{head, cta}
}

func numbersSections(rc *reportcopy.Copy, locale string, d CalculatorReportData, psychics Action) []Section {
	n := d.Numbers
	t := func(x string) string { return rc.T(locale, x) }
	lbl := func(key string) string { return rc.Label(locale, key) }
	values := map[string]int{"lifePath": n.LifePath.Number, "personalYear": n.PersonalYear, "personalMonth": n.PersonalMonth}
	cards := Section{Title: lbl("numbersTitle")}
	for _, c := range rc.Numbers {
		cards.Items = append(cards.Items, Item{Caption: t(c.Label), Title: fmt.Sprint(values[c.Key]), Body: t(c.Body)})
	}
	cards.Paragraphs = []string{rc.LabelF(locale, "personalCycles", longDate(locale, n.AsOf))}

	portrait, ok := rc.Portrait(n.LifePath.Number)
	if !ok {
		portrait, _ = rc.Portrait(9)
	}
	meaning := Section{Title: rc.LabelF(locale, "lifePathTitle", fmt.Sprint(n.LifePath.Number)),
		Paragraphs: []string{
			t(portrait.Title),
			t(portrait.Body),
			lbl("howItAddsUp") + " " + strings.Join(LifePathSteps(rc, locale, n.LifePath), " · "),
		}}
	cta := Section{Title: lbl("lifePathCtaTitle"), Paragraphs: []string{lbl("lifePathCtaBody")}, Action: &psychics}
	return []Section{cards, meaning, cta}
}

// LifePathSteps are the lines of "How it adds up": the month, the day and the
// year each reduced, then the total ("Month: 11", "Day: 29 → 11",
// "Year: 1990 → 1", "Total: 23 → 5"). The web's lifePathSteps prints the same
// lines; both are checked against testdata/report_parity.json.
func LifePathSteps(rc *reportcopy.Copy, locale string, r astro.Reduction) []string {
	last := func(step string) string {
		if i := strings.LastIndex(step, "="); i >= 0 {
			return step[i+1:]
		}
		return step
	}
	chain := func(first int, steps []string) string {
		parts := []string{fmt.Sprint(first)}
		for _, s := range steps {
			parts = append(parts, last(s))
		}
		return strings.Join(parts, " → ")
	}
	out := make([]string, 0, len(r.Components)+1)
	for _, c := range r.Components {
		label := "year"
		switch c.Label {
		case "month", "day":
			label = c.Label
		}
		out = append(out, rc.LabelF(locale, "stepLine", rc.Label(locale, label), chain(c.Value, c.Steps)))
	}
	return append(out, rc.LabelF(locale, "stepTotal", chain(r.Total, r.Final)))
}

// reportFootnote is the disclaimer and the gazetteer attribution.
func reportFootnote(rc *reportcopy.Copy, locale string, d CalculatorReportData) string {
	var parts []string
	if d.Disclaimer != "" {
		parts = append(parts, rc.T(locale, d.Disclaimer))
	}
	if d.Attribution != "" {
		parts = append(parts, d.Attribution)
	}
	return strings.Join(parts, " ")
}

// ReportFragment is the URL fragment (with its leading "#") that reopens a
// result on the page: `#v=1&d=1990-03-14&t=08%3A45&p=gn%3A2988507&h=whole_sign`.
// It is the encoding of frontend/src/lib/astro/fragment.ts (checked against
// testdata/report_parity.json): keys in the order v, d, t, p, h; the time only
// with a birth time; the house system only for natal charts and only when it
// is not Placidus.
func ReportFragment(kind, birthDate, birthTime, placeID, houseSystem string) string {
	var b strings.Builder
	b.WriteString("#v=1&d=" + url.QueryEscape(birthDate))
	if kind != KindPathOfLife {
		if birthTime != "" {
			b.WriteString("&t=" + url.QueryEscape(birthTime))
		}
		if placeID != "" {
			b.WriteString("&p=" + url.QueryEscape(placeID))
		}
	}
	if kind == KindNatalChart && houseSystem != "" && houseSystem != "placidus" {
		b.WriteString("&h=" + url.QueryEscape(houseSystem))
	}
	return b.String()
}

var (
	monthsFR = [...]string{"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"}
	monthsES = [...]string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}
)

// longDate is the date in the locale's long style, like
// Intl.DateTimeFormat(locale, {dateStyle: "long"}): "November 4, 2026",
// "4 novembre 2026", "4 de noviembre de 2026".
func longDate(locale, date string) string {
	d, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return date
	}
	switch locale {
	case "fr":
		return fmt.Sprintf("%d %s %d", d.Day(), monthsFR[d.Month()-1], d.Year())
	case "es":
		return fmt.Sprintf("%d de %s de %d", d.Day(), monthsES[d.Month()-1], d.Year())
	}
	return fmt.Sprintf("%s %d, %d", d.Month(), d.Day(), d.Year())
}
