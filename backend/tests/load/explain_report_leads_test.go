//go:build load

package load

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 6.0 (§26.4): the e-mail leads of the calculators' report. The admin
// list pages by keyset (created_at, id) and filters by marketing status, by
// calculator (jsonb containment, GIN) and by exact address (the unique hash);
// every request looks its address up by hash. Seed the volume with
//
//	go run ./cmd/seed -profile=large -extend "email leads"
func reportLeadChecks(db *gorm.DB) []queryCheck {
	var hash string
	if db.Raw(`SELECT email_hash FROM email_leads ORDER BY created_at DESC LIMIT 1`).Row().Scan(&hash) != nil || hash == "" {
		hash = "0000000000000000000000000000000000000000000000000000000000000000"
	}
	var cursor repository.Cursor
	var at time.Time
	var id string
	if db.Raw(`SELECT created_at, id::text FROM email_leads ORDER BY created_at DESC, id DESC OFFSET 500 LIMIT 1`).Row().Scan(&at, &id) == nil {
		cursor = repository.Cursor{At: at, ID: id}
	} else {
		cursor = repository.Cursor{At: time.Now(), ID: "00000000-0000-7000-8000-000000000000"}
	}
	leads := func(f func(ctx context.Context, r repository.EmailLeadRepository)) func(context.Context, *gorm.DB) {
		return func(ctx context.Context, d *gorm.DB) { f(ctx, repository.NewEmailLeadRepository(d)) }
	}
	return []queryCheck{
		{"report leads: lookup by address hash", 10 * time.Millisecond, leads(func(ctx context.Context, r repository.EmailLeadRepository) {
			_, _ = r.ForUpdateByHash(ctx, hash)
		})},
		{"report leads: admin list (all)", 25 * time.Millisecond, leads(func(ctx context.Context, r repository.EmailLeadRepository) {
			_, _ = r.List(ctx, repository.LeadFilter{Limit: 26})
		})},
		{"report leads: admin list (page 20)", 25 * time.Millisecond, leads(func(ctx context.Context, r repository.EmailLeadRepository) {
			_, _ = r.List(ctx, repository.LeadFilter{Limit: 26, Cursor: &cursor})
		})},
		{"report leads: admin list (subscribed)", 25 * time.Millisecond, leads(func(ctx context.Context, r repository.EmailLeadRepository) {
			_, _ = r.List(ctx, repository.LeadFilter{Status: domain.MarketingSubscribed, Limit: 26})
		})},
		{"report leads: admin list (subscribed, page 20)", 25 * time.Millisecond, leads(func(ctx context.Context, r repository.EmailLeadRepository) {
			_, _ = r.List(ctx, repository.LeadFilter{Status: domain.MarketingSubscribed, Limit: 26, Cursor: &cursor})
		})},
		{"report leads: admin list (calculator)", 150 * time.Millisecond, leads(func(ctx context.Context, r repository.EmailLeadRepository) {
			_, _ = r.List(ctx, repository.LeadFilter{Kind: domain.ReportNatalChart, Limit: 26})
		})},
		{"report leads: admin search by address", 10 * time.Millisecond, leads(func(ctx context.Context, r repository.EmailLeadRepository) {
			_, _ = r.List(ctx, repository.LeadFilter{EmailHash: hash, Limit: 26})
		})},
		{"report leads: CSV export batch", 50 * time.Millisecond, leads(func(ctx context.Context, r repository.EmailLeadRepository) {
			_, _ = r.List(ctx, repository.LeadFilter{Status: domain.MarketingSubscribed, Limit: 500, Cursor: &cursor})
		})},
	}
}
