package e2e

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/migrations"
)

// The API brings an empty database to the current schema when it starts,
// and starting again on an up-to-date schema is a no-op.
func TestStartupMigratesAFreshDatabase(t *testing.T) {
	admin, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	name := "staryield_startup_" + uuid.NewString()[:8]
	if err := admin.Exec("CREATE DATABASE " + name).Error; err != nil {
		t.Fatalf("create database: %v", err)
	}
	// Runs after the apps below shut down (cleanups are LIFO).
	t.Cleanup(func() { admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)") })

	u, err := url.Parse(testDSN)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	fresh := func(cfg *config.Config) { cfg.Database.DSN = u.String() }

	e := newEnv(t, fresh)
	want, err := migrations.Load()
	if err != nil || len(want) == 0 {
		t.Fatalf("embedded migrations: %v", err)
	}
	var versions []string
	if err := e.app.DB.Raw("SELECT version FROM schema_migrations ORDER BY version").Scan(&versions).Error; err != nil {
		t.Fatalf("schema_migrations: %v", err)
	}
	if len(versions) != len(want) || versions[len(versions)-1] != want[len(want)-1].Version {
		t.Fatalf("applied %v, want all %d migrations up to %s", versions, len(want), want[len(want)-1].Version)
	}
	// Migration 0042: a fresh database holds the referral program without the
	// retired launch date (0036 seeded it as null), and the CHECK accepts that.
	assertReferralProgramWithoutLaunch(t, e.app.DB)
	// The migrated schema serves real requests.
	e.ensurePacks()
	var packs struct {
		Items []map[string]any `json:"items"`
	}
	e.client().must(http.StatusOK, http.MethodGet, "/api/v1/credit-packs", nil, &packs)
	if len(packs.Items) == 0 {
		t.Fatal("no credit packs served from the migrated database")
	}

	// A restart (or a second replica) finds nothing pending.
	again := newEnv(t, fresh)
	var count int64
	if err := again.app.DB.Raw("SELECT count(*) FROM schema_migrations").Scan(&count).Error; err != nil || count != int64(len(want)) {
		t.Fatalf("after restart: %d versions (%v)", count, err)
	}
}

// assertReferralProgramWithoutLaunch checks the state migration 0042 leaves:
// the stored referral_program has no launched_at, a value without it passes
// the CHECK, and the CHECK still rejects an out-of-range value. The row is
// restored afterwards.
func assertReferralProgramWithoutLaunch(t *testing.T, db *gorm.DB) {
	t.Helper()
	var hasKey bool
	if err := db.Raw(`SELECT value ? 'launched_at' FROM site_settings WHERE key = 'referral_program'`).Scan(&hasKey).Error; err != nil || hasKey {
		t.Fatalf("referral_program still has launched_at: %v (%v)", hasKey, err)
	}
	var value string
	if err := db.Raw(`SELECT value::text FROM site_settings WHERE key = 'referral_program'`).Scan(&value).Error; err != nil || value == "" {
		t.Fatalf("referral_program: %q %v", value, err)
	}
	t.Cleanup(func() {
		_ = db.Exec(`UPDATE site_settings SET value = ?::jsonb WHERE key = 'referral_program'`, value).Error
	})
	if err := db.Exec(`UPDATE site_settings SET value = '{"enabled": true, "reward_minutes": 7, "max_rewards_per_referrer": 30}'::jsonb
		WHERE key = 'referral_program'`).Error; err != nil {
		t.Fatalf("a value without launched_at must pass the CHECK: %v", err)
	}
	if err := db.Exec(`UPDATE site_settings SET value = '{"enabled": true, "reward_minutes": 0, "max_rewards_per_referrer": 30}'::jsonb
		WHERE key = 'referral_program'`).Error; err == nil {
		t.Fatal("the CHECK no longer rejects reward_minutes = 0")
	}
}

// Migration 0042 on a database whose rewards were launched: the stamped date is
// stripped, the editable fields stay, and the new CHECK
// takes the result.
func TestMigration0042StripsAStampedLaunchDate(t *testing.T) {
	admin, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	name := "staryield_mig42_" + uuid.NewString()[:8]
	if err := admin.Exec("CREATE DATABASE " + name).Error; err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() { admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)") })
	u, err := url.Parse(testDSN)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	scratch, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = scratch.Close() })

	all, err := migrations.Load()
	if err != nil {
		t.Fatal(err)
	}
	const target = "0042_referral_rewards_always_on"
	if all[len(all)-1].Version < target {
		t.Fatalf("migration %s is missing", target)
	}
	if err := db.Exec(`CREATE TABLE schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`).Error; err != nil {
		t.Fatal(err)
	}
	for _, m := range all {
		if m.Version >= target {
			continue
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(m.SQL).Error; err != nil {
				return err
			}
			return tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", m.Version).Error
		}); err != nil {
			t.Fatalf("apply %s: %v", m.Version, err)
		}
	}
	// The program as it was after a launch.
	if err := db.Exec(`UPDATE site_settings SET value = '{"enabled": false, "reward_minutes": 9, "max_rewards_per_referrer": 12, "launched_at": "2026-09-01T10:00:00Z"}'::jsonb
		WHERE key = 'referral_program'`).Error; err != nil {
		t.Fatal(err)
	}
	applied, err := migrations.Up(context.Background(), db, zap.NewNop())
	if err != nil || len(applied) == 0 || applied[0] != target {
		t.Fatalf("applied %v: %v", applied, err)
	}
	var enabled bool
	var minutes, max int
	if err := db.Raw(`SELECT (value ->> 'enabled')::boolean, (value ->> 'reward_minutes')::int, (value ->> 'max_rewards_per_referrer')::int
		FROM site_settings WHERE key = 'referral_program'`).Row().Scan(&enabled, &minutes, &max); err != nil || enabled || minutes != 9 || max != 12 {
		t.Fatalf("the editable fields must survive: %v %d %d %v", enabled, minutes, max, err)
	}
	assertReferralProgramWithoutLaunch(t, db)
}
