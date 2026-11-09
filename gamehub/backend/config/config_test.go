package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":    "postgres://u:p@localhost:5432/gamehub",
		"JWT_SECRET":      strings.Repeat("s", 32),
		"ALLOWED_ORIGINS": "http://localhost:5173",
	}
}

func TestLoadOK(t *testing.T) {
	cfg, err := load(env(validEnv()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want default 8080", cfg.Port)
	}
	if cfg.DatabaseURL == "" || len(cfg.AllowedOrigins) != 1 {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestLoadUsesPortFromEnv(t *testing.T) {
	e := validEnv()
	e["PORT"] = "10000"
	cfg, err := load(env(e))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "10000" {
		t.Errorf("Port = %q, want 10000", cfg.Port)
	}
}

func TestLoadReportsEveryProblem(t *testing.T) {
	_, err := load(env(map[string]string{}))
	if err == nil {
		t.Fatal("want error when everything is missing")
	}
	for _, want := range []string{"DATABASE_URL", "JWT_SECRET", "ALLOWED_ORIGINS"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

func TestLoadRejectsShortSecret(t *testing.T) {
	e := validEnv()
	e["JWT_SECRET"] = strings.Repeat("s", 31)
	if _, err := load(env(e)); err == nil {
		t.Fatal("31-char secret must be rejected")
	}
}

func TestLoadSplitsAndTrimsOrigins(t *testing.T) {
	e := validEnv()
	e["ALLOWED_ORIGINS"] = " http://localhost:5173 , https://gamehub.vercel.app,, "
	cfg, err := load(env(e))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"http://localhost:5173", "https://gamehub.vercel.app"}
	if len(cfg.AllowedOrigins) != len(want) {
		t.Fatalf("origins = %v, want %v", cfg.AllowedOrigins, want)
	}
	for i := range want {
		if cfg.AllowedOrigins[i] != want[i] {
			t.Errorf("origin[%d] = %q, want %q", i, cfg.AllowedOrigins[i], want[i])
		}
	}
}
