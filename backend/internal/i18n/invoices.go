package i18n

// revisions 6.0 (docs/platform-extensions.md §26.6): invoices and credit
// notes. These strings are the labels of the invoice PDF (rendered in the
// invoice's locale), the e-mail copy that links and attaches the documents and
// the server messages of the invoice endpoints. Merged at start-up like
// receipts.go; the French and Spanish wording follows the web app's copy.

var frInvoices = map[string]string{
	// Server messages.
	"Invoices are not available yet.":                       "Les factures ne sont pas encore disponibles.",
	"Invoice not found.":                                    "Facture introuvable.",
	"Enter both dates (from and to) of the export.":         "Saisissez les deux dates (du et au) de l'export.",
	"The export covers at most 366 days.":                   "L'export couvre 366 jours au maximum.",
	"Enter a valid date (YYYY-MM-DD).":                      "Saisissez une date valide (AAAA-MM-JJ).",
	"The end date must not be before the start date.":       "La date de fin ne peut pas précéder la date de début.",
	"This payment cannot be invoiced yet.":                  "Ce paiement ne peut pas encore être facturé.",
	"The export is too large. Choose a shorter date range.": "L'export est trop volumineux. Choisissez une période plus courte.",
	// PDF labels.
	"Invoice":              "Facture",
	"Credit note":          "Avoir",
	"Issue date":           "Date d'émission",
	"Billed to":            "Facturé à",
	"Country":              "Pays",
	"Description":          "Description",
	"Qty":                  "Qté",
	"Unit price":           "Prix unitaire",
	"Amount":               "Montant",
	"Subtotal":             "Sous-total",
	"Tax":                  "Taxe",
	"Not applicable":       "Non applicable",
	"Total":                "Total",
	"Total paid":           "Total payé",
	"Total refunded":       "Total remboursé",
	"Payment details":      "Détails du paiement",
	"Payment provider":     "Prestataire de paiement",
	"Payment method":       "Moyen de paiement",
	"Payment reference":    "Référence du paiement",
	"Paid on":              "Payé le",
	"Refunded on":          "Remboursé le",
	"Statement descriptor": "Libellé sur le relevé",
	"Original invoice":     "Facture d'origine",
	"Credits have no cash value and cannot be exchanged for money.": "Les crédits n'ont aucune valeur monétaire et ne peuvent pas être échangés contre de l'argent.",
	// E-mail copy.
	"View invoice":     "Voir la facture",
	"View credit note": "Voir l'avoir",
	"Your invoice is attached to this e-mail as a PDF.":     "Votre facture est jointe à cet e-mail au format PDF.",
	"Your credit note is attached to this e-mail as a PDF.": "Votre avoir est joint à cet e-mail au format PDF.",
}

var esInvoices = map[string]string{
	// Server messages.
	"Invoices are not available yet.":                       "Las facturas aún no están disponibles.",
	"Invoice not found.":                                    "Factura no encontrada.",
	"Enter both dates (from and to) of the export.":         "Introduce las dos fechas (desde y hasta) de la exportación.",
	"The export covers at most 366 days.":                   "La exportación abarca 366 días como máximo.",
	"Enter a valid date (YYYY-MM-DD).":                      "Introduce una fecha válida (AAAA-MM-DD).",
	"The end date must not be before the start date.":       "La fecha de fin no puede ser anterior a la de inicio.",
	"This payment cannot be invoiced yet.":                  "Este pago aún no se puede facturar.",
	"The export is too large. Choose a shorter date range.": "La exportación es demasiado grande. Elige un período más corto.",
	// PDF labels.
	"Invoice":              "Factura",
	"Credit note":          "Nota de crédito",
	"Issue date":           "Fecha de emisión",
	"Billed to":            "Facturado a",
	"Country":              "País",
	"Description":          "Descripción",
	"Qty":                  "Cant.",
	"Unit price":           "Precio unitario",
	"Amount":               "Importe",
	"Subtotal":             "Subtotal",
	"Tax":                  "Impuestos",
	"Not applicable":       "No aplicable",
	"Total":                "Total",
	"Total paid":           "Total pagado",
	"Total refunded":       "Total reembolsado",
	"Payment details":      "Detalles del pago",
	"Payment provider":     "Proveedor de pago",
	"Payment method":       "Método de pago",
	"Payment reference":    "Referencia del pago",
	"Paid on":              "Pagado el",
	"Refunded on":          "Reembolsado el",
	"Statement descriptor": "Descriptor en el extracto",
	"Original invoice":     "Factura original",
	"Credits have no cash value and cannot be exchanged for money.": "Los créditos no tienen valor en efectivo y no se pueden canjear por dinero.",
	// E-mail copy.
	"View invoice":     "Ver la factura",
	"View credit note": "Ver la nota de crédito",
	"Your invoice is attached to this e-mail as a PDF.":     "Tu factura va adjunta a este correo en PDF.",
	"Your credit note is attached to this e-mail as a PDF.": "Tu nota de crédito va adjunta a este correo en PDF.",
}

func init() {
	for k, v := range frInvoices {
		fr[k] = v
	}
	for k, v := range esInvoices {
		es[k] = v
	}
	templates = buildTemplates()
}
