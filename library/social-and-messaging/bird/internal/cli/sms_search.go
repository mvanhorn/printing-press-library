// Copyright 2026 cleverai-zh. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/bird/internal/store"
	"github.com/spf13/cobra"
)

type smsSearchHit struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversationId,omitempty"`
	Direction      string `json:"direction,omitempty"`
	Status         string `json:"status,omitempty"`
	Body           string `json:"body,omitempty"`
	CreatedAt      string `json:"createdAt,omitempty"`
}

func newSmsSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		from   string
		to     string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "search <text>",
		Short: "Full-text search over message bodies, optionally filtered by sender or recipient phone number.",
		Long: `Searches the local messages and conversations_messages bodies for the
provided text, optionally narrowing by participant identifier with --from
or --to (E.164 phone number).

Local-only — run 'bird-pp-cli sync' first.`,
		Example:     "  bird-pp-cli sms search \"otp\" --json --select id,createdAt,body\n  bird-pp-cli sms search \"order\" --to +31612345678 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := args[0]
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

			// PATCH: when both --from and --to are provided, intersect the two
			// participant sets so the query restricts to conversations
			// involving BOTH numbers. The previous code
			//
			//   identifier := to
			//   if identifier == "" { identifier = from }
			//
			// silently discarded --from whenever --to was also set, so a
			// caller running `sms search "X" --from +A --to +B` got all
			// conversations involving +B (with --from quietly ignored) --
			// not the intersection they almost certainly wanted. Surfaced by
			// Greptile P1 in the PR #417 fifth review pass. Direction-aware
			// filtering (--from = outgoing, --to = incoming) is left as a
			// follow-up: it requires a direction field in the synced message
			// rows that the current schema doesn't surface uniformly.
			var convFilter map[string]struct{}
			switch {
			case from != "" && to != "":
				fromSet, ferr := conversationsForIdentifier(db, from)
				if ferr != nil {
					return ferr
				}
				toSet, terr := conversationsForIdentifier(db, to)
				if terr != nil {
					return terr
				}
				convFilter = make(map[string]struct{}, len(fromSet))
				for c := range fromSet {
					if _, ok := toSet[c]; ok {
						convFilter[c] = struct{}{}
					}
				}
			case from != "":
				convFilter, err = conversationsForIdentifier(db, from)
				if err != nil {
					return err
				}
			case to != "":
				convFilter, err = conversationsForIdentifier(db, to)
				if err != nil {
					return err
				}
			}

			hits, err := scanForBody(db, query, convFilter, limit)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&from, "from", "", "Phone number we sent from (E.164)")
	cmd.Flags().StringVar(&to, "to", "", "Phone number we sent to (E.164)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max hits to return")
	return cmd
}

func conversationsForIdentifier(db *store.Store, identifier string) (map[string]struct{}, error) {
	rows, err := db.DB().Query(`SELECT conversations_id, data FROM conversations_participants`)
	if err != nil {
		return nil, fmt.Errorf("query participants: %w", err)
	}
	defer rows.Close()
	out := make(map[string]struct{})
	for rows.Next() {
		var convID string
		var raw []byte
		if err := rows.Scan(&convID, &raw); err != nil {
			return nil, err
		}
		var p map[string]any
		if json.Unmarshal(raw, &p) != nil {
			continue
		}
		if v, ok := p["identifierValue"].(string); ok && strings.EqualFold(v, identifier) {
			out[convID] = struct{}{}
		}
	}
	// PATCH: catch mid-iteration scan errors so a truncated participant
	// set doesn't silently narrow downstream search results. See Greptile
	// P1 in PR #417 ninth review pass.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("query participants: %w", err)
	}
	return out, nil
}

func scanForBody(db *store.Store, query string, convFilter map[string]struct{}, limit int) ([]smsSearchHit, error) {
	q := strings.ToLower(query)
	out := make([]smsSearchHit, 0, 32)
	rows, err := db.DB().Query(`SELECT id, conversations_id, data FROM conversations_messages`)
	if err != nil {
		return nil, fmt.Errorf("scan conversations_messages: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		if limit > 0 && len(out) >= limit {
			break
		}
		var id, convID string
		var raw []byte
		if err := rows.Scan(&id, &convID, &raw); err != nil {
			return nil, err
		}
		if convFilter != nil {
			if _, ok := convFilter[convID]; !ok {
				continue
			}
		}
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		body := extractBodyText(m)
		if body == "" {
			continue
		}
		if !strings.Contains(strings.ToLower(body), q) {
			continue
		}
		hit := smsSearchHit{ID: id, ConversationID: convID, Body: body}
		if s, ok := m["direction"].(string); ok {
			hit.Direction = s
		}
		if s, ok := m["status"].(string); ok {
			hit.Status = s
		}
		if s, ok := m["createdAt"].(string); ok {
			hit.CreatedAt = s
		}
		out = append(out, hit)
	}
	// PATCH: surface mid-iteration scan errors; without this a query
	// failure mid-scan would silently truncate the search result set.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan conversations_messages: %w", err)
	}
	return out, nil
}

func extractBodyText(m map[string]any) string {
	if body, ok := m["body"].(map[string]any); ok {
		if textObj, ok := body["text"].(map[string]any); ok {
			if t, ok := textObj["text"].(string); ok {
				return t
			}
		}
	}
	if t, ok := m["body"].(string); ok {
		return t
	}
	return ""
}
