// Command seed loads idempotent reference data (advisors, services, credit
// packs) and optional bootstrap accounts.
//
// Accounts are created only when their credentials are supplied through
// environment variables, so no default passwords exist:
//
//	STARYIELD_SEED_ADMIN_EMAIL / STARYIELD_SEED_ADMIN_PASSWORD
//	STARYIELD_SEED_PSYCHIC_PASSWORD   (logins <slug>@advisors.staryield.net)
//	STARYIELD_SEED_CLIENT_EMAIL / STARYIELD_SEED_CLIENT_PASSWORD
//	STARYIELD_SEED_CLIENT_CREDITS     (default 500)
//
// With -admin-only it bootstraps the administrator alone (for production):
// it creates the account, or resets the password of an existing
// administrator, and seeds nothing else. Its credentials must come from the
// shell environment; the .env file is ignored for them.
//
// With -profile=large it then generates a deterministic synthetic
// population for load and strain tests (about 200k clients, 2k advisors,
// 100k conversations, 2M messages, 1M ledger rows by default; see the
// -users, -psychics, … flags). Every synthetic account uses the
// @load.staryield.test domain and the password Load!Test#2026. It refuses
// to run twice unless -force is given; never run it against production.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/migrations"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/seed"
)

func main() {
	time.Local = time.UTC
	envFile := flag.String("env-file", "", "path to a .env file (default ./.env if present); real environment variables win")
	adminOnly := flag.Bool("admin-only", false, "only create the administrator from STARYIELD_SEED_ADMIN_EMAIL/STARYIELD_SEED_ADMIN_PASSWORD, or reset its password; seed nothing else")
	profile := flag.String("profile", "default", "default (reference data and demo accounts) or large (plus a synthetic load-test population)")
	large := seed.DefaultLargeOptions()
	flag.IntVar(&large.Users, "users", large.Users, "large profile: clients")
	flag.IntVar(&large.Psychics, "psychics", large.Psychics, "large profile: advisors")
	flag.IntVar(&large.Conversations, "conversations", large.Conversations, "large profile: client–advisor conversations")
	flag.IntVar(&large.MessagesPerConversation, "messages-per-conversation", large.MessagesPerConversation, "large profile: average messages per conversation")
	flag.IntVar(&large.Reviews, "reviews", large.Reviews, "large profile: target number of reviews")
	flag.IntVar(&large.LedgerRows, "ledger-rows", large.LedgerRows, "large profile: credit ledger rows")
	flag.Uint64Var(&large.Seed, "seed", large.Seed, "large profile: random seed (same seed, same data)")
	flag.IntVar(&large.BatchSize, "batch", large.BatchSize, "large profile: insert batch size")
	flag.BoolVar(&large.Force, "force", false, "large profile: generate another population even if load data exists")
	extend := flag.String("extend", "", "large profile: comma-separated generators to run on the existing population only (e.g. \"waiting lists,payment transactions\")")
	flag.Parse()
	if *profile != "default" && *profile != "large" {
		fmt.Fprintln(os.Stderr, "-profile must be default or large")
		os.Exit(2)
	}
	// -admin-only takes its credentials from the shell environment only, read
	// before the .env file is loaded, so the example credentials committed in
	// .env.example can never reach a production database by accident.
	var admin seed.Account
	if *adminOnly {
		var err error
		if admin, err = adminFromEnv(os.Getenv); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
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
	timeout := 2 * time.Minute
	if *profile == "large" {
		if cfg.IsProduction() {
			fmt.Fprintln(os.Stderr, "-profile=large is refused in production")
			os.Exit(2)
		}
		timeout = 3 * time.Hour
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db, err := database.Open(ctx, cfg.Database, log)
	if err != nil {
		log.Fatal("database", zap.Error(err))
	}
	if _, err := migrations.Up(ctx, db, log); err != nil {
		log.Fatal("migrate", zap.Error(err))
	}
	hasher := security.NewPasswordHasher(cfg.Auth.Argon2Memory, cfg.Auth.Argon2Iterations, cfg.Auth.Argon2Parallelism)
	if *adminOnly {
		if _, err := seed.EnsureAdmin(ctx, db, hasher, admin, log); err != nil {
			log.Fatal("admin bootstrap failed", zap.Error(err))
		}
		return
	}
	opts := seed.Options{PsychicPassword: os.Getenv("STARYIELD_SEED_PSYCHIC_PASSWORD"), DemoCredits: 500}
	if e := os.Getenv("STARYIELD_SEED_ADMIN_EMAIL"); e != "" {
		opts.Admin = &seed.Account{Email: e, Password: os.Getenv("STARYIELD_SEED_ADMIN_PASSWORD")}
	}
	if e := os.Getenv("STARYIELD_SEED_CLIENT_EMAIL"); e != "" {
		opts.DemoClient = &seed.Account{Email: e, Password: os.Getenv("STARYIELD_SEED_CLIENT_PASSWORD")}
	}
	if v := os.Getenv("STARYIELD_SEED_CLIENT_CREDITS"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			opts.DemoCredits = n
		}
	}
	if err := seed.Run(ctx, db, hasher, opts, log); err != nil {
		log.Fatal("seed failed", zap.Error(err))
	}
	if *profile == "large" {
		// Seal generated notes with the API's key so they read back.
		if key, err := cfg.EncryptionKey(); err == nil {
			if sealer, err := security.NewSealer(key); err == nil {
				large.Sealer = sealer
			}
		}
		if *extend != "" {
			rep, err := seed.Extend(ctx, db, strings.Split(*extend, ","), large, log)
			if err != nil {
				log.Fatal("extend failed", zap.Error(err))
			}
			log.Info("large seed extended", zap.String("tag", rep.Tag), zap.Any("rows", rep.Counts), zap.Duration("elapsed", rep.Duration))
			return
		}
		rep, err := seed.Large(ctx, db, hasher, large, log)
		if errors.Is(err, seed.ErrAlreadySeeded) {
			log.Warn("load data already present; nothing generated (use -force to add another population)")
			return
		}
		if err != nil {
			log.Fatal("large seed failed", zap.Error(err))
		}
		log.Info("large seed complete", zap.String("tag", rep.Tag), zap.Any("rows", rep.Counts), zap.Duration("elapsed", rep.Duration))
	}
	log.Info("seed complete")
}

// adminFromEnv reads the administrator credentials for -admin-only.
func adminFromEnv(getenv func(string) string) (seed.Account, error) {
	acct := seed.Account{Email: strings.TrimSpace(getenv("STARYIELD_SEED_ADMIN_EMAIL")), Password: getenv("STARYIELD_SEED_ADMIN_PASSWORD")}
	var missing []string
	if acct.Email == "" {
		missing = append(missing, "STARYIELD_SEED_ADMIN_EMAIL")
	}
	if acct.Password == "" {
		missing = append(missing, "STARYIELD_SEED_ADMIN_PASSWORD")
	}
	if len(missing) > 0 {
		return seed.Account{}, fmt.Errorf("-admin-only needs %s set in the shell environment (the .env file is not used for these)", strings.Join(missing, " and "))
	}
	return acct, nil
}
