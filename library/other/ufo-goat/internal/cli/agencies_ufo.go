// Custom agencies command that queries the local SQLite store.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ufo-goat/internal/store"

	"github.com/spf13/cobra"
)

func newUFOAgenciesCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "agencies",
		Short: "List all contributing agencies with file counts and type breakdown",
		Long: `Show all agencies that contributed declassified UAP files, with counts
by file type (PDF, VID, IMG) and date range coverage.`,
		Example: `  # Show agency summary
  ufo-goat-pp-cli agencies

  # JSON output
  ufo-goat-pp-cli agencies --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("ufo-goat-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'ufo-goat-pp-cli sync' first.", err)
			}
			defer db.Close()
			_ = db.EnsureUFOSchema()

			count, _ := db.GetFileCount()
			if count == 0 {
				return fmt.Errorf("no files in local store. Run 'ufo-goat-pp-cli sync' first")
			}

			agencies, err := db.GetAgencySummary()
			if err != nil {
				return err
			}

			if len(agencies) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No agency data found.")
				return nil
			}

			// JSON output
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(agencies)
				filtered := json.RawMessage(data)
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				}
				return printOutput(cmd.OutOrStdout(), filtered, true)
			}

			// CSV output
			if flags.csv {
				data, _ := json.Marshal(agencies)
				return printCSV(cmd.OutOrStdout(), json.RawMessage(data))
			}

			// Table output
			w := cmd.OutOrStdout()
			tw := newTabWriter(w)
			fmt.Fprintln(tw, strings.Join([]string{
				bold("AGENCY"), bold("FILES"), bold("PDFs"), bold("VIDEOS"), bold("IMAGES"), bold("DATE RANGE"),
			}, "\t"))

			for _, a := range agencies {
				dateRange := ""
				if dr, ok := a["date_range"].(string); ok {
					dateRange = dr
				}
				if dateRange == "" {
					dateRange = "-"
				}
				fmt.Fprintf(tw, "%s\t%v\t%v\t%v\t%v\t%s\n",
					a["agency"],
					a["count"],
					a["pdfs"],
					a["videos"],
					a["images"],
					truncate(dateRange, 25),
				)
			}
			tw.Flush()

			fmt.Fprintf(w, "\nTotal: %d files across %d agencies\n", count, len(agencies))
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// keep store import
var _ *store.Store
