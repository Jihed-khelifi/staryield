package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// Growth: the referral loop (migration 0036). Every statement goes through the
// GORM builder (no hand-written SQL) and every hot one is index-backed; the
// EXPLAIN checks in tests/load/explain_referrals_test.go prove it on large data.

// ReferralStats are one inviter's counters (counts and minutes only).
type ReferralStats struct {
	// Joined counts friends who signed up with the link and were not rejected.
	Joined int64
	// Qualified counts friends whose reward was earned (qualified or granted).
	Qualified      int64
	PendingMinutes int64
	GrantedMinutes int64
}

// ReferralFilter selects referrals for the admin list.
type ReferralFilter struct {
	Status  domain.ReferralStatus
	Flagged *bool
	Cursor  *Cursor
	Limit   int
}

// ReferralStatusTotals is the admin summary of one status.
type ReferralStatusTotals struct {
	Status  domain.ReferralStatus
	Count   int64
	Flagged int64
	Minutes int64
}

// ReferralInviterCount is one row of the top inviters.
type ReferralInviterCount struct {
	ReferrerID uuid.UUID
	Qualified  int64
	Minutes    int64
}

// ReferralRepository persists referral codes and referrals.
type ReferralRepository interface {
	// CodeOf returns a user's code; ErrNotFound when they have none yet.
	CodeOf(ctx context.Context, userID uuid.UUID) (string, error)
	// CreateCode stores a code for a user unless the user already has one or
	// the code is taken (ON CONFLICT DO NOTHING, so a collision never aborts
	// the caller's transaction); false means nothing was stored.
	CreateCode(ctx context.Context, userID uuid.UUID, code string, at time.Time) (bool, error)
	// InviterByCode returns the owner of a code when they are an active client.
	InviterByCode(ctx context.Context, code string) (uuid.UUID, error)

	// Create inserts a referral; false when the friend already has one.
	Create(ctx context.Context, r *domain.Referral) (bool, error)
	// ByReferee returns the referral of a friend (ErrNotFound when none).
	ByReferee(ctx context.Context, refereeID uuid.UUID) (*domain.Referral, error)
	// ByRefereeForUpdate locks and returns the referral of a friend.
	ByRefereeForUpdate(ctx context.Context, refereeID uuid.UUID) (*domain.Referral, error)
	// ForUpdate locks and returns a referral by id.
	ForUpdate(ctx context.Context, id uuid.UUID) (*domain.Referral, error)
	// Save writes the mutable columns of a referral.
	Save(ctx context.Context, r *domain.Referral) error

	// JoinedSince counts the referrals credited to an inviter since a time.
	JoinedSince(ctx context.Context, referrerID uuid.UUID, since time.Time) (int64, error)
	// SharesSessionNetwork reports whether the user has an active device
	// seen since a time from the given keyed IP hash.
	SharesSessionNetwork(ctx context.Context, userID uuid.UUID, ipHash string, since time.Time) (bool, error)
	// VisitorOwner returns the account a browser (anonymous id) was first
	// linked to; uuid.Nil when the browser is unknown or linked to nobody.
	VisitorOwner(ctx context.Context, anonymousID string) (uuid.UUID, error)

	// EmailVerified reports whether the user's address is verified.
	EmailVerified(ctx context.Context, userID uuid.UUID) (bool, error)
	// QualifyingSession returns a settled reading of the client with at least
	// one billed or free minute (nil when none).
	QualifyingSession(ctx context.Context, clientID uuid.UUID) (*uuid.UUID, error)
	// LockInviter serializes the cap check of one inviter (locks their code row).
	LockInviter(ctx context.Context, referrerID uuid.UUID) error
	// Rewarded counts the friends whose reward an inviter earned (qualified or
	// granted); they are what the per-inviter cap counts.
	Rewarded(ctx context.Context, referrerID uuid.UUID) (int64, error)
	// ReadyToQualify lists friends in status joined who are verified and have
	// a qualifying reading (the sweep that catches a missed hook), oldest first.
	ReadyToQualify(ctx context.Context, limit int) ([]uuid.UUID, error)

	// Stats returns one inviter's counters in one aggregate query.
	Stats(ctx context.Context, referrerID uuid.UUID) (ReferralStats, error)
	// HasInvited reports whether at least one friend joined with the user's
	// code, whatever became of the invite (the onboarding checklist).
	HasInvited(ctx context.Context, referrerID uuid.UUID) (bool, error)
	// InvitersAmong counts the clients who signed up in [from, to) and invited
	// at least one friend (the lifecycle funnel's invite step).
	InvitersAmong(ctx context.Context, from, to time.Time) (int64, error)

	// ClaimGrantable locks up to limit qualified, unflagged referrals of active
	// inviters, oldest first, skipping rows other workers hold.
	ClaimGrantable(ctx context.Context, limit int) ([]domain.Referral, error)
	// MarkGranted sets the claimed referrals to granted; it returns how many
	// were still qualified.
	MarkGranted(ctx context.Context, ids []uuid.UUID, at time.Time) (int64, error)
	// PendingGrants totals the referrals the grant job will claim (qualified,
	// unflagged, inviter active): rows and minutes.
	PendingGrants(ctx context.Context) (rows, minutes int64, err error)

	// List returns one page of referrals (admin), newest first.
	List(ctx context.Context, f ReferralFilter) ([]domain.Referral, error)
	// Totals returns the count, flagged count and minutes of every status.
	Totals(ctx context.Context) ([]ReferralStatusTotals, error)
	// TopInviters returns the inviters with the most rewarded friends.
	TopInviters(ctx context.Context, limit int) ([]ReferralInviterCount, error)
	// Names resolves display names.
	Names(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)

	// PurgeForUser erases an account's referral data (account erasure): their
	// code and the friends they invited are deleted; as a friend they stay
	// only as an anonymous decision.
	PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error)
}

// GormReferralRepository implements ReferralRepository.
type GormReferralRepository struct{ db *gorm.DB }

// NewReferralRepository returns a GORM referral repository.
func NewReferralRepository(db *gorm.DB) *GormReferralRepository {
	return &GormReferralRepository{db: db}
}

func (r *GormReferralRepository) conn(ctx context.Context) *gorm.DB { return database.Conn(ctx, r.db) }

// CodeOf implements ReferralRepository.
func (r *GormReferralRepository) CodeOf(ctx context.Context, userID uuid.UUID) (string, error) {
	var c domain.ReferralCode
	if err := r.conn(ctx).First(&c, "user_id = ?", userID).Error; err != nil {
		return "", notFound(err)
	}
	return c.Code, nil
}

// CreateCode implements ReferralRepository.
func (r *GormReferralRepository) CreateCode(ctx context.Context, userID uuid.UUID, code string, at time.Time) (bool, error) {
	res := r.conn(ctx).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&domain.ReferralCode{UserID: userID, Code: code, CreatedAt: at, UpdatedAt: at})
	return res.RowsAffected > 0, res.Error
}

// InviterByCode implements ReferralRepository.
func (r *GormReferralRepository) InviterByCode(ctx context.Context, code string) (uuid.UUID, error) {
	var row struct{ UserID uuid.UUID }
	res := r.conn(ctx).Table("referral_codes AS c").Select("c.user_id").
		Joins("JOIN users u ON u.id = c.user_id").
		Where("c.code = ? AND u.role = ? AND u.status = ?", code, domain.RoleClient, domain.UserActive).
		Limit(1).Scan(&row)
	if res.Error != nil {
		return uuid.Nil, res.Error
	}
	if res.RowsAffected == 0 {
		return uuid.Nil, ErrNotFound
	}
	return row.UserID, nil
}

// Create implements ReferralRepository.
func (r *GormReferralRepository) Create(ctx context.Context, ref *domain.Referral) (bool, error) {
	res := r.conn(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "referee_id"}}, DoNothing: true}).Create(ref)
	return res.RowsAffected > 0, res.Error
}

// ByReferee implements ReferralRepository.
func (r *GormReferralRepository) ByReferee(ctx context.Context, refereeID uuid.UUID) (*domain.Referral, error) {
	var ref domain.Referral
	if err := r.conn(ctx).First(&ref, "referee_id = ?", refereeID).Error; err != nil {
		return nil, notFound(err)
	}
	return &ref, nil
}

// ByRefereeForUpdate implements ReferralRepository.
func (r *GormReferralRepository) ByRefereeForUpdate(ctx context.Context, refereeID uuid.UUID) (*domain.Referral, error) {
	var ref domain.Referral
	if err := r.conn(ctx).Clauses(forUpdate).First(&ref, "referee_id = ?", refereeID).Error; err != nil {
		return nil, notFound(err)
	}
	return &ref, nil
}

// ForUpdate implements ReferralRepository.
func (r *GormReferralRepository) ForUpdate(ctx context.Context, id uuid.UUID) (*domain.Referral, error) {
	var ref domain.Referral
	if err := r.conn(ctx).Clauses(forUpdate).First(&ref, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &ref, nil
}

// Save implements ReferralRepository.
func (r *GormReferralRepository) Save(ctx context.Context, ref *domain.Referral) error {
	flags := ref.Flags
	if flags == nil {
		flags = domain.StringList{}
	}
	res := r.conn(ctx).Model(&domain.Referral{}).Where("id = ?", ref.ID).Updates(map[string]any{
		"status": ref.Status, "reward_minutes": ref.RewardMinutes, "flagged": ref.Flagged, "flags": flags,
		"reject_reason": ref.RejectReason, "first_session_id": ref.FirstSessionID, "qualified_at": ref.QualifiedAt,
		"granted_at": ref.GrantedAt, "decided_by": ref.DecidedBy, "decided_at": ref.DecidedAt})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// JoinedSince implements ReferralRepository (referrals_referrer_joined_idx).
func (r *GormReferralRepository) JoinedSince(ctx context.Context, referrerID uuid.UUID, since time.Time) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&domain.Referral{}).Where("referrer_id = ? AND joined_at >= ?", referrerID, since).Count(&n).Error
	return n, err
}

// SharesSessionNetwork implements ReferralRepository (user_sessions_active_idx).
func (r *GormReferralRepository) SharesSessionNetwork(ctx context.Context, userID uuid.UUID, ipHash string, since time.Time) (bool, error) {
	if ipHash == "" {
		return false, nil
	}
	var n int64
	err := r.conn(ctx).Model(&domain.UserSession{}).
		Where("user_id = ? AND revoked_at IS NULL AND last_seen_at >= ? AND ip_hash = ?", userID, since, ipHash).
		Limit(1).Count(&n).Error
	return n > 0, err
}

// VisitorOwner implements ReferralRepository (web_visitors primary key).
func (r *GormReferralRepository) VisitorOwner(ctx context.Context, anonymousID string) (uuid.UUID, error) {
	var owners []uuid.UUID
	err := r.conn(ctx).Table("web_visitors").Where("anonymous_id = ? AND user_id IS NOT NULL", anonymousID).
		Limit(1).Pluck("user_id", &owners).Error
	if err != nil || len(owners) == 0 {
		return uuid.Nil, err
	}
	return owners[0], nil
}

// EmailVerified implements ReferralRepository.
func (r *GormReferralRepository) EmailVerified(ctx context.Context, userID uuid.UUID) (bool, error) {
	var n int64
	err := r.conn(ctx).Model(&domain.User{}).Where("id = ? AND email_verified_at IS NOT NULL", userID).Count(&n).Error
	return n > 0, err
}

// QualifyingSession implements ReferralRepository (chat_sessions_client_idx).
func (r *GormReferralRepository) QualifyingSession(ctx context.Context, clientID uuid.UUID) (*uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.conn(ctx).Model(&domain.Session{}).
		Where("client_id = ? AND status = ? AND (billed_minutes > 0 OR free_minutes_used > 0)", clientID, domain.SessionSettled).
		Order("settled_at, id").Limit(1).Pluck("id", &ids).Error
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	return &ids[0], nil
}

// LockInviter implements ReferralRepository.
func (r *GormReferralRepository) LockInviter(ctx context.Context, referrerID uuid.UUID) error {
	var c domain.ReferralCode
	return notFound(r.conn(ctx).Clauses(forUpdate).Select("user_id").First(&c, "user_id = ?", referrerID).Error)
}

// Rewarded implements ReferralRepository (referrals_referrer_status_idx).
func (r *GormReferralRepository) Rewarded(ctx context.Context, referrerID uuid.UUID) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&domain.Referral{}).
		Where("referrer_id = ? AND status IN ?", referrerID, []domain.ReferralStatus{domain.ReferralQualified, domain.ReferralGranted}).
		Count(&n).Error
	return n, err
}

// ReadyToQualify implements ReferralRepository (referrals_joined_idx, then the
// users primary key and chat_sessions_client_idx per candidate).
func (r *GormReferralRepository) ReadyToQualify(ctx context.Context, limit int) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.conn(ctx).Table("referrals AS r").
		Joins("JOIN users u ON u.id = r.referee_id AND u.email_verified_at IS NOT NULL").
		Where("r.status = ?", domain.ReferralJoined).
		Where("EXISTS (SELECT 1 FROM chat_sessions s WHERE s.client_id = r.referee_id AND s.status = ? AND (s.billed_minutes > 0 OR s.free_minutes_used > 0))",
			domain.SessionSettled).
		Order("r.joined_at").Limit(limit).Pluck("r.referee_id", &ids).Error
	return ids, err
}

// Stats implements ReferralRepository: one aggregate over the inviter's rows
// (referrals_referrer_status_idx).
func (r *GormReferralRepository) Stats(ctx context.Context, referrerID uuid.UUID) (ReferralStats, error) {
	var row struct {
		Joined         int64
		Qualified      int64
		PendingMinutes int64
		GrantedMinutes int64
	}
	err := r.conn(ctx).Model(&domain.Referral{}).
		Select(`count(*) FILTER (WHERE status IN ('joined', 'qualified', 'granted')) AS joined,
			count(*) FILTER (WHERE status IN ('qualified', 'granted')) AS qualified,
			COALESCE(sum(reward_minutes) FILTER (WHERE status = 'qualified'), 0) AS pending_minutes,
			COALESCE(sum(reward_minutes) FILTER (WHERE status = 'granted'), 0) AS granted_minutes`).
		Where("referrer_id = ?", referrerID).Scan(&row).Error
	return ReferralStats(row), err
}

// HasInvited implements ReferralRepository: one probe of
// referrals_referrer_status_idx.
func (r *GormReferralRepository) HasInvited(ctx context.Context, referrerID uuid.UUID) (bool, error) {
	// Pluck into a slice: GORM's Scan takes a lone uuid.UUID ([16]byte) for a
	// byte array and fails on the text value, so the step never completed.
	var ids []uuid.UUID
	err := r.conn(ctx).Model(&domain.Referral{}).Where("referrer_id = ?", referrerID).Limit(1).Pluck("id", &ids).Error
	return len(ids) > 0, err
}

// InvitersAmong implements ReferralRepository: an index range of
// users_role_created_idx (the cohort), then one probe of
// referrals_referrer_status_idx per account, so the cost follows the cohort.
func (r *GormReferralRepository) InvitersAmong(ctx context.Context, from, to time.Time) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&domain.User{}).
		Where("role = ? AND created_at >= ? AND created_at < ?", domain.RoleClient, from, to).
		Where("EXISTS (SELECT 1 FROM referrals r WHERE r.referrer_id = users.id)").
		Count(&n).Error
	return n, err
}

// ClaimGrantable implements ReferralRepository (referrals_grant_idx).
func (r *GormReferralRepository) ClaimGrantable(ctx context.Context, limit int) ([]domain.Referral, error) {
	var out []domain.Referral
	err := r.conn(ctx).Model(&domain.Referral{}).
		Where("status = ? AND NOT flagged", domain.ReferralQualified).
		Where("EXISTS (SELECT 1 FROM users u WHERE u.id = referrals.referrer_id AND u.status = ?)", domain.UserActive).
		Order("qualified_at, id").Limit(limit).Clauses(forUpdateSkipLocked).Find(&out).Error
	return out, err
}

// MarkGranted implements ReferralRepository.
func (r *GormReferralRepository) MarkGranted(ctx context.Context, ids []uuid.UUID, at time.Time) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := r.conn(ctx).Model(&domain.Referral{}).Where("id IN ? AND status = ?", ids, domain.ReferralQualified).
		Updates(map[string]any{"status": domain.ReferralGranted, "granted_at": at})
	return res.RowsAffected, res.Error
}

// PendingGrants implements ReferralRepository: exactly the rows ClaimGrantable
// will claim (qualified, not held, active inviter), so the admin is never
// promised a grant the job skips (referrals_grant_idx, then the users
// primary key per row).
func (r *GormReferralRepository) PendingGrants(ctx context.Context) (int64, int64, error) {
	var row struct{ Rows, Minutes int64 }
	err := r.conn(ctx).Model(&domain.Referral{}).Select("count(*) AS rows, COALESCE(sum(reward_minutes), 0) AS minutes").
		Where("status = ? AND NOT flagged", domain.ReferralQualified).
		Where("EXISTS (SELECT 1 FROM users u WHERE u.id = referrals.referrer_id AND u.status = ?)", domain.UserActive).
		Scan(&row).Error
	return row.Rows, row.Minutes, err
}

// List implements ReferralRepository (referrals_recent_idx,
// referrals_status_recent_idx, referrals_flagged_recent_idx).
func (r *GormReferralRepository) List(ctx context.Context, f ReferralFilter) ([]domain.Referral, error) {
	q := r.conn(ctx).Model(&domain.Referral{})
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Flagged != nil {
		q = q.Where("flagged = ?", *f.Flagged)
	}
	q = applyCursor(q, f.Cursor, "created_at", "id")
	var out []domain.Referral
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(f.Limit, 21, 101)).Find(&out).Error
	return out, err
}

// Totals implements ReferralRepository (referrals_summary_idx, index only).
func (r *GormReferralRepository) Totals(ctx context.Context) ([]ReferralStatusTotals, error) {
	var out []ReferralStatusTotals
	err := r.conn(ctx).Model(&domain.Referral{}).
		Select("status, count(*) AS count, count(*) FILTER (WHERE flagged) AS flagged, COALESCE(sum(reward_minutes), 0) AS minutes").
		Group("status").Scan(&out).Error
	return out, err
}

// TopInviters implements ReferralRepository (referrals_referrer_status_idx).
func (r *GormReferralRepository) TopInviters(ctx context.Context, limit int) ([]ReferralInviterCount, error) {
	var out []ReferralInviterCount
	err := r.conn(ctx).Model(&domain.Referral{}).
		Select("referrer_id, count(*) AS qualified, COALESCE(sum(reward_minutes), 0) AS minutes").
		Where("status IN ?", []domain.ReferralStatus{domain.ReferralQualified, domain.ReferralGranted}).
		Group("referrer_id").Order("count(*) DESC, referrer_id").Limit(limit).Scan(&out).Error
	return out, err
}

// Names implements ReferralRepository.
func (r *GormReferralRepository) Names(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID          uuid.UUID
		DisplayName string
	}
	if err := r.conn(ctx).Model(&domain.User{}).Select("id, display_name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, u := range rows {
		out[u.ID] = u.DisplayName
	}
	return out, nil
}

// PurgeForUser implements ReferralRepository.
func (r *GormReferralRepository) PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	db := r.conn(ctx)
	var total int64
	for _, del := range []struct {
		model any
		where string
	}{
		{&domain.ReferralCode{}, "user_id = ?"},
		{&domain.Referral{}, "referrer_id = ?"},
		// A friend who never finished has nothing left to decide.
		{&domain.Referral{}, "referee_id = ? AND status = 'joined'"},
	} {
		res := db.Where(del.where, userID).Delete(del.model)
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
	}
	// Earned, rejected and granted rows keep their decision, anonymously.
	res := db.Model(&domain.Referral{}).Where("referee_id = ?", userID).
		Updates(map[string]any{"referee_id": nil, "signup_ip_hash": "", "first_session_id": nil})
	return total + res.RowsAffected, res.Error
}
