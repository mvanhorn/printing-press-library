package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/openfda/internal/store"
	"github.com/spf13/cobra"
)

func newDrugRecallsCorrelateCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var drug string
	var windowDays int

	cmd := &cobra.Command{
		Use:   "correlate",
		Short: "Correlate drug recalls with adverse event timing",
		Long: `Find recalls for a drug and correlate with adverse event reports
to identify whether event frequency changed around recall dates.
Uses a configurable time window (default 90 days) before and after each recall.`,
		Example: `  # Correlate recalls and events for a drug
  openfda-pp-cli drug-recalls correlate --drug ACETAMINOPHEN

  # Use a 180-day window
  openfda-pp-cli drug-recalls correlate --drug ASPIRIN --window 180 --json`,
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

			// 1. Find recalls for this drug
			recallQuery := `
				SELECT r.data
				FROM resources r
				WHERE r.resource_type = 'drug-recalls'
				AND (UPPER(json_extract(r.data, '$.product_description')) LIKE ?
				  OR UPPER(json_extract(r.data, '$.recalling_firm')) LIKE ?)
			`
			recallRows, err := db.Query(recallQuery, "%"+drugUpper+"%", "%"+drugUpper+"%")
			if err != nil {
				return fmt.Errorf("querying recalls: %w", err)
			}

			type recallInfo struct {
				ReportDate     string `json:"report_date"`
				RecallingFirm  string `json:"recalling_firm"`
				Classification string `json:"classification"`
				Reason         string `json:"reason_for_recall"`
				Status         string `json:"status"`
				ParsedDate     time.Time `json:"-"`
			}

			var recalls []recallInfo
			for recallRows.Next() {
				var dataStr string
				if err := recallRows.Scan(&dataStr); err != nil {
					continue
				}
				var recall map[string]interface{}
				if err := json.Unmarshal([]byte(dataStr), &recall); err != nil {
					continue
				}

				reportDate, _ := recall["report_date"].(string)
				parsed := parseFDADate(reportDate)
				if parsed.IsZero() {
					continue
				}

				reason, _ := recall["reason_for_recall"].(string)
				ri := recallInfo{
					ReportDate:     reportDate,
					RecallingFirm:  strOrEmpty(recall["recalling_firm"]),
					Classification: strOrEmpty(recall["classification"]),
					Reason:         truncate(reason, 80),
					Status:         strOrEmpty(recall["status"]),
					ParsedDate:     parsed,
				}
				recalls = append(recalls, ri)
			}
			recallRows.Close()

			sort.Slice(recalls, func(i, j int) bool {
				return recalls[i].ParsedDate.Before(recalls[j].ParsedDate)
			})

			// 2. Find all adverse events for this drug with dates.
			// The json_each join on $.patient.drug produces duplicate rows when a
			// report lists multiple matching drugs. Dedup by safetyreportid.
			eventQuery := `
				SELECT json_extract(r.data, '$.receiptdate'),
				       json_extract(r.data, '$.safetyreportid')
				FROM resources r, json_each(json_extract(r.data, '$.patient.drug')) je
				WHERE r.resource_type = 'drug-events'
				AND UPPER(json_extract(je.value, '$.medicinalproduct')) LIKE ?
				AND json_extract(r.data, '$.receiptdate') IS NOT NULL
			`
			eventRows, err := db.Query(eventQuery, "%"+drugUpper+"%")
			if err != nil {
				return fmt.Errorf("querying events: %w", err)
			}

			var eventDates []time.Time
			seenEvents := make(map[string]bool)
			for eventRows.Next() {
				var dateStr, reportID string
				if err := eventRows.Scan(&dateStr, &reportID); err != nil {
					continue
				}
				if reportID != "" && seenEvents[reportID] {
					continue
				}
				if reportID != "" {
					seenEvents[reportID] = true
				}
				parsed := parseFDADate(dateStr)
				if !parsed.IsZero() {
					eventDates = append(eventDates, parsed)
				}
			}
			eventRows.Close()

			sort.Slice(eventDates, func(i, j int) bool {
				return eventDates[i].Before(eventDates[j])
			})

			// 3. For each recall, count events in windows before and after
			window := time.Duration(windowDays) * 24 * time.Hour

			type recallCorrelation struct {
				RecallDate     string `json:"recall_date"`
				RecallingFirm  string `json:"recalling_firm"`
				Classification string `json:"classification"`
				Reason         string `json:"reason"`
				EventsBefore   int    `json:"events_before_window"`
				EventsAfter    int    `json:"events_after_window"`
				Delta          int    `json:"delta"`
				DeltaPercent   string `json:"delta_percent"`
			}

			var correlations []recallCorrelation
			for _, recall := range recalls {
				before := 0
				after := 0
				windowStart := recall.ParsedDate.Add(-window)
				windowEnd := recall.ParsedDate.Add(window)

				for _, ed := range eventDates {
					if ed.After(windowStart) && !ed.After(recall.ParsedDate) {
						before++
					} else if ed.After(recall.ParsedDate) && !ed.After(windowEnd) {
						after++
					}
				}

				delta := after - before
				deltaPct := "N/A"
				if before > 0 {
					deltaPct = fmt.Sprintf("%+.1f%%", float64(delta)/float64(before)*100)
				}

				correlations = append(correlations, recallCorrelation{
					RecallDate:     recall.ReportDate,
					RecallingFirm:  recall.RecallingFirm,
					Classification: recall.Classification,
					Reason:         recall.Reason,
					EventsBefore:   before,
					EventsAfter:    after,
					Delta:          delta,
					DeltaPercent:   deltaPct,
				})
			}

			result := map[string]interface{}{
				"drug":         drug,
				"window_days":  windowDays,
				"total_recalls": len(recalls),
				"total_events":  len(eventDates),
				"correlations": correlations,
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Recall-Event Correlation: %s\n", drug)
			fmt.Fprintf(cmd.OutOrStdout(), "Window: %d days | Recalls: %d | Total events: %d\n\n", windowDays, len(recalls), len(eventDates))

			if len(correlations) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No recalls found for this drug.")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "RECALL DATE\tFIRM\tCLASS\tEVENTS BEFORE\tEVENTS AFTER\tDELTA")
			for _, c := range correlations {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\t%s\n",
					c.RecallDate, truncate(c.RecallingFirm, 25), c.Classification,
					c.EventsBefore, c.EventsAfter, c.DeltaPercent)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&drug, "drug", "", "Drug name to correlate (required)")
	cmd.Flags().IntVar(&windowDays, "window", 90, "Window in days before/after recall")

	return cmd
}

// parseFDADate parses FDA date formats: YYYYMMDD or YYYY-MM-DD.
func parseFDADate(s string) time.Time {
	s = strings.TrimSpace(s)
	if len(s) == 8 {
		t, err := time.Parse("20060102", s)
		if err == nil {
			return t
		}
	}
	if len(s) >= 10 {
		t, err := time.Parse("2006-01-02", s[:10])
		if err == nil {
			return t
		}
	}
	return time.Time{}
}

func strOrEmpty(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
