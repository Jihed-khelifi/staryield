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
package main

import (
	"context"
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
	flag.Parse()
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
