// Hand-written novel feature: links rollup.
// Performance dashboard aggregated by tag or folder — clicks, leads, sales
// rolled up across every link wearing each label.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type rollupRow struct {
	Group  string  `json:"group"`
	Count  int     `json:"linkCount"`
	Clicks float64 `json:"clicks"`
	Leads  float64 `json:"leads,omitempty"`
	Sales  float64 `json:"sales,omitempty"`
}

func newLinksRollupCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var groupBy string
	var byMetric string

	cmd := &cobra.Command{
		Use:   "rollup",
		Short: "Performance dashboard aggregated by tag or folder",
		Long: `Aggregate clicks, leads, and sales across every link wearing each tag,
folder, or domain label. The local store joins links × analytics aggregates
for arbitrary slice-and-dice the API doesn't expose together.`,
		Example: strings.Trim(`
  # Roll up clicks by tag
  dub-pp-cli links rollup --group-by tag --json

  # Roll up by folder
  dub-pp-cli links rollup --group-by folder --json

  # Roll up by domain (default)
  dub-pp-cli links rollup --json
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

			var groupCol string
			switch strings.ToLower(groupBy) {
			case "tag", "tags":
				groupCol = "tag_names"
			case "folder", "folders":
				groupCol = "folder_id"
			case "domain", "domains", "":
				groupCol = "domain"
			default:
				return usageErr(fmt.Errorf("--group-by must be one of: tag, folder, domain"))
			}
			_ = byMetric

			rows, err := db.DB().QueryContext(cmd.Context(), fmt.Sprintf(`
				SELECT COALESCE(%s,''), COALESCE(data,'{}')
				FROM links
			`, groupCol))
			if err != nil {
				return fmt.Errorf("querying links: %w", err)
			}
			defer rows.Close()

			agg := map[string]*rollupRow{}
			seen := 0
			for rows.Next() {
				var groupVal string
				var blob sql.NullString
				if err := rows.Scan(&groupVal, &blob); err != nil {
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

				keys := []string{groupVal}
				if groupCol == "tag_names" && groupVal != "" {
					keys = strings.Split(groupVal, ",")
				}
				if groupVal == "" {
					keys = []string{"(none)"}
				}
				for _, k := range keys {
					k = strings.TrimSpace(k)
					if k == "" {
						k = "(none)"
					}
					r, ok := agg[k]
					if !ok {
						r = &rollupRow{Group: k}
						agg[k] = r
					}
					r.Count++
					r.Clicks += clicks
					r.Leads += leads
					r.Sales += sales
				}
			}
			if seen == 0 {
				return hintEmptyStore("links")
			}

			var results []rollupRow
			for _, r := range agg {
				results = append(results, *r)
			}
			// Sort by chosen metric descending.
			less := func(a, b rollupRow) bool {
				switch strings.ToLower(byMetric) {
				case "leads":
					return a.Leads > b.Leads
				case "sales":
					return a.Sales > b.Sales
				default:
					return a.Clicks > b.Clicks
				}
			}
			for i := 0; i < len(results); i++ {
				for j := i + 1; j < len(results); j++ {
					if less(results[j], results[i]) {
						results[i], results[j] = results[j], results[i]
					}
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No data to roll up.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-8s %-10s %-10s %-10s\n", strings.ToUpper(groupBy), "LINKS", "CLICKS", "LEADS", "SALES")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-8d %-10.0f %-10.0f %-10.0f\n",
					truncate(r.Group, 30), r.Count, r.Clicks, r.Leads, r.Sales)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&groupBy, "group-by", "domain", "Roll up by tag, folder, or domain")
	cmd.Flags().StringVar(&byMetric, "by", "clicks", "Sort metric: clicks, leads, or sales")
	return cmd
}
