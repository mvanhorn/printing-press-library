package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/openfda/internal/store"
	"github.com/spf13/cobra"
)

func newDrugEventsPolypharmacyCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var drugs string
	var topN int

	cmd := &cobra.Command{
		Use:   "polypharmacy",
		Short: "Analyze adverse events for drug combinations",
		Long: `Find adverse event reports where ALL specified drugs appear together,
compare reaction frequencies to single-drug baselines, and highlight
reactions that are unique to or more frequent in the combination.`,
		Example: `  # Check combination of two drugs
  openfda-pp-cli drug-events polypharmacy --drugs "ASPIRIN,WARFARIN"

  # Top 10 reactions for a triple combo
  openfda-pp-cli drug-events polypharmacy --drugs "LISINOPRIL,METFORMIN,ATORVASTATIN" --top 10 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if drugs == "" {
				return usageErr(fmt.Errorf("--drugs is required (comma-separated, at least 2)"))
			}
			drugList := strings.Split(drugs, ",")
			for i := range drugList {
				drugList[i] = strings.TrimSpace(drugList[i])
			}
			if len(drugList) < 2 {
				return usageErr(fmt.Errorf("at least 2 drugs required for polypharmacy analysis"))
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

			// Step 1: Find reports containing ALL specified drugs
			// We query for the first drug, then filter in-memory for the rest
			firstDrug := strings.ToUpper(drugList[0])
			query := `
				SELECT r.data
				FROM resources r, json_each(json_extract(r.data, '$.patient.drug')) je
				WHERE r.resource_type = 'drug-events'
				AND UPPER(json_extract(je.value, '$.medicinalproduct')) LIKE ?
			`
			rows, err := db.Query(query, "%"+firstDrug+"%")
			if err != nil {
				return fmt.Errorf("querying drug events: %w", err)
			}

			// Collect combo reports
			comboReactions := make(map[string]int) // reaction -> count
			comboReportCount := 0
			seen := make(map[string]bool)

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

				// Check if ALL drugs are present
				patient, ok := event["patient"].(map[string]interface{})
				if !ok {
					continue
				}
				drugArr, ok := patient["drug"].([]interface{})
				if !ok {
					continue
				}

				// Collect drug names in this report
				reportDrugs := make(map[string]bool)
				for _, d := range drugArr {
					if dm, ok := d.(map[string]interface{}); ok {
						if mp, ok := dm["medicinalproduct"].(string); ok {
							reportDrugs[strings.ToUpper(mp)] = true
						}
					}
				}

				// Check all target drugs are present
				allPresent := true
				for _, targetDrug := range drugList {
					targetUpper := strings.ToUpper(targetDrug)
					found := false
					for rd := range reportDrugs {
						if strings.Contains(rd, targetUpper) {
							found = true
							break
						}
					}
					if !found {
						allPresent = false
						break
					}
				}
				if !allPresent {
					continue
				}

				if reportID != "" {
					seen[reportID] = true
				}
				comboReportCount++

				// Collect reactions
				if reactions, ok := patient["reaction"].([]interface{}); ok {
					for _, r := range reactions {
						if rm, ok := r.(map[string]interface{}); ok {
							if rpt, ok := rm["reactionmeddrapt"].(string); ok {
								comboReactions[strings.ToUpper(rpt)]++
							}
						}
					}
				}
			}
			rows.Close()

			// Step 2: Get single-drug baselines for each drug
			type baselineInfo struct {
				Drug      string         `json:"drug"`
				Reports   int            `json:"reports"`
				Reactions map[string]int `json:"-"`
			}
			var baselines []baselineInfo

			for _, drugName := range drugList {
				drugUpper := strings.ToUpper(drugName)
				bQuery := `
					SELECT r.data
					FROM resources r, json_each(json_extract(r.data, '$.patient.drug')) je
					WHERE r.resource_type = 'drug-events'
					AND UPPER(json_extract(je.value, '$.medicinalproduct')) LIKE ?
				`
				bRows, err := db.Query(bQuery, "%"+drugUpper+"%")
				if err != nil {
					continue
				}

				baseline := baselineInfo{
					Drug:      drugName,
					Reactions: make(map[string]int),
				}
				bSeen := make(map[string]bool)

				for bRows.Next() {
					var dataStr string
					if err := bRows.Scan(&dataStr); err != nil {
						continue
					}
					var event map[string]interface{}
					if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
						continue
					}
					rid, _ := event["safetyreportid"].(string)
					if rid != "" && bSeen[rid] {
						continue
					}
					if rid != "" {
						bSeen[rid] = true
					}
					baseline.Reports++

					if patient, ok := event["patient"].(map[string]interface{}); ok {
						if reactions, ok := patient["reaction"].([]interface{}); ok {
							for _, r := range reactions {
								if rm, ok := r.(map[string]interface{}); ok {
									if rpt, ok := rm["reactionmeddrapt"].(string); ok {
										baseline.Reactions[strings.ToUpper(rpt)]++
									}
								}
							}
						}
					}
				}
				bRows.Close()
				baselines = append(baselines, baseline)
			}

			// Step 3: Compare combo vs baselines
			type polyReaction struct {
				Reaction       string  `json:"reaction"`
				ComboCount     int     `json:"combo_count"`
				ComboPercent   float64 `json:"combo_percent"`
				UniqueToCombo  bool    `json:"unique_to_combo"`
				Elevated       bool    `json:"elevated"`
				MaxBaselinePct float64 `json:"max_baseline_percent"`
			}

			var polyResults []polyReaction
			for rxn, count := range comboReactions {
				comboPct := 0.0
				if comboReportCount > 0 {
					comboPct = float64(count) / float64(comboReportCount) * 100
				}

				maxBaselinePct := 0.0
				inAnyBaseline := false
				for _, b := range baselines {
					if bCount, ok := b.Reactions[rxn]; ok {
						inAnyBaseline = true
						bPct := float64(bCount) / float64(b.Reports) * 100
						if bPct > maxBaselinePct {
							maxBaselinePct = bPct
						}
					}
				}

				pr := polyReaction{
					Reaction:       rxn,
					ComboCount:     count,
					ComboPercent:   comboPct,
					UniqueToCombo:  !inAnyBaseline,
					Elevated:       comboPct > maxBaselinePct*1.5,
					MaxBaselinePct: maxBaselinePct,
				}
				polyResults = append(polyResults, pr)
			}

			// Sort: unique first, then by elevation ratio, then by count
			sort.Slice(polyResults, func(i, j int) bool {
				if polyResults[i].UniqueToCombo != polyResults[j].UniqueToCombo {
					return polyResults[i].UniqueToCombo
				}
				if polyResults[i].Elevated != polyResults[j].Elevated {
					return polyResults[i].Elevated
				}
				return polyResults[i].ComboCount > polyResults[j].ComboCount
			})

			if topN > 0 && len(polyResults) > topN {
				polyResults = polyResults[:topN]
			}

			// Build baseline summaries
			type baselineSummary struct {
				Drug    string `json:"drug"`
				Reports int    `json:"reports"`
			}
			var bSummaries []baselineSummary
			for _, b := range baselines {
				bSummaries = append(bSummaries, baselineSummary{Drug: b.Drug, Reports: b.Reports})
			}

			output := map[string]interface{}{
				"drugs":               drugList,
				"combo_reports":       comboReportCount,
				"single_drug_baselines": bSummaries,
				"reactions":           polyResults,
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Polypharmacy Analysis: %s\n", strings.Join(drugList, " + "))
			fmt.Fprintf(cmd.OutOrStdout(), "Combination reports: %d\n", comboReportCount)
			for _, b := range baselines {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s alone: %d reports\n", b.Drug, b.Reports)
			}
			fmt.Fprintln(cmd.OutOrStdout())

			if len(polyResults) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No reports found with all specified drugs together.")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "REACTION\tCOMBO %\tBASELINE %\tFLAG")
			for _, r := range polyResults {
				flag := ""
				if r.UniqueToCombo {
					flag = "UNIQUE"
				} else if r.Elevated {
					flag = "ELEVATED"
				}
				fmt.Fprintf(tw, "%s\t%.1f%%\t%.1f%%\t%s\n",
					truncate(r.Reaction, 30), r.ComboPercent, r.MaxBaselinePct, flag)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&drugs, "drugs", "", "Comma-separated drug names (at least 2, required)")
	cmd.Flags().IntVar(&topN, "top", 20, "Number of top reactions to show")

	return cmd
}
