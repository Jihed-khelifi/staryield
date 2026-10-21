package e2e

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
)

type localizedError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details struct {
			Fields map[string]string `json:"fields"`
		} `json:"details"`
	} `json:"error"`
}

func TestServerMessagesFollowTheUsersLanguage(t *testing.T) {
	e := newEnv(t)
	for locale, want := range map[string]struct{ message, email, terms, auth string }{
		"fr": {"Certains champs nécessitent votre attention.", "Saisissez une adresse e-mail valide.",
			"Veuillez accepter les conditions d'utilisation et la politique de confidentialité.", "Veuillez vous connecter pour continuer."},
		"es": {"Algunos campos necesitan tu atención.", "Introduce un correo electrónico válido.",
			"Acepta los Términos de servicio y la Política de privacidad.", "Inicia sesión para continuar."},
		"en": {"Some fields need attention.", "Enter a valid email address.",
			"Please accept the Terms of Service and Privacy Policy.", "Please sign in to continue."},
	} {
		c := e.client()
		c.locale = locale
		r := c.call(http.MethodPost, "/api/v1/auth/register", map[string]any{"email": "nope", "password": "x", "accept_terms": false})
		var body localizedError
		r.decode(t, &body)
		if body.Error.Code != "validation_failed" || body.Error.Message != want.message || body.Error.Details.Fields["email"] != want.email ||
			body.Error.Details.Fields["accept_terms"] != want.terms {
			t.Errorf("%s: %+v", locale, body.Error)
		}
		// Codes stay stable across languages.
		r = c.call(http.MethodGet, "/api/v1/me", nil)
		r.decode(t, &body)
		if body.Error.Code != "unauthenticated" || body.Error.Message != want.auth {
			t.Errorf("%s unauthenticated: %+v", locale, body.Error)
		}
	}

	// Accept-Language is used when X-Locale is absent; templates keep values.
	c := e.registerClient()
	r := c.raw(http.MethodPatch, "/api/v1/me/profile", jsonBody(t, map[string]any{"location": longText(61)}),
		map[string]string{"Content-Type": "application/json", "X-CSRF-Token": csrfOf(c), "Accept-Language": "es-ES,es;q=0.9"})
	var body localizedError
	r.decode(t, &body)
	if body.Error.Details.Fields["location"] != "Usa como máximo 60 caracteres." {
		t.Errorf("accept-language: %+v", body.Error)
	}
	c.locale = "fr"
	r = c.call(http.MethodGet, "/api/v1/chat/sessions/00000000-0000-0000-0000-000000000000", nil)
	r.decode(t, &body)
	if body.Error.Code != "not_found" || body.Error.Message != "Séance introuvable." {
		t.Errorf("not found: %+v", body.Error)
	}

	// Realtime error frames are localized with the connection's locale.
	c.locale = "es"
	w := c.connect("")
	defer w.conn.Close()
	ev := w.request(realtime.FrameMessageSend, map[string]any{"session_id": "00000000-0000-0000-0000-000000000000", "body": "", "client_msg_id": "m-123456"})
	if ev.Type != realtime.EventError || ev.Err == nil {
		t.Fatalf("expected error frame: %+v", ev)
	}
	b, _ := json.Marshal(ev.Err)
	fields, _ := ev.Err.Details["fields"].(map[string]any)
	if ev.Err.Message != "Algunos campos necesitan tu atención." || fields["session_id"] != "Este campo es obligatorio." {
		t.Fatalf("frame not localized: %s", b)
	}
}

func longText(n int) string {
	out := make([]byte, n)
	for i := range out {
		out[i] = 'a'
	}
	return string(out)
}
