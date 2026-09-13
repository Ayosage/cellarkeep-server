package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadRequiresEveryVariable(t *testing.T) {
	_, err := Load(env(map[string]string{
		"DATABASE_URL":  "postgres://x",
		"SESSION_SECRET": strings.Repeat("a", 32),
	}))
	if err == nil || !strings.Contains(err.Error(), "CLIENT_ORIGIN") {
		t.Fatalf("want CLIENT_ORIGIN error, got %v", err)
	}
}

func TestLoadRejectsShortSecret(t *testing.T) {
	_, err := Load(env(map[string]string{
		"DATABASE_URL":  "postgres://x",
		"SESSION_SECRET": "short",
		"CLIENT_ORIGIN": "http://localhost:3000",
	}))
	if err == nil || !strings.Contains(err.Error(), "SESSION_SECRET") {
		t.Fatalf("want SESSION_SECRET error, got %v", err)
	}
}

func TestLoadDefaultsAddr(t *testing.T) {
	c, err := Load(env(map[string]string{
		"DATABASE_URL":  "postgres://x",
		"SESSION_SECRET": strings.Repeat("a", 32),
		"CLIENT_ORIGIN": "http://localhost:3000",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":8080" {
		t.Fatalf("want :8080, got %q", c.Addr)
	}
	if !c.InsecureCookies() {
		t.Fatal("localhost origin should allow insecure cookies")
	}
}
