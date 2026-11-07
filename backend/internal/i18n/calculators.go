package i18n

// Validation messages of the calculator analytics reports (revisions 3.0,
// GET /admin/analytics/calculators and /conversions). Merged into the main
// catalogues at start-up.

var frCalculators = map[string]string{
	"The reporting window is limited to 366 days.": "La période du rapport est limitée à 366 jours.",
	"Use natal-chart, path-of-life or astrology.":  "Utilisez natal-chart, path-of-life ou astrology.",
	"Use en, fr or es.":                            "Utilisez en, fr ou es.",
	"Use a UTM source or (direct).":                "Utilisez une source UTM ou (direct).",
}

var esCalculators = map[string]string{
	"The reporting window is limited to 366 days.": "El periodo del informe se limita a 366 días.",
	"Use natal-chart, path-of-life or astrology.":  "Usa natal-chart, path-of-life o astrology.",
	"Use en, fr or es.":                            "Usa en, fr o es.",
	"Use a UTM source or (direct).":                "Usa una fuente UTM o (direct).",
}

func init() {
	for k, v := range frCalculators {
		fr[k] = v
	}
	for k, v := range esCalculators {
		es[k] = v
	}
	templates = buildTemplates()
}
