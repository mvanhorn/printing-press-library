// PATCH(novel-feature): events residency — collapse runs of same-name +
// same-venue events into one row per residency with first/last/count.
// Hand-authored on top of the generator output.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketmaster/internal/store"

	"github.com/spf13/cobra"
)

func newEventsResidencyCmd(flags *rootFlags) *cobra.Command {
	var window int
	var venueID string
	var minNights int

	cmd := &cobra.Command{
		Use:   "residency",
		Short: "Collapse runs of same-name + same-venue events into one row per residency",
		Long: strings.TrimSpace(`
Group local events by (name, venue) and report each residency as a single
row with first_date, last_date, night_count, and the list of event IDs. The
canonical Broadway / opera / comedy residency view — a 16-night opera
season shows as one entry, not 16.

Run 'sync --resource events' first to populate the local store.
`),
		Example: strings.Trim(`
  ticketmaster-pp-cli events residency --window 60 --json
  ticketmaster-pp-cli events residency --venue-id KovZpZAFkvEA --min-nights 3
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

			whereParts := []string{
				`json_extract(data, '$.dates.start.localDate') IS NOT NULL`,
			}
			argv := []any{}
			if window > 0 {
				whereParts = append(whereParts,
					`date(json_extract(data, '$.dates.start.localDate')) BETWEEN date('now') AND date('now', ?)`)
				argv = append(argv, fmt.Sprintf("+%d days", window))
			}
			if venueID != "" {
				whereParts = append(whereParts,
					`EXISTS (SELECT 1 FROM json_each(json_extract(data, '$._embedded.venues')) AS v
					         WHERE json_extract(v.value, '$.id') = ?)`)
				argv = append(argv, venueID)
			}
			where := strings.Join(whereParts, " AND ")

			q := `WITH e AS (
				SELECT id,
				       json_extract(data, '$.name') AS name,
				       json_extract(data, '$._embedded.venues[0].id') AS venue_id,
				       json_extract(data, '$._embedded.venues[0].name') AS venue_name,
				       json_extract(data, '$._embedded.venues[0].city.name') AS city,
				       json_extract(data, '$.classifications[0].segment.name') AS segment,
				       json_extract(data, '$.classifications[0].genre.name') AS genre,
				       json_extract(data, '$.dates.start.localDate') AS local_date,
				       data
				FROM events
				WHERE ` + where + `
			)
			SELECT name, venue_id, venue_name, city, segment, genre,
			       MIN(local_date) AS first_date,
			       MAX(local_date) AS last_date,
			       COUNT(*) AS night_count,
			       GROUP_CONCAT(id, ',') AS ids
			FROM e
			GROUP BY name, venue_id
			HAVING night_count >= ?
			ORDER BY first_date ASC NULLS LAST, name`

			argv = append(argv, minNights)
			rows, err := db.DB().QueryContext(cmd.Context(), q, argv...)
			if err != nil {
				return fmt.Errorf("residency query: %w", err)
			}
			defer rows.Close()

			type residency struct {
				Name       string   `json:"name"`
				VenueID    string   `json:"venue_id"`
				VenueName  string   `json:"venue_name"`
				City       string   `json:"city,omitempty"`
				Segment    string   `json:"segment,omitempty"`
				Genre      string   `json:"genre,omitempty"`
				FirstDate  string   `json:"first_date"`
				LastDate   string   `json:"last_date"`
				NightCount int      `json:"night_count"`
				IDs        []string `json:"ids"`
			}
			var out []residency
			for rows.Next() {
				var r residency
				var ids string
				var name, venueName, city, segment, genre, venueID sqlNullString
				var firstDate, lastDate sqlNullString
				if err := rows.Scan(&name, &venueID, &venueName, &city, &segment, &genre,
					&firstDate, &lastDate, &r.NightCount, &ids); err != nil {
					return err
				}
				r.Name = name.String
				r.VenueID = venueID.String
				r.VenueName = venueName.String
				r.City = city.String
				r.Segment = segment.String
				r.Genre = genre.String
				r.FirstDate = firstDate.String
				r.LastDate = lastDate.String
				r.IDs = splitCSV(ids)
				out = append(out, r)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No residencies in window. Try a larger --window or run 'sync --resource events'.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "FIRST\tLAST\tNIGHTS\tEVENT\tVENUE\tCITY")
			for _, r := range out {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\t%s\n",
					r.FirstDate, r.LastDate, r.NightCount,
					truncate(r.Name, 36), truncate(r.VenueName, 24), truncate(r.City, 14))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&window, "window", 60, "Window in days from today (0 = no window filter)")
	cmd.Flags().StringVar(&venueID, "venue-id", "", "Restrict to a single venue ID")
	cmd.Flags().IntVar(&minNights, "min-nights", 2, "Minimum night count to qualify as a residency")
	return cmd
}

// sqlNullString avoids depending on database/sql.NullString here.
type sqlNullString struct {
	String string
	Valid  bool
}

func (n *sqlNullString) Scan(v any) error {
	if v == nil {
		n.String, n.Valid = "", false
		return nil
	}
	switch t := v.(type) {
	case string:
		n.String = t
	case []byte:
		n.String = string(t)
	default:
		n.String = fmt.Sprint(t)
	}
	n.Valid = true
	return nil
}

var _ = json.Marshal // keep import for residency JSON path consistency
