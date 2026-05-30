package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadDotenv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "# comment\nTG_APP_ID = \"123\"\nTG_APP_HASH='abc'\nMALFORMED\n\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	// Ensure a clean slate, then load the file.
	t.Setenv("TG_APP_ID", "")
	t.Setenv("TG_APP_HASH", "")
	if err := readDotenv(path); err != nil {
		t.Fatalf("readDotenv error = %v", err)
	}
	if got := os.Getenv("TG_APP_ID"); got != "123" {
		t.Errorf("TG_APP_ID = %q, want 123 (quotes/space trimmed)", got)
	}
	if got := os.Getenv("TG_APP_HASH"); got != "abc" {
		t.Errorf("TG_APP_HASH = %q, want abc", got)
	}
}

func TestReadDotenvMissingFileErrors(t *testing.T) {
	if err := readDotenv(filepath.Join(t.TempDir(), "nope.env")); err == nil {
		t.Error("missing file should return an error (Load ignores it)")
	}
}

func TestLoadDefaults(t *testing.T) {
	// Empty env => bundled defaults.
	t.Setenv(envAppID, "")
	t.Setenv(envAppHash, "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AppID != defaultAppID {
		t.Errorf("AppID = %d, want default %d", cfg.AppID, defaultAppID)
	}
	if cfg.AppHash != defaultAppHash {
		t.Errorf("AppHash = %q, want default", cfg.AppHash)
	}
	if cfg.SessionPath != defaultSession {
		t.Errorf("SessionPath = %q, want %q", cfg.SessionPath, defaultSession)
	}
}

func TestLoadOverride(t *testing.T) {
	t.Setenv(envAppID, "999")
	t.Setenv(envAppHash, "deadbeef")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AppID != 999 || cfg.AppHash != "deadbeef" {
		t.Errorf("override not applied: got id=%d hash=%q", cfg.AppID, cfg.AppHash)
	}
}

func TestLoadPartialOverrideIgnored(t *testing.T) {
	// Only one of the two set => override must NOT apply (avoid mismatched pair).
	t.Setenv(envAppID, "999")
	t.Setenv(envAppHash, "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AppID != defaultAppID {
		t.Errorf("partial override should be ignored, got AppID=%d", cfg.AppID)
	}
}

func TestLoadMalformedAppID(t *testing.T) {
	t.Setenv(envAppID, "not-a-number")
	t.Setenv(envAppHash, "x")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-numeric TG_APP_ID")
	}
}
