// Package database opens the PostgreSQL connection pool through GORM (pgx
// driver) and provides context-propagated transactions so services can
// compose repository calls atomically without leaking *gorm.DB upward.
package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
)

// Open connects to PostgreSQL and verifies connectivity within ctx.
func Open(ctx context.Context, cfg config.DatabaseConfig, log *zap.Logger) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: cfg.DSN}), &gorm.Config{
		Logger:                                   NewGormLogger(log, cfg.SlowQuery),
		NowFunc:                                  func() time.Time { return time.Now().UTC().Truncate(time.Microsecond) },
		SkipDefaultTransaction:                   true,
		PrepareStmt:                              false,
		DisableForeignKeyConstraintWhenMigrating: true,
		TranslateError:                           false,
	})
	if err != nil {
		return nil, fmt.Errorf("database: open: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("database: pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}
	return db, nil
}

type txKey struct{}

// TxManager runs functions inside a database transaction carried by ctx.
type TxManager interface {
	// WithinTx runs fn in a transaction. Nested calls join the outer
	// transaction. The transaction rolls back if fn returns an error or panics.
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// GormTxManager implements TxManager with GORM.
type GormTxManager struct{ db *gorm.DB }

// NewTxManager returns a TxManager bound to db.
func NewTxManager(db *gorm.DB) *GormTxManager { return &GormTxManager{db: db} }

// WithinTx implements TxManager.
func (m *GormTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// Conn returns the transaction stored in ctx, or base bound to ctx. Every
// repository query goes through Conn so cancellations and deadlines reach the
// driver and queries participate in the caller's transaction.
func Conn(ctx context.Context, base *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}
	return base.WithContext(ctx)
}

// InTx reports whether ctx carries a transaction.
func InTx(ctx context.Context) bool {
	_, ok := ctx.Value(txKey{}).(*gorm.DB)
	return ok
}

// PostgreSQL error codes used for translation.
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
	pgCheckViolation      = "23514"
	pgSerialization       = "40001"
	pgDeadlock            = "40P01"
	pgRaiseException      = "P0001"
)

// IsUniqueViolation reports whether err is a unique-constraint violation,
// optionally on a specific constraint name.
func IsUniqueViolation(err error, constraint ...string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != pgUniqueViolation {
		return false
	}
	if len(constraint) == 0 {
		return true
	}
	for _, c := range constraint {
		if pgErr.ConstraintName == c {
			return true
		}
	}
	return false
}

// IsForeignKeyViolation reports whether err is a foreign-key violation.
func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation
}

// IsCheckViolation reports whether err is a CHECK-constraint violation.
func IsCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgCheckViolation
}

// IsRetryable reports serialization failures and deadlocks.
func IsRetryable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == pgSerialization || pgErr.Code == pgDeadlock)
}

// IsRaisedException reports errors raised by PL/pgSQL triggers (e.g. the
// append-only ledger guard).
func IsRaisedException(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgRaiseException
}

// IsNotFound reports gorm.ErrRecordNotFound.
func IsNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }

// gormLogger adapts GORM logging to the request-scoped zap logger.
type gormLogger struct {
	base      *zap.Logger
	slow      time.Duration
	level     gormlogger.LogLevel
	paramless bool
}

// NewGormLogger returns a GORM logger writing through zap. SQL parameters are
// never logged (ParamsFilter) so PII and secrets stay out of logs.
func NewGormLogger(log *zap.Logger, slow time.Duration) gormlogger.Interface {
	return &gormLogger{base: log, slow: slow, level: gormlogger.Warn, paramless: true}
}

func (l *gormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	cp := *l
	cp.level = level
	return &cp
}

func (l *gormLogger) logger(ctx context.Context) *zap.Logger {
	if log := logger.FromContext(ctx); log != nil && log != zap.L() {
		return log
	}
	return l.base
}

func (l *gormLogger) Info(ctx context.Context, msg string, args ...any) {
	if l.level >= gormlogger.Info {
		l.logger(ctx).Info(fmt.Sprintf(msg, args...), zap.String("component", "gorm"))
	}
}

func (l *gormLogger) Warn(ctx context.Context, msg string, args ...any) {
	if l.level >= gormlogger.Warn {
		l.logger(ctx).Warn(fmt.Sprintf(msg, args...), zap.String("component", "gorm"))
	}
}

func (l *gormLogger) Error(ctx context.Context, msg string, args ...any) {
	if l.level >= gormlogger.Error {
		l.logger(ctx).Error(fmt.Sprintf(msg, args...), zap.String("component", "gorm"))
	}
}

// ParamsFilter strips bound parameters from logged SQL.
func (l *gormLogger) ParamsFilter(_ context.Context, sql string, _ ...any) (string, []any) {
	return sql, nil
}

func (l *gormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level <= gormlogger.Silent {
		return
	}
	elapsed := time.Since(begin)
	switch {
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound) && !errors.Is(err, context.Canceled) && l.level >= gormlogger.Error:
		sql, rows := fc()
		l.logger(ctx).Error("database query failed",
			zap.String("component", "gorm"), zap.Error(err), zap.String("sql", truncate(sql, 2000)),
			zap.Int64("rows", rows), zap.Duration("elapsed", elapsed))
	case l.slow > 0 && elapsed > l.slow && l.level >= gormlogger.Warn:
		sql, rows := fc()
		l.logger(ctx).Warn("slow database query",
			zap.String("component", "gorm"), zap.String("sql", truncate(sql, 2000)),
			zap.Int64("rows", rows), zap.Duration("elapsed", elapsed))
	case l.level >= gormlogger.Info:
		sql, rows := fc()
		l.logger(ctx).Debug("database query", zap.String("component", "gorm"),
			zap.String("sql", truncate(sql, 2000)), zap.Int64("rows", rows), zap.Duration("elapsed", elapsed))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
