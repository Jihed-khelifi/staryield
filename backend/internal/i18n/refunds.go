package i18n

// Copy of refund requests (revisions 5.0, §24.4): errors, validation messages
// and the notification e-mails. Merged at start-up like pricing.go.

var frRefunds = map[string]string{
	"A refund request for this payment is already open.":                   "Une demande de remboursement pour ce paiement est déjà ouverte.",
	"Only credit pack payments can be refunded this way.":                  "Seuls les paiements de packs de crédits peuvent être remboursés de cette manière.",
	"This payment was not completed, so there is nothing to refund.":       "Ce paiement n'a pas abouti : il n'y a rien à rembourser.",
	"This payment has already been refunded in full.":                      "Ce paiement a déjà été remboursé en totalité.",
	"The refund window for unused credits has ended for this payment.":     "Le délai de remboursement des crédits inutilisés est écoulé pour ce paiement.",
	"This payment is not eligible for a refund request.":                   "Ce paiement n'est pas éligible à une demande de remboursement.",
	"Refund requests are not available right now. Please contact support.": "Les demandes de remboursement ne sont pas disponibles pour le moment. Veuillez contacter le support.",
	"Choose a payment.":                                          "Choisissez un paiement.",
	"The revenue report is limited to 366 days.":                 "Le rapport de revenus est limité à 366 jours.",
	"Choose a reason.":                                           "Choisissez un motif.",
	"Details can be at most 2000 characters.":                    "Les précisions ne peuvent pas dépasser 2000 caractères.",
	"This key was already used for another payment.":             "Cette clé a déjà été utilisée pour un autre paiement.",
	"This refund request has already been resolved.":             "Cette demande de remboursement a déjà été traitée.",
	"Write a note of 5 to 1000 characters.":                      "Rédigez une note de 5 à 1000 caractères.",
	"Use a whole number of credits between 1 and %d.":            "Utilisez un nombre entier de crédits compris entre 1 et %d.",
	"Refund request not found.":                                  "Demande de remboursement introuvable.",
	"Choose how the request was received.":                       "Indiquez comment la demande a été reçue.",
	"We received your refund request":                            "Nous avons reçu votre demande de remboursement",
	"We received your refund request %s.":                        "Nous avons reçu votre demande de remboursement %s.",
	"We received your refund request.":                           "Nous avons reçu votre demande de remboursement.",
	"Our team will review it and reply by email and in the app.": "Notre équipe va l'examiner et vous répondra par e-mail et dans l'application.",
	"View your requests":                                         "Voir vos demandes",
	"Update on your refund request":                              "Mise à jour de votre demande de remboursement",
	"Your refund request %s was approved.":                       "Votre demande de remboursement %s a été acceptée.",
	"Your refund request was approved.":                          "Votre demande de remboursement a été acceptée.",
	"Amount refunded: %s.":                                       "Montant remboursé : %s.",
	"It goes back to your original payment method; your bank may take a few days to show it.": "Il est recrédité sur votre moyen de paiement d'origine ; votre banque peut mettre quelques jours à l'afficher.",
	"Your refund request %s was answered with courtesy credits.":                              "Votre demande de remboursement %s a reçu une réponse sous forme de crédits offerts.",
	"Your refund request was answered with courtesy credits.":                                 "Votre demande de remboursement a reçu une réponse sous forme de crédits offerts.",
	"Your refund request %s was reviewed and could not be approved.":                          "Votre demande de remboursement %s a été examinée mais n'a pas pu être acceptée.",
	"Your refund request was reviewed and could not be approved.":                             "Votre demande de remboursement a été examinée mais n'a pas pu être acceptée.",
	"Open your billing settings to read our note.":                                            "Ouvrez vos paramètres de facturation pour lire notre message.",
}

var esRefunds = map[string]string{
	"A refund request for this payment is already open.":                   "Ya hay una solicitud de reembolso abierta para este pago.",
	"Only credit pack payments can be refunded this way.":                  "Solo los pagos de paquetes de créditos pueden reembolsarse de esta manera.",
	"This payment was not completed, so there is nothing to refund.":       "Este pago no se completó, así que no hay nada que reembolsar.",
	"This payment has already been refunded in full.":                      "Este pago ya se reembolsó por completo.",
	"The refund window for unused credits has ended for this payment.":     "El plazo de reembolso de los créditos sin usar ha terminado para este pago.",
	"This payment is not eligible for a refund request.":                   "Este pago no es apto para una solicitud de reembolso.",
	"Refund requests are not available right now. Please contact support.": "Las solicitudes de reembolso no están disponibles en este momento. Contacta con el soporte.",
	"Choose a payment.":                                          "Elige un pago.",
	"The revenue report is limited to 366 days.":                 "El informe de ingresos está limitado a 366 días.",
	"Choose a reason.":                                           "Elige un motivo.",
	"Details can be at most 2000 characters.":                    "Los detalles no pueden superar los 2000 caracteres.",
	"This key was already used for another payment.":             "Esta clave ya se usó para otro pago.",
	"This refund request has already been resolved.":             "Esta solicitud de reembolso ya se resolvió.",
	"Write a note of 5 to 1000 characters.":                      "Escribe una nota de 5 a 1000 caracteres.",
	"Use a whole number of credits between 1 and %d.":            "Usa un número entero de créditos de entre 1 y %d.",
	"Refund request not found.":                                  "Solicitud de reembolso no encontrada.",
	"Choose how the request was received.":                       "Indica cómo se recibió la solicitud.",
	"We received your refund request":                            "Hemos recibido tu solicitud de reembolso",
	"We received your refund request %s.":                        "Hemos recibido tu solicitud de reembolso %s.",
	"We received your refund request.":                           "Hemos recibido tu solicitud de reembolso.",
	"Our team will review it and reply by email and in the app.": "Nuestro equipo la revisará y te responderá por correo y en la aplicación.",
	"View your requests":                                         "Ver tus solicitudes",
	"Update on your refund request":                              "Novedades sobre tu solicitud de reembolso",
	"Your refund request %s was approved.":                       "Tu solicitud de reembolso %s fue aceptada.",
	"Your refund request was approved.":                          "Tu solicitud de reembolso fue aceptada.",
	"Amount refunded: %s.":                                       "Importe reembolsado: %s.",
	"It goes back to your original payment method; your bank may take a few days to show it.": "Vuelve a tu método de pago original; tu banco puede tardar unos días en mostrarlo.",
	"Your refund request %s was answered with courtesy credits.":                              "Tu solicitud de reembolso %s se resolvió con créditos de cortesía.",
	"Your refund request was answered with courtesy credits.":                                 "Tu solicitud de reembolso se resolvió con créditos de cortesía.",
	"Your refund request %s was reviewed and could not be approved.":                          "Tu solicitud de reembolso %s fue revisada y no pudo aceptarse.",
	"Your refund request was reviewed and could not be approved.":                             "Tu solicitud de reembolso fue revisada y no pudo aceptarse.",
	"Open your billing settings to read our note.":                                            "Abre tus ajustes de facturación para leer nuestra nota.",
}

// Merge the refund catalogues into the main ones, then rebuild the %s/%d templates.
func init() {
	for k, v := range frRefunds {
		fr[k] = v
	}
	for k, v := range esRefunds {
		es[k] = v
	}
	templates = buildTemplates()
}
