package i18n

// Growth (calc stream): the tarot birth card is the fourth calculator counted
// by the calculator analytics reports (GET /admin/analytics/calculators and
// /conversions), whose kind filter now names it. Merged into the main
// catalogues at start-up like calculators.go.

var frCalculatorTarot = map[string]string{
	"Use natal-chart, path-of-life, astrology or tarot-birth-card.": "Utilisez natal-chart, path-of-life, astrology ou tarot-birth-card.",
}

var esCalculatorTarot = map[string]string{
	"Use natal-chart, path-of-life, astrology or tarot-birth-card.": "Usa natal-chart, path-of-life, astrology o tarot-birth-card.",
}

func init() {
	for k, v := range frCalculatorTarot {
		fr[k] = v
	}
	for k, v := range esCalculatorTarot {
		es[k] = v
	}
	templates = buildTemplates()
}
