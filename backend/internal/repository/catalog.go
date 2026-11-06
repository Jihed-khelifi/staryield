package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// AdvisorCardRow is the slice of an advisor needed for directory cards. It
// deliberately leaves out long text (about, philosophy) and services so the
// catalogue can be held in memory and rebuilt cheaply.
type AdvisorCardRow struct {
	UserID          uuid.UUID
	Slug            string
	Name            string
	Headline        string
	ImageURL        string
	Zodiac          string
	Skills          domain.StringList `gorm:"type:jsonb"`
	Rating          float64
	ReviewsCount    int
	ReadingsCount   int
	YearsExperience int
	RatePerMinute   int64
	Availability    domain.Availability
	Featured        bool
	FeaturedRank    int
	SortOrder       int
	LastOnlineAt    *time.Time
}

// CatalogRepository reads the public advisor directory.
type CatalogRepository interface {
	// ListedCards returns every listed advisor of an active account in the
	// curated directory order (featured, rank, sort order, name).
	ListedCards(ctx context.Context) ([]AdvisorCardRow, error)
}

// GormCatalogRepository implements CatalogRepository.
type GormCatalogRepository struct{ db *gorm.DB }

// NewCatalogRepository returns a catalogue repository.
func NewCatalogRepository(db *gorm.DB) *GormCatalogRepository { return &GormCatalogRepository{db: db} }

// ListedCards implements CatalogRepository.
func (r *GormCatalogRepository) ListedCards(ctx context.Context) ([]AdvisorCardRow, error) {
	var out []AdvisorCardRow
	err := database.Conn(ctx, r.db).Model(&domain.Psychic{}).
		Select("psychics.user_id, psychics.slug, psychics.name, psychics.headline, psychics.image_url, psychics.zodiac, "+
			"psychics.skills, psychics.rating, psychics.reviews_count, psychics.readings_count, psychics.years_experience, "+
			"psychics.rate_per_minute, psychics.availability, psychics.featured, psychics.featured_rank, psychics.sort_order, "+
			"psychics.last_online_at").
		// The role predicate lets the planner use users_active_psychics_idx
		// instead of scanning every account.
		Joins("JOIN users ON users.id = psychics.user_id AND users.role = ? AND users.status = ?", domain.RolePsychic, domain.UserActive).
		Where("psychics.listed").
		Order("psychics.featured DESC, psychics.featured_rank ASC, psychics.sort_order ASC, psychics.name ASC").
		Scan(&out).Error
	return out, err
}
