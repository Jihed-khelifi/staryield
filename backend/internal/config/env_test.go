package config

import (
	"bufio"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// restoreEnv snapshots the process environment and restores it after the
// test, undoing variables a .env file set.
func restoreEnv(t *testing.T) {
	t.Helper()
	saved := os.Environ()
	t.Cleanup(func() {
		os.Clearenv()
		for _, kv := range saved {
			k, v, _ := strings.Cut(kv, "=")
			_ = os.Setenv(k, v)
		}
	})
}

// chdirTemp runs the rest of the test in an empty directory.
func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestEverySettingHasOneVariableAndARoundTrippingDefault(t *testing.T) {
	clearPlatformEnv(t)
	vars := Variables()
	if len(vars) < 150 {
		t.Fatalf("only %d settings found", len(vars))
	}
	seen := map[string]bool{}
	for _, v := range vars {
		name := v.Names[0]
		if name != EnvVar(v.Key) || !strings.HasPrefix(name, EnvPrefix) || seen[name] {
			t.Errorf("bad or duplicate variable %q for %s", name, v.Key)
		}
		seen[name] = true
	}
	for key := range aliases {
		if !seen[EnvVar(key)] {
			t.Errorf("alias for unknown setting %s", key)
		}
	}

	// Setting every variable to its documented default reproduces the
	// built-in configuration: the defaults are valid .env syntax.
	want, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vars {
		t.Setenv(v.Names[0], v.Default)
	}
	got, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("defaults do not round-trip:\n got %+v\nwant %+v", got, want)
	}
}

var envLine = regexp.MustCompile(`^#?\s*([A-Z][A-Z0-9_]*)=`)

func TestEnvExampleDocumentsEverySetting(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "..", ".env.example"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	documented := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if m := envLine.FindStringSubmatch(strings.TrimSpace(sc.Text())); m != nil {
			documented[m[1]] = true
		}
	}
	known := map[string]bool{}
	for _, v := range Variables() {
		known[v.Names[0]] = true
		if !documented[v.Names[0]] {
			t.Errorf(".env.example does not document %s (default %q)", v.Names[0], v.Default)
		}
	}
	for name := range documented {
		if strings.HasPrefix(name, EnvPrefix) && !known[name] && !hasAnyPrefix(name, unmanagedPrefixes) {
			t.Errorf(".env.example documents unknown variable %s", name)
		}
	}
}

func TestEnvExampleIsAValidDevelopmentEnvFile(t *testing.T) {
	clearPlatformEnv(t)
	restoreEnv(t)
	path, _ := filepath.Abs(filepath.Join("..", "..", ".env.example"))
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.DSN != "postgres://postgres:postgres@127.0.0.1:5432/staryield?sslmode=disable" || cfg.Log.Format != "console" {
		t.Fatalf("example values not applied: %q %q", cfg.Database.DSN, cfg.Log.Format)
	}
	if os.Getenv("STARYIELD_SEED_ADMIN_EMAIL") == "" {
		t.Fatal("seed variables must reach the environment for the seed command")
	}
	if unknown := UnknownEnv(); len(unknown) != 0 {
		t.Fatalf("example sets unknown variables: %v", unknown)
	}
}

func TestEnvFileSyntax(t *testing.T) {
	clearPlatformEnv(t)
	restoreEnv(t)
	chdirTemp(t)
	file := strings.Join([]string{
		"# comment",
		"",
		"export STARYIELD_MAIL_FROM=\"Staryield <hello@staryield.net>\"",
		"STARYIELD_ADMIN_TOTP_ISSUER='Literal \\n issuer'",
		"STARYIELD_APP_FRONTEND_URL=https://staryield.net # inline comment",
		"DATABASE_URL=postgres://u:p@db:5432/x?sslmode=require&a=b",
		"STARYIELD_REDIS_KEY_PREFIX=\"tab\\there\"",
		"STARYIELD_LINKS_ALLOWED_HOSTS = a.test , b.test\r",
		"   STARYIELD_CHAT_GRACE_PERIOD=2m",
	}, "\n")
	if err := os.WriteFile(".env", []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string][2]string{
		"mail.from":         {cfg.Mail.From, "Staryield <hello@staryield.net>"},
		"admin.totp_issuer": {cfg.Admin.TOTPIssuer, `Literal \n issuer`},
		"app.frontend_url":  {cfg.App.FrontendURL, "https://staryield.net"},
		"database.dsn":      {cfg.Database.DSN, "postgres://u:p@db:5432/x?sslmode=require&a=b"},
		"redis.key_prefix":  {cfg.Redis.KeyPrefix, "tab\there"},
		"links.hosts":       {strings.Join(cfg.Links.AllowedHosts, "|"), "a.test|b.test"},
		"chat.grace_period": {cfg.Chat.GracePeriod.String(), "2m0s"},
	}
	for name, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", name, c[0], c[1])
		}
	}
}

func TestEnvFileNeverOverridesTheEnvironment(t *testing.T) {
	clearPlatformEnv(t)
	restoreEnv(t)
	chdirTemp(t)
	if err := os.WriteFile(".env", []byte("STARYIELD_APP_FRONTEND_URL=https://from-file.test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STARYIELD_APP_FRONTEND_URL", "https://from-env.test")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.FrontendURL != "https://from-env.test" {
		t.Fatalf("frontend url %q", cfg.App.FrontendURL)
	}
}

func TestEnvFileErrors(t *testing.T) {
	clearPlatformEnv(t)
	restoreEnv(t)
	dir := chdirTemp(t)

	// No ./.env is fine (Railway ships none); a named file must exist.
	if _, err := Load(""); err != nil {
		t.Fatalf("missing default .env: %v", err)
	}
	if _, err := Load(filepath.Join(dir, "missing.env")); err == nil {
		t.Fatal("missing explicit env file accepted")
	}

	for _, bad := range []string{
		"STARYIELD_AUTH_JWT_SECRET super-secret-value",
		"STARYIELD_AUTH_JWT_SECRET=\"super-secret-value",
		"STARYIELD_AUTH_JWT_SECRET='super-secret-value",
		"BAD-NAME=super-secret-value",
	} {
		if err := os.WriteFile(".env", []byte("# ok\n"+bad+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := Load("")
		if err == nil || !strings.Contains(err.Error(), "line 2") {
			t.Errorf("%q: expected a line 2 error, got %v", bad, err)
			continue
		}
		if strings.Contains(err.Error(), "super-secret-value") {
			t.Errorf("error leaks the line content: %v", err)
		}
	}
}

func TestInvalidValuesNameTheVariable(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("STARYIELD_HTTP_READ_TIMEOUT", "thirty")
	t.Setenv("STARYIELD_WORKER_ENABLED", "maybe")
	t.Setenv("STARYIELD_AUTH_ARGON2_PARALLELISM", "300")
	t.Setenv("STARYIELD_DATABASE_MAX_OPEN_CONNS", "2.5")
	t.Setenv("STARYIELD_RATE_LIMIT_SCALE", "fast")
	_, err := Load("")
	if err == nil {
		t.Fatal("invalid values accepted")
	}
	for _, want := range []string{
		`STARYIELD_HTTP_READ_TIMEOUT must be a duration`,
		`STARYIELD_WORKER_ENABLED must be true or false`,
		`STARYIELD_AUTH_ARGON2_PARALLELISM must be a whole number within 0..255`,
		`STARYIELD_DATABASE_MAX_OPEN_CONNS must be a whole number`,
		`STARYIELD_RATE_LIMIT_SCALE must be a number`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}
}

func TestBlankVariableFallsBackToPlatformAlias(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("STARYIELD_DATABASE_DSN", "  ")
	t.Setenv("DATABASE_URL", "postgres://platform/db")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.DSN != "postgres://platform/db" {
		t.Fatalf("dsn %q", cfg.Database.DSN)
	}
}

func TestYAMLConfigFilesAreIgnored(t *testing.T) {
	clearPlatformEnv(t)
	restoreEnv(t)
	chdirTemp(t)
	yaml := []byte("app:\n  env: production\nhttp:\n  addr: \":7777\"\n")
	for _, name := range []string{"config.yaml", "config.yml"} {
		if err := os.WriteFile(name, yaml, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.IsProduction() || cfg.HTTP.Addr != ":8080" {
		t.Fatalf("a YAML file was read: env=%s addr=%s", cfg.App.Env, cfg.HTTP.Addr)
	}
}

func TestUnknownEnvReportsTyposOnly(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("STARYIELD_HTTP_ADRESS", ":1")
	t.Setenv("STARYIELD_SEED_ADMIN_EMAIL", "admin@example.test")
	t.Setenv("STARYIELD_TEST_DATABASE_DSN", "postgres://test")
	t.Setenv("STARYIELD_HTTP_ADDR", ":8080")
	unknown := UnknownEnv()
	if len(unknown) != 1 || unknown[0] != "STARYIELD_HTTP_ADRESS" {
		t.Fatalf("unknown: %v", unknown)
	}
}
