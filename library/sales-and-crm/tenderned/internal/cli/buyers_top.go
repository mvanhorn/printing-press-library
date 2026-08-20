package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newBuyersTopCmd(flags *rootFlags) *cobra.Command {
	var dbPath, cpv, nuts, since string
	var topN int

	cmd := &cobra.Command{
		Use:   "top",
		Short: "Top-N contracting authorities by notice count in a CPV/NUTS/date slice",
		Long: `Pure local-SQLite GROUP BY aggregation — TenderNed exposes no aggregation
endpoint. Answers questions like "which buyers are most active in CPV 72 in NUTS NL33 since 2025-01-01".`,
		Example: "  tenderned-pp-cli buyers top --cpv 72000000-5 --nuts NL33 --since 2025-01-01",
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
			for _, n := range notices {
				if !tnHasCPV(n, cpvList) {
					continue
				}
				if nuts != "" {
					ok := false
					for _, nc := range n.NUTSCodes {
						if nc.Code == nuts {
							ok = true
							break
						}
					}
					if !ok {
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
			}
			type bc struct {
				Buyer string `json:"buyer"`
				Count int    `json:"count"`
			}
			list := make([]bc, 0, len(counts))
			for k, c := range counts {
				list = append(list, bc{k, c})
			}
			sort.Slice(list, func(i, j int) bool { return list[i].Count > list[j].Count })
			if topN > 0 && len(list) > topN {
				list = list[:topN]
			}
			result := map[string]any{
				"top": list,
				"filters": map[string]any{
					"cpv": cpvList, "nuts": nuts, "since": since,
				},
			}
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			for _, b := range list {
				fmt.Fprintf(cmd.OutOrStdout(), "  %4d  %s\n", b.Count, b.Buyer)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&cpv, "cpv", "", "Comma-separated CPV stripes")
	cmd.Flags().StringVar(&nuts, "nuts", "", "NUTS region code")
	cmd.Flags().StringVar(&since, "since", "", "Earliest publication date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&topN, "top", 20, "Top-N buyers to surface")
	return cmd
}
