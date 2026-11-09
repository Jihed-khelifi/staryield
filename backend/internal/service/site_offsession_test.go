package service

// revisions 3.3: the off_session_messages site setting (§22.3).

import (
	"testing"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
)

func TestOffSessionSettingsSeedAndHook(t *testing.T) {
	f := newSiteFixture(t)
	got := f.svc.OffSessionSettings(f.ctx)
	if !got.Enabled || got.ClientDailyLimit != 20 || got.AdvisorDailyLimit != 20 {
		t.Fatalf("seed: %+v", got)
	}
	if got.LimitFor(false) != 20 || got.LimitFor(true) != 20 {
		t.Fatalf("limit for role: %+v", got)
	}
	cur, err := f.svc.OffSessionMessages(f.ctx)
	if err != nil || cur.Version != 1 || cur.Stats != (OffSessionStats{}) {
		t.Fatalf("setting: %+v %v", cur, err)
	}
}

func TestOffSessionSettingsUnreadableAndNothingCachedIsDisabled(t *testing.T) {
	f := newSiteFixture(t)
	f.repo.fail = true
	got := f.svc.OffSessionSettings(f.ctx)
	if got.Enabled || got.ClientDailyLimit != 0 || got.AdvisorDailyLimit != 0 {
		t.Fatalf("unreadable must disable the feature: %+v", got)
	}
	// A malformed stored document never reads as more permissive.
	f.repo.fail = false
	f.repo.settings["off_session_messages"] = f.repo.settingsWith(map[string]any{"enabled": "yes", "client_daily_limit": float64(9999), "advisor_daily_limit": float64(-4)})
	f.svc.dropLocal()
	_ = f.kv.Del(f.ctx, siteCacheKey)
	got = f.svc.OffSessionSettings(f.ctx)
	if got.Enabled || got.ClientDailyLimit != 500 || got.AdvisorDailyLimit != 0 {
		t.Fatalf("malformed document: %+v", got)
	}
}

func TestOffSessionSettingsKeepLastGoodCopyDuringAnOutage(t *testing.T) {
	f := newSiteFixture(t)
	if !f.svc.OffSessionSettings(f.ctx).Enabled {
		t.Fatal("seed")
	}
	f.repo.fail = true
	f.svc.dropLocal()
	_ = f.kv.Del(f.ctx, siteCacheKey)
	// The in-process copy is stale but kept as the outage fallback.
	if !f.svc.OffSessionSettings(f.ctx).Enabled {
		t.Fatal("the last good copy must be served during an outage")
	}
}

func TestOffSessionUpdateValidationVersionAndAudit(t *testing.T) {
	f := newSiteFixture(t)
	for _, in := range []OffSessionInput{{}, {Enabled: ptr(true)}, {Enabled: ptr(true), ClientDailyLimit: ptr(5)},
		{Enabled: ptr(true), ClientDailyLimit: ptr(-1), AdvisorDailyLimit: ptr(5)}, {Enabled: ptr(true), ClientDailyLimit: ptr(5), AdvisorDailyLimit: ptr(501)},
		{Enabled: ptr(true), ClientDailyLimit: ptr(5), AdvisorDailyLimit: ptr(5), ExpectedVersion: ptr(0)}} {
		if _, err := f.svc.UpdateOffSessionMessages(f.ctx, in); !isCode(err, apperror.CodeValidation) {
			t.Errorf("%+v accepted: %v", in, err)
		}
	}
	fields := siteFields(t, func() error {
		_, err := f.svc.UpdateOffSessionMessages(f.ctx, OffSessionInput{ClientDailyLimit: ptr(501), AdvisorDailyLimit: ptr(-3)})
		return err
	}())
	if len(fields) != 3 || fields["enabled"] == "" || fields["client_daily_limit"] != msgOffSessionLimit || fields["advisor_daily_limit"] != msgOffSessionLimit {
		t.Fatalf("fields: %v", fields)
	}
	for _, n := range []int{0, 500} { // both bounds are valid
		if _, err := f.svc.UpdateOffSessionMessages(f.ctx, OffSessionInput{Enabled: ptr(true), ClientDailyLimit: ptr(n), AdvisorDailyLimit: ptr(n)}); err != nil {
			t.Fatalf("%d refused: %v", n, err)
		}
	}
	cur, _ := f.svc.OffSessionMessages(f.ctx)
	if _, err := f.svc.UpdateOffSessionMessages(f.ctx, OffSessionInput{Enabled: ptr(true), ClientDailyLimit: ptr(3), AdvisorDailyLimit: ptr(4), ExpectedVersion: ptr(cur.Version - 1)}); !isCode(err, apperror.CodeVersionConflict) {
		t.Fatalf("stale version: %v", err)
	} else if e, _ := apperror.As(err); e.Details["current_version"] != cur.Version {
		t.Fatalf("details: %v", e.Details)
	}
	out, err := f.svc.UpdateOffSessionMessages(f.ctx, OffSessionInput{Enabled: ptr(false), ClientDailyLimit: ptr(3), AdvisorDailyLimit: ptr(4), ExpectedVersion: ptr(cur.Version)})
	if err != nil || out.Version != cur.Version+1 || out.Enabled || out.ClientDailyLimit != 3 || out.AdvisorDailyLimit != 4 || out.UpdatedBy == nil || out.UpdatedBy.Name != "Ada Admin" {
		t.Fatalf("saved: %+v %v", out, err)
	}
	// The change is live at once on this instance (cache dropped), and
	// disabling keeps the limits.
	if got := f.svc.OffSessionSettings(f.ctx); got.Enabled || got.ClientDailyLimit != 3 {
		t.Fatalf("hook: %+v", got)
	}
	// An unchanged save neither bumps the version nor writes an audit row.
	n := len(f.audit.actions())
	same, _ := f.svc.UpdateOffSessionMessages(f.ctx, OffSessionInput{Enabled: ptr(false), ClientDailyLimit: ptr(3), AdvisorDailyLimit: ptr(4)})
	if same.Version != out.Version || len(f.audit.actions()) != n {
		t.Fatal("no-op save changed state")
	}
	last := f.audit.rows[len(f.audit.rows)-1]
	if last.Action != "site.off_session_messages_updated" || last.ActorID == nil || *last.ActorID != f.admin {
		t.Fatalf("audit: %+v", last)
	}
	before, _ := last.Metadata["before"].(map[string]any)
	after, _ := last.Metadata["after"].(map[string]any)
	if before["enabled"] != true || before["client_daily_limit"] != 500 || after["enabled"] != false || after["advisor_daily_limit"] != 4 {
		t.Fatalf("audit metadata: %v", last.Metadata)
	}
}

func TestOffSessionStatsComeFromTheRepository(t *testing.T) {
	f := newSiteFixture(t)
	f.repo.offMessages, f.repo.offSenders, f.repo.offHits = 12, 4, 3
	cur, err := f.svc.OffSessionMessages(f.ctx)
	if err != nil || cur.Stats.Messages24h != 12 || cur.Stats.Senders24h != 4 || cur.Stats.LimitHits24h != 3 {
		t.Fatalf("%+v %v", cur, err)
	}
}
