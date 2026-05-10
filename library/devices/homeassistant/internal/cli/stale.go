package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"homeassistant-pp-cli/internal/cliutil"
	"homeassistant-pp-cli/internal/store"
)

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var maxAge time.Duration

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Check whether the local data store needs a sync",
		Example: `  # Check if data is older than 1 hour
  homeassistant-pp-cli stale --max-age 1h

  # Check staleness as JSON for agents
  homeassistant-pp-cli stale --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if cliutil.IsVerifyEnv() {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"stale":       true,
						"entity_count": 0,
						"reason":      "verify mode",
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Store is empty — run 'homeassistant-pp-cli sync' first.")
				return nil
			}

			db, err := store.Open("")
			if err != nil {
				return err
			}

			count, err := db.StateCount()
			if err != nil {
				return err
			}

			if count == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"stale":       true,
						"entity_count": 0,
						"reason":      "no data",
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Store is empty — run 'homeassistant-pp-cli sync' first.")
				return nil
			}

			lastSyncStr, _ := db.GetMeta("last_synced_at")
			stale := lastSyncStr == ""

			if !stale && maxAge > 0 {
				lastSync, err := time.Parse(time.RFC3339, lastSyncStr)
				if err == nil {
					stale = time.Since(lastSync) > maxAge
				} else {
					stale = true // parse error, assume stale
				}
			}

			result := map[string]any{
				"stale":        stale,
				"entity_count": count,
			}
			if lastSyncStr != "" {
				result["last_sync_entities"] = lastSyncStr
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			if stale {
				fmt.Fprintf(cmd.OutOrStdout(), "Store may be stale (%d entities). Run 'homeassistant-pp-cli sync' to refresh.\n", count)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Store is fresh (%d entities).\n", count)
			}
			return nil
		},
	}

	cmd.Flags().DurationVar(&maxAge, "max-age", time.Hour, "Maximum acceptable age of cached data")

	return cmd
}
