package i18n

// revisions 5.1 (docs/platform-extensions.md §25): the memberships switch, the
// purchase consent message, the company footer of every e-mail and the
// confirmation e-mails (receipt, refunds, memberships). Merged at start-up like
// channels.go. Wording follows the web app's copy dictionaries.

var frReceipts = map[string]string{
	"Memberships are not available.": "Les abonnements ne sont pas disponibles.",
	"Accept the Terms of Use, Privacy Policy and Refund & Cancellation Policy to continue.": "Acceptez les Conditions d'utilisation, la Politique de confidentialité et la Politique de remboursement et d'annulation pour continuer.",
	// Footer of every e-mail.
	"Support is available 24/7. We reply within 1–2 days.":            "Le support est disponible 24 h/24 et 7 j/7. Nous répondons sous 1 à 2 jours.",
	"Astrology and psychic readings for entertainment purposes only.": "Astrologie et voyance à des fins de divertissement uniquement.",
	"Support": "Assistance",
	// Links and labels.
	"Refund & Cancellation Policy": "Politique de remboursement et d'annulation",
	"Terms of Use":                 "Conditions d'utilisation",
	"View your billing":            "Voir votre facturation",
	"Cancel membership":            "Résilier l'abonnement",
	// Receipt.
	"Your receipt: %d credits":                               "Votre reçu : %d crédits",
	"Thank you for your purchase":                            "Merci pour votre achat",
	"One-time payment. No subscription or automatic top-up.": "Paiement unique. Aucun abonnement ni rechargement automatique.",
	"This charge appears on your card statement as %s.":      "Ce débit apparaît sur votre relevé de carte sous le nom %s.",
	"Credit pack":         "Pack de crédits",
	"Credits":             "Crédits",
	"credits":             "crédits",
	"Amount paid":         "Montant payé",
	"Date and time (UTC)": "Date et heure (UTC)",
	"Reference":           "Référence",
	"Card":                "Carte",
	// Refunds.
	"Your refund was issued":                                                            "Votre remboursement a été effectué",
	"We refunded %s to your original payment method.":                                   "Nous avons remboursé %s sur votre moyen de paiement d'origine.",
	"It goes back to your original payment method.":                                     "Il est reversé sur votre moyen de paiement d'origine.",
	"It appears as a credit from %s; your bank may take 5–10 business days to show it.": "Il apparaît comme un crédit de %s ; votre banque peut mettre 5 à 10 jours ouvrés pour l'afficher.",
	"Your bank may take 5–10 business days to show it.":                                 "Votre banque peut mettre 5 à 10 jours ouvrés pour l'afficher.",
	"Amount refunded":     "Montant remboursé",
	"Original purchase":   "Achat d'origine",
	"Purchase date (UTC)": "Date d'achat (UTC)",
	// Memberships.
	"Your membership has started":                                              "Votre abonnement a commencé",
	"Your %s membership has started. Thank you for joining us.":                "Votre abonnement %s a commencé. Merci de nous avoir rejoints.",
	"Your membership has started. Thank you for joining us.":                   "Votre abonnement a commencé. Merci de nous avoir rejoints.",
	"You were charged %s.":                                                     "Vous avez été débité de %s.",
	"Amount charged":                                                           "Montant débité",
	"It renews automatically every month until you cancel.":                    "Il se renouvelle automatiquement chaque mois jusqu'à ce que vous le résiliiez.",
	"It renews automatically every %d months until you cancel.":                "Il se renouvelle automatiquement tous les %d mois jusqu'à ce que vous le résiliiez.",
	"Membership plan":                                                          "Formule d'abonnement",
	"Billing period":                                                           "Période de facturation",
	"Next renewal date":                                                        "Prochaine date de renouvellement",
	"You can cancel online at any time from Settings, in the Billing section.": "Vous pouvez résilier en ligne à tout moment depuis les Paramètres, dans la section Facturation.",
}

var esReceipts = map[string]string{
	"Memberships are not available.": "Las membresías no están disponibles.",
	"Accept the Terms of Use, Privacy Policy and Refund & Cancellation Policy to continue.": "Acepta los Términos de uso, la Política de privacidad y la Política de reembolso y cancelación para continuar.",
	// Footer of every e-mail.
	"Support is available 24/7. We reply within 1–2 days.":            "El soporte está disponible 24/7. Respondemos en 1–2 días.",
	"Astrology and psychic readings for entertainment purposes only.": "Astrología y lecturas psíquicas solo con fines de entretenimiento.",
	"Support": "Soporte",
	// Links and labels.
	"Refund & Cancellation Policy": "Política de reembolso y cancelación",
	"Terms of Use":                 "Términos de uso",
	"View your billing":            "Ver tu facturación",
	"Cancel membership":            "Cancelar la membresía",
	// Receipt.
	"Your receipt: %d credits":                               "Tu recibo: %d créditos",
	"Thank you for your purchase":                            "Gracias por tu compra",
	"One-time payment. No subscription or automatic top-up.": "Pago único. Sin suscripción ni recarga automática.",
	"This charge appears on your card statement as %s.":      "Este cargo aparece en el extracto de tu tarjeta como %s.",
	"Credit pack":         "Paquete de créditos",
	"Credits":             "Créditos",
	"credits":             "créditos",
	"Amount paid":         "Importe pagado",
	"Date and time (UTC)": "Fecha y hora (UTC)",
	"Reference":           "Referencia",
	"Card":                "Tarjeta",
	// Refunds.
	"Your refund was issued":                                                            "Tu reembolso se ha realizado",
	"We refunded %s to your original payment method.":                                   "Hemos reembolsado %s a tu método de pago original.",
	"It goes back to your original payment method.":                                     "Vuelve a tu método de pago original.",
	"It appears as a credit from %s; your bank may take 5–10 business days to show it.": "Aparece como un abono de %s; tu banco puede tardar de 5 a 10 días hábiles en mostrarlo.",
	"Your bank may take 5–10 business days to show it.":                                 "Tu banco puede tardar de 5 a 10 días hábiles en mostrarlo.",
	"Amount refunded":     "Importe reembolsado",
	"Original purchase":   "Compra original",
	"Purchase date (UTC)": "Fecha de compra (UTC)",
	// Memberships.
	"Your membership has started":                                              "Tu membresía ha comenzado",
	"Your %s membership has started. Thank you for joining us.":                "Tu membresía %s ha comenzado. Gracias por unirte.",
	"Your membership has started. Thank you for joining us.":                   "Tu membresía ha comenzado. Gracias por unirte.",
	"You were charged %s.":                                                     "Se te ha cobrado %s.",
	"Amount charged":                                                           "Importe cobrado",
	"It renews automatically every month until you cancel.":                    "Se renueva automáticamente cada mes hasta que la canceles.",
	"It renews automatically every %d months until you cancel.":                "Se renueva automáticamente cada %d meses hasta que la canceles.",
	"Membership plan":                                                          "Plan de membresía",
	"Billing period":                                                           "Periodo de facturación",
	"Next renewal date":                                                        "Próxima fecha de renovación",
	"You can cancel online at any time from Settings, in the Billing section.": "Puedes cancelar en línea en cualquier momento desde Ajustes, en la sección Facturación.",
}

func init() {
	for k, v := range frReceipts {
		fr[k] = v
	}
	for k, v := range esReceipts {
		es[k] = v
	}
	templates = buildTemplates()
}
