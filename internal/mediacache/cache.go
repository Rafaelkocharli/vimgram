// Package mediacache manages the on-disk cache for downloaded Telegram media.
package mediacache

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

// Dir returns the platform-specific cache directory for vimgram media.
// The directory is created if it does not exist.
func Dir() (string, error) {
	base, err := baseDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "vimgram", "media")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mediacache: create dir: %w", err)
	}
	return dir, nil
}

// Path returns the full path for a cached file given its Telegram file ID
// and extension (e.g. ".jpg"). The path is deterministic: the same ID always
// maps to the same path, regardless of whether the file exists yet.
func Path(dir string, fileID int64, ext string) string {
	return filepath.Join(dir, strconv.FormatInt(fileID, 10)+ext)
}

// Exists reports whether the file at path already exists on disk.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Delete removes the cached file at path. Returns nil if the file does not
// exist (idempotent).
func Delete(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func baseDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Caches"), nil
	case "windows":
		dir := os.Getenv("LOCALAPPDATA")
		if dir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			dir = home
		}
		return dir, nil
	default: // Linux and others
		if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
			return dir, nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".cache"), nil
	}
}
