// pp:data-source local

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/spf13/cobra"
)

const tbCLIName = "thunderbird-pp-cli"

// tbDefaultStorePath is the store of the selected Thunderbird profile under dataDir; an unresolvable selection gets an empty store, never another profile's.
func tbDefaultStorePath(dataDir string) string {
	profileDir, err := tbprofile.Resolve(tbProfileSelector(nil), tbprofile.RootDir())
	if err != nil {
		return filepath.Join(dataDir, "profiles", "unresolved", "data.db")
	}
	return tbStoreDBPath(dataDir, profileDir)
}

// tbStoreDBPath keys the store by profile directory: account and folder ids repeat across profiles.
func tbStoreDBPath(dataDir, profileDir string) string {
	p, err := filepath.Abs(profileDir)
	if err != nil {
		p = filepath.Clean(profileDir)
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		p = strings.ToLower(p)
	}
	sum := sha256.Sum256([]byte(p))
	return filepath.Join(dataDir, "profiles", hex.EncodeToString(sum[:6]), "data.db")
}

// tbSyncDBPath is the default store of an already resolved profile.
func tbSyncDBPath(profileDir string) string {
	dir, err := cliutil.DataDir()
	if err != nil {
		return defaultDBPath(tbCLIName)
	}
	return tbStoreDBPath(dir, profileDir)
}

// tbOpenStore opens the synced store read-only. When the database does not
// exist it prints the sync hint to stderr and returns nil, nil.
func tbOpenStore(cmd *cobra.Command) (*store.Store, error) {
	dbPath := defaultDBPath(tbCLIName)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local data yet; run: %s sync\n", tbCLIName)
		return nil, nil
	}
	return store.OpenReadOnlyContext(cmd.Context(), dbPath)
}

// tbOpenStoreQuiet is tbOpenStore without the stderr hint, for commands
// with a non-store fallback.
func tbOpenStoreQuiet(cmd *cobra.Command) (*store.Store, error) {
	dbPath := defaultDBPath(tbCLIName)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil
	}
	return store.OpenReadOnlyContext(cmd.Context(), dbPath)
}

// tbEmitEmpty prints an empty JSON array for machine output.
func tbEmitEmpty(cmd *cobra.Command, flags *rootFlags) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		return nil
	}
	return printJSONFiltered(cmd.OutOrStdout(), []any{}, flags)
}

// tbLoadDocs decodes every stored document of resourceType into T.
func tbLoadDocs[T any](db *store.Store, resourceType string) ([]T, error) {
	rows, err := db.DB().Query(`SELECT data FROM resources WHERE resource_type = ? ORDER BY id`, resourceType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]T, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var v T
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// tbLastSync returns the most recent sync time of any resource.
func tbLastSync(db *store.Store) time.Time {
	var latest time.Time
	for _, r := range tbResourceTypes {
		_, t, _, err := db.GetSyncState(r)
		if err == nil && t.After(latest) {
			latest = t
		}
	}
	return latest
}

func tbHumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
