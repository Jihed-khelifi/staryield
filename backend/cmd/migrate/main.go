// Command migrate applies or reports database migrations.
//
//	go run ./cmd/migrate up
//	go run ./cmd/migrate status
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/migrations"
)

func main() {
	time.Local = time.UTC
	envFile := flag.String("env-file", "", "path to a .env file (default ./.env if present); real environment variables win")
	flag.Parse()
	cmd := flag.Arg(0)
	if cmd == "" {
		cmd = "up"
	}
	cfg, err := config.Load(*envFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		os.Exit(2)
	}
	log, _ := logger.New(cfg.Log, cfg.App)
	if unknown := config.UnknownEnv(); len(unknown) > 0 {
		log.Warn("ignoring unknown configuration variables (typo?)", zap.Strings("names", unknown))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	db, err := database.Open(ctx, cfg.Database, log)
	if err != nil {
		log.Fatal("database", zap.Error(err))
	}
	switch cmd {
	case "up":
		applied, err := migrations.Up(ctx, db, log)
		if err != nil {
			log.Fatal("migrate up", zap.Error(err))
		}
		log.Info("migrations complete", zap.Strings("applied", applied))
	case "status":
		applied, pending, err := migrations.Status(ctx, db)
		if err != nil {
			log.Fatal("migrate status", zap.Error(err))
		}
		fmt.Printf("applied: %v\npending: %v\n", applied, pending)
	default:
		fmt.Fprintln(os.Stderr, "usage: migrate [up|status]")
		os.Exit(2)
	}
}
