// Copyright 2026 alex-puckhaber. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/ghl/internal/store"

	"github.com/spf13/cobra"
)

// newKPICmd is the top-level group for cross-entity daily KPI commands.
func newKPICmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "kpi",
		Short:       "Cross-entity KPI tickers from the local store",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newKPITodayCmd(flags))
	return cmd
}

func newKPITodayCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var dayStart string

	cmd := &cobra.Command{
		Use:         "today",
		Short:       "One-line metric ticker for today",
		Long:        "Aggregates new contacts, SMS sent, appointments booked, opportunities moved, and kill-switch trips for the rolling 24-hour window. JSON-friendly for dashboard cron jobs.",
		Example:     "  ghl-pp-cli kpi today --json\n  ghl-pp-cli kpi today --day-start 06:00",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("ghl-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'ghl-pp-cli sync' first", err)
			}
			defer db.Close()

			cutoff, label := computeDayCutoff(dayStart)
			cutoffStr := cutoff.Format(time.RFC3339)

			newContacts := countSince(db, `"contacts"`, []string{"$.dateAdded", "$.createdAt", "$.created_at"}, cutoffStr)
			smsSent := countSinceWhere(db, `"messages"`,
				[]string{"$.dateAdded", "$.createdAt"},
				cutoffStr,
				`(LOWER(json_extract(data, '$.direction')) = 'outbound' AND LOWER(COALESCE(json_extract(data, '$.messageType'), json_extract(data, '$.type'))) = 'sms')`,
			)
			apptsBooked := countSince(db, `"appointments"`,
				[]string{"$.dateAdded", "$.createdAt", "$.created_at", "$.startTime"},
				cutoffStr,
			)
			oppsMoved := countSince(db, `"opportunities"`, []string{"$.updatedAt", "$.updated_at"}, cutoffStr)
			killswitchTrips := countKillswitchTrips(db, cutoffStr)

			report := map[string]any{
				"window":           label,
				"cutoff":           cutoffStr,
				"new_contacts":     newContacts,
				"sms_sent":         smsSent,
				"appointments":     apptsBooked,
				"opportunities":    oppsMoved,
				"killswitch_trips": killswitchTrips,
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"KPI %s | contacts:%d sms:%d appts:%d opps:%d killswitch:%d\n",
				label, newContacts, smsSent, apptsBooked, oppsMoved, killswitchTrips,
			)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/ghl-pp-cli/data.db)")
	cmd.Flags().StringVar(&dayStart, "day-start", "00:00", "Start of the day in 24h local time (default: midnight)")
	return cmd
}

func computeDayCutoff(dayStart string) (time.Time, string) {
	now := time.Now()
	parts := strings.SplitN(strings.TrimSpace(dayStart), ":", 2)
	hour := 0
	min := 0
	if len(parts) > 0 {
		fmt.Sscanf(parts[0], "%d", &hour)
	}
	if len(parts) > 1 {
		fmt.Sscanf(parts[1], "%d", &min)
	}
	cutoff := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
	if cutoff.After(now) {
		cutoff = cutoff.AddDate(0, 0, -1)
	}
	label := cutoff.Format("2006-01-02") + " (since " + cutoff.Format("15:04") + ")"
	return cutoff, label
}

func countSince(db *store.Store, table string, jsonPaths []string, cutoff string) int {
	if len(jsonPaths) == 0 {
		return 0
	}
	coalesced := buildCoalesce(jsonPaths)
	q := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s >= ?`, table, coalesced)
	var n int
	row := db.DB().QueryRow(q, cutoff)
	_ = row.Scan(&n)
	return n
}

func countSinceWhere(db *store.Store, table string, jsonPaths []string, cutoff, extraWhere string) int {
	coalesced := buildCoalesce(jsonPaths)
	q := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s >= ? AND %s`, table, coalesced, extraWhere)
	var n int
	row := db.DB().QueryRow(q, cutoff)
	_ = row.Scan(&n)
	return n
}

func buildCoalesce(paths []string) string {
	if len(paths) == 1 {
		return fmt.Sprintf("json_extract(data, '%s')", paths[0])
	}
	parts := make([]string, 0, len(paths))
	for _, p := range paths {
		parts = append(parts, fmt.Sprintf("json_extract(data, '%s')", p))
	}
	return "COALESCE(" + strings.Join(parts, ", ") + ")"
}

// countKillswitchTrips approximates the number of NEW kill-switch tag
// applications today by counting contacts whose `dateUpdated` is within the
// window AND who currently have a kill-switch tag. It's a best-effort metric:
// without per-tag-application audit logs in the API, we use "updated today
// AND now has kill-switch" as a proxy for "got kill-switched today."
func countKillswitchTrips(db *store.Store, cutoff string) int {
	rows, err := db.DB().Query(
		`SELECT data FROM "contacts"
		  WHERE COALESCE(json_extract(data, '$.dateUpdated'), json_extract(data, '$.updatedAt'), json_extract(data, '$.updated_at')) >= ?`,
		cutoff,
	)
	if err != nil {
		return 0
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		if killswitchTagOf(data) != "" {
			n++
		}
	}
	return n
}
