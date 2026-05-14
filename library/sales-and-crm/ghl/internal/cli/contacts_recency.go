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

func newContactsRecencyCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var tagFilter string
	var overWindow string
	var limit int

	cmd := &cobra.Command{
		Use:         "recency",
		Short:       "Last-inbound / last-outbound timestamps per contact",
		Long:        "Joins contacts and messages locally; shows last inbound and last outbound message timestamps per contact and sorts by oldest-last-touch. Useful for coach roster reviews.",
		Example:     "  ghl-pp-cli contacts recency --tag client --over 14d --json\n  ghl-pp-cli contacts recency --over 30d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("ghl-pp-cli")
			}
			var thresholdAt string
			if overWindow != "" {
				dur, err := parseSince(overWindow)
				if err != nil {
					return err
				}
				thresholdAt = time.Now().Add(-dur).UTC().Format(time.RFC3339)
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'ghl-pp-cli sync' first", err)
			}
			defer db.Close()

			rows, err := db.Query(`SELECT id, data FROM "contacts"`)
			if err != nil {
				return fmt.Errorf("querying contacts: %w", err)
			}
			defer rows.Close()

			type rec struct {
				ID           string `json:"id"`
				Name         string `json:"name,omitempty"`
				Phone        string `json:"phone,omitempty"`
				LastInbound  string `json:"last_inbound_at,omitempty"`
				LastOutbound string `json:"last_outbound_at,omitempty"`
				LastTouch    string `json:"last_touch_at,omitempty"`
				DaysSince    int    `json:"days_since_last_touch"`
				Killswitch   string `json:"killswitch,omitempty"`
			}
			var hits []rec
			now := time.Now()
			for rows.Next() {
				var id string
				var data []byte
				if err := rows.Scan(&id, &data); err != nil {
					continue
				}
				if tagFilter != "" && !contactHasTag(data, tagFilter) {
					continue
				}
				inb, outb := contactLastMessages(db, id)
				touch := inb
				if outb > touch {
					touch = outb
				}
				if thresholdAt != "" && touch >= thresholdAt {
					continue
				}
				days := 0
				if touch != "" {
					if t, err := time.Parse(time.RFC3339, touch); err == nil {
						days = int(now.Sub(t).Hours() / 24)
					}
				} else {
					days = 9999 // never-touched
				}
				hits = append(hits, rec{
					ID:           id,
					Name:         strings.TrimSpace(extractStr(data, "firstName") + " " + extractStr(data, "lastName")),
					Phone:        extractStr(data, "phone"),
					LastInbound:  inb,
					LastOutbound: outb,
					LastTouch:    touch,
					DaysSince:    days,
					Killswitch:   killswitchTagOf(data),
				})
			}
			sort.Slice(hits, func(i, j int) bool { return hits[i].DaysSince > hits[j].DaysSince })
			if limit > 0 && len(hits) > limit {
				hits = hits[:limit]
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"over": overWindow, "tag": tagFilter, "count": len(hits), "contacts": hits}, flags)
			}
			if len(hits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching contacts found.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-26s  %-5s  %-22s  %s\n", "ID", "DAYS", "NAME", "LAST TOUCH")
			fmt.Fprintf(cmd.OutOrStdout(), "%-26s  %-5s  %-22s  %s\n", strings.Repeat("-", 26), strings.Repeat("-", 5), strings.Repeat("-", 22), strings.Repeat("-", 19))
			for _, h := range hits {
				name := h.Name
				if len(name) > 22 {
					name = name[:19] + "..."
				}
				touch := h.LastTouch
				if touch == "" {
					touch = "(never)"
				}
				days := fmt.Sprintf("%d", h.DaysSince)
				if h.DaysSince >= 9999 {
					days = "n/a"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-26s  %-5s  %-22s  %s\n", h.ID, days, name, touch)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/ghl-pp-cli/data.db)")
	cmd.Flags().StringVar(&tagFilter, "tag", "", "Restrict to contacts with this tag (case-insensitive substring match)")
	cmd.Flags().StringVar(&overWindow, "over", "", "Only show contacts whose last touchpoint is older than this window (e.g. 14d)")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max rows to return")
	return cmd
}

func contactHasTag(data []byte, want string) bool {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return false
	}
	rawTags, _ := obj["tags"].([]any)
	wl := strings.ToLower(want)
	for _, t := range rawTags {
		if s, ok := t.(string); ok {
			if strings.Contains(strings.ToLower(s), wl) {
				return true
			}
		}
	}
	return false
}

func contactLastMessages(db *store.Store, contactID string) (string, string) {
	rows, err := db.DB().Query(
		`SELECT json_extract(data, '$.dateAdded'),
		         LOWER(COALESCE(json_extract(data, '$.direction'), ''))
		  FROM "messages"
		  WHERE json_extract(data, '$.contactId') = ?`,
		contactID,
	)
	if err != nil {
		return "", ""
	}
	defer rows.Close()
	inb := ""
	outb := ""
	for rows.Next() {
		var ts, dir string
		if err := rows.Scan(&ts, &dir); err != nil {
			continue
		}
		if ts == "" {
			continue
		}
		if dir == "inbound" {
			if ts > inb {
				inb = ts
			}
		} else if dir == "outbound" {
			if ts > outb {
				outb = ts
			}
		}
	}
	return inb, outb
}
