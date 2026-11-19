package service

import (
	"context"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
)

// revisions 6.0 (docs/platform-extensions.md §26.5): the payment policies
// admin. The setting lives in site_settings.payment_policies (migration 0031)
// and follows the off-session pattern (§22.3): PUT with the version read, 409
// version_conflict when stale, step-up, audit billing.policies.updated, cache
// drop and Pub/Sub invalidation through SiteService.

// msgThreeDSecure is the validation message of an unknown 3-D Secure policy.
const msgThreeDSecure = "Choose automatic or any."

// PaymentPoliciesInput is PUT /admin/billing/policies. Every policy is
// required; the version read (expected_version, or its alias version) guards
// against overwriting a newer change.
type PaymentPoliciesInput struct {
	BlockPurchasesOnDispute *bool   `json:"block_purchases_on_dispute"`
	DisputeClawback         *bool   `json:"dispute_clawback"`
	EFWAutoRefund           *bool   `json:"efw_auto_refund"`
	ThreeDSecure            *string `json:"three_d_secure"`
	ExpectedVersion         *int    `json:"expected_version"`
	Version                 *int    `json:"version"`
}

// PaymentPoliciesSetting is the setting as the admin console shows it.
type PaymentPoliciesSetting struct {
	PaymentPolicies
	UpdatedBy *UserRef `json:"updated_by,omitempty"`
}

func (s *SiteService) paymentPoliciesView(ctx context.Context, st *domain.SiteSetting) *PaymentPoliciesSetting {
	out := &PaymentPoliciesSetting{PaymentPolicies: paymentPoliciesFrom(*st)}
	if st.UpdatedBy != nil {
		if name, ok := s.names(ctx, st.UpdatedBy)[*st.UpdatedBy]; ok {
			out.UpdatedBy = &UserRef{ID: *st.UpdatedBy, Name: name}
		}
	}
	return out
}

// PaymentPoliciesSetting returns the stored setting (database, never the cache).
func (s *SiteService) PaymentPoliciesSetting(ctx context.Context) (*PaymentPoliciesSetting, error) {
	st, err := s.d.Repo.Setting(ctx, domain.SiteSettingPaymentPolicies)
	if err != nil {
		return nil, mapRepoErr(err, "Setting")
	}
	return s.paymentPoliciesView(ctx, st), nil
}

func policiesAuditDoc(p PaymentPolicies) map[string]any {
	return map[string]any{"block_purchases_on_dispute": p.BlockPurchasesOnDispute, "dispute_clawback": p.DisputeClawback,
		"efw_auto_refund": p.EFWAutoRefund, "three_d_secure": p.ThreeDSecure}
}

// UpdatePaymentPolicies replaces the setting. A stale version is a 409
// version_conflict; an unchanged document writes nothing.
func (s *SiteService) UpdatePaymentPolicies(ctx context.Context, in PaymentPoliciesInput) (*PaymentPoliciesSetting, error) {
	fields := map[string]string{}
	for key, v := range map[string]*bool{"block_purchases_on_dispute": in.BlockPurchasesOnDispute, "dispute_clawback": in.DisputeClawback,
		"efw_auto_refund": in.EFWAutoRefund} {
		if v == nil {
			fields[key] = msgBannerRequired
		}
	}
	if in.ThreeDSecure == nil {
		fields["three_d_secure"] = msgBannerRequired
	} else if *in.ThreeDSecure != payment.ThreeDSAutomatic && *in.ThreeDSecure != payment.ThreeDSAny {
		fields["three_d_secure"] = msgThreeDSecure
	}
	expected := in.ExpectedVersion
	if expected == nil {
		expected = in.Version
	}
	if expected != nil && *expected < 1 {
		fields["expected_version"] = msgSiteWrongType
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	var before, after PaymentPolicies
	var changed bool
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		changed = false
		st, err := s.d.Repo.SettingForUpdate(ctx, domain.SiteSettingPaymentPolicies)
		if err != nil {
			return err
		}
		before = paymentPoliciesFrom(*st)
		if expected != nil && *expected != st.Version {
			return apperror.VersionConflict(st.Version)
		}
		after = PaymentPolicies{BlockPurchasesOnDispute: *in.BlockPurchasesOnDispute, DisputeClawback: *in.DisputeClawback,
			EFWAutoRefund: *in.EFWAutoRefund, ThreeDSecure: *in.ThreeDSecure}
		if samePolicies(before, after) {
			return nil
		}
		changed = true
		return s.d.Repo.SaveSetting(ctx, domain.SiteSettingPaymentPolicies, domain.JSONMap(policiesAuditDoc(after)),
			st.Version+1, actor(ctx), s.d.Clock.Now())
	})
	if err != nil {
		return nil, mapRepoErr(err, "Setting")
	}
	if changed {
		s.changed(ctx)
		s.record(ctx, "billing.policies.updated", "site_setting", domain.SiteSettingPaymentPolicies, map[string]any{
			"before": policiesAuditDoc(before), "after": policiesAuditDoc(after)})
	}
	return s.PaymentPoliciesSetting(ctx)
}

func samePolicies(a, b PaymentPolicies) bool {
	return a.BlockPurchasesOnDispute == b.BlockPurchasesOnDispute && a.DisputeClawback == b.DisputeClawback &&
		a.EFWAutoRefund == b.EFWAutoRefund && a.ThreeDSecure == b.ThreeDSecure
}
