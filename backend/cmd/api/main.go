// Command api runs the Staryield HTTP/WebSocket API.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/app"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
)

func main() {
	// All timestamps are handled in UTC (database scans included).
	time.Local = time.UTC
	configPath := flag.String("config", "", "path to a YAML config file (default ./config.yaml if present)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		os.Exit(2)
	}
	log, err := logger.New(cfg.Log, cfg.App)
	if err != nil {
		fmt.Fprintln(os.Stderr, "logger error:", err)
		os.Exit(2)
	}
	defer func() { _ = log.Sync() }()
	zap.ReplaceGlobals(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("starting", zap.String("addr", cfg.HTTP.Addr), zap.Bool("railway", config.OnRailway()),
		zap.String("storage", cfg.Storage.Driver), zap.String("payments", cfg.Payments.Provider), zap.String("mail", cfg.Mail.Driver))
	a, err := app.New(ctx, cfg, log, app.Overrides{})
	if err != nil {
		// Error level (not Fatal) so log platforms classify the line correctly.
		log.Error("startup failed", zap.Error(err))
		_ = log.Sync()
		os.Exit(1)
	}
	a.Start(ctx)

	errCh := make(chan error, 1)
	go func() { errCh <- a.ListenAndServe() }()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			log.Error("http server failed", zap.Error(err))
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	a.Shutdown(shutdownCtx)
}
