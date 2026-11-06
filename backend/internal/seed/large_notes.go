package seed

import (
	"context"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// Load-test data for client notes and advisor presence history. Registered
// as large-profile generators, so they run after the built-in chats step and
// can attach to its sessions.
func init() {
	RegisterLarge(Generator{Name: "client notes", Run: func(_ context.Context, d *Dataset) error { return d.clientNotes() }})
	RegisterLarge(Generator{Name: "presence history", Run: func(_ context.Context, d *Dataset) error { return d.presenceHistory() }})
}

var notePhrases = []string{
	"Asked about a career change; anxious about timing.",
	"Recently moved cities and feels isolated.",
	"Relationship questions; partner travels a lot.",
	"Prefers short, direct answers. Likes tarot over astrology.",
	"Wants a follow-up after the new moon.",
	"Mentioned an upcoming job interview next week.",
	"Responds well to the Star card; hopeful tone.",
	"Family tension around the holidays.",
	"First reading ever; explain the spread slowly.",
	"Returning client; asked about the same decision again.",
	"Interested in their rising sign and its meaning.",
	"Planning a business launch this spring.",
}

// clientNotes writes notes for ~60% of advisor–client pairs that met, 1–5
// each, ~25% shared, ~5% soft-deleted. Bodies are sealed like the API does;
// without a sealer the step is skipped.
func (d *Dataset) clientNotes() error {
	if d.Sealer == nil {
		d.Log.Warn("client notes skipped: no sealer configured")
		return nil
	}
	type pair struct{ psychic, client uuid.UUID }
	firstMet := map[pair]time.Time{}
	convOf := map[pair]uuid.UUID{}
	for _, s := range d.Sessions {
		if s.StartedAt.IsZero() {
			continue
		}
		k := pair{s.PsychicID, s.ClientID}
		if t, ok := firstMet[k]; !ok || s.StartedAt.Before(t) {
			firstMet[k], convOf[k] = s.StartedAt, s.ConversationID
		}
	}
	keys := make([]pair, 0, len(firstMet))
	for k := range firstMet {
		keys = append(keys, k)
	}
	// Deterministic order regardless of map iteration.
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].psychic != keys[j].psychic {
			return keys[i].psychic.String() < keys[j].psychic.String()
		}
		return keys[i].client.String() < keys[j].client.String()
	})
	batch := make([]domain.ClientNote, 0, d.Opts.BatchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := d.insert("client_notes", &batch, len(batch), true)
		batch = batch[:0]
		return err
	}
	for _, k := range keys {
		if d.Rand.Float64() >= 0.6 {
			continue
		}
		met := firstMet[k]
		conv := convOf[k]
		for i, n := 0, 1+d.Rand.IntN(5); i < n; i++ {
			at := met
			if span := d.Now.Sub(met); span > time.Second {
				at = met.Add(time.Duration(d.Rand.Int64N(int64(span/time.Second))) * time.Second)
			}
			body := notePhrases[d.Rand.IntN(len(notePhrases))]
			note := domain.ClientNote{ID: domain.NewID(), ClientID: k.client, AuthorID: k.psychic, ConversationID: &conv,
				BodyChars: utf8.RuneCountInString(body), Shared: d.Rand.Float64() < 0.25, CreatedAt: at, UpdatedAt: at}
			sealed, err := d.Sealer.Seal(body, "note:"+note.ID.String()+":"+note.AuthorID.String())
			if err != nil {
				return err
			}
			note.BodyEnc = sealed
			if d.Rand.Float64() < 0.05 {
				note.DeletedAt.Time, note.DeletedAt.Valid = at.Add(time.Hour), true
			}
			batch = append(batch, note)
			if len(batch) == cap(batch) {
				if err := flush(); err != nil {
					return err
				}
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}
	d.Log.Info("client notes generated", zap.Int("rows", d.Counts["client_notes"]))
	return nil
}

// presenceHistory gives every advisor 60 days of availability: one to three
// stretches a day (online, busy or away). A quarter of advisors end with an
// open interval (currently available).
func (d *Dataset) presenceHistory() error {
	statuses := []domain.PresenceStatus{domain.PresenceOnline, domain.PresenceOnline, domain.PresenceBusy, domain.PresenceAway}
	batch := make([]domain.PresenceInterval, 0, d.Opts.BatchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := d.insert("psychic_presence_intervals", &batch, len(batch), true)
		batch = batch[:0]
		return err
	}
	day := 24 * time.Hour
	start := d.Now.Add(-60 * day).Truncate(day)
	for _, psychic := range d.Psychics {
		var last *domain.PresenceInterval
		for dayStart := start; dayStart.Before(d.Now.Truncate(day)); dayStart = dayStart.Add(day) {
			at := dayStart.Add(time.Duration(6+d.Rand.IntN(6)) * time.Hour)
			for i, n := 0, 1+d.Rand.IntN(3); i < n; i++ {
				end := at.Add(time.Duration(30+d.Rand.IntN(210)) * time.Minute)
				iv := domain.PresenceInterval{ID: domain.NewID(), PsychicID: psychic, Status: statuses[d.Rand.IntN(len(statuses))],
					StartedAt: at, CreatedAt: at, UpdatedAt: end}
				e := end
				iv.EndedAt = &e
				batch = append(batch, iv)
				last = &batch[len(batch)-1]
				at = end.Add(time.Duration(15+d.Rand.IntN(120)) * time.Minute)
				if len(batch) == cap(batch) {
					if err := flush(); err != nil {
						return err
					}
					last = nil
				}
			}
		}
		// Leave the last interval open for some advisors (still online).
		if last != nil && d.Rand.Float64() < 0.25 {
			last.EndedAt = nil
		}
	}
	if err := flush(); err != nil {
		return err
	}
	d.Log.Info("presence history generated", zap.Int("rows", d.Counts["psychic_presence_intervals"]))
	return nil
}
