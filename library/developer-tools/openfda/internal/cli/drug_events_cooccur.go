package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/openfda/internal/store"
	"github.com/spf13/cobra"
)

func newDrugEventsCooccurCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var drug string
	var topN int

	cmd := &cobra.Command{
		Use:   "cooccur",
		Short: "Find reaction co-occurrence patterns for a drug",
		Long: `Analyze adverse event reports for a drug to find which reactions
commonly occur together in the same report. Computes pairwise co-occurrence
counts across all reports mentioning the drug.`,
		Example: `  # Find top reaction pairs for a drug
  openfda-pp-cli drug-events cooccur --drug ASPIRIN

  # Top 5 pairs as JSON
  openfda-pp-cli drug-events cooccur --drug ACETAMINOPHEN --top 5 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if drug == "" {
				return usageErr(fmt.Errorf("--drug is required"))
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

			drugUpper := strings.ToUpper(drug)

			// Get all events for this drug, grouped by safetyreportid
			query := `
				SELECT r.data
				FROM resources r, json_each(json_extract(r.data, '$.patient.drug')) je
				WHERE r.resource_type = 'drug-events'
				AND UPPER(json_extract(je.value, '$.medicinalproduct')) LIKE ?
			`
			rows, err := db.Query(query, "%"+drugUpper+"%")
			if err != nil {
				return fmt.Errorf("querying drug events: %w", err)
			}
			defer rows.Close()

			// Collect reactions per report
			reportReactions := make(map[string][]string) // safetyreportid -> reactions
			var skippedNoID int
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
				if reportID == "" {
					skippedNoID++
					continue
				}
				if _, seen := reportReactions[reportID]; seen {
					continue
				}

				var reactions []string
				if patient, ok := event["patient"].(map[string]interface{}); ok {
					if rxns, ok := patient["reaction"].([]interface{}); ok {
						for _, r := range rxns {
							if rm, ok := r.(map[string]interface{}); ok {
								if rpt, ok := rm["reactionmeddrapt"].(string); ok {
									reactions = append(reactions, strings.ToUpper(rpt))
								}
							}
						}
					}
				}
				if len(reactions) > 1 {
					reportReactions[reportID] = reactions
				}
			}

			if skippedNoID > 0 {
				fmt.Fprintf(os.Stderr, "warning: %d events without safetyreportid were skipped\n", skippedNoID)
			}

			// Compute pairwise co-occurrence
			type pair struct {
				A string
				B string
			}
			pairCounts := make(map[pair]int)

			for _, reactions := range reportReactions {
				// Sort reactions for consistent pair ordering
				sort.Strings(reactions)
				// Deduplicate within this report
				unique := dedupeStrings(reactions)
				for i := 0; i < len(unique); i++ {
					for j := i + 1; j < len(unique); j++ {
						p := pair{A: unique[i], B: unique[j]}
						pairCounts[p]++
					}
				}
			}

			type cooccurResult struct {
				Reaction1 string `json:"reaction_1"`
				Reaction2 string `json:"reaction_2"`
				Count     int    `json:"count"`
			}

			var results []cooccurResult
			for p, count := range pairCounts {
				results = append(results, cooccurResult{
					Reaction1: p.A,
					Reaction2: p.B,
					Count:     count,
				})
			}
			sort.Slice(results, func(i, j int) bool {
				return results[i].Count > results[j].Count
			})
			if topN > 0 && len(results) > topN {
				results = results[:topN]
			}

			output := map[string]interface{}{
				"drug":          drug,
				"total_reports": len(reportReactions),
				"total_pairs":   len(pairCounts),
				"top_pairs":     results,
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Reaction Co-occurrence: %s\n", drug)
			fmt.Fprintf(cmd.OutOrStdout(), "Reports with 2+ reactions: %d | Unique pairs: %d\n\n", len(reportReactions), len(pairCounts))

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No co-occurring reaction pairs found.")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "REACTION 1\tREACTION 2\tCO-OCCURRENCES")
			for _, r := range results {
				fmt.Fprintf(tw, "%s\t%s\t%d\n", truncate(r.Reaction1, 30), truncate(r.Reaction2, 30), r.Count)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&drug, "drug", "", "Drug name (required)")
	cmd.Flags().IntVar(&topN, "top", 20, "Number of top pairs to show")

	return cmd
}

func dedupeStrings(sorted []string) []string {
	if len(sorted) == 0 {
		return sorted
	}
	result := []string{sorted[0]}
	for i := 1; i < len(sorted); i++ {
		if sorted[i] != sorted[i-1] {
			result = append(result, sorted[i])
		}
	}
	return result
}
