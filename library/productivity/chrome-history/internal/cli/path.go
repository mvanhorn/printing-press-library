package cli

import (
	"os"
	"path/filepath"
	"strings"
)

func snapshotPath() (string, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if strings.TrimSpace(base) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".cache")
	}
	dir := filepath.Join(base, "chrome-history")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "snapshot.db"), nil
}
