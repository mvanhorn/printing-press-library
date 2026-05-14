// Copyright 2026 alex-puckhaber. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/ghl/internal/store"

	"github.com/spf13/cobra"
)

func newInboxCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "inbox",
		Short:       "Conversation triage helpers",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newInboxTriageCmd(flags))
	return cmd
}

func newInboxTriageCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string
	var limit int
	var includeAIOff bool

	cmd := &cobra.Command{
		Use:         "triage",
		Short:       "Unread inbound conversations idle for the window, kill-switch-aware",
		Long:        "Returns conversations with unread inbound messages where no outbound reply happened within the window AND the contact is not tagged `ai off` (unless --include-ai-off). One-line per conversation for token-efficient agent loops.",
		Example:     "  ghl-pp-cli inbox triage --since 4h --json\n  ghl-pp-cli inbox triage --since 24h",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("ghl-pp-cli")
			}
			dur, err := parseSince(since)
			if err != nil {
				return err
			}
			cutoff := time.Now().Add(-dur).UTC().Format(time.RFC3339)

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'ghl-pp-cli sync' first", err)
			}
			defer db.Close()

			// Pull conversations with unread_count > 0; we'll re-check direction
			// of the latest message and the contact's kill-switch state in Go.
			rows, err := db.Query(`SELECT id, data FROM "conversations" WHERE unread_count > 0`)
			if err != nil {
				return fmt.Errorf("querying conversations: %w", err)
			}
			defer rows.Close()

			type triageHit struct {
				ConversationID string `json:"conversation_id"`
				ContactID      string `json:"contact_id"`
				ContactName    string `json:"contact_name,omitempty"`
				Unread         int    `json:"unread"`
				LastInboundAt  string `json:"last_inbound_at,omitempty"`
				IdleHours      int    `json:"idle_hours"`
				Killswitch     string `json:"killswitch,omitempty"`
			}
			var hits []triageHit
			now := time.Now()

			for rows.Next() {
				if limit > 0 && len(hits) >= limit {
					break
				}
				var convID string
				var data []byte
				if err := rows.Scan(&convID, &data); err != nil {
					continue
				}
				contactID := firstString(data, "contactId", "contact_id")
				unread := 0
				fmt.Sscanf(firstString(data, "unreadCount"), "%d", &unread)
				if unread == 0 {
					// fallback: read column-promoted unread_count via a second query
					row := db.DB().QueryRow(`SELECT unread_count FROM "conversations" WHERE id = ?`, convID)
					var rawUnread *float64
					_ = row.Scan(&rawUnread)
					if rawUnread != nil {
						unread = int(*rawUnread)
					}
				}
				if unread == 0 {
					continue
				}
				lastInbound, lastOutbound := lastMessageDirections(db, convID)
				if lastInbound == "" {
					continue
				}
				if lastInbound < cutoff && lastOutbound < lastInbound {
					// Inbound is older than the cutoff window — drop noisy stale rows.
					continue
				}
				if lastInbound > lastOutbound {
					// Has unread + most recent message is inbound -> needs reply.
					idleHours := 0
					if t, err := time.Parse(time.RFC3339, lastInbound); err == nil {
						idleHours = int(now.Sub(t).Hours())
					}
					ks := ""
					contactName := ""
					if contactID != "" {
						if cdata, ok := lookupContact(db, contactID); ok {
							ks = killswitchTagOf(cdata)
							contactName = strings.TrimSpace(extractStr(cdata, "firstName") + " " + extractStr(cdata, "lastName"))
						}
					}
					if ks == "ai off" && !includeAIOff {
						continue
					}
					hits = append(hits, triageHit{
						ConversationID: convID,
						ContactID:      contactID,
						ContactName:    contactName,
						Unread:         unread,
						LastInboundAt:  lastInbound,
						IdleHours:      idleHours,
						Killswitch:     ks,
					})
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"since": since, "cutoff": cutoff, "count": len(hits), "conversations": hits}, flags)
			}
			if len(hits) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Inbox clear (since %s).\n", cutoff)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d conversation(s) need attention since %s:\n\n", len(hits), cutoff)
			for _, h := range hits {
				ksMarker := ""
				if h.Killswitch != "" {
					ksMarker = " [" + h.Killswitch + "]"
				}
				name := h.ContactName
				if name == "" {
					name = h.ContactID
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %2dh idle  unread=%d  %s%s\n", h.ConversationID, h.IdleHours, h.Unread, name, ksMarker)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/ghl-pp-cli/data.db)")
	cmd.Flags().StringVar(&since, "since", "4h", "Idle window: e.g. 1h, 4h, 24h (default: 4h)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max conversations to return")
	cmd.Flags().BoolVar(&includeAIOff, "include-ai-off", false, "Include conversations where the contact is tagged `ai off` (default: hide)")
	return cmd
}

// lastMessageDirections returns (lastInboundAt, lastOutboundAt) for the given
// conversation by inspecting cached messages.
func lastMessageDirections(db *store.Store, convID string) (string, string) {
	rows, err := db.DB().Query(
		`SELECT json_extract(data, '$.dateAdded'),
		         LOWER(COALESCE(json_extract(data, '$.direction'), ''))
		  FROM "messages"
		  WHERE conversations_id = ?`,
		convID,
	)
	if err != nil {
		return "", ""
	}
	defer rows.Close()
	lastInbound := ""
	lastOutbound := ""
	for rows.Next() {
		var ts, dir string
		if err := rows.Scan(&ts, &dir); err != nil {
			continue
		}
		if ts == "" {
			continue
		}
		if dir == "inbound" {
			if ts > lastInbound {
				lastInbound = ts
			}
		} else {
			if ts > lastOutbound {
				lastOutbound = ts
			}
		}
	}
	return lastInbound, lastOutbound
}
