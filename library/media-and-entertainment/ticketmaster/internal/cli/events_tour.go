// PATCH(novel-feature): events tour — for a given attraction, return every
// upcoming event sorted by date with on-sale flag derived from
// dates.status.code + sales.public.startDateTime. Hand-authored.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketmaster/internal/store"

	"github.com/spf13/cobra"
)

func newEventsTourCmd(flags *rootFlags) *cobra.Command {
	var onSaleWindow int

	cmd := &cobra.Command{
		Use:   "tour <attraction-id-or-name>",
		Short: "Every upcoming event for an attraction, sorted by date with on-sale-soon flag",
		Long: strings.TrimSpace(`
Filter the local events table by attraction ID OR name (case-insensitive
substring match against $._embedded.attractions[*].name), sort by start date,
and project city/venue/status/on-sale fields. Events whose public on-sale
falls within --on-sale-window days are flagged "soon".

Run 'sync --resource events' first to populate the local store.
`),
		Example: strings.Trim(`
  ticketmaster-pp-cli events tour KovZ917Ahkk --on-sale-window 7
  ticketmaster-pp-cli events tour "Florence + The Machine" --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			needle := strings.TrimSpace(args[0])
			dbPath := defaultDBPath("ticketmaster-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			q := `WITH e AS (
				SELECT id,
				       json_extract(data, '$.name') AS name,
				       json_extract(data, '$._embedded.venues[0].name') AS venue_name,
				       json_extract(data, '$._embedded.venues[0].city.name') AS city,
				       json_extract(data, '$._embedded.venues[0].state.stateCode') AS state,
				       json_extract(data, '$.dates.start.localDate') AS local_date,
				       json_extract(data, '$.dates.start.dateTime') AS date_time,
				       json_extract(data, '$.dates.status.code') AS status_code,
				       json_extract(data, '$.sales.public.startDateTime') AS on_sale_at,
				       data
				FROM events
				WHERE EXISTS (
				  SELECT 1 FROM json_each(json_extract(data, '$._embedded.attractions')) AS a
				  WHERE json_extract(a.value, '$.id') = ?
				     OR LOWER(json_extract(a.value, '$.name')) LIKE LOWER(?)
				)
			)
			SELECT id, name, venue_name, city, state, local_date, date_time, status_code, on_sale_at
			FROM e
			WHERE local_date IS NULL OR date(local_date) >= date('now')
			ORDER BY local_date ASC NULLS LAST`

			rows, err := db.DB().QueryContext(cmd.Context(), q, needle, "%"+needle+"%")
			if err != nil {
				return err
			}
			defer rows.Close()

			type tourLeg struct {
				ID         string `json:"id"`
				Name       string `json:"name"`
				Venue      string `json:"venue"`
				City       string `json:"city,omitempty"`
				State      string `json:"state,omitempty"`
				Date       string `json:"date,omitempty"`
				DateTime   string `json:"date_time,omitempty"`
				StatusCode string `json:"status_code,omitempty"`
				OnSaleAt   string `json:"on_sale_at,omitempty"`
				OnSaleSoon bool   `json:"on_sale_soon"`
			}

			windowEnd := time.Now().Add(time.Duration(onSaleWindow) * 24 * time.Hour).UTC()
			var out []tourLeg
			for rows.Next() {
				var t tourLeg
				var id, name, venue, city, state, date, datetime, status, onsale sqlNullString
				if err := rows.Scan(&id, &name, &venue, &city, &state, &date, &datetime, &status, &onsale); err != nil {
					return err
				}
				t.ID = id.String
				t.Name = name.String
				t.Venue = venue.String
				t.City = city.String
				t.State = state.String
				t.Date = date.String
				t.DateTime = datetime.String
				t.StatusCode = status.String
				t.OnSaleAt = onsale.String
				if t.OnSaleAt != "" {
					if osAt, err := time.Parse(time.RFC3339, t.OnSaleAt); err == nil {
						if osAt.After(time.Now()) && osAt.Before(windowEnd) {
							t.OnSaleSoon = true
						}
					}
				}
				out = append(out, t)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No tour stops for %q. Run 'sync --resource events' to populate.\n", needle)
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DATE\tCITY\tVENUE\tEVENT\tSTATUS\tON-SALE")
			for _, t := range out {
				flag := ""
				if t.OnSaleSoon {
					flag = "SOON " + t.OnSaleAt
				} else if t.OnSaleAt != "" {
					flag = t.OnSaleAt
				}
				cityState := t.City
				if t.State != "" {
					cityState = t.City + ", " + t.State
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
					t.Date, truncate(cityState, 18), truncate(t.Venue, 24),
					truncate(t.Name, 28), t.StatusCode, flag)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&onSaleWindow, "on-sale-window", 7, "Days from now considered 'on-sale soon'")
	return cmd
}
