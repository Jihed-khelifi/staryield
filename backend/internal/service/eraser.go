package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	mailer "github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

// BillingCanceler ends paid memberships and stored payment methods before
// an account is erased (implemented by the billing module).
type BillingCanceler interface {
	CancelForErasure(ctx context.Context, userID uuid.UUID) error
}

// UserDataPurger hard-deletes one module's rows about a user (erasure).
type UserDataPurger interface {
	PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error)
}

// EraserDeps are the account eraser's collaborators. Everything below
// Hasher is optional (nil skips that clean-up).
type EraserDeps struct {
	Tx       database.TxManager
	Users    repository.UserRepository
	Tokens   repository.TokenRepository
	Psychics repository.PsychicRepository
	Hasher   *security.PasswordHasher
	Clock    clock.Clock

	Notes    repository.NoteRepository
	Push     repository.PushRepository
	Actions  repository.ActionTokenRepository
	Exports  repository.ExportRepository
	Sessions repository.SessionRepository
	Consents repository.ConsentRepository
	// Waitlist: the user's waiting lists and the clients waiting for them.
	Waitlist UserDataPurger
	// Support: contact-form tickets linked to the account.
	Support UserDataPurger
	// Advisor: photo submissions and their private images, the published
	// profile photo and the sealed phone number (advisor settings).
	Advisor     UserDataPurger
	AvatarStore storage.ObjectStore
	ExportStore storage.ObjectStore
	Auth        *AuthService
	AdminAuth   *AdminAuthService
	Chat        *ChatService
	PsychicSvc  *PsychicService
	Events      EventPublisher
	Mail        EmailQueue
	Billing     BillingCanceler
}

// AccountEraser anonymises an account: personal data (e-mail, names, birth
// data, avatar, notes, devices, push subscriptions, consents, exports) is
// erased while the append-only ledger, earnings and audit history are kept
// under the pseudonymous id. Admin deletion and self-service closure share
// it.
type AccountEraser struct{ d EraserDeps }

// NewAccountEraser wires the eraser.
func NewAccountEraser(d EraserDeps) *AccountEraser {
	if d.Events == nil {
		d.Events = NoopPublisher{}
	}
	return &AccountEraser{d: d}
}

// SetBilling installs the billing hook (wired once the billing module is).
func (e *AccountEraser) SetBilling(b BillingCanceler) { e.d.Billing = b }

// EraseRequest describes one erasure.
type EraseRequest struct {
	Reason string
	// Notify sends the closing e-mail to the address before it is erased.
	Notify bool
	// OnlyClosureDue erases only accounts still scheduled for closure
	// whose date has passed (the worker; a reactivation wins the race).
	OnlyClosureDue bool
}

// ErrNotErasable reports an account that is already erased or (for the
// closure worker) no longer due.
var ErrNotErasable = errors.New("account not erasable")

// Erase anonymises account id and returns it as it was before.
func (e *AccountEraser) Erase(ctx context.Context, id uuid.UUID, req EraseRequest) (*domain.User, error) {
	u, err := e.d.Users.GetByID(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	if u.Status == domain.UserDeleted {
		return nil, ErrNotErasable
	}
	if e.d.Billing != nil {
		if err := e.d.Billing.CancelForErasure(ctx, id); err != nil {
			return nil, apperror.Unavailable(err)
		}
	}
	profile, _ := e.d.Users.GetProfile(ctx, id)
	var exportKeys []string
	if e.d.Exports != nil {
		jobs, err := e.d.Exports.ListAllForUser(ctx, id)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for _, j := range jobs {
			if j.ObjectKey != "" {
				exportKeys = append(exportKeys, j.ObjectKey)
			}
		}
	}
	now := e.d.Clock.Now()
	unusable, err := e.d.Hasher.HashContext(ctx, security.MustRandomToken(32))
	if err != nil {
		return nil, hashError(err)
	}
	err = e.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		locked, err := e.d.Users.GetForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if locked.Status == domain.UserDeleted {
			return ErrNotErasable
		}
		if req.OnlyClosureDue && (locked.Status != domain.UserDeactivated || locked.DeactivatedBy != domain.DeactivatedByClosure ||
			locked.ClosureScheduledAt == nil || locked.ClosureScheduledAt.After(now)) {
			return ErrNotErasable
		}
		if req.Notify && e.d.Mail != nil {
			lang := normalizeLocale(locked.Locale)
			if err := e.d.Mail.Enqueue(ctx, EmailRequest{UserID: &id, To: locked.Email, Locale: lang, Category: domain.EmailSecurity,
				Template: "account_closed", Content: mailer.AccountClosed(lang)}); err != nil {
				return err
			}
		}
		if err := e.d.Users.Updates(ctx, id, map[string]any{
			"email": "deleted+" + id.String() + "@deleted.invalid", "display_name": "Deleted user", "password_hash": unusable,
			"status": domain.UserDeleted, "status_reason": req.Reason, "status_changed_at": now, "deleted_at": now,
			"deactivated_by": "", "closure_scheduled_at": nil, "email_verified_at": nil,
			"token_version": gorm.Expr("token_version + 1"),
		}); err != nil {
			return err
		}
		if profile != nil {
			if err := e.d.Users.UpdateProfile(ctx, id, map[string]any{
				"nickname": "", "birth_date": nil, "birth_time": "", "birth_place_id": "", "birth_place": "", "birth_place_label": "",
				"birth_latitude": nil, "birth_longitude": nil, "birth_timezone": "", "location": "", "avatar_url": "",
				"avatar_key": "", "country_code": "", "timezone": "", "share_birth_details_with_advisors": false,
				"notify_email": false, "notify_push": false, "notify_marketing": false,
			}); err != nil {
				return err
			}
		}
		if u.Role == domain.RolePsychic {
			if err := e.d.Psychics.Updates(ctx, id, map[string]any{"listed": false, "featured": false, "availability": domain.AvailabilityOffline}); err != nil && !errors.Is(err, repository.ErrNotFound) {
				return err
			}
		}
		// Notes written by or about the user are erased, not anonymised:
		// they are free text and may identify the person.
		if e.d.Notes != nil {
			if _, err := e.d.Notes.PurgeForUser(ctx, id); err != nil {
				return err
			}
		}
		if _, err := e.d.Tokens.RevokeAllForUser(ctx, id, "account_deleted", now); err != nil {
			return err
		}
		return e.eraseExtras(ctx, id, now)
	})
	if errors.Is(err, ErrNotErasable) {
		return nil, err
	}
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	e.cleanUp(ctx, u, profile, exportKeys)
	return u, nil
}

// eraseExtras removes the identity module's per-user rows.
func (e *AccountEraser) eraseExtras(ctx context.Context, id uuid.UUID, now time.Time) error {
	if e.d.Push != nil {
		if _, err := e.d.Push.DeleteAllForUser(ctx, id); err != nil {
			return err
		}
	}
	if e.d.Actions != nil {
		if _, err := e.d.Actions.DeleteForUser(ctx, id); err != nil {
			return err
		}
	}
	if e.d.Consents != nil {
		if _, err := e.d.Consents.DeleteForUser(ctx, id); err != nil {
			return err
		}
	}
	if e.d.Exports != nil {
		if _, err := e.d.Exports.SoftDeleteForUser(ctx, id, now); err != nil {
			return err
		}
	}
	if e.d.Sessions != nil {
		// Devices go with their (already revoked) refresh tokens.
		if _, err := e.d.Sessions.DeleteForUser(ctx, id); err != nil {
			return err
		}
	}
	for _, p := range []UserDataPurger{e.d.Waitlist, e.d.Support, e.d.Advisor} {
		if p == nil {
			continue
		}
		if _, err := p.PurgeForUser(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// cleanUp runs the side effects that follow a committed erasure.
func (e *AccountEraser) cleanUp(ctx context.Context, u *domain.User, profile *domain.Profile, exportKeys []string) {
	ctx = database.Detach(ctx)
	if profile != nil && profile.AvatarKey != "" && e.d.AvatarStore != nil {
		warnErr(ctx, "eraser: delete avatar object", e.d.AvatarStore.Delete(ctx, profile.AvatarKey))
	}
	for _, key := range exportKeys {
		if e.d.ExportStore != nil {
			warnErr(ctx, "eraser: delete export object", e.d.ExportStore.Delete(ctx, key))
		}
	}
	if e.d.Auth != nil {
		e.d.Auth.InvalidateStatus(ctx, u.ID)
	}
	warnErr(ctx, "eraser: realtime revoke", e.d.Events.Revoke(ctx, u.ID))
	if e.d.AdminAuth != nil {
		e.d.AdminAuth.RevokeAllFor(ctx, u.ID, "account_deleted")
		e.d.AdminAuth.ResetMFA(ctx, u.ID)
	}
	if e.d.Chat != nil {
		e.d.Chat.EndAllForUser(ctx, u.ID)
	}
	if u.Role == domain.RolePsychic && e.d.PsychicSvc != nil {
		e.d.PsychicSvc.DirectoryChanged() // unlisted: leave the public directory now
		e.d.PsychicSvc.PublishStatus(ctx, u.ID)
	}
	logger.FromContext(ctx).Info("account erased", zap.String("user_id", u.ID.String()), zap.String("role", string(u.Role)))
}

// SetEraser makes account deletion use a fully wired eraser (identity).
func (s *AdminService) SetEraser(e *AccountEraser) { s.eraser = e }
