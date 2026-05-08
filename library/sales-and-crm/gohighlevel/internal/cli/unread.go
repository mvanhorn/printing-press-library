// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gohighlevel/internal/store"
)

type unreadRow struct {
	ConversationID string `json:"conversation_id"`
	LocationID     string `json:"location_id"`
	ContactID      string `json:"contact_id"`
	ContactName    string `json:"contact_name"`
	AssignedTo     string `json:"assigned_to"`
	UnreadCount    int    `json:"unread_count"`
	LastInbound    string `json:"last_inbound_at"`
	Type           string `json:"type"`
}

func newUnreadCmd(flags *rootFlags) *cobra.Command {
	var location string
	var since string
	var assignedTo string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "unread",
		Short:       "List inbound conversations with no outbound reply across one or all locations",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `List conversation threads where the most-recent inbound message has no
outbound reply after it. Cross-location aggregation no MCP server can do.

--location all aggregates across every synced location.
--since 1h  filters to threads with inbound activity in the window.
--assigned-to me uses the env var GHL_USER_ID; pass an explicit user id otherwise.
`,
		Example: strings.Trim(`
  # All locations, my queue, fresh inbound only
  gohighlevel-pp-cli unread --location all --since 1h --assigned-to me --json

  # One location, top 50
  gohighlevel-pp-cli unread --location loc_abc123 --limit 50
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

			where := []string{"COALESCE(json_extract(data, '$.unreadCount'), 0) > 0"}
			args2 := []any{}
			if location != "" && location != "all" {
				where = append(where, "json_extract(data, '$.locationId') = ?")
				args2 = append(args2, location)
			}
			if since != "" {
				ts, err := parseSinceDuration(since)
				if err != nil {
					return fmt.Errorf("invalid --since value %q: %w", since, err)
				}
				where = append(where, "COALESCE(json_extract(data, '$.lastMessageDate'), json_extract(data, '$.dateUpdated'), '') >= ?")
				args2 = append(args2, ts.Format(time.RFC3339))
			}
			if assignedTo != "" {
				userID := assignedTo
				if userID == "me" {
					if env := os.Getenv("GHL_USER_ID"); env != "" {
						userID = env
					}
				}
				if userID != "me" && userID != "" {
					where = append(where, "json_extract(data, '$.assignedTo') = ?")
					args2 = append(args2, userID)
				}
			}

			q := fmt.Sprintf(`
				SELECT
					id,
					COALESCE(json_extract(data, '$.locationId'), '') AS loc,
					COALESCE(json_extract(data, '$.contactId'), '') AS contact_id,
					COALESCE(json_extract(data, '$.fullName'), json_extract(data, '$.contactName'), '') AS contact_name,
					COALESCE(json_extract(data, '$.assignedTo'), '') AS assigned_to,
					COALESCE(json_extract(data, '$.unreadCount'), 0) AS unread,
					COALESCE(json_extract(data, '$.lastMessageDate'), json_extract(data, '$.dateUpdated'), '') AS last_inbound,
					COALESCE(json_extract(data, '$.type'), 'TYPE_UNKNOWN') AS type
				FROM conversations
				WHERE %s
				ORDER BY last_inbound DESC
				LIMIT ?
			`, strings.Join(where, " AND "))
			args2 = append(args2, limit)

			rows, err := db.DB().QueryContext(cmd.Context(), q, args2...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			var out []unreadRow
			for rows.Next() {
				var r unreadRow
				if scanErr := rows.Scan(&r.ConversationID, &r.LocationID, &r.ContactID, &r.ContactName, &r.AssignedTo, &r.UnreadCount, &r.LastInbound, &r.Type); scanErr != nil {
					continue
				}
				out = append(out, r)
			}

			result := struct {
				Count int         `json:"count"`
				Since string      `json:"since,omitempty"`
				Rows  []unreadRow `json:"rows"`
			}{
				Count: len(out),
				Since: since,
				Rows:  out,
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Unread — %d conversation(s)\n", len(out))
			fmt.Fprintln(cmd.OutOrStdout(), "Conversation\tContact\tLocation\tUnread\tLast")
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%d\t%s\n",
					r.ConversationID, firstNonEmpty(r.ContactName, r.ContactID), r.LocationID, r.UnreadCount, r.LastInbound)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&location, "location", "all", "Location id, or 'all' for every synced location")
	cmd.Flags().StringVar(&since, "since", "", "Only threads with inbound activity in this window (e.g. 1h, 7d)")
	cmd.Flags().StringVar(&assignedTo, "assigned-to", "", "User id, or 'me' (requires GHL_USER_ID env)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max rows")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	return cmd
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}
