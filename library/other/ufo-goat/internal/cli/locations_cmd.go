// Locations command — aggregate incidents by geographic location.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ufo-goat/internal/store"

	"github.com/spf13/cobra"
)

func newLocationsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "locations",
		Short: "Aggregate incidents by geographic location",
		Long: `Show all unique incident locations with file counts, contributing agencies,
and date ranges. Useful for mapping and spatial analysis.`,
		Example: `  # Show all locations
  ufo-goat-pp-cli locations

  # JSON output
  ufo-goat-pp-cli locations --json`,
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

			locations, err := db.GetLocations()
			if err != nil {
				return err
			}

			if len(locations) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No location data found.")
				return nil
			}

			// JSON output
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(locations)
				filtered := json.RawMessage(data)
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				}
				return printOutput(cmd.OutOrStdout(), filtered, true)
			}

			// CSV output
			if flags.csv {
				data, _ := json.Marshal(locations)
				return printCSV(cmd.OutOrStdout(), json.RawMessage(data))
			}

			// Table output
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%d unique incident locations\n\n", len(locations))

			tw := newTabWriter(w)
			fmt.Fprintln(tw, strings.Join([]string{
				bold("LOCATION"), bold("COUNT"), bold("AGENCIES"), bold("DATE RANGE"),
			}, "\t"))

			for _, loc := range locations {
				agencies := strings.Join(loc.Agencies, ", ")
				dateRange := loc.DateRange
				if dateRange == "" {
					dateRange = "-"
				}
				fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n",
					truncate(loc.Location, 30),
					loc.Count,
					truncate(agencies, 20),
					truncate(dateRange, 25),
				)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// Ensure store is imported
var _ = store.LocationSummary{}
