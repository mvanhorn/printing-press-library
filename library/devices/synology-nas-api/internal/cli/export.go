package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/devices/synology-nas-api/internal/store"
)

func newExportCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var resourceType string
	var format string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export synced NAS resources to stdout in JSON or CSV format",
		Long: `Export all synced data from the local SQLite store. Supports JSON (default)
and CSV output formats. Filter by resource type to export a single table.

Run 'synology-nas-api-pp-cli sync' first to populate the local database.`,
		Example: `  # Export all synced data as JSON
  synology-nas-api-pp-cli export

  # Export only webapi resources
  synology-nas-api-pp-cli export --resource webapi

  # Export as CSV
  synology-nas-api-pp-cli export --format csv > nas-data.csv

  # Export with field selection
  synology-nas-api-pp-cli export --resource webapi --json --select id,status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("synology-nas-api-pp-cli")
			}

			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'synology-nas-api-pp-cli sync' first.", err)
			}
			defer db.Close()

			status, err := db.Status()
			if err != nil {
				return fmt.Errorf("reading store status: %w", err)
			}

			var allResults []json.RawMessage
			if resourceType != "" {
				results, err := db.List(resourceType, 0)
				if err != nil {
					return fmt.Errorf("listing %s: %w", resourceType, err)
				}
				allResults = results
			} else {
				for rt := range status {
					results, err := db.List(rt, 0)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", rt, err)
						continue
					}
					allResults = append(allResults, results...)
				}
			}

			if len(allResults) == 0 {
				if flags.quiet {
					return nil
				}
				fmt.Fprintln(os.Stderr, "No data to export. Run 'synology-nas-api-pp-cli sync' first.")
				return nil
			}

			data, _ := json.Marshal(allResults)
			if format == "csv" && !flags.asJSON {
				flags.csv = true
			}
			return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(data), flags)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/synology-nas-api-pp-cli/data.db)")
	cmd.Flags().StringVar(&resourceType, "resource", "", "Export only this resource type")
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json or csv")

	return cmd
}
