package e2e

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// The advisor's client card shows natal placements from stored birth data;
// the date and place of birth only appear once the client opts in, and the
// birth time and coordinates never do.
func TestClientCardBirthDetailsNeedConsent(t *testing.T) {
	e := newEnv(t)
	client, psychic, _, _, s := startedSession(t, e, 10, 250)
	cardPath := "/api/v1/chat/conversations/" + s.ConversationID.String() + "/client"

	var card service.ClientProfileView
	psychic.must(http.StatusOK, http.MethodGet, cardPath, nil, &card)
	if card.BirthDetailsShared || card.BirthDetails != nil {
		t.Fatalf("birth details shown without consent: %+v", card)
	}
	if card.Astro == nil || len(card.Astro.Placements) < 8 || card.Astro.TimeKnown {
		t.Fatalf("placements from the birth date: %+v", card.Astro)
	}
	for _, p := range card.Astro.Placements {
		if p.Degree != nil || p.House != 0 {
			t.Fatalf("degrees and houses need a birth time: %+v", p)
		}
		if p.Body == "Moon" && card.Astro.MoonSign == "" {
			t.Fatal("an uncertain Moon must not be listed")
		}
	}

	client.must(http.StatusOK, http.MethodPatch, "/api/v1/me/settings", map[string]any{
		"profile": map[string]any{"birth_time": "14:30", "birth_place_id": "gn:2988507", "birth_place_label": "Paris, Île-de-France, France"},
		"privacy": map[string]any{"share_birth_details_with_advisors": true},
	}, nil)
	card = service.ClientProfileView{}
	r := psychic.must(http.StatusOK, http.MethodGet, cardPath, nil, &card)
	if !card.BirthDetailsShared || card.BirthDetails == nil || card.BirthDetails.DateOfBirth != "1990-05-17" ||
		card.BirthDetails.Place != "Paris, Île-de-France, France" {
		t.Fatalf("shared birth details: %+v", card.BirthDetails)
	}
	if !card.Astro.TimeKnown || card.Astro.RisingSign == "" {
		t.Fatalf("time and place known: %+v", card.Astro)
	}
	withHouse := 0
	for _, p := range card.Astro.Placements {
		if p.Degree == nil || *p.Degree < 0 || *p.Degree > 29 {
			t.Fatalf("degree within the sign: %+v", p)
		}
		if p.House >= 1 && p.House <= 12 {
			withHouse++
		}
	}
	if withHouse != len(card.Astro.Placements) {
		t.Fatalf("every placement has a house: %+v", card.Astro.Placements)
	}
	for _, banned := range []string{"14:30", "latitude", "longitude", "48.85", "2.35", "timezone", client.user.Email} {
		if strings.Contains(string(r.body), banned) {
			t.Fatalf("client card leaks %q: %s", banned, r.body)
		}
	}

	// Turning sharing off hides them again.
	client.must(http.StatusOK, http.MethodPatch, "/api/v1/me/settings", map[string]any{
		"privacy": map[string]any{"share_birth_details_with_advisors": false}}, nil)
	var after service.ClientProfileView
	psychic.must(http.StatusOK, http.MethodGet, cardPath, nil, &after)
	if after.BirthDetailsShared || after.BirthDetails != nil {
		t.Fatalf("birth details still shown after opting out: %+v", after.BirthDetails)
	}
}
