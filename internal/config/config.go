// Package config loads runtime configuration: Telegram app credentials and
// the session file path. Bundled defaults are baked in; an optional .env
// next to the binary can override them.
package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// Bundled Telegram app credentials. The override rules live in Load.
const (
	defaultAppID    = 31452204
	defaultAppHash  = "7be152ba05c87019d22948ae3188b8e9"
	defaultSession  = "session.json"
	envFile         = ".env"
	envAppID        = "TG_APP_ID"
	envAppHash      = "TG_APP_HASH"
)

// Config is the resolved runtime configuration.
type Config struct {
	AppID       int
	AppHash     string
	SessionPath string
}

// Load resolves config from bundled defaults, optionally overridden by .env.
// Both TG_APP_ID and TG_APP_HASH must be present for the override to apply.
// Returns an error only if env values are malformed (non-numeric AppID).
func Load() (Config, error) {
	cfg := Config{
		AppID:       defaultAppID,
		AppHash:     defaultAppHash,
		SessionPath: defaultSession,
	}

	// .env is optional; missing/unreadable file is not an error.
	_ = readDotenv(envFile)

	envID := os.Getenv(envAppID)
	envHash := os.Getenv(envAppHash)
	if envID == "" || envHash == "" {
		return cfg, nil
	}
	id, err := strconv.Atoi(envID)
	if err != nil {
		return cfg, &MalformedError{Key: envAppID, Reason: "must be a number"}
	}
	cfg.AppID = id
	cfg.AppHash = envHash
	return cfg, nil
}

// MalformedError signals a structural problem with a config value.
type MalformedError struct {
	Key    string
	Reason string
}

func (e *MalformedError) Error() string { return e.Key + ": " + e.Reason }

// readDotenv loads KEY=VALUE pairs from the given file into the process env.
func readDotenv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = strings.Trim(val, `"'`)
		_ = os.Setenv(key, val)
	}
	return sc.Err()
}
