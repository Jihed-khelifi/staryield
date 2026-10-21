package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// ReviewFilter narrows review listings.
type ReviewFilter struct {
	PsychicID *uuid.UUID
	ClientID  *uuid.UUID
	Status    domain.ReviewStatus
	Rating    int
	Cursor    *Cursor
	Limit     int
}

// RatingStats aggregates an advisor's visible reviews.
type RatingStats struct {
	Count        int64
	Sum          int64
	Distribution map[int]int64
}

// ReviewRepository persists advisor ratings.
type ReviewRepository interface {
	Create(ctx context.Context, r *domain.Review) error
	Get(ctx context.Context, id uuid.UUID) (*domain.Review, error)
	GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.Review, error)
	GetBySession(ctx context.Context, sessionID uuid.UUID) (*domain.Review, error)
	Update(ctx context.Context, id uuid.UUID, fields map[string]any) error
	List(ctx context.Context, f ReviewFilter) ([]domain.Review, error)
	Stats(ctx context.Context, psychicID uuid.UUID, from, to *time.Time) (*RatingStats, error)
	// AdjustPsychicRating applies a delta to the advisor's rating aggregate
	// and recomputes the displayed rating (rating_sum / reviews_count).
	AdjustPsychicRating(ctx context.Context, psychicID uuid.UUID, sumDelta float64, countDelta int) error
}

// GormReviewRepository implements ReviewRepository.
type GormReviewRepository struct{ db *gorm.DB }

// NewReviewRepository returns a GORM review repository.
func NewReviewRepository(db *gorm.DB) *GormReviewRepository { return &GormReviewRepository{db: db} }

func (r *GormReviewRepository) Create(ctx context.Context, rv *domain.Review) error {
	if err := database.Conn(ctx, r.db).Create(rv).Error; err != nil {
		if isUnique(err, "psychic_reviews_session_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormReviewRepository) Get(ctx context.Context, id uuid.UUID) (*domain.Review, error) {
	var rv domain.Review
	if err := database.Conn(ctx, r.db).First(&rv, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &rv, nil
}

func (r *GormReviewRepository) GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.Review, error) {
	var rv domain.Review
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).First(&rv, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &rv, nil
}

func (r *GormReviewRepository) GetBySession(ctx context.Context, sessionID uuid.UUID) (*domain.Review, error) {
	var rv domain.Review
	if err := database.Conn(ctx, r.db).First(&rv, "session_id = ?", sessionID).Error; err != nil {
		return nil, notFound(err)
	}
	return &rv, nil
}

func (r *GormReviewRepository) Update(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	fields["updated_at"] = time.Now().UTC()
	return database.Conn(ctx, r.db).Model(&domain.Review{}).Where("id = ?", id).Updates(fields).Error
}

func (r *GormReviewRepository) List(ctx context.Context, f ReviewFilter) ([]domain.Review, error) {
	q := database.Conn(ctx, r.db).Model(&domain.Review{})
	if f.PsychicID != nil {
		q = q.Where("psychic_id = ?", *f.PsychicID)
	}
	if f.ClientID != nil {
		q = q.Where("client_id = ?", *f.ClientID)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Rating >= 1 && f.Rating <= 5 {
		q = q.Where("rating = ?", f.Rating)
	}
	q = applyCursor(q, f.Cursor, "created_at", "id")
	var out []domain.Review
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(f.Limit, 20, 200)).Find(&out).Error
	return out, err
}

func (r *GormReviewRepository) Stats(ctx context.Context, psychicID uuid.UUID, from, to *time.Time) (*RatingStats, error) {
	q := database.Conn(ctx, r.db).Model(&domain.Review{}).Select("rating, count(*) AS n").
		Where("psychic_id = ? AND status = ?", psychicID, domain.ReviewVisible)
	if from != nil {
		q = q.Where("created_at >= ?", *from)
	}
	if to != nil {
		q = q.Where("created_at < ?", *to)
	}
	var rows []struct {
		Rating int
		N      int64
	}
	if err := q.Group("rating").Scan(&rows).Error; err != nil {
		return nil, err
	}
	st := &RatingStats{Distribution: map[int]int64{1: 0, 2: 0, 3: 0, 4: 0, 5: 0}}
	for _, row := range rows {
		st.Distribution[row.Rating] = row.N
		st.Count += row.N
		st.Sum += int64(row.Rating) * row.N
	}
	return st, nil
}

func (r *GormReviewRepository) AdjustPsychicRating(ctx context.Context, psychicID uuid.UUID, sumDelta float64, countDelta int) error {
	return database.Conn(ctx, r.db).Exec(`
		UPDATE psychics SET
			rating_sum    = GREATEST(rating_sum + ?, 0),
			reviews_count = GREATEST(reviews_count + ?, 0),
			-- With no reviews left an advisor shows the new-advisor default.
			rating        = CASE WHEN reviews_count + ? > 0
			                     THEN LEAST(5, GREATEST(0, round((rating_sum + ?) / (reviews_count + ?), 1)))
			                     ELSE 5.0 END,
			updated_at    = now()
		WHERE user_id = ?`, sumDelta, countDelta, countDelta, sumDelta, countDelta, psychicID).Error
}
