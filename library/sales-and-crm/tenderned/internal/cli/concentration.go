package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newConcentrationCmd(flags *rootFlags) *cobra.Command {
	var dbPath, cpv, nuts, since string
	var topN int

	cmd := &cobra.Command{
		Use:   "concentration",
		Short: "Buyer-concentration HHI for a CPV/NUTS slice (mirrors eu-tenders concentration)",
		Long: `Herfindahl-Hirschman Index of buyer concentration for a slice of the Dutch
procurement market. HHI < 1500 = competitive, 1500-2500 = moderate, > 2500 = concentrated.

Operates on the local SQLite snapshot. Run 'tenderned-pp-cli sync' first.`,
		Example: "  tenderned-pp-cli concentration --cpv 72000000-5 --since 2024-01-01",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := tnOpenStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			notices, err := tnLoadNotices(cmd.Context(), s)
			if err != nil {
				return err
			}
			cpvList := tnSplitCSV(cpv)
			counts := map[string]int{}
			total := 0
			for _, n := range notices {
				if !tnHasCPV(n, cpvList) {
					continue
				}
				if nuts != "" {
					matched := false
					for _, nc := range n.NUTSCodes {
						if nc.Code == nuts {
							matched = true
							break
						}
					}
					if !matched {
						continue
					}
				}
				if since != "" && n.PublicatieDatum < since {
					continue
				}
				if n.OpdrachtgeverNaam == "" {
					continue
				}
				counts[n.OpdrachtgeverNaam]++
				total++
			}
			var hhi float64
			type bs struct {
				Buyer string
				Count int
				Share float64
			}
			rows := make([]bs, 0, len(counts))
			for k, c := range counts {
				share := float64(c) / float64(total) * 100.0
				hhi += share * share
				rows = append(rows, bs{Buyer: k, Count: c, Share: share})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Count > rows[j].Count })
			if topN > 0 && len(rows) > topN {
				rows = rows[:topN]
			}
			bracket := "competitive (<1500)"
			if total == 0 || len(counts) == 0 {
				bracket = "no data"
			} else if hhi >= 2500 {
				bracket = "concentrated (>=2500)"
			} else if hhi >= 1500 {
				bracket = "moderate (1500-2500)"
			}
			result := map[string]any{
				"hhi":            hhi,
				"interpretation": bracket,
				"total_notices":  total,
				"unique_buyers":  len(counts),
				"top_buyers":     rows,
				"filters": map[string]any{
					"cpv": cpvList, "nuts": nuts, "since": since,
				},
			}
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "HHI %.0f (%s) — %d notices across %d buyers\n", hhi, bracket, total, len(counts))
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "  %5.2f%% (%4d) | %s\n", r.Share, r.Count, r.Buyer)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&cpv, "cpv", "", "Comma-separated CPV stripes")
	cmd.Flags().StringVar(&nuts, "nuts", "", "NUTS region code")
	cmd.Flags().StringVar(&since, "since", "", "Earliest publication date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&topN, "top", 10, "Top-N buyers to surface in output")
	return cmd
}
