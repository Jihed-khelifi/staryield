package i18n

// Copy of the pricing module (revisions 5.0, §24.1 and §24.2): admin
// validation messages and the welcome-offer refusals. Merged at start-up like
// site.go.

var frPricing = map[string]string{
	"Another pack was just made the reference. Reload and try again.":                              "Un autre pack vient d'être défini comme référence. Rechargez la page et réessayez.",
	"Choose standard or intro.":                                                                    "Choisissez « standard » ou « intro ».",
	"Enter the pack name in English.":                                                              "Saisissez le nom du pack en anglais.",
	"Only one welcome pack can be active. Deactivate the current one first.":                       "Un seul pack de bienvenue peut être actif. Désactivez d'abord l'actuel.",
	"Only the welcome offer can be limited to new clients.":                                        "Seule l'offre de bienvenue peut être réservée aux nouveaux clients.",
	"Send every pack exactly once.":                                                                "Envoyez chaque pack une seule fois, sans en oublier.",
	"The maximum rate must be at least the minimum.":                                               "Le tarif maximum doit être supérieur ou égal au minimum.",
	"The reference pack must be an active standard pack.":                                          "Le pack de référence doit être un pack standard actif.",
	"The welcome offer has already been used on this account.":                                     "L'offre de bienvenue a déjà été utilisée sur ce compte.",
	"The welcome offer has already been used with this payment method.":                            "L'offre de bienvenue a déjà été utilisée avec ce moyen de paiement.",
	"The welcome offer is for new clients only.":                                                   "L'offre de bienvenue est réservée aux nouveaux clients.",
	"The welcome offer isn't available right now.":                                                 "L'offre de bienvenue n'est pas disponible pour le moment.",
	"This can't be changed after the pack is created.":                                             "Cette valeur ne peut plus être modifiée une fois le pack créé.",
	"This code is already used by another pack.":                                                   "Ce code est déjà utilisé par un autre pack.",
	"Use a lowercase code of 2 to 40 letters, digits and underscores.":                             "Utilisez un code en minuscules de 2 à 40 caractères : lettres, chiffres et tirets bas.",
	"Use a rate between %d and %d credits.":                                                        "Utilisez un tarif compris entre %d et %d crédits.",
	"Use a whole number between 0 and 100000.":                                                     "Utilisez un nombre entier compris entre 0 et 100000.",
	"Use a whole number between 0 and 3000.":                                                       "Utilisez un nombre entier compris entre 0 et 3000.",
	"Use a whole number between 1 and 10000.":                                                      "Utilisez un nombre entier compris entre 1 et 10000.",
	"Use a whole number between 1 and 100000.":                                                     "Utilisez un nombre entier compris entre 1 et 100000.",
	"Use a whole number between 7 and 365.":                                                        "Utilisez un nombre entier compris entre 7 et 365.",
	"We couldn't verify this card for the welcome offer. Choose another pack or try another card.": "Nous n'avons pas pu vérifier cette carte pour l'offre de bienvenue. Choisissez un autre pack ou essayez une autre carte.",
}

var esPricing = map[string]string{
	"Another pack was just made the reference. Reload and try again.":                              "Otro paquete acaba de convertirse en la referencia. Recarga la página e inténtalo de nuevo.",
	"Choose standard or intro.":                                                                    "Elige «standard» o «intro».",
	"Enter the pack name in English.":                                                              "Escribe el nombre del paquete en inglés.",
	"Only one welcome pack can be active. Deactivate the current one first.":                       "Solo puede haber un paquete de bienvenida activo. Desactiva primero el actual.",
	"Only the welcome offer can be limited to new clients.":                                        "Solo la oferta de bienvenida puede limitarse a clientes nuevos.",
	"Send every pack exactly once.":                                                                "Envía cada paquete una sola vez, sin dejar ninguno fuera.",
	"The maximum rate must be at least the minimum.":                                               "La tarifa máxima debe ser igual o superior a la mínima.",
	"The reference pack must be an active standard pack.":                                          "El paquete de referencia debe ser un paquete estándar activo.",
	"The welcome offer has already been used on this account.":                                     "La oferta de bienvenida ya se ha usado en esta cuenta.",
	"The welcome offer has already been used with this payment method.":                            "La oferta de bienvenida ya se ha usado con este método de pago.",
	"The welcome offer is for new clients only.":                                                   "La oferta de bienvenida es solo para clientes nuevos.",
	"The welcome offer isn't available right now.":                                                 "La oferta de bienvenida no está disponible en este momento.",
	"This can't be changed after the pack is created.":                                             "Este valor no se puede cambiar una vez creado el paquete.",
	"This code is already used by another pack.":                                                   "Este código ya lo usa otro paquete.",
	"Use a lowercase code of 2 to 40 letters, digits and underscores.":                             "Usa un código en minúsculas de 2 a 40 caracteres: letras, números y guiones bajos.",
	"Use a rate between %d and %d credits.":                                                        "Usa una tarifa de entre %d y %d créditos.",
	"Use a whole number between 0 and 100000.":                                                     "Usa un número entero de entre 0 y 100000.",
	"Use a whole number between 0 and 3000.":                                                       "Usa un número entero de entre 0 y 3000.",
	"Use a whole number between 1 and 10000.":                                                      "Usa un número entero de entre 1 y 10000.",
	"Use a whole number between 1 and 100000.":                                                     "Usa un número entero de entre 1 y 100000.",
	"Use a whole number between 7 and 365.":                                                        "Usa un número entero de entre 7 y 365.",
	"We couldn't verify this card for the welcome offer. Choose another pack or try another card.": "No hemos podido verificar esta tarjeta para la oferta de bienvenida. Elige otro paquete o prueba con otra tarjeta.",
}

// Merge the pricing catalogues into the main ones, then rebuild the %s/%d
// templates (package variables are initialised before init functions run).
func init() {
	for k, v := range frPricing {
		fr[k] = v
	}
	for k, v := range esPricing {
		es[k] = v
	}
	templates = buildTemplates()
}
