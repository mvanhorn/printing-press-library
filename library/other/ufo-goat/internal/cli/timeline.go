// Timeline command — chronological incident timeline.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ufo-goat/internal/store"

	"github.com/spf13/cobra"
)

func newTimelineCmd(flags *rootFlags) *cobra.Command {
	var flagAfter string
	var flagBefore string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "timeline",
		Short: "Show chronological incident timeline spanning 1944-2025",
		Long: `Display UAP incidents in chronological order, grouped by decade.
Filter by date range with --after and --before.`,
		Example: `  # Show full timeline
  ufo-goat-pp-cli timeline

  # Show incidents from the 1940s
  ufo-goat-pp-cli timeline --after 1940-01-01 --before 1949-12-31

  # JSON output
  ufo-goat-pp-cli timeline --json`,
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

			files, err := db.GetTimeline(flagAfter, flagBefore)
			if err != nil {
				return err
			}

			if len(files) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No incidents with parseable dates found.")
				return nil
			}

			// JSON output
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(files)
				filtered := json.RawMessage(data)
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				return printOutput(cmd.OutOrStdout(), filtered, true)
			}

			// Group by decade for human output
			type decadeGroup struct {
				decade string
				files  []store.UFOFile
			}
			var groups []decadeGroup
			currentDecade := ""

			for _, f := range files {
				decade := "Unknown"
				if len(f.ParsedDate) >= 4 {
					year := f.ParsedDate[:3]
					decade = year + "0s"
				}
				if decade != currentDecade {
					groups = append(groups, decadeGroup{decade: decade})
					currentDecade = decade
				}
				groups[len(groups)-1].files = append(groups[len(groups)-1].files, f)
			}

			w := cmd.OutOrStdout()
			for _, g := range groups {
				fmt.Fprintf(w, "\n%s (%d incidents)\n", bold(g.decade), len(g.files))
				fmt.Fprintln(w, strings.Repeat("─", 60))

				tw := newTabWriter(w)
				for _, f := range g.files {
					loc := f.IncidentLocation
					if loc == "" {
						loc = "-"
					}
					fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n",
						f.ParsedDate,
						f.Agency,
						truncate(f.Title, 40),
						truncate(loc, 25),
					)
				}
				tw.Flush()
			}

			fmt.Fprintf(w, "\n%d incidents with dated records\n", len(files))
			return nil
		},
	}

	cmd.Flags().StringVar(&flagAfter, "after", "", "Show incidents after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagBefore, "before", "", "Show incidents before this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
