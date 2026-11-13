package seed

import "github.com/Jihed-khelifi/staryield/backend/internal/domain"

// CanonicalPacks is the pricing catalog of the pricing spec (§24.1), in
// canonical order. Migration 0026 inserts the same rows; a test keeps the two
// in step.
func CanonicalPacks() []domain.CreditPack {
	l := func(en, fr, es string) domain.L10n { return domain.L10n{"en": en, "fr": fr, "es": es} }
	return []domain.CreditPack{
		{Code: "welcome_600", Kind: domain.PackIntro, Name: "Welcome Pack", Credits: 600, PriceCents: 1000, Currency: "USD",
			Active: true, NewClientsOnly: true, SortOrder: 1,
			NameL10n:    l("Welcome Pack", "Pack de bienvenue", "Pack de bienvenida"),
			TaglineL10n: l("Extra time to explore your first paid reading.", "Du temps en plus pour découvrir votre première lecture payante.", "Más tiempo para explorar tu primera lectura de pago."),
			LabelL10n:   domain.L10n{}},
		{Code: "quick_150", Kind: domain.PackStandard, Name: "Quick Insight", Credits: 150, PriceCents: 2000, Currency: "USD",
			Active: true, Reference: true, SortOrder: 2,
			NameL10n:    l("Quick Insight", "Éclairage express", "Consulta rápida"),
			TaglineL10n: l("For one focused question.", "Pour une question précise.", "Para una pregunta concreta."),
			LabelL10n:   l("One focused question", "Une question précise", "Una pregunta concreta")},
		{Code: "starter_315", Kind: domain.PackStandard, Name: "Starter", Credits: 315, PriceCents: 4000, Currency: "USD",
			Active: true, SortOrder: 3,
			NameL10n:    l("Starter", "Démarrage", "Inicial"),
			TaglineL10n: l("A little more time for a fuller conversation.", "Un peu plus de temps pour une conversation plus complète.", "Un poco más de tiempo para una conversación más completa."),
			LabelL10n:   domain.L10n{}},
		{Code: "popular_675", Kind: domain.PackStandard, Name: "Most Popular", Credits: 675, PriceCents: 8000, Currency: "USD",
			Active: true, Featured: true, SortOrder: 4,
			NameL10n:    l("Most Popular", "Le plus populaire", "Más popular"),
			TaglineL10n: l("The best balance of price and reading time.", "Le meilleur équilibre entre prix et durée de lecture.", "El mejor equilibrio entre precio y tiempo de lectura."),
			LabelL10n:   domain.L10n{}},
		{Code: "deep_1350", Kind: domain.PackStandard, Name: "Deep Dive", Credits: 1350, PriceCents: 15000, Currency: "USD",
			Active: true, SortOrder: 5,
			NameL10n:    l("Deep Dive", "Exploration approfondie", "Inmersión profunda"),
			TaglineL10n: l("For multiple questions or a longer reading.", "Pour plusieurs questions ou une lecture plus longue.", "Para varias preguntas o una lectura más larga."),
			LabelL10n:   domain.L10n{}},
		{Code: "premium_2400", Kind: domain.PackStandard, Name: "Premium", Credits: 2400, PriceCents: 25000, Currency: "USD",
			Active: true, SortOrder: 6,
			NameL10n:    l("Premium", "Premium", "Premium"),
			TaglineL10n: l("Our best ongoing rate for regular readings.", "Notre meilleur tarif durable pour les lectures régulières.", "Nuestra mejor tarifa continuada para lecturas regulares."),
			LabelL10n:   domain.L10n{}},
	}
}
