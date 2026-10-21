package e2e

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
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
