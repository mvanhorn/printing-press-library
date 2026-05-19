// Copyright 2026 cleverai-zh. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/bird/internal/store"
	"github.com/spf13/cobra"
)

type customerMessageRow struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversationId,omitempty"`
	Direction      string `json:"direction,omitempty"`
	Status         string `json:"status,omitempty"`
	Body           string `json:"body,omitempty"`
	CreatedAt      string `json:"createdAt,omitempty"`
}

func newMessagesFromCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "from <e164>",
		Short: "List every message exchanged with one phone number across all conversations.",
		Long: `Joins the local participants → conversations → messages tables to return every
message the workspace has exchanged with a given phone number, regardless of
which conversation or channel it lived in. Reads from the local store; run
'bird-pp-cli sync' first to populate it.`,
		Example:     "  bird-pp-cli messages from +31612345678 --json\n  bird-pp-cli messages from +14155550100 --json --select id,createdAt,body",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			e164 := args[0]
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

			rows, err := queryCustomerMessages(db, e164)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to ~/.cache/bird-pp-cli/store.db)")
	return cmd
}

func queryCustomerMessages(db *store.Store, identifier string) ([]customerMessageRow, error) {
	out := make([]customerMessageRow, 0, 32)

	// Pull conversation_messages whose participant identifierValue matches.
	// The participants are stored as JSON in conversations_participants with
	// columns conversations_id (FK) and data (JSON containing identifierKey/identifierValue).
	// Strategy: fetch all conversations_participants once, filter in Go (rare
	// enough that scanning the table is acceptable for v1).
	q := `SELECT conversations_id, data FROM conversations_participants`
	pRows, err := db.DB().Query(q)
	if err != nil {
		return nil, fmt.Errorf("query participants: %w", err)
	}
	defer pRows.Close()
	convIDs := make(map[string]struct{})
	for pRows.Next() {
		var convID string
		var raw []byte
		if err := pRows.Scan(&convID, &raw); err != nil {
			return nil, err
		}
		var p map[string]any
		if json.Unmarshal(raw, &p) != nil {
			continue
		}
		if v, ok := p["identifierValue"].(string); ok && strings.EqualFold(v, identifier) {
			convIDs[convID] = struct{}{}
		}
	}
	// PATCH: surface mid-iteration participants scan errors. See Greptile
	// P1 in PR #417 ninth review pass.
	if err := pRows.Err(); err != nil {
		return nil, fmt.Errorf("query participants: %w", err)
	}
	if len(convIDs) == 0 {
		return out, nil
	}
	for cid := range convIDs {
		mq := `SELECT id, data FROM conversations_messages WHERE conversations_id = ? ORDER BY id`
		mRows, err := db.DB().Query(mq, cid)
		if err != nil {
			return nil, fmt.Errorf("query messages for conversation %s: %w", cid, err)
		}
		for mRows.Next() {
			var id string
			var raw []byte
			if err := mRows.Scan(&id, &raw); err != nil {
				mRows.Close()
				return nil, err
			}
			row := customerMessageRow{ID: id, ConversationID: cid}
			var m map[string]any
			if json.Unmarshal(raw, &m) == nil {
				if s, ok := m["direction"].(string); ok {
					row.Direction = s
				}
				if s, ok := m["status"].(string); ok {
					row.Status = s
				}
				if s, ok := m["createdAt"].(string); ok {
					row.CreatedAt = s
				}
				if body, ok := m["body"].(map[string]any); ok {
					if textObj, ok := body["text"].(map[string]any); ok {
						if t, ok := textObj["text"].(string); ok {
							row.Body = t
						}
					}
				}
			}
			out = append(out, row)
		}
		// PATCH: same rows.Err() audit on the per-conversation messages scan.
		if err := mRows.Err(); err != nil {
			mRows.Close()
			return nil, fmt.Errorf("scan messages for conversation %s: %w", cid, err)
		}
		mRows.Close()
	}
	return out, nil
}
