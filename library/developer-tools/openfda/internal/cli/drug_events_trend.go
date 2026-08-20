package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/openfda/internal/store"
	"github.com/spf13/cobra"
)

func newDrugEventsTrendCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var drug string
	var reaction string
	var interval string

	cmd := &cobra.Command{
		Use:   "trend",
		Short: "Track adverse event signal over time for a drug",
		Long: `Analyze locally synced drug adverse event reports to produce a time series
of event counts for a specific drug, optionally filtered by reaction.
Data must be synced first with the sync command.`,
		Example: `  # Quarterly trend for acetaminophen
  openfda-pp-cli drug-events trend --drug ACETAMINOPHEN --interval quarter

  # Monthly trend for a specific reaction
  openfda-pp-cli drug-events trend --drug ASPIRIN --reaction NAUSEA --interval month --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if drug == "" {
				return usageErr(fmt.Errorf("--drug is required"))
			}
			if interval == "" {
				interval = "quarter"
			}
			switch interval {
			case "quarter", "month", "year":
			default:
				return usageErr(fmt.Errorf("--interval must be quarter, month, or year"))
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

			// Query all drug-events where the drug appears in patient.drug array
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

			buckets := make(map[string]int)
			total := 0
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

				// Filter by reaction if specified
				if reaction != "" {
					reactionUpper := strings.ToUpper(reaction)
					matched := false
					if patient, ok := event["patient"].(map[string]interface{}); ok {
						if reactions, ok := patient["reaction"].([]interface{}); ok {
							for _, r := range reactions {
								if rm, ok := r.(map[string]interface{}); ok {
									if rpt, ok := rm["reactionmeddrapt"].(string); ok {
										if strings.Contains(strings.ToUpper(rpt), reactionUpper) {
											matched = true
											break
										}
									}
								}
							}
						}
					}
					if !matched {
						continue
					}
				}

				// Extract receiptdate and bucket
				receiptDate, _ := event["receiptdate"].(string)
				if receiptDate == "" {
					continue
				}
				bucket := dateToBucket(receiptDate, interval)
				if bucket != "" {
					reportID, _ := event["safetyreportid"].(string)
					key := reportID + "|" + bucket
					if reportID != "" && seen[key] {
						continue
					}
					if reportID != "" {
						seen[key] = true
					}
					buckets[bucket]++
					total++
				}
			}

			type bucketEntry struct {
				Period string `json:"period"`
				Count  int    `json:"count"`
			}
			var sorted []bucketEntry
			for k, v := range buckets {
				sorted = append(sorted, bucketEntry{Period: k, Count: v})
			}
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].Period < sorted[j].Period
			})

			result := map[string]interface{}{
				"drug":     drug,
				"interval": interval,
				"total":    total,
				"buckets":  sorted,
			}
			if reaction != "" {
				result["reaction"] = reaction
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Drug Signal Trend: %s\n", drug)
			if reaction != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Reaction filter: %s\n", reaction)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Interval: %s | Total events: %d\n\n", interval, total)

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "PERIOD\tCOUNT\tBAR")
			maxCount := 0
			for _, b := range sorted {
				if b.Count > maxCount {
					maxCount = b.Count
				}
			}
			for _, b := range sorted {
				barLen := 0
				if maxCount > 0 {
					barLen = b.Count * 40 / maxCount
				}
				if barLen == 0 && b.Count > 0 {
					barLen = 1
				}
				bar := strings.Repeat("█", barLen)
				fmt.Fprintf(tw, "%s\t%d\t%s\n", b.Period, b.Count, bar)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&drug, "drug", "", "Drug name to track (required)")
	cmd.Flags().StringVar(&reaction, "reaction", "", "Filter by specific reaction")
	cmd.Flags().StringVar(&interval, "interval", "quarter", "Time interval: quarter, month, or year")

	return cmd
}

// dateToBucket converts an FDA date string (YYYYMMDD) to a period bucket.
func dateToBucket(date, interval string) string {
	if len(date) < 6 {
		return ""
	}
	year := date[:4]
	month := date[4:6]

	switch interval {
	case "year":
		return year
	case "month":
		return year + "-" + month
	case "quarter":
		m := 0
		fmt.Sscanf(month, "%d", &m)
		q := (m-1)/3 + 1
		return fmt.Sprintf("%s-Q%d", year, q)
	default:
		return year
	}
}
