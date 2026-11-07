package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 3.0: site settings (§15).

// SponsoredTotals sums the sponsored chats of a period.
type SponsoredTotals struct {
	Sessions int64
	Live     int64
	Minutes  int64
	Credits  int64
}

// SiteRepository persists the site settings documents and promo banners.
type SiteRepository interface {
	// Setting returns one settings document.
	Setting(ctx context.Context, key string) (*domain.SiteSetting, error)
	// Settings returns every settings document.
	Settings(ctx context.Context) ([]domain.SiteSetting, error)
	// SettingForUpdate locks and returns a settings document.
	SettingForUpdate(ctx context.Context, key string) (*domain.SiteSetting, error)
	// SaveSetting replaces a document's value and version.
	SaveSetting(ctx context.Context, key string, value domain.JSONMap, version int, by *uuid.UUID, at time.Time) error

	// Banners lists every banner in display order.
	Banners(ctx context.Context) ([]domain.SiteBanner, error)
	// BannerForUpdate locks and returns a banner.
	BannerForUpdate(ctx context.Context, id uuid.UUID) (*domain.SiteBanner, error)
	CreateBanner(ctx context.Context, b *domain.SiteBanner) error
	// SaveBanner writes every column of a banner.
	SaveBanner(ctx context.Context, b *domain.SiteBanner) error
	DeleteBanner(ctx context.Context, id uuid.UUID) error
	// SetBannerOrder gives the banners the sort_order of their position.
	SetBannerOrder(ctx context.Context, ids []uuid.UUID, by *uuid.UUID, at time.Time) error

	// UserNames resolves display names (for "updated by").
	UserNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)

	// SponsoredTotals sums sponsored chats requested at or after since.
	SponsoredTotals(ctx context.Context, since time.Time) (SponsoredTotals, error)
	// SponsoredEarnings sums the advisors' net earnings of those chats.
	SponsoredEarnings(ctx context.Context, since time.Time) (int64, error)
}

// GormSiteRepository implements SiteRepository.
type GormSiteRepository struct{ db *gorm.DB }

// NewSiteRepository returns a GORM site repository.
func NewSiteRepository(db *gorm.DB) *GormSiteRepository { return &GormSiteRepository{db: db} }

// Setting implements SiteRepository.
func (r *GormSiteRepository) Setting(ctx context.Context, key string) (*domain.SiteSetting, error) {
	var s domain.SiteSetting
	if err := database.Conn(ctx, r.db).First(&s, "key = ?", key).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

// Settings implements SiteRepository.
func (r *GormSiteRepository) Settings(ctx context.Context) ([]domain.SiteSetting, error) {
	var out []domain.SiteSetting
	err := database.Conn(ctx, r.db).Order("key").Find(&out).Error
	return out, err
}

// SettingForUpdate implements SiteRepository.
func (r *GormSiteRepository) SettingForUpdate(ctx context.Context, key string) (*domain.SiteSetting, error) {
	var s domain.SiteSetting
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).First(&s, "key = ?", key).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

// SaveSetting implements SiteRepository.
func (r *GormSiteRepository) SaveSetting(ctx context.Context, key string, value domain.JSONMap, version int, by *uuid.UUID, at time.Time) error {
	res := database.Conn(ctx, r.db).Model(&domain.SiteSetting{}).Where("key = ?", key).
		Updates(map[string]any{"value": value, "version": version, "updated_by": by, "updated_at": at})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Banners implements SiteRepository.
func (r *GormSiteRepository) Banners(ctx context.Context) ([]domain.SiteBanner, error) {
	var out []domain.SiteBanner
	err := database.Conn(ctx, r.db).Order("sort_order, id").Find(&out).Error
	return out, err
}

// BannerForUpdate implements SiteRepository.
func (r *GormSiteRepository) BannerForUpdate(ctx context.Context, id uuid.UUID) (*domain.SiteBanner, error) {
	var b domain.SiteBanner
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).First(&b, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &b, nil
}

// CreateBanner implements SiteRepository.
func (r *GormSiteRepository) CreateBanner(ctx context.Context, b *domain.SiteBanner) error {
	return database.Conn(ctx, r.db).Create(b).Error
}

// SaveBanner implements SiteRepository.
func (r *GormSiteRepository) SaveBanner(ctx context.Context, b *domain.SiteBanner) error {
	return database.Conn(ctx, r.db).Save(b).Error
}

// DeleteBanner implements SiteRepository.
func (r *GormSiteRepository) DeleteBanner(ctx context.Context, id uuid.UUID) error {
	res := database.Conn(ctx, r.db).Delete(&domain.SiteBanner{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// SetBannerOrder implements SiteRepository.
func (r *GormSiteRepository) SetBannerOrder(ctx context.Context, ids []uuid.UUID, by *uuid.UUID, at time.Time) error {
	conn := database.Conn(ctx, r.db)
	for i, id := range ids {
		if err := conn.Model(&domain.SiteBanner{}).Where("id = ? AND sort_order <> ?", id, i).
			Updates(map[string]any{"sort_order": i, "updated_by": by, "updated_at": at}).Error; err != nil {
			return err
		}
	}
	return nil
}

// UserNames implements SiteRepository.
func (r *GormSiteRepository) UserNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID          uuid.UUID
		DisplayName string
	}
	if err := database.Conn(ctx, r.db).Table("users").Select("id, display_name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.DisplayName
	}
	return out, nil
}

// SponsoredTotals implements SiteRepository. Only chats that billed a
// sponsored minute count; Live is how many of them are still open.
func (r *GormSiteRepository) SponsoredTotals(ctx context.Context, since time.Time) (SponsoredTotals, error) {
	var t SponsoredTotals
	err := database.Conn(ctx, r.db).Raw(`
		SELECT count(*) AS sessions,
		       count(*) FILTER (WHERE status IN ('accepted', 'active', 'disconnected')) AS live,
		       COALESCE(sum(sponsored_minutes), 0) AS minutes,
		       COALESCE(sum(sponsored_credits), 0) AS credits
		FROM chat_sessions
		WHERE sponsored AND requested_at >= ? AND sponsored_minutes > 0`, since).Scan(&t).Error
	return t, err
}

// SponsoredEarnings implements SiteRepository.
func (r *GormSiteRepository) SponsoredEarnings(ctx context.Context, since time.Time) (int64, error) {
	var n int64
	err := database.Conn(ctx, r.db).Raw(`
		SELECT COALESCE(sum(e.net_credits), 0)
		FROM chat_sessions s
		JOIN psychic_earnings e ON e.source_type = 'chat_session' AND e.source_id = s.id AND e.entry_kind = 'earning'
		WHERE s.sponsored AND s.requested_at >= ? AND s.sponsored_minutes > 0`, since).Scan(&n).Error
	return n, err
}
