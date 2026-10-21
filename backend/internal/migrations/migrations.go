// Package migrations applies the embedded, versioned SQL schema. Migrations
// run inside a transaction under a PostgreSQL advisory lock so several
// instances starting at once never race.
package migrations

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

//go:embed sql/*.sql
var files embed.FS

// advisoryLockID is an arbitrary constant identifying the migration lock.
const advisoryLockID = 727274001

// Migration is one versioned SQL file.
type Migration struct {
	Version string
	SQL     string
}

// Load returns embedded migrations sorted by version.
func Load() ([]Migration, error) {
	entries, err := fs.ReadDir(files, "sql")
	if err != nil {
		return nil, err
	}
	out := make([]Migration, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		body, err := files.ReadFile("sql/" + e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, Migration{Version: strings.TrimSuffix(e.Name(), ".sql"), SQL: string(body)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// Up applies all pending migrations and returns the versions applied.
func Up(ctx context.Context, db *gorm.DB, log *zap.Logger) ([]string, error) {
	migrations, err := Load()
	if err != nil {
		return nil, fmt.Errorf("migrations: load: %w", err)
	}

	var applied []string
	// Migration SQL is large; keep it out of slow-query logs.
	quiet := db.Session(&gorm.Session{Logger: gormlogger.Discard})
	err = quiet.WithContext(ctx).Connection(func(conn *gorm.DB) error {
		if err := conn.Exec("SELECT pg_advisory_lock(?)", advisoryLockID).Error; err != nil {
			return fmt.Errorf("acquire lock: %w", err)
		}
		defer conn.Exec("SELECT pg_advisory_unlock(?)", advisoryLockID)

		if err := conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`).Error; err != nil {
			return fmt.Errorf("create schema_migrations: %w", err)
		}

		var done []string
		if err := conn.Raw("SELECT version FROM schema_migrations").Scan(&done).Error; err != nil {
			return fmt.Errorf("read schema_migrations: %w", err)
		}
		seen := make(map[string]bool, len(done))
		for _, v := range done {
			seen[v] = true
		}

		for _, m := range migrations {
			if seen[m.Version] {
				continue
			}
			start := time.Now()
			if err := conn.Transaction(func(tx *gorm.DB) error {
				if err := tx.Exec(m.SQL).Error; err != nil {
					return err
				}
				return tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", m.Version).Error
			}); err != nil {
				return fmt.Errorf("apply %s: %w", m.Version, err)
			}
			log.Info("migration applied", zap.String("version", m.Version), zap.Duration("elapsed", time.Since(start)))
			applied = append(applied, m.Version)
		}
		return nil
	})
	if err != nil {
		return applied, err
	}
	if len(migrations) > 0 {
		log.Info("database schema up to date", zap.String("version", migrations[len(migrations)-1].Version), zap.Strings("applied", applied))
	}
	return applied, nil
}

// Status returns applied and pending versions.
func Status(ctx context.Context, db *gorm.DB) (applied, pending []string, err error) {
	migrations, err := Load()
	if err != nil {
		return nil, nil, err
	}
	var done []string
	if err := db.WithContext(ctx).Raw("SELECT version FROM schema_migrations ORDER BY version").Scan(&done).Error; err != nil {
		return nil, nil, err
	}
	seen := map[string]bool{}
	for _, v := range done {
		seen[v] = true
	}
	for _, m := range migrations {
		if !seen[m.Version] {
			pending = append(pending, m.Version)
		}
	}
	return done, pending, nil
}
