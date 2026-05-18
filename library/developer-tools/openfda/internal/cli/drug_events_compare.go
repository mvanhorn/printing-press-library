package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/openfda/internal/store"
	"github.com/spf13/cobra"
)

func newDrugEventsCompareCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var drugs string
	var topN int

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare adverse reaction profiles between drugs",
		Long: `Compare the reaction frequency distributions across multiple drugs
using locally synced adverse event data. Shows side-by-side comparison
of the top N reactions for each drug.`,
		Example: `  # Compare two drugs
  openfda-pp-cli drug-events compare --drugs "ASPIRIN,IBUPROFEN"

  # Top 5 reactions comparison as JSON
  openfda-pp-cli drug-events compare --drugs "ASPIRIN,ACETAMINOPHEN,NAPROXEN" --top 5 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if drugs == "" {
				return usageErr(fmt.Errorf("--drugs is required (comma-separated list)"))
			}
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("openfda-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'openfda-pp-cli sync' first.", err)
			}
			defer db.Close()

			drugList := strings.Split(drugs, ",")
			for i := range drugList {
				drugList[i] = strings.TrimSpace(drugList[i])
			}

			type drugProfile struct {
				Drug       string                `json:"drug"`
				TotalReports int                 `json:"total_reports"`
				Reactions  []reactionFrequency   `json:"reactions"`
			}

			var profiles []drugProfile
			allReactions := make(map[string]bool)

			for _, drugName := range drugList {
				drugUpper := strings.ToUpper(drugName)
				query := `
					SELECT r.data
					FROM resources r, json_each(json_extract(r.data, '$.patient.drug')) je
					WHERE r.resource_type = 'drug-events'
					AND UPPER(json_extract(je.value, '$.medicinalproduct')) LIKE ?
				`
				rows, err := db.Query(query, "%"+drugUpper+"%")
				if err != nil {
					return fmt.Errorf("querying events for %s: %w", drugName, err)
				}

				reactionCounts := make(map[string]int)
				totalReports := 0
				seen := make(map[string]bool) // dedupe by safetyreportid

				for rows.Next() {
					var dataStr string
					if err := rows.Scan(&dataStr); err != nil {
						continue
					}
					var event map[string]interface{}
					if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
						continue
					}

					reportID, _ := event["safetyreportid"].(string)
					if reportID != "" && seen[reportID] {
						continue
					}
					if reportID != "" {
						seen[reportID] = true
					}
					totalReports++

					if patient, ok := event["patient"].(map[string]interface{}); ok {
						if reactions, ok := patient["reaction"].([]interface{}); ok {
							for _, r := range reactions {
								if rm, ok := r.(map[string]interface{}); ok {
									if rpt, ok := rm["reactionmeddrapt"].(string); ok {
										rpt = strings.ToUpper(rpt)
										reactionCounts[rpt]++
										allReactions[rpt] = true
									}
								}
							}
						}
					}
				}
				rows.Close()

				var freqs []reactionFrequency
				for rxn, count := range reactionCounts {
					pct := 0.0
					if totalReports > 0 {
						pct = float64(count) / float64(totalReports) * 100
					}
					freqs = append(freqs, reactionFrequency{
						Reaction:   rxn,
						Count:      count,
						Percentage: pct,
					})
				}
				sort.Slice(freqs, func(i, j int) bool {
					return freqs[i].Count > freqs[j].Count
				})
				if topN > 0 && len(freqs) > topN {
					freqs = freqs[:topN]
				}

				profiles = append(profiles, drugProfile{
					Drug:         drugName,
					TotalReports: totalReports,
					Reactions:    freqs,
				})
			}

			result := map[string]interface{}{
				"comparison": profiles,
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			// Table output
			for _, p := range profiles {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s (%d reports)\n", bold(p.Drug), p.TotalReports)
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, "REACTION\tCOUNT\tPERCENT")
				for _, r := range p.Reactions {
					fmt.Fprintf(tw, "%s\t%d\t%.1f%%\n", r.Reaction, r.Count, r.Percentage)
				}
				tw.Flush()
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&drugs, "drugs", "", "Comma-separated list of drug names to compare (required)")
	cmd.Flags().IntVar(&topN, "top", 10, "Number of top reactions to show per drug")

	return cmd
}

type reactionFrequency struct {
	Reaction   string  `json:"reaction"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}
