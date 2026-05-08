// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gohighlevel/internal/store"
)

type slaBreachRow struct {
	ConversationID string `json:"conversation_id"`
	LocationID     string `json:"location_id"`
	ContactID      string `json:"contact_id"`
	ContactName    string `json:"contact_name"`
	AssignedTo     string `json:"assigned_to"`
	LastInbound    string `json:"last_inbound_at"`
	MinutesSilent  int    `json:"minutes_silent"`
}

func newSlaBreachCmd(flags *rootFlags) *cobra.Command {
	var threshold string
	var businessHours bool
	var location string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "sla-breach",
		Short:       "Conversations whose last inbound message has no outbound reply within --threshold",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Detect SLA breaches: conversation threads where the last activity is an
inbound message older than --threshold (e.g. 30m, 2h, 1d) with no outbound
reply since. Optionally restrict to business hours per location.

Run 'gohighlevel-pp-cli sync' first; this is a local-store query.
`,
		Example: strings.Trim(`
  # 30-minute SLA across all locations, business hours only
  gohighlevel-pp-cli sla-breach --threshold 30m --business-hours --location all --json

  # 2-hour SLA, single location, ignore business hours
  gohighlevel-pp-cli sla-breach --threshold 2h --location loc_abc123
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("gohighlevel-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'gohighlevel-pp-cli sync' first.", err)
			}
			defer db.Close()

			d, err := time.ParseDuration(threshold)
			if err != nil {
				return fmt.Errorf("invalid --threshold %q: %w", threshold, err)
			}
			cutoff := time.Now().Add(-d).Format(time.RFC3339)

			where := []string{
				"COALESCE(json_extract(data, '$.unreadCount'), 0) > 0",
				"COALESCE(json_extract(data, '$.lastMessageDate'), json_extract(data, '$.dateUpdated'), '') < ?",
			}
			argv := []any{cutoff}
			if location != "" && location != "all" {
				where = append(where, "json_extract(data, '$.locationId') = ?")
				argv = append(argv, location)
			}

			q := fmt.Sprintf(`
				SELECT
					id,
					COALESCE(json_extract(data, '$.locationId'), '') AS loc,
					COALESCE(json_extract(data, '$.contactId'), '') AS contact_id,
					COALESCE(json_extract(data, '$.fullName'), json_extract(data, '$.contactName'), '') AS contact_name,
					COALESCE(json_extract(data, '$.assignedTo'), '') AS assigned_to,
					COALESCE(json_extract(data, '$.lastMessageDate'), json_extract(data, '$.dateUpdated'), '') AS last_inbound
				FROM conversations
				WHERE %s
				ORDER BY last_inbound ASC
				LIMIT ?
			`, strings.Join(where, " AND "))
			argv = append(argv, limit)

			rows, err := db.DB().QueryContext(cmd.Context(), q, argv...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			now := time.Now()
			var out []slaBreachRow
			for rows.Next() {
				var r slaBreachRow
				if scanErr := rows.Scan(&r.ConversationID, &r.LocationID, &r.ContactID, &r.ContactName, &r.AssignedTo, &r.LastInbound); scanErr != nil {
					continue
				}
				if r.LastInbound != "" {
					if parsed, perr := time.Parse(time.RFC3339, r.LastInbound); perr == nil {
						r.MinutesSilent = int(now.Sub(parsed).Minutes())
						if businessHours && !inBusinessHours(parsed, now) {
							continue
						}
					}
				}
				out = append(out, r)
			}

			result := struct {
				Threshold     string         `json:"threshold"`
				BusinessHours bool           `json:"business_hours"`
				Count         int            `json:"count"`
				Rows          []slaBreachRow `json:"rows"`
			}{Threshold: threshold, BusinessHours: businessHours, Count: len(out), Rows: out}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "SLA breaches — %d (threshold=%s, business-hours=%v)\n", len(out), threshold, businessHours)
			fmt.Fprintln(cmd.OutOrStdout(), "Conversation\tContact\tLocation\tMinutesSilent")
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%d\n", r.ConversationID, firstNonEmpty(r.ContactName, r.ContactID), r.LocationID, r.MinutesSilent)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&threshold, "threshold", "30m", "Reply-time threshold (e.g. 30m, 2h, 1d)")
	cmd.Flags().BoolVar(&businessHours, "business-hours", false, "Only count time inside Mon-Fri 09:00-17:00 local")
	cmd.Flags().StringVar(&location, "location", "all", "Location id, or 'all' for every synced location")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max rows")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	return cmd
}

// inBusinessHours is a coarse Mon-Fri 09:00-17:00 check that ignores
// per-location config (which the GHL API does expose under
// /locations/:id but isn't always synced). When the user opts in to
// --business-hours we apply this generic floor; per-location overrides
// would replace this once locations.business_hours sync lands.
func inBusinessHours(t time.Time, now time.Time) bool {
	wd := t.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return false
	}
	h := t.Hour()
	return h >= 9 && h < 17
}
