// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/store"
	"github.com/spf13/cobra"
)

const tbCLIName = "thunderbird-pp-cli"

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
