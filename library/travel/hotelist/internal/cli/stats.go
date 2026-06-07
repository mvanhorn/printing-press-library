// Hand-authored `stats` command: summarize what's in the local Hotelist mirror.
// Not generated.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Summarize the local Hotelist mirror (cached cities, hotels, snapshots)",
		Long: "Report what is currently stored in the local SQLite mirror: how many cities are in the " +
			"reference table, how many hotels have been cached from prior queries, and how many watch " +
			"snapshots exist. Hotelist's full live dataset is ~84,000 hotels across ~6,000 cities; this " +
			"command reports only what you have fetched locally. Data is scraped from hotelist.com " +
			"(community/AI-rated, not an official API).",
		Example:     "  hotelist-pp-cli stats --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openHotelStore(cmd.Context(), flags)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			cities, _ := db.Count("city")
			hotels, _ := db.Count("hotel")
			snapshots := countSnapshots(db)
			watches := countWatchScopes(db)

			out := cmd.OutOrStdout()
			view := map[string]any{
				"source":            hotelistSource,
				"disclaimer":        hotelistDisclaimer,
				"cached_cities":     cities,
				"cached_hotels":     hotels,
				"watch_snapshots":   snapshots,
				"watch_scopes":      watches,
				"live_dataset_note": "Hotelist's live dataset is ~84,000 hotels across ~6,000 cities and 153 countries; this CLI mirrors only what you fetch.",
			}
			if !wantsHumanTable(out, flags) {
				return printJSONFiltered(out, view, flags)
			}
			fmt.Fprintln(out, "Local Hotelist mirror")
			fmt.Fprintf(out, "  cached cities:   %d\n", cities)
			fmt.Fprintf(out, "  cached hotels:   %d\n", hotels)
			fmt.Fprintf(out, "  watch scopes:    %d\n", watches)
			fmt.Fprintf(out, "  watch snapshots: %d\n", snapshots)
			fmt.Fprintf(out, "\n%s\n", hotelistDisclaimer)
			return nil
		},
	}
	return cmd
}
