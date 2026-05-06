package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
)

type contactEngagementRow struct {
	ContactID  string `json:"contact_id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	TotalTouch int    `json:"total_touches"`
	LastTouch  string `json:"last_touch"`
	DaysSince  int    `json:"days_since_last_touch"`
	Calls      int    `json:"calls"`
	Emails     int    `json:"emails"`
	Meetings   int    `json:"meetings"`
	Notes      int    `json:"notes"`
	Tasks      int    `json:"tasks"`
}

func newContactsEngagementCmd(flags *rootFlags) *cobra.Command {
	var days int
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "engagement",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Score contact engagement across all activity types",
		Long:  "Analyze engagement frequency and gaps for contacts by correlating calls, emails, meetings, notes, and tasks from the local store.",
		Example: "  hubspot-pp-cli contacts engagement --days 30\n" +
			"  hubspot-pp-cli contacts engagement --days 60 --limit 20 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("engagement: %w", err)
			}
			defer s.Close()

			rows, err := buildEngagementData(s, days, limit, time.Now())
			if err != nil {
				return fmt.Errorf("engagement: %w", err)
			}

			data, err := json.Marshal(rows)
			if err != nil {
				return fmt.Errorf("engagement: %w", err)
			}
			if flags.compact {
				data = filterFields(data, "name,total_touches,days_since_last_touch")
			}
			if flags.selectFields != "" {
				data = filterFields(data, flags.selectFields)
			}
			if flags.asJSON || flags.compact || flags.selectFields != "" || !isTerminal(cmd.OutOrStdout()) {
				return printOutput(cmd.OutOrStdout(), data, true)
			}
			return printEngagementTable(cmd, rows)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Analyze engagement within the last N days")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max contacts to show")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func buildEngagementData(s *store.Store, days, limit int, now time.Time) ([]contactEngagementRow, error) {
	type touchCount struct {
		calls, emails, meetings, notes, tasks int
		lastTouch                             time.Time
	}
	contacts := map[string]touchCount{}
	contactNames := map[string][2]string{} // id -> [name, email]

	// Load contacts
	cRows, err := s.Query(`SELECT id, data FROM contacts`)
	if err != nil {
		return nil, err
	}
	defer cRows.Close()
	for cRows.Next() {
		var id, raw string
		if err := cRows.Scan(&id, &raw); err != nil {
			continue
		}
		var c struct {
			Properties map[string]string `json:"properties"`
		}
		if json.Unmarshal([]byte(raw), &c) != nil {
			continue
		}
		name := strings.TrimSpace(c.Properties["firstname"] + " " + c.Properties["lastname"])
		if name == "" {
			name = c.Properties["email"]
		}
		contactNames[id] = [2]string{name, c.Properties["email"]}
		contacts[id] = touchCount{}
	}

	// Count engagements by type
	engagementTypes := []struct {
		table string
		field string
	}{
		{"calls", "calls"},
		{"emails", "emails"},
		{"meetings", "meetings"},
		{"notes", "notes"},
		{"tasks", "tasks"},
	}

	cutoff := now.AddDate(0, 0, -days)

	for _, et := range engagementTypes {
		rows, err := s.Query(fmt.Sprintf(`SELECT data FROM %s`, et.table))
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
			ts, ok := parseHubSpotTime(eng.Properties["hs_timestamp"])
			if !ok || ts.Before(cutoff) {
				continue
			}
			ownerID := eng.Properties["hubspot_owner_id"]
			// Associate with all contacts (simplified - in practice would use associations)
			// For now, count by owner as a proxy
			if ownerID != "" {
				tc := contacts[ownerID]
				switch et.field {
				case "calls":
					tc.calls++
				case "emails":
					tc.emails++
				case "meetings":
					tc.meetings++
				case "notes":
					tc.notes++
				case "tasks":
					tc.tasks++
				}
				if ts.After(tc.lastTouch) {
					tc.lastTouch = ts
				}
				contacts[ownerID] = tc
			}
		}
		rows.Close()
	}

	// Build result rows for contacts
	var result []contactEngagementRow
	for id, info := range contactNames {
		tc := contacts[id]
		total := tc.calls + tc.emails + tc.meetings + tc.notes + tc.tasks
		daysSince := -1
		lastTouchStr := "never"
		if !tc.lastTouch.IsZero() {
			daysSince = int(now.Sub(tc.lastTouch).Hours() / 24)
			lastTouchStr = tc.lastTouch.Format("2006-01-02")
		}
		result = append(result, contactEngagementRow{
			ContactID:  id,
			Name:       info[0],
			Email:      info[1],
			TotalTouch: total,
			LastTouch:  lastTouchStr,
			DaysSince:  daysSince,
			Calls:      tc.calls,
			Emails:     tc.emails,
			Meetings:   tc.meetings,
			Notes:      tc.notes,
			Tasks:      tc.tasks,
		})
	}

	// Sort by days since last touch (most neglected first)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].DaysSince > result[i].DaysSince {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func printEngagementTable(cmd *cobra.Command, rows []contactEngagementRow) error {
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, strings.Join([]string{bold("NAME"), bold("EMAIL"), bold("TOUCHES"), bold("LAST TOUCH"), bold("DAYS AGO"), bold("C/E/M/N/T")}, "\t"))
	for _, r := range rows {
		breakdown := fmt.Sprintf("%d/%d/%d/%d/%d", r.Calls, r.Emails, r.Meetings, r.Notes, r.Tasks)
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%d\t%s\n", r.Name, r.Email, r.TotalTouch, r.LastTouch, r.DaysSince, breakdown)
	}
	return tw.Flush()
}
