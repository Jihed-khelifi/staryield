// Package logger builds the zap logger and carries request-scoped loggers
// through context.Context so every layer logs with the same correlation
// fields (request_id, user_id, route, ...).
package logger

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
)

type ctxKey struct{}

// New builds a production-grade zap logger from configuration.
func New(cfg config.LogConfig, app config.AppConfig) (*zap.Logger, error) {
	level, err := zapcore.ParseLevel(strings.ToLower(cfg.Level))
	if err != nil {
		return nil, fmt.Errorf("logger: invalid level %q: %w", cfg.Level, err)
	}

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encCfg.EncodeDuration = zapcore.MillisDurationEncoder

	zcfg := zap.Config{
		Level:             zap.NewAtomicLevelAt(level),
		Development:       app.Env == config.EnvDevelopment,
		DisableStacktrace: false,
		Sampling: &zap.SamplingConfig{
			Initial:    200,
			Thereafter: 50,
		},
		Encoding:         cfg.Format,
		EncoderConfig:    encCfg,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	if cfg.Format == "console" {
		zcfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	log, err := zcfg.Build(zap.AddStacktrace(zapcore.ErrorLevel))
	if err != nil {
		return nil, fmt.Errorf("logger: build: %w", err)
	}
	return log.With(
		zap.String("service", app.Name),
		zap.String("env", app.Env),
		zap.String("instance", app.InstanceID),
	), nil
}

// WithContext returns a copy of ctx carrying log.
func WithContext(ctx context.Context, log *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, log)
}

// FromContext returns the request-scoped logger, or the global logger when
// ctx carries none. It never returns nil.
func FromContext(ctx context.Context) *zap.Logger {
	if ctx != nil {
		if log, ok := ctx.Value(ctxKey{}).(*zap.Logger); ok && log != nil {
			return log
		}
	}
	return zap.L()
}

// With returns ctx with a logger enriched by fields.
func With(ctx context.Context, fields ...zap.Field) context.Context {
	return WithContext(ctx, FromContext(ctx).With(fields...))
}

// RedactEmail masks the local part of an e-mail address for logs ("j***@x.com").
func RedactEmail(email string) string {
	at := strings.LastIndexByte(email, '@')
	if at <= 0 {
		return "***"
	}
	return email[:1] + "***" + email[at:]
}
