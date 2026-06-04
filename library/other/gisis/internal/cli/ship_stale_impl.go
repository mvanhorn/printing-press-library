// Hand-authored — NOT generated. Implements `gisis-pp-cli ship stale`: list
// cached vessels whose particulars have not been refreshed within a threshold,
// so a watchlist can be kept current. Read-only against the local store.
package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/gisis/internal/store"

	"github.com/spf13/cobra"
)

// parseOlderThan accepts day-suffixed thresholds ("30d", "7d") in addition to
// Go durations ("24h", "90m"). An empty string defaults to 30 days. Shared by
// `ship stale` and `ship refresh --older-than`.
func parseOlderThan(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 30 * 24 * time.Hour, nil
	}
	if rest, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(rest)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("invalid --older-than %q: use forms like 7d, 24h, 90m", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("invalid --older-than %q: use forms like 7d, 24h, 90m", s)
	}
	return d, nil
}

func newShipStaleCmd(flags *rootFlags) *cobra.Command {
	var flagOlderThan string
	var flagPinned bool

	cmd := &cobra.Command{
		Use:         "stale",
		Short:       "List cached vessels not refreshed within a threshold (default 30d).",
		Long:        "Scans the local cache and returns vessels whose last fetch is older than --older-than, oldest first. Pair with 'ship refresh --older-than' to bring them current. Use --pinned to limit the check to your watchlist.",
		Example:     "  gisis-pp-cli ship stale --older-than 30d --json\n  gisis-pp-cli ship stale --pinned --older-than 7d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dur, err := parseOlderThan(flagOlderThan)
			if err != nil {
				return usageErr(err)
			}
			db, err := openStoreForRead(cmd.Context(), "gisis-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if db == nil {
				return printJSONFiltered(cmd.OutOrStdout(), []store.ShipRow{}, flags)
			}
			defer db.Close()

			rows, err := db.StaleShips(time.Now().UTC().Add(-dur), flagPinned)
			if err != nil {
				return fmt.Errorf("querying local ship cache: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&flagOlderThan, "older-than", "30d", "Age threshold, e.g. 7d, 24h, 90m")
	cmd.Flags().BoolVar(&flagPinned, "pinned", false, "Only consider pinned (watchlisted) vessels")
	return cmd
}
