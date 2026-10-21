package main

import (
	"strings"
	"testing"
)

func TestAdminFromEnv(t *testing.T) {
	env := func(vars map[string]string) func(string) string {
		return func(k string) string { return vars[k] }
	}

	acct, err := adminFromEnv(env(map[string]string{
		"STARYIELD_SEED_ADMIN_EMAIL":    " owner@staryield.net ",
		"STARYIELD_SEED_ADMIN_PASSWORD": "Str0ng!Passw0rd#",
	}))
	if err != nil || acct.Email != "owner@staryield.net" || acct.Password != "Str0ng!Passw0rd#" {
		t.Fatalf("account: %+v %v", acct, err)
	}

	for name, c := range map[string]struct {
		vars map[string]string
		want string
	}{
		"nothing":       {map[string]string{}, "STARYIELD_SEED_ADMIN_EMAIL and STARYIELD_SEED_ADMIN_PASSWORD"},
		"no password":   {map[string]string{"STARYIELD_SEED_ADMIN_EMAIL": "owner@staryield.net"}, "needs STARYIELD_SEED_ADMIN_PASSWORD set"},
		"blank e-mail":  {map[string]string{"STARYIELD_SEED_ADMIN_EMAIL": "  ", "STARYIELD_SEED_ADMIN_PASSWORD": "x"}, "needs STARYIELD_SEED_ADMIN_EMAIL set"},
		"secret hidden": {map[string]string{"STARYIELD_SEED_ADMIN_PASSWORD": "Hunter2!Secret#"}, "STARYIELD_SEED_ADMIN_EMAIL"},
	} {
		_, err := adminFromEnv(env(c.vars))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want %q, got %v", name, c.want, err)
		}
		if err != nil && strings.Contains(err.Error(), "Hunter2") {
			t.Errorf("%s: error leaks the password: %v", name, err)
		}
	}
}
