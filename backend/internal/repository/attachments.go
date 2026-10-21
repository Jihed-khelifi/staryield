package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// AttachmentRepository persists private chat image metadata.
type AttachmentRepository interface {
	Create(ctx context.Context, a *domain.Attachment) error
	Get(ctx context.Context, id uuid.UUID) (*domain.Attachment, error)
	ForMessages(ctx context.Context, messageIDs []uuid.UUID) (map[uuid.UUID][]domain.Attachment, error)
	ListForUploader(ctx context.Context, uploaderID uuid.UUID) ([]domain.Attachment, error)
}

// GormAttachmentRepository implements AttachmentRepository.
type GormAttachmentRepository struct{ db *gorm.DB }

// NewAttachmentRepository returns a GORM attachment repository.
func NewAttachmentRepository(db *gorm.DB) *GormAttachmentRepository {
	return &GormAttachmentRepository{db: db}
}

func (r *GormAttachmentRepository) Create(ctx context.Context, a *domain.Attachment) error {
	return database.Conn(ctx, r.db).Create(a).Error
}

func (r *GormAttachmentRepository) Get(ctx context.Context, id uuid.UUID) (*domain.Attachment, error) {
	var a domain.Attachment
	if err := database.Conn(ctx, r.db).First(&a, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &a, nil
}

func (r *GormAttachmentRepository) ForMessages(ctx context.Context, messageIDs []uuid.UUID) (map[uuid.UUID][]domain.Attachment, error) {
	out := map[uuid.UUID][]domain.Attachment{}
	if len(messageIDs) == 0 {
		return out, nil
	}
	var rows []domain.Attachment
	if err := database.Conn(ctx, r.db).Where("message_id IN ?", messageIDs).Order("created_at").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, a := range rows {
		out[a.MessageID] = append(out[a.MessageID], a)
	}
	return out, nil
}

func (r *GormAttachmentRepository) ListForUploader(ctx context.Context, uploaderID uuid.UUID) ([]domain.Attachment, error) {
	var rows []domain.Attachment
	err := database.Conn(ctx, r.db).Where("uploader_id = ?", uploaderID).Find(&rows).Error
	return rows, err
}
