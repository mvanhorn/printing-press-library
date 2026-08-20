// PATCH(novel-feature): events by-classification — local aggregation grouping
// events by segment+genre with counts and example events per bucket.
// Hand-authored.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketmaster/internal/store"

	"github.com/spf13/cobra"
)

func newEventsByClassificationCmd(flags *rootFlags) *cobra.Command {
	var window int
	var dmaID string
	var examples int

	cmd := &cobra.Command{
		Use:   "by-classification",
		Short: "Group local events by segment+genre with counts and example events per bucket",
		Long: strings.TrimSpace(`
Local join of events × classifications grouped by segment and genre,
returning event count + N example events per leaf. The bucketed view
local-scene trackers and newsletter authors reach for.

Run 'sync --resource events' first to populate the local store.
`),
		Example: strings.Trim(`
  ticketmaster-pp-cli events by-classification --window 60 --json
  ticketmaster-pp-cli events by-classification --dma 383 --window 14 --examples 5
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dbPath := defaultDBPath("ticketmaster-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			conds := []string{`json_extract(data, '$.dates.start.localDate') IS NOT NULL`}
			argv := []any{}
			if window > 0 {
				conds = append(conds, `date(json_extract(data, '$.dates.start.localDate')) BETWEEN date('now') AND date('now', ?)`)
				argv = append(argv, fmt.Sprintf("+%d days", window))
			}
			if dmaID != "" {
				conds = append(conds,
					`EXISTS (SELECT 1 FROM json_each(json_extract(data, '$._embedded.venues[0].dmas')) AS d
					         WHERE CAST(json_extract(d.value, '$.id') AS TEXT) = ?)`)
				argv = append(argv, dmaID)
			}
			where := strings.Join(conds, " AND ")

			q := `WITH e AS (
				SELECT
					COALESCE(json_extract(data, '$.classifications[0].segment.name'), 'Unknown') AS segment,
					COALESCE(json_extract(data, '$.classifications[0].genre.name'), 'Unknown') AS genre,
					json_extract(data, '$.name') AS name,
					json_extract(data, '$._embedded.venues[0].name') AS venue,
					json_extract(data, '$.dates.start.localDate') AS local_date
				FROM events WHERE ` + where + `
			)
			SELECT segment, genre, COUNT(*) AS n,
			       GROUP_CONCAT(name || ' @ ' || venue || ' (' || local_date || ')', X'1F') AS samples
			FROM (SELECT segment, genre, name, venue, local_date FROM e ORDER BY local_date) g
			GROUP BY segment, genre
			ORDER BY n DESC, segment, genre`

			rows, err := db.DB().QueryContext(cmd.Context(), q, argv...)
			if err != nil {
				return fmt.Errorf("by-classification query: %w", err)
			}
			defer rows.Close()

			type bucket struct {
				Segment  string   `json:"segment"`
				Genre    string   `json:"genre"`
				Count    int      `json:"count"`
				Examples []string `json:"examples,omitempty"`
			}
			var out []bucket
			for rows.Next() {
				var b bucket
				var samples sqlNullString
				if err := rows.Scan(&b.Segment, &b.Genre, &b.Count, &samples); err != nil {
					return err
				}
				if samples.Valid {
					parts := strings.Split(samples.String, "\x1F")
					if examples > 0 && len(parts) > examples {
						parts = parts[:examples]
					}
					b.Examples = parts
				}
				out = append(out, b)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No classified events in window.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "COUNT\tSEGMENT\tGENRE\tEXAMPLE")
			for _, b := range out {
				ex := ""
				if len(b.Examples) > 0 {
					ex = truncate(b.Examples[0], 56)
				}
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", b.Count, b.Segment, b.Genre, ex)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&window, "window", 60, "Window in days from today (0 = no window filter)")
	cmd.Flags().StringVar(&dmaID, "dma", "", "Restrict to a single DMA ID")
	cmd.Flags().IntVar(&examples, "examples", 3, "Max example events per bucket (use 0 to include all)")
	return cmd
}
