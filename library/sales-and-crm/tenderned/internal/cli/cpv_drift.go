package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newCPVDriftCmd(flags *rootFlags) *cobra.Command {
	var dbPath, buyer, nuts string
	var years, topN int

	cmd := &cobra.Command{
		Use:   "cpv-drift",
		Short: "Year-over-year CPV mix for a buyer or NUTS region (mirrors eu-tenders cpv-drift)",
		Long: `Year-bucketed top-CPV (2-digit prefix) distribution for one buyer or NUTS
region; surfaces whether the buyer's purchasing mix is shifting.

Operates on the local SQLite snapshot. Run 'tenderned-pp-cli sync' first.`,
		Example: "  tenderned-pp-cli cpv-drift --buyer \"Gemeente Amsterdam\" --years 3",
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
			perYear := map[int]map[string]int{}
			needleBuyer := strings.ToLower(strings.TrimSpace(buyer))
			for _, n := range notices {
				if needleBuyer != "" && !strings.Contains(strings.ToLower(n.OpdrachtgeverNaam), needleBuyer) {
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
				y := tnYear(n.PublicatieDatum)
				if y == 0 {
					continue
				}
				for _, c := range n.CPVCodes {
					if !c.IsHoofdOpdracht && len(n.CPVCodes) > 1 {
						continue // count primary CPV only when multiple are present
					}
					code := strings.SplitN(c.Code, "-", 2)[0]
					if len(code) < 2 {
						continue
					}
					key := code[:2]
					if perYear[y] == nil {
						perYear[y] = map[string]int{}
					}
					perYear[y][key]++
					break
				}
			}
			ys := make([]int, 0, len(perYear))
			for y := range perYear {
				ys = append(ys, y)
			}
			sort.Sort(sort.Reverse(sort.IntSlice(ys)))
			if years > 0 && len(ys) > years {
				ys = ys[:years]
			}
			sort.Ints(ys)

			report := []map[string]any{}
			for _, y := range ys {
				m := perYear[y]
				type kv struct {
					K string
					V int
				}
				rows := make([]kv, 0, len(m))
				for k, v := range m {
					rows = append(rows, kv{k, v})
				}
				sort.Slice(rows, func(i, j int) bool { return rows[i].V > rows[j].V })
				if topN > 0 && len(rows) > topN {
					rows = rows[:topN]
				}
				top := make([]map[string]any, len(rows))
				for i, r := range rows {
					top[i] = map[string]any{"cpv2": r.K, "count": r.V}
				}
				report = append(report, map[string]any{"year": y, "top_cpv2": top})
			}
			result := map[string]any{
				"buyer": buyer,
				"nuts":  nuts,
				"drift": report,
			}
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "CPV drift — buyer=%q nuts=%q\n", buyer, nuts)
			for _, row := range report {
				fmt.Fprintf(cmd.OutOrStdout(), "  %v:\n", row["year"])
				for _, t := range row["top_cpv2"].([]map[string]any) {
					fmt.Fprintf(cmd.OutOrStdout(), "    CPV %s — %d\n", t["cpv2"], t["count"])
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&buyer, "buyer", "", "Substring match on buyer name")
	cmd.Flags().StringVar(&nuts, "nuts", "", "NUTS region code")
	cmd.Flags().IntVar(&years, "years", 3, "Number of trailing years to include")
	cmd.Flags().IntVar(&topN, "top", 8, "Top-N CPV-2 codes per year")
	return cmd
}
