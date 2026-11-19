package seed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// Load-test data for the calculators' e-mail report (revisions 6.0, §26.4):
// e-mail leads in every marketing state, so the admin list, its filters and the
// address lookup are checked at realistic sizes (TestExplainHotQueries
// "report leads: …"). Run it alone with
//
//	go run ./cmd/seed -profile=large -extend "email leads"
//
// The addresses are sealed placeholders (the load tests never read them) and
// their hashes derive from the run's tag, so every run adds its own leads.
func init() {
	RegisterLarge(Generator{Name: "email leads", Run: func(_ context.Context, d *Dataset) error { return d.emailLeads() }})
}

// emailLeads inserts one lead for every second synthetic user: 60% never
// subscribed, 30% subscribed (a share of them from a pre-ticked box), 10%
// unsubscribed; created over the history window.
func (d *Dataset) emailLeads() error {
	n := max(d.Opts.Users/2, 1)
	kinds := []string{"natal-chart", "path-of-life", "astrology"}
	locales := []string{"en", "fr", "es"}
	countries := []string{"US", "FR", "ES", "GB", "DE", "CA", "BR", "MX", ""}
	batch := make([]domain.EmailLead, 0, d.Opts.BatchSize)
	flush := func() error {
		err := d.insert("email_leads", &batch, len(batch), true)
		batch = batch[:0]
		return err
	}
	for i := 0; i < n; i++ {
		at := d.Past()
		sum := sha256.Sum256([]byte(fmt.Sprintf("lead:%s:%d", d.Tag, i)))
		first := d.Rand.IntN(len(kinds))
		l := domain.EmailLead{ID: d.ID(at), EmailHash: hex.EncodeToString(sum[:]), EmailSealed: "seed", Locale: locales[d.Rand.IntN(3)],
			Country: countries[d.Rand.IntN(len(countries))], Source: kinds[first], Kinds: domain.StringList{kinds[first]},
			ReportsSent: 1 + d.Rand.IntN(3), MarketingStatus: domain.MarketingNone, LastRequestAt: at,
			Timestamps: domain.Timestamps{CreatedAt: at, UpdatedAt: at}}
		if d.Rand.IntN(4) == 0 {
			l.Kinds = append(l.Kinds, kinds[(first+1)%len(kinds)])
		}
		switch r := d.Rand.Float64(); {
		case r < 0.4:
			version, prechecked := "2026-11-19", d.Rand.IntN(3) == 0
			consent := at.Add(time.Second)
			l.ConsentAt, l.ConsentPrechecked, l.ConsentTextVersion = &consent, &prechecked, &version
			l.MarketingStatus = domain.MarketingSubscribed
			if r >= 0.3 { // unsubscribed later
				gone := consent.Add(time.Duration(1+d.Rand.IntN(60*24)) * time.Minute)
				l.MarketingStatus, l.UnsubscribedAt, l.UpdatedAt = domain.MarketingUnsubscribed, &gone, gone
			}
		}
		if len(d.Clients) > 0 && d.Rand.IntN(10) == 0 {
			id := d.Clients[d.Rand.IntN(len(d.Clients))]
			l.UserID = &id
		}
		batch = append(batch, l)
		if len(batch) == cap(batch) {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}
	d.Log.Info("email leads generated", zap.Int("rows", d.Counts["email_leads"]))
	return nil
}
