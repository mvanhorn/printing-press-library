package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
)

type dealCoverageRow struct {
	DealID       string `json:"deal_id"`
	DealName     string `json:"deal_name"`
	Stage        string `json:"stage"`
	Amount       string `json:"amount"`
	Owner        string `json:"owner"`
	DaysSinceAny int    `json:"days_since_any_engagement"`
	TotalTouch   int    `json:"total_touches"`
	Risk         string `json:"risk"`
}

func newDealsCoverageCmd(flags *rootFlags) *cobra.Command {
	var pipelineID, dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "coverage",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Find open deals with low engagement coverage",
		Long:  "Identify open deals where associated contacts have little or no recent engagement activity.",
		Example: "  hubspot-pp-cli deals coverage\n" +
			"  hubspot-pp-cli deals coverage --pipeline default --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("coverage: %w", err)
			}
			defer s.Close()

			rows, err := buildCoverageData(s, pipelineID, limit, time.Now())
			if err != nil {
				return fmt.Errorf("coverage: %w", err)
			}

			data, err := json.Marshal(rows)
			if err != nil {
				return fmt.Errorf("coverage: %w", err)
			}
			if flags.compact {
				data = filterFields(data, "deal_name,stage,days_since_any_engagement,risk")
			}
			if flags.selectFields != "" {
				data = filterFields(data, flags.selectFields)
			}
			if flags.asJSON || flags.compact || flags.selectFields != "" || !isTerminal(cmd.OutOrStdout()) {
				return printOutput(cmd.OutOrStdout(), data, true)
			}
			return printCoverageTable(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&pipelineID, "pipeline", "", "Filter by pipeline ID")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max deals to show")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func buildCoverageData(s *store.Store, pipelineFilter string, limit int, now time.Time) ([]dealCoverageRow, error) {
	// Get most recent engagement timestamp per owner (simplified proxy)
	ownerLastTouch := map[string]time.Time{}
	ownerTouchCount := map[string]int{}

	for _, table := range []string{"calls", "emails", "meetings", "notes"} {
		rows, err := s.Query(fmt.Sprintf(`SELECT data FROM %s`, table))
		if err != nil {
			continue
		}
		for rows.Next() {
			var raw string
			if err := rows.Scan(&raw); err != nil {
				continue
			}
			var eng struct {
				Properties map[string]string `json:"properties"`
			}
			if json.Unmarshal([]byte(raw), &eng) != nil {
				continue
			}
			ownerID := eng.Properties["hubspot_owner_id"]
			if ownerID == "" {
				continue
			}
			ownerTouchCount[ownerID]++
			ts, ok := parseHubSpotTime(eng.Properties["hs_timestamp"])
			if ok && ts.After(ownerLastTouch[ownerID]) {
				ownerLastTouch[ownerID] = ts
			}
		}
		rows.Close()
	}

	// Load owner names
	ownerNames := map[string]string{}
	oRows, err := s.Query(`SELECT id, data FROM owners`)
	if err == nil {
		defer oRows.Close()
		for oRows.Next() {
			var id, raw string
			if err := oRows.Scan(&id, &raw); err != nil {
				continue
			}
			var o struct {
				FirstName string `json:"firstName"`
				LastName  string `json:"lastName"`
			}
			if json.Unmarshal([]byte(raw), &o) == nil {
				ownerNames[id] = strings.TrimSpace(o.FirstName + " " + o.LastName)
			}
		}
	}

	// Analyze deals
	dRows, err := s.Query(`SELECT data FROM deals`)
	if err != nil {
		return nil, err
	}
	defer dRows.Close()

	var result []dealCoverageRow
	for dRows.Next() {
		var raw string
		if err := dRows.Scan(&raw); err != nil {
			continue
		}
		var d struct {
			ID         string            `json:"id"`
			Properties map[string]string `json:"properties"`
		}
		if json.Unmarshal([]byte(raw), &d) != nil {
			continue
		}

		stage := d.Properties["dealstage"]
		pipeline := d.Properties["pipeline"]
		if stage == "closedwon" || stage == "closedlost" {
			continue
		}
		if pipelineFilter != "" && pipeline != pipelineFilter {
			continue
		}

		ownerID := d.Properties["hubspot_owner_id"]
		ownerName := ownerNames[ownerID]
		if ownerName == "" {
			ownerName = ownerID
		}

		daysSince := 999
		touches := 0
		if lt, ok := ownerLastTouch[ownerID]; ok && !lt.IsZero() {
			daysSince = int(now.Sub(lt).Hours() / 24)
		}
		touches = ownerTouchCount[ownerID]

		risk := "LOW"
		if daysSince > 30 {
			risk = "HIGH"
		} else if daysSince > 14 {
			risk = "MEDIUM"
		}

		result = append(result, dealCoverageRow{
			DealID:       d.ID,
			DealName:     d.Properties["dealname"],
			Stage:        stage,
			Amount:       d.Properties["amount"],
			Owner:        ownerName,
			DaysSinceAny: daysSince,
			TotalTouch:   touches,
			Risk:         risk,
		})
	}

	// Sort by days since engagement descending (highest risk first)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].DaysSinceAny > result[i].DaysSinceAny {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func printCoverageTable(cmd *cobra.Command, rows []dealCoverageRow) error {
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, strings.Join([]string{bold("DEAL"), bold("STAGE"), bold("AMOUNT"), bold("OWNER"), bold("DAYS SILENT"), bold("TOUCHES"), bold("RISK")}, "\t"))
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%d\t%s\n", r.DealName, r.Stage, r.Amount, r.Owner, r.DaysSinceAny, r.TotalTouch, r.Risk)
	}
	return tw.Flush()
}
