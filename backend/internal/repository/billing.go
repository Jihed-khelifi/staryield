package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// openSubscriptionStatuses are the statuses of a user's current membership.
var openSubscriptionStatuses = []domain.SubscriptionStatus{domain.SubIncomplete, domain.SubActive, domain.SubPastDue}

// SubscriptionFilter narrows admin subscription listings.
type SubscriptionFilter struct {
	UserID *uuid.UUID
	Status domain.SubscriptionStatus
	Cursor *Cursor
	Limit  int
}

// TransactionFilter narrows transaction listings.
type TransactionFilter struct {
	UserID *uuid.UUID
	Status domain.TransactionStatus
	Kind   domain.TransactionKind
	Cursor *Cursor
	Limit  int
}

// BillingRepository persists plans, payment methods, subscriptions,
// invoices, grants, money movements, routing rules and webhook receipts.
type BillingRepository interface {
	// Plans and versions.
	ListPlans(ctx context.Context, activeOnly bool) ([]domain.SubscriptionPlan, error)
	GetPlan(ctx context.Context, id uuid.UUID) (*domain.SubscriptionPlan, error)
	CreatePlan(ctx context.Context, p *domain.SubscriptionPlan) error
	UpdatePlan(ctx context.Context, id uuid.UUID, fields map[string]any) error
	SoftDeletePlan(ctx context.Context, id uuid.UUID, at time.Time) error
	ListVersions(ctx context.Context, planIDs []uuid.UUID) ([]domain.PlanVersion, error)
	GetVersion(ctx context.Context, id uuid.UUID) (*domain.PlanVersion, error)
	MaxVersion(ctx context.Context, planID uuid.UUID) (int, error)
	CreateVersion(ctx context.Context, v *domain.PlanVersion) error
	SetCurrentVersion(ctx context.Context, planID, versionID uuid.UUID, at time.Time) error
	CountOpenSubscriptionsForPlan(ctx context.Context, planID uuid.UUID) (int64, error)

	// Provider customers.
	GetCustomer(ctx context.Context, userID uuid.UUID, provider string) (*domain.PaymentCustomer, error)
	GetCustomerByID(ctx context.Context, id uuid.UUID) (*domain.PaymentCustomer, error)
	CreateCustomer(ctx context.Context, c *domain.PaymentCustomer) error
	ListCustomers(ctx context.Context, userID uuid.UUID) ([]domain.PaymentCustomer, error)
	SoftDeleteCustomer(ctx context.Context, id uuid.UUID, at time.Time) error

	// Saved payment methods (live rows only unless stated).
	ListMethods(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error)
	GetMethod(ctx context.Context, userID, id uuid.UUID) (*domain.PaymentMethod, error)
	GetMethodAnyState(ctx context.Context, id uuid.UUID) (*domain.PaymentMethod, error)
	GetMethodsAnyState(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.PaymentMethod, error)
	GetMethodByKey(ctx context.Context, userID uuid.UUID, key string) (*domain.PaymentMethod, error)
	GetMethodByFingerprint(ctx context.Context, userID uuid.UUID, provider, fingerprint string) (*domain.PaymentMethod, error)
	CountMethods(ctx context.Context, userID uuid.UUID) (int64, error)
	CreateMethod(ctx context.Context, m *domain.PaymentMethod) error
	UpdateMethod(ctx context.Context, id uuid.UUID, fields map[string]any) error
	ClearDefaultMethod(ctx context.Context, userID uuid.UUID, at time.Time) error
	SoftDeleteMethod(ctx context.Context, id uuid.UUID, at time.Time) error

	// Subscriptions.
	CreateSubscription(ctx context.Context, s *domain.Subscription) error
	GetSubscription(ctx context.Context, id uuid.UUID) (*domain.Subscription, error)
	GetSubscriptionForUpdate(ctx context.Context, id uuid.UUID) (*domain.Subscription, error)
	GetOpenSubscription(ctx context.Context, userID uuid.UUID) (*domain.Subscription, error)
	LatestSubscription(ctx context.Context, userID uuid.UUID) (*domain.Subscription, error)
	GetSubscriptionByKey(ctx context.Context, userID uuid.UUID, key string) (*domain.Subscription, error)
	UpdateSubscription(ctx context.Context, id uuid.UUID, fields map[string]any) error
	OpenSubscriptionsUsingMethod(ctx context.Context, methodID uuid.UUID) ([]domain.Subscription, error)
	ClaimDueRenewals(ctx context.Context, now time.Time, limit int) ([]domain.Subscription, error)
	ClaimDueGrants(ctx context.Context, now time.Time, limit int) ([]domain.Subscription, error)
	ClaimGraceExpired(ctx context.Context, now time.Time, limit int) ([]domain.Subscription, error)
	ClaimStaleIncomplete(ctx context.Context, createdBefore, now time.Time, limit int) ([]domain.Subscription, error)
	ListSubscriptions(ctx context.Context, f SubscriptionFilter) ([]domain.Subscription, error)

	// Invoices and grants.
	CreateInvoice(ctx context.Context, inv *domain.SubscriptionInvoice) error
	GetInvoice(ctx context.Context, subscriptionID uuid.UUID, periodIndex int) (*domain.SubscriptionInvoice, error)
	UpdateInvoice(ctx context.Context, id uuid.UUID, fields map[string]any) error
	CreateGrant(ctx context.Context, g *domain.SubscriptionGrant) error
	GetGrant(ctx context.Context, subscriptionID uuid.UUID, grantIndex int) (*domain.SubscriptionGrant, error)
	GrantsForPeriod(ctx context.Context, subscriptionID uuid.UUID, periodIndex int) ([]domain.SubscriptionGrant, error)
	UpdateGrant(ctx context.Context, id uuid.UUID, fields map[string]any) error

	// Money movements.
	CreateTransaction(ctx context.Context, t *domain.PaymentTransaction) error
	GetTransaction(ctx context.Context, id uuid.UUID) (*domain.PaymentTransaction, error)
	GetTransactionForUpdate(ctx context.Context, id uuid.UUID) (*domain.PaymentTransaction, error)
	GetTransactionByKey(ctx context.Context, provider, key string) (*domain.PaymentTransaction, error)
	GetTransactionByProviderRef(ctx context.Context, provider, ref string) (*domain.PaymentTransaction, error)
	GetTransactionByPurchase(ctx context.Context, purchaseID uuid.UUID) (*domain.PaymentTransaction, error)
	UpdateTransaction(ctx context.Context, id uuid.UUID, fields map[string]any) error
	ListTransactions(ctx context.Context, f TransactionFilter) ([]domain.PaymentTransaction, error)
	ClaimStalePending(ctx context.Context, now time.Time, limit int) ([]domain.PaymentTransaction, error)
	CountChildren(ctx context.Context, parentID uuid.UUID) (int64, error)
	PendingRefund(ctx context.Context, parentID uuid.UUID) (*domain.PaymentTransaction, error)

	// Routing rules.
	ListRules(ctx context.Context) ([]domain.RoutingRule, error)
	GetRule(ctx context.Context, id uuid.UUID) (*domain.RoutingRule, error)
	CreateRule(ctx context.Context, r *domain.RoutingRule) error
	UpdateRule(ctx context.Context, id uuid.UUID, fields map[string]any) error
	SoftDeleteRule(ctx context.Context, id uuid.UUID, at time.Time) error

	// Webhook receipts.
	RecordWebhook(ctx context.Context, e *domain.ProviderWebhookEvent) (fresh bool, err error)
	GetWebhook(ctx context.Context, provider, eventID string) (*domain.ProviderWebhookEvent, error)
	MarkWebhookProcessed(ctx context.Context, id uuid.UUID, at time.Time) error
}

// GormBillingRepository implements BillingRepository.
type GormBillingRepository struct{ db *gorm.DB }

// NewBillingRepository returns a GORM billing repository.
func NewBillingRepository(db *gorm.DB) *GormBillingRepository { return &GormBillingRepository{db: db} }

func (r *GormBillingRepository) conn(ctx context.Context) *gorm.DB { return database.Conn(ctx, r.db) }

// ---------------------------------------------------------------- plans

func (r *GormBillingRepository) ListPlans(ctx context.Context, activeOnly bool) ([]domain.SubscriptionPlan, error) {
	q := r.conn(ctx)
	if activeOnly {
		q = q.Where("active = ?", true)
	}
	var out []domain.SubscriptionPlan
	err := q.Order("sort_order ASC, created_at ASC").Limit(100).Find(&out).Error
	return out, err
}

func (r *GormBillingRepository) GetPlan(ctx context.Context, id uuid.UUID) (*domain.SubscriptionPlan, error) {
	var p domain.SubscriptionPlan
	if err := r.conn(ctx).First(&p, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &p, nil
}

func (r *GormBillingRepository) CreatePlan(ctx context.Context, p *domain.SubscriptionPlan) error {
	if err := r.conn(ctx).Create(p).Error; err != nil {
		if isUnique(err, "subscription_plans_code_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormBillingRepository) UpdatePlan(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	res := r.conn(ctx).Model(&domain.SubscriptionPlan{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormBillingRepository) SoftDeletePlan(ctx context.Context, id uuid.UUID, at time.Time) error {
	res := r.conn(ctx).Model(&domain.SubscriptionPlan{}).Where("id = ?", id).
		Updates(map[string]any{"deleted_at": at, "active": false, "updated_at": at})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormBillingRepository) ListVersions(ctx context.Context, planIDs []uuid.UUID) ([]domain.PlanVersion, error) {
	if len(planIDs) == 0 {
		return nil, nil
	}
	var out []domain.PlanVersion
	err := r.conn(ctx).Where("plan_id IN ?", planIDs).Order("plan_id, version DESC").Find(&out).Error
	return out, err
}

func (r *GormBillingRepository) GetVersion(ctx context.Context, id uuid.UUID) (*domain.PlanVersion, error) {
	var v domain.PlanVersion
	if err := r.conn(ctx).First(&v, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &v, nil
}

func (r *GormBillingRepository) MaxVersion(ctx context.Context, planID uuid.UUID) (int, error) {
	var n *int
	err := r.conn(ctx).Model(&domain.PlanVersion{}).Where("plan_id = ?", planID).Select("max(version)").Scan(&n).Error
	if err != nil || n == nil {
		return 0, err
	}
	return *n, nil
}

func (r *GormBillingRepository) CreateVersion(ctx context.Context, v *domain.PlanVersion) error {
	if err := r.conn(ctx).Create(v).Error; err != nil {
		if isUnique(err, "subscription_plan_versions_key", "subscription_plan_versions_current_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

// SetCurrentVersion makes versionID the plan's current version (inside a
// transaction: the partial unique index allows one current version).
func (r *GormBillingRepository) SetCurrentVersion(ctx context.Context, planID, versionID uuid.UUID, at time.Time) error {
	db := r.conn(ctx)
	if err := db.Model(&domain.PlanVersion{}).Where("plan_id = ? AND is_current AND id <> ?", planID, versionID).
		Updates(map[string]any{"is_current": false, "updated_at": at}).Error; err != nil {
		return err
	}
	res := db.Model(&domain.PlanVersion{}).Where("plan_id = ? AND id = ?", planID, versionID).
		Updates(map[string]any{"is_current": true, "updated_at": at})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormBillingRepository) CountOpenSubscriptionsForPlan(ctx context.Context, planID uuid.UUID) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&domain.Subscription{}).Where("plan_id = ? AND status IN ?", planID, openSubscriptionStatuses).Count(&n).Error
	return n, err
}

// ------------------------------------------------------------- customers

func (r *GormBillingRepository) GetCustomer(ctx context.Context, userID uuid.UUID, provider string) (*domain.PaymentCustomer, error) {
	var c domain.PaymentCustomer
	if err := r.conn(ctx).First(&c, "user_id = ? AND provider = ?", userID, provider).Error; err != nil {
		return nil, notFound(err)
	}
	return &c, nil
}

func (r *GormBillingRepository) GetCustomerByID(ctx context.Context, id uuid.UUID) (*domain.PaymentCustomer, error) {
	var c domain.PaymentCustomer
	if err := r.conn(ctx).Unscoped().First(&c, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &c, nil
}

func (r *GormBillingRepository) CreateCustomer(ctx context.Context, c *domain.PaymentCustomer) error {
	if err := r.conn(ctx).Create(c).Error; err != nil {
		if isUnique(err, "payment_customers_user_provider_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormBillingRepository) ListCustomers(ctx context.Context, userID uuid.UUID) ([]domain.PaymentCustomer, error) {
	var out []domain.PaymentCustomer
	err := r.conn(ctx).Where("user_id = ?", userID).Order("created_at").Limit(20).Find(&out).Error
	return out, err
}

func (r *GormBillingRepository) SoftDeleteCustomer(ctx context.Context, id uuid.UUID, at time.Time) error {
	return r.conn(ctx).Model(&domain.PaymentCustomer{}).Where("id = ?", id).
		Updates(map[string]any{"deleted_at": at, "updated_at": at}).Error
}

// -------------------------------------------------------- payment methods

func (r *GormBillingRepository) ListMethods(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error) {
	var out []domain.PaymentMethod
	err := r.conn(ctx).Where("user_id = ?", userID).Order("is_default DESC, created_at DESC, id DESC").Limit(20).Find(&out).Error
	return out, err
}

func (r *GormBillingRepository) GetMethod(ctx context.Context, userID, id uuid.UUID) (*domain.PaymentMethod, error) {
	var m domain.PaymentMethod
	if err := r.conn(ctx).First(&m, "id = ? AND user_id = ?", id, userID).Error; err != nil {
		return nil, notFound(err)
	}
	return &m, nil
}

func (r *GormBillingRepository) GetMethodAnyState(ctx context.Context, id uuid.UUID) (*domain.PaymentMethod, error) {
	var m domain.PaymentMethod
	if err := r.conn(ctx).Unscoped().First(&m, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &m, nil
}

func (r *GormBillingRepository) GetMethodsAnyState(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.PaymentMethod, error) {
	out := map[uuid.UUID]domain.PaymentMethod{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []domain.PaymentMethod
	if err := r.conn(ctx).Unscoped().Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, m := range rows {
		out[m.ID] = m
	}
	return out, nil
}

func (r *GormBillingRepository) GetMethodByKey(ctx context.Context, userID uuid.UUID, key string) (*domain.PaymentMethod, error) {
	var m domain.PaymentMethod
	if err := r.conn(ctx).Unscoped().First(&m, "user_id = ? AND idempotency_key = ?", userID, key).Error; err != nil {
		return nil, notFound(err)
	}
	return &m, nil
}

func (r *GormBillingRepository) GetMethodByFingerprint(ctx context.Context, userID uuid.UUID, provider, fingerprint string) (*domain.PaymentMethod, error) {
	var m domain.PaymentMethod
	if err := r.conn(ctx).First(&m, "user_id = ? AND provider = ? AND fingerprint_hash = ? AND status = ?", userID, provider, fingerprint,
		domain.MethodActive).Error; err != nil {
		return nil, notFound(err)
	}
	return &m, nil
}

func (r *GormBillingRepository) CountMethods(ctx context.Context, userID uuid.UUID) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&domain.PaymentMethod{}).Where("user_id = ?", userID).Count(&n).Error
	return n, err
}

func (r *GormBillingRepository) CreateMethod(ctx context.Context, m *domain.PaymentMethod) error {
	if err := r.conn(ctx).Create(m).Error; err != nil {
		if isUnique(err, "payment_methods_idempotency_key", "payment_methods_default_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormBillingRepository) UpdateMethod(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	res := r.conn(ctx).Model(&domain.PaymentMethod{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormBillingRepository) ClearDefaultMethod(ctx context.Context, userID uuid.UUID, at time.Time) error {
	return r.conn(ctx).Model(&domain.PaymentMethod{}).Where("user_id = ? AND is_default", userID).
		Updates(map[string]any{"is_default": false, "updated_at": at}).Error
}

func (r *GormBillingRepository) SoftDeleteMethod(ctx context.Context, id uuid.UUID, at time.Time) error {
	res := r.conn(ctx).Model(&domain.PaymentMethod{}).Where("id = ?", id).
		Updates(map[string]any{"deleted_at": at, "is_default": false, "status": domain.MethodDetached, "updated_at": at})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------- subscriptions

func (r *GormBillingRepository) CreateSubscription(ctx context.Context, s *domain.Subscription) error {
	if err := r.conn(ctx).Create(s).Error; err != nil {
		switch {
		case isUnique(err, "subscriptions_idempotency_key"):
			return ErrDuplicate
		case isUnique(err, "subscriptions_open_user_key"):
			return ErrSubscriptionOpen
		}
		return err
	}
	return nil
}

// ErrSubscriptionOpen means the user already has an open membership.
var ErrSubscriptionOpen = errors.New("repository: subscription already open")

func (r *GormBillingRepository) GetSubscription(ctx context.Context, id uuid.UUID) (*domain.Subscription, error) {
	var s domain.Subscription
	if err := r.conn(ctx).First(&s, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *GormBillingRepository) GetSubscriptionForUpdate(ctx context.Context, id uuid.UUID) (*domain.Subscription, error) {
	var s domain.Subscription
	if err := r.conn(ctx).Clauses(forUpdate).First(&s, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *GormBillingRepository) GetOpenSubscription(ctx context.Context, userID uuid.UUID) (*domain.Subscription, error) {
	var s domain.Subscription
	if err := r.conn(ctx).Where("user_id = ? AND status IN ?", userID, openSubscriptionStatuses).First(&s).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *GormBillingRepository) LatestSubscription(ctx context.Context, userID uuid.UUID) (*domain.Subscription, error) {
	var s domain.Subscription
	if err := r.conn(ctx).Where("user_id = ?", userID).Order("created_at DESC, id DESC").First(&s).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *GormBillingRepository) GetSubscriptionByKey(ctx context.Context, userID uuid.UUID, key string) (*domain.Subscription, error) {
	var s domain.Subscription
	if err := r.conn(ctx).First(&s, "user_id = ? AND idempotency_key = ?", userID, key).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *GormBillingRepository) UpdateSubscription(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	res := r.conn(ctx).Model(&domain.Subscription{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormBillingRepository) OpenSubscriptionsUsingMethod(ctx context.Context, methodID uuid.UUID) ([]domain.Subscription, error) {
	var out []domain.Subscription
	err := r.conn(ctx).Where("payment_method_id = ? AND status IN ?", methodID, openSubscriptionStatuses).Limit(10).Find(&out).Error
	return out, err
}

// claim locks up to limit rows matching where, skipping rows other workers
// hold, so replicas process disjoint subscriptions.
func (r *GormBillingRepository) claim(ctx context.Context, limit int, order string, where string, args ...any) ([]domain.Subscription, error) {
	var out []domain.Subscription
	err := r.conn(ctx).Clauses(forUpdateSkipLocked).Where(where, args...).Order(order).Limit(clampLimit(limit, 50, 500)).Find(&out).Error
	return out, err
}

// ClaimDueRenewals claims subscriptions whose period ended or whose retry
// is due, and that no worker currently leases.
func (r *GormBillingRepository) ClaimDueRenewals(ctx context.Context, now time.Time, limit int) ([]domain.Subscription, error) {
	return r.claim(ctx, limit, "next_attempt_at ASC",
		"status IN ? AND next_attempt_at <= ? AND (locked_until IS NULL OR locked_until <= ?)",
		[]domain.SubscriptionStatus{domain.SubActive, domain.SubPastDue}, now, now)
}

// ClaimDueGrants claims subscriptions with a monthly grant due inside a
// paid multi-month period.
func (r *GormBillingRepository) ClaimDueGrants(ctx context.Context, now time.Time, limit int) ([]domain.Subscription, error) {
	return r.claim(ctx, limit, "next_grant_at ASC",
		"status IN ? AND next_grant_at IS NOT NULL AND next_grant_at <= ? AND (locked_until IS NULL OR locked_until <= ?)",
		[]domain.SubscriptionStatus{domain.SubActive, domain.SubPastDue}, now, now)
}

// ClaimGraceExpired claims past-due subscriptions whose grace period ended.
func (r *GormBillingRepository) ClaimGraceExpired(ctx context.Context, now time.Time, limit int) ([]domain.Subscription, error) {
	return r.claim(ctx, limit, "grace_until ASC",
		"status = ? AND grace_until IS NOT NULL AND grace_until <= ? AND (locked_until IS NULL OR locked_until <= ?)",
		domain.SubPastDue, now, now)
}

// ClaimStaleIncomplete claims memberships whose first charge never resolved.
func (r *GormBillingRepository) ClaimStaleIncomplete(ctx context.Context, createdBefore, now time.Time, limit int) ([]domain.Subscription, error) {
	return r.claim(ctx, limit, "created_at ASC",
		"status = ? AND created_at <= ? AND (locked_until IS NULL OR locked_until <= ?)", domain.SubIncomplete, createdBefore, now)
}

func (r *GormBillingRepository) ListSubscriptions(ctx context.Context, f SubscriptionFilter) ([]domain.Subscription, error) {
	q := r.conn(ctx)
	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	q = applyCursor(q, f.Cursor, "created_at", "id")
	var out []domain.Subscription
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(f.Limit, 20, 100)).Find(&out).Error
	return out, err
}

// ------------------------------------------------------- invoices, grants

func (r *GormBillingRepository) CreateInvoice(ctx context.Context, inv *domain.SubscriptionInvoice) error {
	if err := r.conn(ctx).Create(inv).Error; err != nil {
		if isUnique(err, "subscription_invoices_period_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormBillingRepository) GetInvoice(ctx context.Context, subscriptionID uuid.UUID, periodIndex int) (*domain.SubscriptionInvoice, error) {
	var inv domain.SubscriptionInvoice
	if err := r.conn(ctx).First(&inv, "subscription_id = ? AND period_index = ?", subscriptionID, periodIndex).Error; err != nil {
		return nil, notFound(err)
	}
	return &inv, nil
}

func (r *GormBillingRepository) UpdateInvoice(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	return r.conn(ctx).Model(&domain.SubscriptionInvoice{}).Where("id = ?", id).Updates(fields).Error
}

func (r *GormBillingRepository) CreateGrant(ctx context.Context, g *domain.SubscriptionGrant) error {
	if err := r.conn(ctx).Create(g).Error; err != nil {
		if isUnique(err, "subscription_grants_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormBillingRepository) GetGrant(ctx context.Context, subscriptionID uuid.UUID, grantIndex int) (*domain.SubscriptionGrant, error) {
	var g domain.SubscriptionGrant
	if err := r.conn(ctx).First(&g, "subscription_id = ? AND grant_index = ?", subscriptionID, grantIndex).Error; err != nil {
		return nil, notFound(err)
	}
	return &g, nil
}

func (r *GormBillingRepository) GrantsForPeriod(ctx context.Context, subscriptionID uuid.UUID, periodIndex int) ([]domain.SubscriptionGrant, error) {
	var out []domain.SubscriptionGrant
	err := r.conn(ctx).Where("subscription_id = ? AND period_index = ?", subscriptionID, periodIndex).Order("grant_index").Find(&out).Error
	return out, err
}

func (r *GormBillingRepository) UpdateGrant(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	return r.conn(ctx).Model(&domain.SubscriptionGrant{}).Where("id = ?", id).Updates(fields).Error
}

// --------------------------------------------------------- transactions

func (r *GormBillingRepository) CreateTransaction(ctx context.Context, t *domain.PaymentTransaction) error {
	if err := r.conn(ctx).Create(t).Error; err != nil {
		if isUnique(err, "payment_transactions_idempotency_key", "payment_transactions_provider_ref_key", "payment_transactions_purchase_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormBillingRepository) GetTransaction(ctx context.Context, id uuid.UUID) (*domain.PaymentTransaction, error) {
	var t domain.PaymentTransaction
	if err := r.conn(ctx).First(&t, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

func (r *GormBillingRepository) GetTransactionForUpdate(ctx context.Context, id uuid.UUID) (*domain.PaymentTransaction, error) {
	var t domain.PaymentTransaction
	if err := r.conn(ctx).Clauses(forUpdate).First(&t, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

func (r *GormBillingRepository) GetTransactionByKey(ctx context.Context, provider, key string) (*domain.PaymentTransaction, error) {
	var t domain.PaymentTransaction
	if err := r.conn(ctx).First(&t, "provider = ? AND idempotency_key = ?", provider, key).Error; err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

func (r *GormBillingRepository) GetTransactionByProviderRef(ctx context.Context, provider, ref string) (*domain.PaymentTransaction, error) {
	var t domain.PaymentTransaction
	if err := r.conn(ctx).First(&t, "provider = ? AND provider_ref = ?", provider, ref).Error; err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

func (r *GormBillingRepository) GetTransactionByPurchase(ctx context.Context, purchaseID uuid.UUID) (*domain.PaymentTransaction, error) {
	var t domain.PaymentTransaction
	if err := r.conn(ctx).First(&t, "credit_purchase_id = ? AND kind = ?", purchaseID, domain.TxCreditPurchase).Error; err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

func (r *GormBillingRepository) UpdateTransaction(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	err := r.conn(ctx).Model(&domain.PaymentTransaction{}).Where("id = ?", id).Updates(fields).Error
	if err != nil && isUnique(err, "payment_transactions_provider_ref_key") {
		return ErrDuplicate
	}
	return err
}

func (r *GormBillingRepository) ListTransactions(ctx context.Context, f TransactionFilter) ([]domain.PaymentTransaction, error) {
	q := r.conn(ctx)
	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Kind != "" {
		q = q.Where("kind = ?", f.Kind)
	}
	q = applyCursor(q, f.Cursor, "created_at", "id")
	var out []domain.PaymentTransaction
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(f.Limit, 20, 100)).Find(&out).Error
	return out, err
}

// ClaimStalePending claims charge attempts whose lease expired without an
// outcome (crash, timeout), for reconciliation with the provider.
func (r *GormBillingRepository) ClaimStalePending(ctx context.Context, now time.Time, limit int) ([]domain.PaymentTransaction, error) {
	var out []domain.PaymentTransaction
	err := r.conn(ctx).Clauses(forUpdateSkipLocked).
		Where("status = ? AND locked_until IS NOT NULL AND locked_until <= ?", domain.TxPending, now).
		Order("locked_until ASC").Limit(clampLimit(limit, 50, 500)).Find(&out).Error
	return out, err
}

// PendingRefund returns a refund of the payment that was issued but not
// booked (crash between the provider call and the ledger update).
func (r *GormBillingRepository) PendingRefund(ctx context.Context, parentID uuid.UUID) (*domain.PaymentTransaction, error) {
	var t domain.PaymentTransaction
	if err := r.conn(ctx).Where("parent_transaction_id = ? AND kind = ? AND status = ?", parentID, domain.TxRefund, domain.TxPending).
		Order("created_at ASC").First(&t).Error; err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

func (r *GormBillingRepository) CountChildren(ctx context.Context, parentID uuid.UUID) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&domain.PaymentTransaction{}).Where("parent_transaction_id = ?", parentID).Count(&n).Error
	return n, err
}

// --------------------------------------------------------------- routing

func (r *GormBillingRepository) ListRules(ctx context.Context) ([]domain.RoutingRule, error) {
	var out []domain.RoutingRule
	err := r.conn(ctx).Order("priority ASC, created_at ASC").Limit(200).Find(&out).Error
	return out, err
}

func (r *GormBillingRepository) GetRule(ctx context.Context, id uuid.UUID) (*domain.RoutingRule, error) {
	var rule domain.RoutingRule
	if err := r.conn(ctx).First(&rule, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &rule, nil
}

func (r *GormBillingRepository) CreateRule(ctx context.Context, rule *domain.RoutingRule) error {
	return r.conn(ctx).Create(rule).Error
}

func (r *GormBillingRepository) UpdateRule(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	res := r.conn(ctx).Model(&domain.RoutingRule{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormBillingRepository) SoftDeleteRule(ctx context.Context, id uuid.UUID, at time.Time) error {
	res := r.conn(ctx).Model(&domain.RoutingRule{}).Where("id = ?", id).Updates(map[string]any{"deleted_at": at, "updated_at": at})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// -------------------------------------------------------------- webhooks

// RecordWebhook stores a receipt; fresh is false when the event was
// already received (a redelivery).
func (r *GormBillingRepository) RecordWebhook(ctx context.Context, e *domain.ProviderWebhookEvent) (bool, error) {
	if err := r.conn(ctx).Create(e).Error; err != nil {
		if isUnique(err, "webhook_events_key") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *GormBillingRepository) GetWebhook(ctx context.Context, provider, eventID string) (*domain.ProviderWebhookEvent, error) {
	var e domain.ProviderWebhookEvent
	if err := r.conn(ctx).First(&e, "provider = ? AND event_id = ?", provider, eventID).Error; err != nil {
		return nil, notFound(err)
	}
	return &e, nil
}

func (r *GormBillingRepository) MarkWebhookProcessed(ctx context.Context, id uuid.UUID, at time.Time) error {
	return r.conn(ctx).Model(&domain.ProviderWebhookEvent{}).Where("id = ?", id).
		Updates(map[string]any{"processed_at": at, "updated_at": at}).Error
}
