package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// EnvPrefix starts every configuration variable.
const EnvPrefix = "STARYIELD_"

// DefaultEnvFile is read from the working directory when present.
const DefaultEnvFile = ".env"

// aliases lists, per setting, the conventional variables hosting platforms
// (Railway, Heroku, Render, Fly…) or the deployment's .env inject. They are
// consulted, in order, only when the STARYIELD_* variable is unset.
var aliases = map[string][]string{
	"database.dsn":    {"DATABASE_URL"},
	"redis.url":       {"REDIS_URL"},
	"app.env":         {"APP_ENV"},
	"app.instance_id": {"RAILWAY_REPLICA_ID"},
	// Cloudflare R2 buckets.
	"storage.advisor_bucket":          {"R2_ADVISOR_IMAGE"},
	"storage.advisor_public_base_url": {"R2_ADVISOR_IMAGE_PUBLIC_URL"},
	"storage.chat_bucket":             {"R2_CHAT_IMAGES_PRIVATE"},
}

// unmanagedPrefixes are STARYIELD_* variables read elsewhere (seed accounts,
// test infrastructure), not by Load.
var unmanagedPrefixes = []string{EnvPrefix + "SEED_", EnvPrefix + "TEST_", EnvPrefix + "E2E"}

// Variable describes one configuration setting.
type Variable struct {
	// Key is the dotted setting path, e.g. "auth.jwt_secret".
	Key string
	// Names are the environment variables that set it, highest precedence
	// first: STARYIELD_AUTH_JWT_SECRET, then any platform aliases.
	Names []string
	// Default is the built-in value in .env syntax.
	Default string

	value reflect.Value
}

// EnvVar returns the STARYIELD_* variable for a dotted setting key
// ("auth.jwt_secret" → "STARYIELD_AUTH_JWT_SECRET").
func EnvVar(key string) string {
	return EnvPrefix + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
}

// Variables lists every setting with its variable names and default value,
// sorted by key.
func Variables() []Variable {
	cfg := defaults()
	return variables(&cfg)
}

// variables walks cfg's `env`-tagged fields; each Variable points at its
// (settable) field.
func variables(cfg *Config) []Variable {
	var out []Variable
	var walk func(v reflect.Value, path []string)
	walk = func(v reflect.Value, path []string) {
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			name := t.Field(i).Tag.Get("env")
			if name == "" {
				panic(fmt.Sprintf("config: field %s.%s has no env tag", t.Name(), t.Field(i).Name))
			}
			p := append(append([]string(nil), path...), name)
			f := v.Field(i)
			if f.Kind() == reflect.Struct {
				walk(f, p)
				continue
			}
			key := strings.Join(p, ".")
			out = append(out, Variable{Key: key, Names: append([]string{EnvVar(key)}, aliases[key]...), Default: format(f), value: f})
		}
	}
	walk(reflect.ValueOf(cfg).Elem(), nil)
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// Load builds the configuration from built-in defaults, Railway defaults
// (when running there), the .env file and the process environment, then
// validates it. envFile names the .env file; empty means DefaultEnvFile when
// it exists. Variables already set in the process environment always win
// over the file.
func Load(envFile string) (*Config, error) {
	if err := loadEnvFile(envFile); err != nil {
		return nil, err
	}
	cfg := defaults()
	if OnRailway() {
		cfg.applyRailwayDefaults()
	}

	var errs []error
	for _, v := range variables(&cfg) {
		name, raw, ok := lookup(v.Names)
		if !ok {
			continue
		}
		if err := parse(v.value, raw); err != nil {
			errs = append(errs, fmt.Errorf("%s %v (got %q)", name, err, raw))
		}
	}
	// PORT is the platform-assigned listen port; an explicit
	// STARYIELD_HTTP_ADDR still wins.
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" && strings.TrimSpace(os.Getenv(EnvVar("http.addr"))) == "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			errs = append(errs, fmt.Errorf("PORT must be a TCP port (got %q)", port))
		} else {
			cfg.HTTP.Addr = ":" + port
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	cfg.normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// OnRailway reports whether the process runs on Railway.
func OnRailway() bool { return os.Getenv("RAILWAY_ENVIRONMENT_ID") != "" }

// UnknownEnv returns STARYIELD_* variables in the environment that match no
// setting (typically typos), sorted. Callers log them; values are never
// returned.
func UnknownEnv() []string {
	known := map[string]bool{}
	for _, v := range Variables() {
		known[v.Names[0]] = true
	}
	var out []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if !strings.HasPrefix(name, EnvPrefix) || known[name] || hasAnyPrefix(name, unmanagedPrefixes) {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// lookup returns the first of names set to a non-blank value. Blank
// variables count as unset, so an empty STARYIELD_* entry falls back to the
// platform alias or the default.
func lookup(names []string) (name, value string, ok bool) {
	for _, n := range names {
		if v, set := os.LookupEnv(n); set && strings.TrimSpace(v) != "" {
			return n, strings.TrimSpace(v), true
		}
	}
	return "", "", false
}

var (
	durationType = reflect.TypeOf(time.Duration(0))
	listType     = reflect.TypeOf([]string(nil))
)

// parse sets field from its .env representation. Lists are comma-separated.
func parse(field reflect.Value, raw string) error {
	switch field.Type() {
	case durationType:
		d, err := time.ParseDuration(raw)
		if err != nil {
			return errors.New("must be a duration such as 500ms, 30s, 15m or 2h")
		}
		field.SetInt(int64(d))
		return nil
	case listType:
		field.Set(reflect.ValueOf(splitList([]string{raw})))
		return nil
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return errors.New("must be true or false")
		}
		field.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, field.Type().Bits())
		if err != nil {
			return errors.New("must be a whole number")
		}
		field.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("must be a whole number within 0..%d", uint64(1)<<field.Type().Bits()-1)
		}
		field.SetUint(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, field.Type().Bits())
		if err != nil {
			return errors.New("must be a number")
		}
		field.SetFloat(f)
	default:
		panic(fmt.Sprintf("config: unsupported setting type %s", field.Type()))
	}
	return nil
}

// format renders a value in the syntax parse accepts.
func format(v reflect.Value) string {
	switch v.Type() {
	case durationType:
		return formatDuration(time.Duration(v.Int()))
	case listType:
		return strings.Join(v.Interface().([]string), ",")
	}
	return fmt.Sprint(v.Interface())
}

// formatDuration prints the shortest exact unit: 90s, 15m, 168h, 250ms.
func formatDuration(d time.Duration) string {
	switch {
	case d == 0:
		return "0s"
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", d/time.Minute)
	case d%time.Second == 0:
		return fmt.Sprintf("%ds", d/time.Second)
	case d%time.Millisecond == 0:
		return fmt.Sprintf("%dms", d/time.Millisecond)
	}
	return d.String()
}

// loadEnvFile copies KEY=VALUE lines from a .env file into the process
// environment without overriding variables that are already set. An empty
// path reads DefaultEnvFile when it exists; an explicit path must exist.
// Platforms such as Railway inject variables directly and ship no file.
//
// Syntax: blank lines and # comments are skipped; "export " is optional;
// values may be single-quoted (literal) or double-quoted (\n, \t, \" and \\
// escapes); an unquoted value ends at " #".
func loadEnvFile(path string) error {
	explicit := path != ""
	if !explicit {
		path = DefaultEnvFile
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !explicit && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("config: read env file: %w", err)
	}
	for i, line := range strings.Split(string(data), "\n") {
		key, value, ok, err := parseEnvLine(line)
		if err != nil {
			// The line itself may hold a secret; report its number only.
			return fmt.Errorf("config: %s line %d: %v", path, i+1, err)
		}
		if !ok {
			continue
		}
		if _, set := os.LookupEnv(key); !set {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("config: %s line %d: %w", path, i+1, err)
			}
		}
	}
	return nil
}

// parseEnvLine parses one .env line; ok is false for blank and comment lines.
func parseEnvLine(line string) (key, value string, ok bool, err error) {
	line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false, nil
	}
	line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	key, value, found := strings.Cut(line, "=")
	key = strings.TrimSpace(key)
	if !found || !validEnvName(key) {
		return "", "", false, errors.New("expected KEY=VALUE")
	}
	value = strings.TrimSpace(value)
	switch {
	case strings.HasPrefix(value, "'"):
		end := strings.Index(value[1:], "'")
		if end < 0 {
			return "", "", false, errors.New("unterminated single quote")
		}
		return key, value[1 : end+1], true, nil
	case strings.HasPrefix(value, `"`):
		var b strings.Builder
		for i := 1; i < len(value); i++ {
			c := value[i]
			switch {
			case c == '"':
				return key, b.String(), true, nil
			case c == '\\' && i+1 < len(value):
				i++
				switch value[i] {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				default:
					b.WriteByte(value[i])
				}
			default:
				b.WriteByte(c)
			}
		}
		return "", "", false, errors.New("unterminated double quote")
	}
	if i := strings.Index(value, " #"); i >= 0 {
		value = strings.TrimSpace(value[:i])
	}
	return key, value, true, nil
}

// validEnvName accepts POSIX-style names: letters, digits and underscores,
// not starting with a digit.
func validEnvName(name string) bool {
	if name == "" || (name[0] >= '0' && name[0] <= '9') {
		return false
	}
	for _, r := range name {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
