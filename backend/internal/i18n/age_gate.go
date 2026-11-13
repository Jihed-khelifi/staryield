package i18n

// revisions 5.0: 18+ age gate (§24.6).

var frAgeGate = map[string]string{
	"Enter your date of birth to continue. Staryield is for adults 18 and over.":                                                      "Saisissez votre date de naissance pour continuer. Staryield est réservé aux adultes de 18 ans et plus.",
	"This account is restricted because Staryield is for adults 18 and over. Contact support if you entered the wrong date of birth.": "Ce compte est restreint, car Staryield est réservé aux adultes de 18 ans et plus. Contactez l'assistance si vous avez saisi une mauvaise date de naissance.",
	"The decision must be clear or restrict.":                                                                                         "La décision doit être « clear » ou « restrict ».",
	"A reason between 5 and 500 characters is required.":                                                                              "Un motif de 5 à 500 caractères est requis.",
	"Age reviews apply to client accounts only.":                                                                                      "Les vérifications d'âge ne concernent que les comptes clients.",
}

var esAgeGate = map[string]string{
	"Enter your date of birth to continue. Staryield is for adults 18 and over.":                                                      "Introduce tu fecha de nacimiento para continuar. Staryield es solo para mayores de 18 años.",
	"This account is restricted because Staryield is for adults 18 and over. Contact support if you entered the wrong date of birth.": "Esta cuenta está restringida porque Staryield es solo para mayores de 18 años. Contacta con soporte si introdujiste una fecha de nacimiento incorrecta.",
	"The decision must be clear or restrict.":                                                                                         "La decisión debe ser «clear» o «restrict».",
	"A reason between 5 and 500 characters is required.":                                                                              "Se requiere un motivo de entre 5 y 500 caracteres.",
	"Age reviews apply to client accounts only.":                                                                                      "Las revisiones de edad solo se aplican a cuentas de clientes.",
}

func init() {
	for k, v := range frAgeGate {
		fr[k] = v
	}
	for k, v := range esAgeGate {
		es[k] = v
	}
	templates = buildTemplates()
}
