// Copyright 2026 alex-puckhaber. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/ghl/internal/store"

	"github.com/spf13/cobra"
)

// newActivityCmd renders a cross-entity timeline of recent activity in the
// location. Combines contacts created, messages inbound/outbound, opportunity
// stage changes, and appointments booked into a single chronological view.
// Local-store only; run `sync --full` first.
func newActivityCmd(flags *rootFlags) *cobra.Command {
	var since string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "activity",
		Short:       "Recent contact, message, opportunity, and appointment activity",
		Long:        "Union of contacts created, inbound/outbound messages, opportunity stage changes, and appointments booked, ordered by timestamp descending. Reads only the local store; run `sync --full` first.",
		Example:     "  ghl-pp-cli activity --since 24h\n  ghl-pp-cli activity --since 7d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("ghl-pp-cli")
			}
			d, err := parseSince(since)
			if err != nil {
				return err
			}
			cutoff := time.Now().Add(-d).UTC().Format(time.RFC3339)
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'ghl-pp-cli sync' first", err)
			}
			defer db.Close()

			type event struct {
				At      string `json:"at"`
				Kind    string `json:"kind"`
				ID      string `json:"id"`
				Subject string `json:"subject,omitempty"`
				Contact string `json:"contact,omitempty"`
			}
			var events []event

			// New contacts
			contactRows, err := db.Query(
				`SELECT id, data FROM "contacts"
				  WHERE COALESCE(
				          json_extract(data, '$.dateAdded'),
				          json_extract(data, '$.created_at'),
				          json_extract(data, '$.createdAt')
				        ) >= ?`,
				cutoff,
			)
			if err == nil {
				defer contactRows.Close()
				for contactRows.Next() {
					var id string
					var data []byte
					if err := contactRows.Scan(&id, &data); err != nil {
						continue
					}
					ts := firstString(data, "dateAdded", "createdAt", "created_at")
					name := strings.TrimSpace(firstString(data, "firstName") + " " + firstString(data, "lastName"))
					events = append(events, event{At: ts, Kind: "contact.created", ID: id, Contact: name})
				}
			}

			// Messages
			msgRows, err := db.Query(
				`SELECT id, data FROM "messages"
				  WHERE COALESCE(
				          json_extract(data, '$.dateAdded'),
				          json_extract(data, '$.created_at'),
				          json_extract(data, '$.createdAt')
				        ) >= ?`,
				cutoff,
			)
			if err == nil {
				defer msgRows.Close()
				for msgRows.Next() {
					var id string
					var data []byte
					if err := msgRows.Scan(&id, &data); err != nil {
						continue
					}
					ts := firstString(data, "dateAdded", "createdAt", "created_at")
					dir := strings.ToLower(firstString(data, "direction"))
					kind := "message.outbound"
					if dir == "inbound" {
						kind = "message.inbound"
					}
					body := firstString(data, "body", "subject")
					if len(body) > 60 {
						body = body[:57] + "..."
					}
					events = append(events, event{At: ts, Kind: kind, ID: id, Subject: body, Contact: firstString(data, "contactId")})
				}
			}

			// Opportunity stage changes
			oppRows, err := db.Query(
				`SELECT id, data FROM "opportunities"
				  WHERE COALESCE(
				          json_extract(data, '$.updatedAt'),
				          json_extract(data, '$.updated_at')
				        ) >= ?`,
				cutoff,
			)
			if err == nil {
				defer oppRows.Close()
				for oppRows.Next() {
					var id string
					var data []byte
					if err := oppRows.Scan(&id, &data); err != nil {
						continue
					}
					ts := firstString(data, "updatedAt", "updated_at")
					events = append(events, event{At: ts, Kind: "opportunity.updated", ID: id, Subject: firstString(data, "name", "title")})
				}
			}

			// Appointments
			apptRows, err := db.Query(
				`SELECT id, data FROM "appointments"
				  WHERE COALESCE(
				          json_extract(data, '$.dateAdded'),
				          json_extract(data, '$.createdAt'),
				          json_extract(data, '$.created_at'),
				          json_extract(data, '$.startTime')
				        ) >= ?`,
				cutoff,
			)
			if err == nil {
				defer apptRows.Close()
				for apptRows.Next() {
					var id string
					var data []byte
					if err := apptRows.Scan(&id, &data); err != nil {
						continue
					}
					ts := firstString(data, "dateAdded", "createdAt", "created_at", "startTime")
					events = append(events, event{At: ts, Kind: "appointment.booked", ID: id, Subject: firstString(data, "title"), Contact: firstString(data, "contactId")})
				}
			}

			sort.Slice(events, func(i, j int) bool { return events[i].At > events[j].At })
			if limit > 0 && len(events) > limit {
				events = events[:limit]
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"since": since, "cutoff": cutoff, "events": events, "count": len(events)}, flags)
			}
			if len(events) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No activity since %s.\nHint: run 'ghl-pp-cli sync --full' if the local store is empty.\n", cutoff)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Activity since %s (%d events):\n\n", cutoff, len(events))
			for _, e := range events {
				detail := e.Subject
				if e.Contact != "" {
					if detail != "" {
						detail = e.Contact + " — " + detail
					} else {
						detail = e.Contact
					}
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %-22s  %s\n", e.At, e.Kind, detail)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "24h", "Time window: e.g. 4h, 24h, 7d, 30d (default: 24h)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/ghl-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max events to return")
	return cmd
}

// parseSince accepts `24h`, `7d`, `30m`, `1d4h`, etc., returning a duration.
// Bare integers are interpreted as hours.
func parseSince(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 24 * time.Hour, nil
	}
	// Convert `Nd` to `N*24h` so time.ParseDuration accepts it.
	if strings.HasSuffix(s, "d") {
		trim := strings.TrimSuffix(s, "d")
		// Allow combined like `1d4h` — split on `d`
		if idx := strings.Index(s, "d"); idx >= 0 {
			daysPart := s[:idx]
			rest := s[idx+1:]
			var days int
			if _, err := fmt.Sscanf(daysPart, "%d", &days); err == nil {
				combined := fmt.Sprintf("%dh%s", days*24, rest)
				if d, err := time.ParseDuration(combined); err == nil {
					return d, nil
				}
			}
			_ = trim
		}
	}
	return time.ParseDuration(s)
}

// firstString returns the first non-empty top-level field from a JSON object.
func firstString(data []byte, keys ...string) string {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := obj[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}
