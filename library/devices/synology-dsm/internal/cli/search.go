package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/devices/synology-dsm/internal/store"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	var resourceType string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across synced NAS resources",
		Long: `Search locally synced data using SQLite FTS5 full-text search.
Results are returned instantly from the local store — no API call needed.

Run 'synology-dsm-pp-cli sync' first to populate the local database.`,
		Example: `  # Search all synced resources for "backup"
  synology-dsm-pp-cli search backup

  # Search only webapi resources
  synology-dsm-pp-cli search "disk health" --resource webapi

  # JSON output with field selection
  synology-dsm-pp-cli search "storage volume" --json --select id,resource_type

  # Limit results
  synology-dsm-pp-cli search "container" --limit 5`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("synology-dsm-pp-cli")
			}

			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'synology-dsm-pp-cli sync' first.", err)
			}
			defer db.Close()

			query := args[0]
			var results []json.RawMessage
			if resourceType != "" {
				results, err = db.SearchByType(resourceType, query, limit)
			} else {
				results, err = db.Search(query, limit)
			}
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}

			if len(results) == 0 {
				if flags.quiet {
					return nil
				}
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
					return nil
				}
				fmt.Fprintln(os.Stderr, "No results found. Try 'synology-dsm-pp-cli sync' to update the local index.")
				return notFoundErr(fmt.Errorf("no results for %q", query))
			}

			data, _ := json.Marshal(results)
			return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(data), flags)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/synology-dsm-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return")
	cmd.Flags().StringVar(&resourceType, "resource", "", "Filter to a specific resource type (e.g. webapi)")
	cmd.MarkFlagRequired("resource")

	return cmd
}
