//go:build load

package load

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// growth: e-mail delivery. The admin console's delivery view over a month of
// outbox rows (seed generator "email outbox").
func mailHealthChecks() []queryCheck {
	weekAgo := func() time.Time { return time.Now().Add(-7 * 24 * time.Hour) }
	return []queryCheck{
		{"mail health: counts per template", 50 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewEmailRepository(d).DeliveryStats(ctx, weekAgo())
		}},
		{"mail health: recent problems", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewEmailRepository(d).Problems(ctx, weekAgo(), 30)
		}},
		{"mail health: backlog", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewEmailRepository(d).Backlog(ctx)
		}},
	}
}
