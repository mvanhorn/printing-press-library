// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// `convo thread` — chronological SMS+email+call thread for a single contact
// reconstructed from the local messages table.
package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newConvoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convo",
		Short: "Conversation reconstruction and lookup",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newConvoThreadCmd(flags))
	return cmd
}

func newConvoThreadCmd(flags *rootFlags) *cobra.Command {
	var contact string
	cmd := &cobra.Command{
		Use:         "thread",
		Short:       "Reconstruct chronological message thread for a contact (SMS+email+call)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if contact == "" {
				if flags.asJSON {
					_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
						"error": "--contact is required (email or contact ID)",
					})
				} else {
					fmt.Fprintln(cmd.ErrOrStderr(), "error: --contact is required (email or contact ID)")
				}
				return usageErr(fmt.Errorf("--contact is required"))
			}
			ctx := cmd.Context()
			s, err := openGHLStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()

			contactID := contact
			if strings.Contains(contact, "@") {
				var hit sql.NullString
				err := s.DB().QueryRowContext(ctx, `
                    SELECT id FROM resources
                    WHERE resource_type IN ('contacts', 'contacts_contacts')
                      AND lower(COALESCE(json_extract(data, '$.email'), '')) = lower(?)
                    LIMIT 1
                `, contact).Scan(&hit)
				if err == nil && hit.Valid {
					contactID = hit.String
				} else {
					return notFoundErr(fmt.Errorf("no contact with email %q in local cache", contact))
				}
			}

			rows, err := s.DB().QueryContext(ctx, `
                SELECT data FROM messages
                WHERE json_extract(data, '$.contactId') = ?
                ORDER BY COALESCE(json_extract(data, '$.dateAdded'),
                                  json_extract(data, '$.dateUpdated')) ASC
            `, contactID)
			if err != nil {
				return apiErr(fmt.Errorf("query messages: %w", err))
			}
			defer rows.Close()

			type msg struct {
				Timestamp string `json:"timestamp"`
				Channel   string `json:"channel"`
				Direction string `json:"direction"`
				From      string `json:"from"`
				Body      string `json:"body"`
			}
			var msgs []msg
			for rows.Next() {
				var raw []byte
				if err := rows.Scan(&raw); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal(raw, &obj); err != nil {
					continue
				}
				ts, _ := obj["dateAdded"].(string)
				if ts == "" {
					ts, _ = obj["dateUpdated"].(string)
				}
				channel, _ := obj["messageType"].(string)
				if channel == "" {
					channel, _ = obj["type"].(string)
				}
				direction, _ := obj["direction"].(string)
				from, _ := obj["from"].(string)
				if from == "" {
					from, _ = obj["fromNumber"].(string)
				}
				body, _ := obj["body"].(string)
				if body == "" {
					body, _ = obj["message"].(string)
				}
				msgs = append(msgs, msg{
					Timestamp: ts,
					Channel:   channel,
					Direction: direction,
					From:      from,
					Body:      body,
				})
			}
			sort.SliceStable(msgs, func(i, j int) bool { return msgs[i].Timestamp < msgs[j].Timestamp })

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), msgs, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "thread for contact %s (%d messages):\n", contactID, len(msgs))
			for _, m := range msgs {
				fmt.Fprintf(w, "[%s] %s/%s %s: %s\n", m.Timestamp, m.Channel, m.Direction, m.From, m.Body)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&contact, "contact", "", "Contact email or ID")
	return cmd
}
