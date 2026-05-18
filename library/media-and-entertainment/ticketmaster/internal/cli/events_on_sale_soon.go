// PATCH(novel-feature): events on-sale-soon — local query for events whose
// public on-sale falls in the next N days. Hand-authored.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketmaster/internal/store"

	"github.com/spf13/cobra"
)

func newEventsOnSaleSoonCmd(flags *rootFlags) *cobra.Command {
	var window int
	var classification string
	var dmaID string

	cmd := &cobra.Command{
		Use:   "on-sale-soon",
		Short: "Events whose public on-sale falls in the next N days, sorted ascending",
		Long: strings.TrimSpace(`
The presale watch view — filters the local events table by
sales.public.startDateTime within --window days. Optionally narrow by
classification (segment/genre) or DMA.

Run 'sync --resource events' first to populate the local store.
`),
		Example: strings.Trim(`
  ticketmaster-pp-cli events on-sale-soon --window 14 --json
  ticketmaster-pp-cli events on-sale-soon --window 7 --classification "Music"
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

			conds := []string{
				`json_extract(data, '$.sales.public.startDateTime') IS NOT NULL`,
				`datetime(json_extract(data, '$.sales.public.startDateTime')) BETWEEN datetime('now') AND datetime('now', ?)`,
			}
			argv := []any{fmt.Sprintf("+%d days", window)}
			if classification != "" {
				conds = append(conds,
					`(json_extract(data, '$.classifications[0].segment.name') = ?
					  OR json_extract(data, '$.classifications[0].genre.name') = ?)`)
				argv = append(argv, classification, classification)
			}
			if dmaID != "" {
				conds = append(conds,
					`EXISTS (SELECT 1 FROM json_each(json_extract(data, '$._embedded.venues[0].dmas')) AS d
					         WHERE CAST(json_extract(d.value, '$.id') AS TEXT) = ?)`)
				argv = append(argv, dmaID)
			}
			q := `SELECT
			        json_extract(data, '$.id'),
			        json_extract(data, '$.name'),
			        json_extract(data, '$._embedded.venues[0].name'),
			        json_extract(data, '$._embedded.venues[0].city.name'),
			        json_extract(data, '$.classifications[0].segment.name'),
			        json_extract(data, '$.dates.start.localDate'),
			        json_extract(data, '$.sales.public.startDateTime')
			      FROM events
			      WHERE ` + strings.Join(conds, " AND ") + `
			      ORDER BY json_extract(data, '$.sales.public.startDateTime') ASC`

			rows, err := db.DB().QueryContext(cmd.Context(), q, argv...)
			if err != nil {
				return fmt.Errorf("on-sale-soon query: %w", err)
			}
			defer rows.Close()

			type row struct {
				ID         string `json:"id"`
				Name       string `json:"name"`
				Venue      string `json:"venue,omitempty"`
				City       string `json:"city,omitempty"`
				Segment    string `json:"segment,omitempty"`
				Date       string `json:"date,omitempty"`
				OnSaleAt   string `json:"on_sale_at"`
			}
			var out []row
			for rows.Next() {
				var r row
				var id, name, venue, city, segment, date, onsale sqlNullString
				if err := rows.Scan(&id, &name, &venue, &city, &segment, &date, &onsale); err != nil {
					return err
				}
				r.ID, r.Name, r.Venue, r.City, r.Segment, r.Date, r.OnSaleAt =
					id.String, name.String, venue.String, city.String, segment.String, date.String, onsale.String
				out = append(out, r)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No events going on-sale in window.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ON-SALE\tEVENT-DATE\tEVENT\tVENUE\tCITY\tSEGMENT")
			for _, r := range out {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
					r.OnSaleAt, r.Date, truncate(r.Name, 32), truncate(r.Venue, 22),
					truncate(r.City, 14), truncate(r.Segment, 14))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&window, "window", 7, "Days from now to scan for upcoming on-sales")
	cmd.Flags().StringVar(&classification, "classification", "", "Filter by segment or genre name (e.g. Music, Rock)")
	cmd.Flags().StringVar(&dmaID, "dma", "", "Restrict to a single DMA ID")
	return cmd
}
