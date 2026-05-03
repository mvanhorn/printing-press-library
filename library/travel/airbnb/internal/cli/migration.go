package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// legacyEnvPrefix and legacyDirName carry the pre-rename identity. The CLI
// shipped as airbnb-vrbo-pp-cli with AIRBNB_VRBO_* env vars and a state dir
// at ~/.airbnb-vrbo-pp-cli; this PR renames everything to airbnb-pp-cli /
// AIRBNB_PP_* / ~/.airbnb-pp-cli, with one release of dual-read so existing
// users do not lose state on upgrade.
const (
	legacyEnvPrefix = "AIRBNB_VRBO_"
	envPrefix       = "AIRBNB_PP_"
	legacyDirName   = ".airbnb-vrbo-pp-cli"
	dirName         = ".airbnb-pp-cli"
	legacyShareName = "airbnb-vrbo-pp-cli"
	shareName       = "airbnb-pp-cli"
)

var (
	legacyMigrationOnce sync.Once
	legacyEnvWarned     bool
)

// MigrateLegacy is called from root.go's PersistentPreRunE on every command
// invocation. The first call performs:
//
//  1. AIRBNB_VRBO_* env vars whose AIRBNB_PP_* counterpart is unset get
//     copied across so the rest of the CLI can read only the new prefix.
//     A single deprecation warning is printed to stderr per process.
//  2. ~/.airbnb-vrbo-pp-cli is renamed to ~/.airbnb-pp-cli when the old
//     directory exists and the new one does not. Same for the SQLite
//     store under ~/.local/share.
//
// Subsequent calls are no-ops via sync.Once. Failures are logged but
// non-fatal — a fresh install with no legacy state should never see them.
func MigrateLegacy(stderr io.Writer) {
	legacyMigrationOnce.Do(func() {
		migrateLegacyEnv(stderr)
		migrateLegacyDirs(stderr)
	})
}

func migrateLegacyEnv(stderr io.Writer) {
	for _, kv := range os.Environ() {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		key, val := kv[:eq], kv[eq+1:]
		if !strings.HasPrefix(key, legacyEnvPrefix) {
			continue
		}
		newKey := envPrefix + strings.TrimPrefix(key, legacyEnvPrefix)
		if _, ok := os.LookupEnv(newKey); ok {
			continue
		}
		_ = os.Setenv(newKey, val)
		if !legacyEnvWarned && stderr != nil {
			fmt.Fprintf(stderr, "warning: %s* env vars are deprecated; use %s* (read both for now, will drop in a future release)\n", legacyEnvPrefix, envPrefix)
			legacyEnvWarned = true
		}
	}
}

func migrateLegacyDirs(stderr io.Writer) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	migrateOne(stderr, filepath.Join(home, legacyDirName), filepath.Join(home, dirName))
	migrateOne(stderr, filepath.Join(home, ".local", "share", legacyShareName), filepath.Join(home, ".local", "share", shareName))
}

func migrateOne(stderr io.Writer, oldPath, newPath string) {
	if oldPath == newPath {
		return
	}
	if _, err := os.Stat(oldPath); err != nil {
		return
	}
	if _, err := os.Stat(newPath); err == nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		if stderr != nil {
			fmt.Fprintf(stderr, "warning: could not migrate %s to %s: %v\n", oldPath, newPath, err)
		}
		return
	}
	if stderr != nil {
		fmt.Fprintf(stderr, "info: migrated state %s -> %s (rename in this release; existing data preserved)\n", oldPath, newPath)
	}
}
