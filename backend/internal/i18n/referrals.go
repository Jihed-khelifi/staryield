package i18n

// Growth: the referral loop. Server messages of the invite endpoints, the
// admin console's referral messages and the three notification e-mails / push
// texts. Merged into the main catalogues at start-up like receipts.go.
// "Referral" is the resource name in "Referral not found.".

var frReferrals = map[string]string{
	"Referrals are not available right now.":                       "Le parrainage n'est pas disponible pour le moment.",
	"Only clients can invite friends.":                             "Seuls les clients peuvent inviter des amis.",
	"Referral":                                                     "Parrainage",
	"Use a whole number between %d and %d.":                        "Utilisez un nombre entier entre %d et %d.",
	"Choose joined, qualified, granted or rejected.":               "Choisissez joined, qualified, granted ou rejected.",
	"Give a reason of 3 to 200 characters.":                        "Indiquez un motif de 3 à 200 caractères.",
	"Only a referral held for review can be approved.":             "Seul un parrainage en attente de vérification peut être approuvé.",
	"Only a referral that is joined or qualified can be rejected.": "Seul un parrainage inscrit ou qualifié peut être rejeté.",
	// Notifications (e-mail and push).
	"A friend you invited joined Staryield":                                                      "Un ami que vous avez invité a rejoint Staryield",
	"A friend signed up with your invite link.":                                                  "Un ami s'est inscrit avec votre lien d'invitation.",
	"Your free minutes are earned once they verify their e-mail address and complete a reading.": "Vos minutes gratuites sont acquises dès qu'il a vérifié son adresse e-mail et terminé une consultation.",
	"See your invites":             "Voir vos invitations",
	"Your free minutes were added": "Vos minutes gratuites ont été ajoutées",
	"%d free minutes from the friends you invited were added to your wallet.": "%d minutes gratuites offertes par les amis que vous avez invités ont été ajoutées à votre portefeuille.",
	"1 free minute from the friends you invited was added to your wallet.":    "1 minute gratuite offerte par les amis que vous avez invités a été ajoutée à votre portefeuille.",
	"Thank you for inviting your friends to Staryield.":                       "Merci d'avoir invité vos amis sur Staryield.",
}

var esReferrals = map[string]string{
	"Referrals are not available right now.":                       "Las invitaciones no están disponibles por ahora.",
	"Only clients can invite friends.":                             "Solo los clientes pueden invitar a amigos.",
	"Referral":                                                     "Invitación",
	"Use a whole number between %d and %d.":                        "Usa un número entero entre %d y %d.",
	"Choose joined, qualified, granted or rejected.":               "Elige joined, qualified, granted o rejected.",
	"Give a reason of 3 to 200 characters.":                        "Indica un motivo de 3 a 200 caracteres.",
	"Only a referral held for review can be approved.":             "Solo se puede aprobar una invitación retenida para revisión.",
	"Only a referral that is joined or qualified can be rejected.": "Solo se puede rechazar una invitación registrada o calificada.",
	// Notifications (e-mail and push).
	"A friend you invited joined Staryield":                                                      "Un amigo al que invitaste se unió a Staryield",
	"A friend signed up with your invite link.":                                                  "Un amigo se registró con tu enlace de invitación.",
	"Your free minutes are earned once they verify their e-mail address and complete a reading.": "Ganas tus minutos gratis cuando verifique su correo y complete una lectura.",
	"See your invites":             "Ver tus invitaciones",
	"Your free minutes were added": "Tus minutos gratis se añadieron",
	"%d free minutes from the friends you invited were added to your wallet.": "Se añadieron a tu monedero %d minutos gratis de los amigos que invitaste.",
	"1 free minute from the friends you invited was added to your wallet.":    "Se añadió a tu monedero 1 minuto gratis de los amigos que invitaste.",
	"Thank you for inviting your friends to Staryield.":                       "Gracias por invitar a tus amigos a Staryield.",
}

func init() {
	for k, v := range frReferrals {
		fr[k] = v
	}
	for k, v := range esReferrals {
		es[k] = v
	}
	templates = buildTemplates()
}
