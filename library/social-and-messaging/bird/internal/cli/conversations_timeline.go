// Copyright 2026 Stephan Stoeber and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/bird/internal/store"
	"github.com/spf13/cobra"
)

type timelineRow struct {
	Timestamp string `json:"timestamp"`
	Kind      string `json:"kind"` // "message" or "participant"
	ID        string `json:"id"`
	Direction string `json:"direction,omitempty"`
	Status    string `json:"status,omitempty"`
	Body      string `json:"body,omitempty"`
	Type      string `json:"type,omitempty"`
}

type timelineView struct {
	ConversationID string        `json:"conversationId"`
	Status         string        `json:"status,omitempty"`
	Participants   []string      `json:"participants,omitempty"`
	Timeline       []timelineRow `json:"timeline"`
}

func newConversationsTimelineCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "timeline <conversation_id>",
		Short: "Render a conversation's messages, participants, and delivery state in chronological order.",
		Long: `Joins the local conversations, conversations_messages, and
conversations_participants tables to produce a single chronological view of
who said what, when, with delivery state per outbound message.

Local-only — run 'bird-pp-cli sync' first.`,
		Example:     "  bird-pp-cli conversations timeline conv_42 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			convID := args[0]
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("bird-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			view := timelineView{ConversationID: convID, Timeline: []timelineRow{}}

			// Conversation metadata
			var status string
			_ = db.DB().QueryRow(`SELECT status FROM conversations WHERE id = ?`, convID).Scan(&status)
			view.Status = status

			// Participants
			pRows, err := db.DB().Query(`SELECT id, data FROM conversations_participants WHERE conversations_id = ?`, convID)
			if err == nil {
				for pRows.Next() {
					var id string
					var raw []byte
					if err := pRows.Scan(&id, &raw); err == nil {
						var p map[string]any
						_ = json.Unmarshal(raw, &p)
						label := id
						if name, ok := p["displayName"].(string); ok && name != "" {
							label = name
						}
						view.Participants = append(view.Participants, label)
					}
				}
				// PATCH: rows.Err() audit so a participants-scan failure
				// surfaces instead of silently producing a partial label list.
				if scanErr := pRows.Err(); scanErr != nil {
					view.Participants = append(view.Participants, fmt.Sprintf("(participants scan error: %v)", scanErr))
				}
				pRows.Close()
			}

			// Messages
			mRows, err := db.DB().Query(`SELECT id, data FROM conversations_messages WHERE conversations_id = ?`, convID)
			if err != nil {
				return fmt.Errorf("query conversation messages: %w", err)
			}
			defer mRows.Close()
			for mRows.Next() {
				var id string
				var raw []byte
				if err := mRows.Scan(&id, &raw); err != nil {
					return err
				}
				row := timelineRow{Kind: "message", ID: id}
				var m map[string]any
				if json.Unmarshal(raw, &m) == nil {
					if s, ok := m["createdAt"].(string); ok {
						row.Timestamp = s
					}
					if s, ok := m["direction"].(string); ok {
						row.Direction = s
					}
					if s, ok := m["status"].(string); ok {
						row.Status = s
					}
					if s, ok := m["type"].(string); ok {
						row.Type = s
					}
					if body, ok := m["body"].(map[string]any); ok {
						if textObj, ok := body["text"].(map[string]any); ok {
							if t, ok := textObj["text"].(string); ok {
								row.Body = t
							}
						}
					}
				}
				view.Timeline = append(view.Timeline, row)
			}
			// PATCH: rows.Err() audit on the messages scan so a truncated
			// timeline is reported instead of silently producing partial
			// output.
			if err := mRows.Err(); err != nil {
				return fmt.Errorf("scan conversation messages: %w", err)
			}
			sort.SliceStable(view.Timeline, func(i, j int) bool {
				return view.Timeline[i].Timestamp < view.Timeline[j].Timestamp
			})
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
