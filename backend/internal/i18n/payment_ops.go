package i18n

// Copy of card payment operations (revisions 6.0, §26.5): errors, the admin
// console's status messages and the staff-facing text of alerts and dispute
// evidence. Merged at start-up like refunds.go.

var frPaymentOps = map[string]string{
	"Your account has a payment dispute in progress. Contact support.":            "Votre compte fait l'objet d'un litige de paiement en cours. Contactez le support.",
	"This payment is disputed. Answer the dispute instead of refunding it.":       "Ce paiement est contesté. Répondez au litige plutôt que de le rembourser.",
	"The payment provider did not answer. Please try again.":                      "Le prestataire de paiement n'a pas répondu. Veuillez réessayer.",
	"The payment provider did not answer. Try again in a minute.":                 "Le prestataire de paiement n'a pas répondu. Réessayez dans une minute.",
	"Payment notifications are not available.":                                    "Les notifications de paiement ne sont pas disponibles.",
	"Payment operations are not available.":                                       "Les opérations de paiement ne sont pas disponibles.",
	"A note (up to 500 characters) is required.":                                  "Une note (500 caractères maximum) est obligatoire.",
	"Choose automatic or any.":                                                    "Choisissez automatique ou toutes.",
	"Use at most %d characters.":                                                  "Utilisez au plus %d caractères.",
	"Use the format YYYY-MM-DD.":                                                  "Utilisez le format AAAA-MM-JJ.",
	"Describe the product to submit the evidence.":                                "Décrivez le produit pour soumettre les preuves.",
	"Evidence was already submitted for this dispute. It cannot be changed.":      "Les preuves ont déjà été soumises pour ce litige. Elles ne peuvent plus être modifiées.",
	"This dispute no longer accepts evidence.":                                    "Ce litige n'accepte plus de preuves.",
	"This dispute is already closed.":                                             "Ce litige est déjà clos.",
	"The provider does not know this dispute.":                                    "Le prestataire ne connaît pas ce litige.",
	"The provider no longer has this event.":                                      "Le prestataire n'a plus cet événement.",
	"This event has no provider id and cannot be fetched again.":                  "Cet événement n'a pas d'identifiant chez le prestataire et ne peut pas être récupéré à nouveau.",
	"This payment provider cannot replay events.":                                 "Ce prestataire de paiement ne permet pas de rejouer les événements.",
	"This payment provider cannot report its account status.":                     "Ce prestataire de paiement ne peut pas indiquer l'état de son compte.",
	"This payment provider does not support answering disputes from the console.": "Ce prestataire de paiement ne permet pas de répondre aux litiges depuis la console.",
	"This payment provider is not configured.":                                    "Ce prestataire de paiement n'est pas configuré.",
	"This payment has nothing left to refund.":                                    "Il ne reste rien à rembourser sur ce paiement.",
	"This warning has already been handled.":                                      "Cette alerte a déjà été traitée.",
	"No webhook endpoint points at this API.":                                     "Aucun point de terminaison de webhook ne pointe vers cette API.",
	"The webhook endpoint is disabled.":                                           "Le point de terminaison de webhook est désactivé.",
	"Some webhook deliveries failed in the last 24 hours.":                        "Certaines livraisons de webhook ont échoué au cours des dernières 24 heures.",
	"The account cannot take charges yet.":                                        "Le compte ne peut pas encore encaisser de paiements.",
	"Payouts are not enabled on the account.":                                     "Les virements ne sont pas activés sur le compte.",
	"The account's statement descriptor does not match the expected one.":         "Le libellé de relevé du compte ne correspond pas à celui attendu.",
	"Refunded automatically.":                                                     "Remboursé automatiquement.",
	"Refund the payment before it is disputed, or dismiss the warning.":           "Remboursez le paiement avant qu'il ne soit contesté, ou écartez l'alerte.",
	"Decide from the console: refund by hand or dismiss the warning.":             "Décidez depuis la console : remboursez à la main ou écartez l'alerte.",
	"The funds stay with the cardholder and no credits were restored.":            "Les fonds restent au titulaire de la carte et aucun crédit n'a été restitué.",
	"Credits taken back: %d.":                                                     "Crédits repris : %d.",
	"The customer has not been paid back. The credits taken back for this refund were not restored: restore them from the console if the customer keeps the service.": "Le client n'a pas été remboursé. Les crédits repris pour ce remboursement n'ont pas été restitués : restituez-les depuis la console si le client conserve le service.",
	"Handle it from the payments pages of the console. Do not forward this message outside the team.":                                                                 "Traitez-le depuis les pages de paiements de la console. Ne transférez pas ce message en dehors de l'équipe.",
	"%s (%s) paid %s on %s.":                          "%s (%s) a payé %s le %s.",
	"No reading has been started since the purchase.": "Aucune consultation n'a été commencée depuis l'achat.",
	"The Refund & Cancellation Policy is published at %s and linked next to the payment button.":                                                    "La Politique de remboursement et d'annulation est publiée sur %s et reliée à côté du bouton de paiement.",
	"The credits are added to the customer's Staryield account as soon as the payment succeeds and are spent on live chat readings, by the minute.": "Les crédits sont ajoutés au compte Staryield du client dès que le paiement aboutit et sont dépensés en consultations par chat en direct, à la minute.",
	"The membership renews monthly until the customer cancels it from the account settings; the customer accepted the terms before subscribing.":    "L'abonnement se renouvelle chaque mois jusqu'à ce que le client le résilie depuis les paramètres du compte ; le client a accepté les conditions avant de s'abonner.",
	"This was a one-time purchase of digital credits. It is not a subscription and there is nothing to cancel.":                                     "Il s'agissait d'un achat unique de crédits numériques. Ce n'est pas un abonnement et il n'y a rien à résilier.",
}

var esPaymentOps = map[string]string{
	"Your account has a payment dispute in progress. Contact support.":            "Tu cuenta tiene una disputa de pago en curso. Contacta con el soporte.",
	"This payment is disputed. Answer the dispute instead of refunding it.":       "Este pago está en disputa. Responde a la disputa en lugar de reembolsarlo.",
	"The payment provider did not answer. Please try again.":                      "El proveedor de pagos no respondió. Inténtalo de nuevo.",
	"The payment provider did not answer. Try again in a minute.":                 "El proveedor de pagos no respondió. Inténtalo de nuevo en un minuto.",
	"Payment notifications are not available.":                                    "Las notificaciones de pago no están disponibles.",
	"Payment operations are not available.":                                       "Las operaciones de pago no están disponibles.",
	"A note (up to 500 characters) is required.":                                  "Se requiere una nota (hasta 500 caracteres).",
	"Choose automatic or any.":                                                    "Elige automático o todas.",
	"Use at most %d characters.":                                                  "Usa como máximo %d caracteres.",
	"Use the format YYYY-MM-DD.":                                                  "Usa el formato AAAA-MM-DD.",
	"Describe the product to submit the evidence.":                                "Describe el producto para enviar las pruebas.",
	"Evidence was already submitted for this dispute. It cannot be changed.":      "Ya se enviaron pruebas para esta disputa. No se pueden modificar.",
	"This dispute no longer accepts evidence.":                                    "Esta disputa ya no admite pruebas.",
	"This dispute is already closed.":                                             "Esta disputa ya está cerrada.",
	"The provider does not know this dispute.":                                    "El proveedor no conoce esta disputa.",
	"The provider no longer has this event.":                                      "El proveedor ya no tiene este evento.",
	"This event has no provider id and cannot be fetched again.":                  "Este evento no tiene identificador en el proveedor y no se puede obtener de nuevo.",
	"This payment provider cannot replay events.":                                 "Este proveedor de pagos no permite reproducir eventos.",
	"This payment provider cannot report its account status.":                     "Este proveedor de pagos no puede informar del estado de su cuenta.",
	"This payment provider does not support answering disputes from the console.": "Este proveedor de pagos no permite responder a disputas desde la consola.",
	"This payment provider is not configured.":                                    "Este proveedor de pagos no está configurado.",
	"This payment has nothing left to refund.":                                    "No queda nada por reembolsar en este pago.",
	"This warning has already been handled.":                                      "Esta alerta ya se ha tratado.",
	"No webhook endpoint points at this API.":                                     "Ningún punto de conexión de webhook apunta a esta API.",
	"The webhook endpoint is disabled.":                                           "El punto de conexión del webhook está desactivado.",
	"Some webhook deliveries failed in the last 24 hours.":                        "Algunas entregas de webhook fallaron en las últimas 24 horas.",
	"The account cannot take charges yet.":                                        "La cuenta aún no puede cobrar pagos.",
	"Payouts are not enabled on the account.":                                     "Los pagos a la cuenta bancaria no están activados.",
	"The account's statement descriptor does not match the expected one.":         "El descriptor del extracto de la cuenta no coincide con el esperado.",
	"Refunded automatically.":                                                     "Reembolsado automáticamente.",
	"Refund the payment before it is disputed, or dismiss the warning.":           "Reembolsa el pago antes de que se dispute, o descarta la alerta.",
	"Decide from the console: refund by hand or dismiss the warning.":             "Decide desde la consola: reembolsa a mano o descarta la alerta.",
	"The funds stay with the cardholder and no credits were restored.":            "Los fondos se quedan con el titular de la tarjeta y no se restituyó ningún crédito.",
	"Credits taken back: %d.":                                                     "Créditos retirados: %d.",
	"The customer has not been paid back. The credits taken back for this refund were not restored: restore them from the console if the customer keeps the service.": "No se ha reembolsado al cliente. Los créditos retirados por este reembolso no se restituyeron: restitúyelos desde la consola si el cliente conserva el servicio.",
	"Handle it from the payments pages of the console. Do not forward this message outside the team.":                                                                 "Gestiónalo desde las páginas de pagos de la consola. No reenvíes este mensaje fuera del equipo.",
	"%s (%s) paid %s on %s.":                          "%s (%s) pagó %s el %s.",
	"No reading has been started since the purchase.": "No se ha iniciado ninguna consulta desde la compra.",
	"The Refund & Cancellation Policy is published at %s and linked next to the payment button.":                                                    "La Política de reembolso y cancelación está publicada en %s y enlazada junto al botón de pago.",
	"The credits are added to the customer's Staryield account as soon as the payment succeeds and are spent on live chat readings, by the minute.": "Los créditos se añaden a la cuenta de Staryield del cliente en cuanto el pago se realiza y se gastan en consultas por chat en directo, por minuto.",
	"The membership renews monthly until the customer cancels it from the account settings; the customer accepted the terms before subscribing.":    "La membresía se renueva cada mes hasta que el cliente la cancele desde los ajustes de la cuenta; el cliente aceptó las condiciones antes de suscribirse.",
	"This was a one-time purchase of digital credits. It is not a subscription and there is nothing to cancel.":                                     "Fue una compra única de créditos digitales. No es una suscripción y no hay nada que cancelar.",
}

// Merge the payment operations catalogues into the main ones, then rebuild the %s/%d templates.
func init() {
	for k, v := range frPaymentOps {
		fr[k] = v
	}
	for k, v := range esPaymentOps {
		es[k] = v
	}
	templates = buildTemplates()
}
