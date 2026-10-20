package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

// Disabled features are refused before any repository is touched (nil
// repositories would panic otherwise) and with a non-retryable code.
func TestDisabledFeaturesAreRefusedUpFront(t *testing.T) {
	credits := NewCreditService(nil, nil, nil, nil, payment.Disabled{}, nil, nil, nil)
	_, err := credits.Purchase(context.Background(), uuid.New(), PurchaseInput{IdempotencyKey: "k"})
	if e, ok := apperror.As(err); !ok || e.Code != apperror.CodeFeatureUnavailable || e.Status != 503 {
		t.Fatalf("purchase: %v", err)
	}

	users := NewUserService(nil, nil, nil, storage.Disabled{}, nil, nil, nil, 18, AvatarSettings{MaxBytes: 1 << 20})
	_, err = users.UploadAvatar(context.Background(), uuid.New(), make([]byte, 128))
	if e, ok := apperror.As(err); !ok || e.Code != apperror.CodeFeatureUnavailable {
		t.Fatalf("avatar: %v", err)
	}
}
