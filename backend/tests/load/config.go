//go:build load

package load

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Config comes from LOAD_* environment variables (all optional).
type Config struct {
	API             string        // LOAD_API_URL, default http://localhost:8080
	Origin          string        // LOAD_ORIGIN, an allowed browser origin
	Duration        time.Duration // LOAD_DURATION per scenario, default 20s
	Scale           float64       // LOAD_SCALE multiplies every arrival rate
	Users           int           // LOAD_USERS signed-in virtual users to create
	ClientEmail     string        // LOAD_CLIENT_EMAIL (seed demo client)
	ClientPassword  string        // LOAD_CLIENT_PASSWORD
	PsychicEmail    string        // LOAD_PSYCHIC_EMAIL (seed advisor)
	PsychicPassword string        // LOAD_PSYCHIC_PASSWORD
	DSN             string        // LOAD_DATABASE_DSN for EXPLAIN checks
	ReportDir       string        // LOAD_REPORT_DIR, default ./results
	Strict          bool          // LOAD_STRICT=1 fails the test on SLO violations
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// LoadConfig reads the configuration.
func LoadConfig() Config {
	dur, err := time.ParseDuration(env("LOAD_DURATION", "20s"))
	if err != nil || dur <= 0 {
		dur = 20 * time.Second
	}
	scale, err := strconv.ParseFloat(env("LOAD_SCALE", "1"), 64)
	if err != nil || scale <= 0 {
		scale = 1
	}
	users, err := strconv.Atoi(env("LOAD_USERS", "20"))
	if err != nil || users < 1 {
		users = 20
	}
	return Config{
		API:             strings.TrimRight(env("LOAD_API_URL", "http://localhost:8080"), "/"),
		Origin:          env("LOAD_ORIGIN", "http://localhost:3000"),
		Duration:        dur,
		Scale:           scale,
		Users:           users,
		ClientEmail:     env("LOAD_CLIENT_EMAIL", "client@staryield.test"),
		ClientPassword:  env("LOAD_CLIENT_PASSWORD", "Cl1ent!Secure#2026"),
		PsychicEmail:    env("LOAD_PSYCHIC_EMAIL", "ramone@advisors.staryield.net"),
		PsychicPassword: env("LOAD_PSYCHIC_PASSWORD", "Psych1c!Secure#2026"),
		DSN:             env("LOAD_DATABASE_DSN", "postgres://postgres:postgres@127.0.0.1:5432/staryield?sslmode=disable"),
		ReportDir:       env("LOAD_REPORT_DIR", "results"),
		Strict:          env("LOAD_STRICT", "") == "1",
	}
}

// Rate scales a base arrival rate.
func (c Config) Rate(base int) int {
	r := int(float64(base)*c.Scale + 0.5)
	if r < 1 {
		r = 1
	}
	return r
}

// RequireAPI fails fast with a helpful message when the API is not running.
func RequireAPI(t *testing.T, cfg Config) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c := NewClient(cfg.API, cfg.Origin)
	if status, err := c.Get(ctx, "/readyz"); err != nil || status != 200 {
		t.Fatalf("API not ready at %s (status %d, %v). Start it first; see tests/load/README.md", cfg.API, status, err)
	}
}

// Report writes results as JSON and logs a summary. With LOAD_STRICT=1 any
// SLO violation fails the test; otherwise violations are reported only, so a
// laptop run still produces numbers.
func Report(t *testing.T, cfg Config, name string, results []Result) {
	t.Helper()
	t.Logf("\n%s", Summary(results))
	if err := os.MkdirAll(cfg.ReportDir, 0o755); err == nil {
		payload := map[string]any{
			"name": name, "at": time.Now().UTC(), "api": cfg.API, "scale": cfg.Scale,
			"duration": cfg.Duration.String(), "go": runtime.Version(), "cpus": runtime.NumCPU(), "results": results,
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		path := filepath.Join(cfg.ReportDir, fmt.Sprintf("%s-%s.json", time.Now().UTC().Format("20060102T150405Z"), name))
		if err := os.WriteFile(path, b, 0o644); err == nil {
			t.Logf("report: %s", path)
		}
	}
	for _, r := range results {
		if len(r.Violations) == 0 {
			continue
		}
		if cfg.Strict {
			t.Errorf("%s: %s", r.Name, strings.Join(r.Violations, "; "))
		} else {
			t.Logf("SLO miss (non-strict) %s: %s", r.Name, strings.Join(r.Violations, "; "))
		}
	}
}
