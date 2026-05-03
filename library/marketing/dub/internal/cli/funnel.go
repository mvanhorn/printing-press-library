// Hand-written novel feature: funnel.
// Click-to-lead-to-sale conversion rates per link or campaign.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type funnelRow struct {
	LinkID      string  `json:"linkId"`
	Domain      string  `json:"domain"`
	Key         string  `json:"key"`
	Clicks      float64 `json:"clicks"`
	Leads       float64 `json:"leads"`
	Sales       float64 `json:"sales"`
	ClickToLead float64 `json:"clickToLeadPct"`
	LeadToSale  float64 `json:"leadToSalePct"`
	ClickToSale float64 `json:"clickToSalePct"`
}

func newFunnelCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var linkKey string
	var minClicks int

	cmd := &cobra.Command{
		Use:   "funnel",
		Short: "Click-to-lead-to-sale conversion rates per link or campaign",
		Long: `Computes conversion ratios for each link in the local store:

  click → lead conversion %
  lead → sale conversion %
  click → sale conversion %

Aggregated locally from analytics counters; the API does not surface the full
funnel for an arbitrary set of links in one call.`,
		Example: strings.Trim(`
  # Top of the funnel: every link, ranked by click-to-sale conversion
  dub-pp-cli funnel --json

  # Single link
  dub-pp-cli funnel --link mylink --json

  # Filter out low-volume noise
  dub-pp-cli funnel --min-clicks 100 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			query := `SELECT id, COALESCE(domain,''), COALESCE("key",''), COALESCE(data,'{}') FROM links`
			args2 := []any{}
			if linkKey != "" {
				query += ` WHERE "key" = ?`
				args2 = append(args2, linkKey)
			}
			rows, err := db.DB().QueryContext(cmd.Context(), query, args2...)
			if err != nil {
				return fmt.Errorf("querying links: %w", err)
			}
			defer rows.Close()
			var results []funnelRow
			seen := 0
			for rows.Next() {
				var id, domain, key string
				var blob sql.NullString
				if err := rows.Scan(&id, &domain, &key, &blob); err != nil {
					return err
				}
				seen++
				blobBytes := []byte(blob.String)
				if !blob.Valid {
					blobBytes = []byte("{}")
				}
				clicks := extractNumber(blobBytes, "clicks")
				leads := extractNumber(blobBytes, "leads")
				sales := extractNumber(blobBytes, "sales")
				if clicks < float64(minClicks) {
					continue
				}
				row := funnelRow{
					LinkID: id, Domain: domain, Key: key,
					Clicks: clicks, Leads: leads, Sales: sales,
				}
				if clicks > 0 {
					row.ClickToLead = leads / clicks * 100
					row.ClickToSale = sales / clicks * 100
				}
				if leads > 0 {
					row.LeadToSale = sales / leads * 100
				}
				results = append(results, row)
			}
			if seen == 0 {
				return hintEmptyStore("links")
			}
			// Sort by click-to-sale descending.
			for i := 0; i < len(results); i++ {
				for j := i + 1; j < len(results); j++ {
					if results[j].ClickToSale > results[i].ClickToSale {
						results[i], results[j] = results[j], results[i]
					}
				}
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No links matched the filter.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %8s %8s %8s %8s %8s %8s\n",
				"KEY", "CLICKS", "LEADS", "SALES", "C→L%", "L→S%", "C→S%")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %8.0f %8.0f %8.0f %8.1f %8.1f %8.1f\n",
					truncate(r.Domain+"/"+r.Key, 30), r.Clicks, r.Leads, r.Sales,
					r.ClickToLead, r.LeadToSale, r.ClickToSale)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&linkKey, "link", "", "Filter to a single link by short-key slug")
	cmd.Flags().IntVar(&minClicks, "min-clicks", 0, "Skip links below this click count")
	return cmd
}
