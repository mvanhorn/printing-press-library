// Copyright 2026 daniel-larson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"flowiseai-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

func newPredictSearchCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string
	var usedTool string
	var citedStore string
	var chatflowID string
	var limit int

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Full-text search across recorded predictions and chat messages",
		Long: `Search the locally synced predictions and chat messages with optional filters
on time window, the cited document store, the tool the flow used, and the
chatflow that produced the prediction.

The agent-friendly columns (--select chatId,question,text) make this the
audit and debug primary for "what did the agent say last week."`,
		Example: "  flowiseai-pp-cli predict search \"mortgage rate\" --since 7d --json --select chatId,question,text",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("flowiseai-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			cutoff := ""
			if sinceStr != "" {
				cutoffTS, dErr := parseSinceDuration(sinceStr)
				if dErr != nil {
					return usageErr(dErr)
				}
				cutoff = cutoffTS.UTC().Format(time.RFC3339)
			}

			like := "%" + strings.ReplaceAll(query, "%", "\\%") + "%"

			// Search predictions table first (question + text are searchable).
			predSQL := `SELECT id, COALESCE(chat_id,'') AS chat_id,
				COALESCE(question,'') AS question,
				COALESCE(text,'') AS text,
				COALESCE(session_id,'') AS session_id,
				COALESCE(synced_at,'') AS synced_at,
				COALESCE(data, '{}') AS data
				FROM prediction
				WHERE (question LIKE ? OR text LIKE ?)`
			predArgs := []any{like, like}
			if cutoff != "" {
				predSQL += " AND synced_at >= ?"
				predArgs = append(predArgs, cutoff)
			}
			predSQL += " ORDER BY synced_at DESC LIMIT ?"
			predArgs = append(predArgs, limit)

			pRows, err := db.DB().QueryContext(cmd.Context(), predSQL, predArgs...)
			if err != nil {
				return fmt.Errorf("predictions query: %w", err)
			}
			type hit struct {
				Source    string          `json:"source"`
				ID        string          `json:"id"`
				ChatID    string          `json:"chatId"`
				Question  string          `json:"question"`
				Text      string          `json:"text"`
				SessionID string          `json:"sessionId,omitempty"`
				SyncedAt  string          `json:"syncedAt,omitempty"`
				UsedTools []string        `json:"usedTools,omitempty"`
				Cited     []string        `json:"citedDocs,omitempty"`
				Raw       json.RawMessage `json:"-"`
			}
			var hits []hit
			for pRows.Next() {
				var h hit
				h.Source = "prediction"
				var raw string
				if err := pRows.Scan(&h.ID, &h.ChatID, &h.Question, &h.Text, &h.SessionID, &h.SyncedAt, &raw); err != nil {
					pRows.Close()
					return fmt.Errorf("scan: %w", err)
				}
				h.Raw = json.RawMessage(raw)
				extractUsedToolsAndDocs(h.Raw, &h.UsedTools, &h.Cited)
				if usedTool != "" && !contains(h.UsedTools, usedTool) {
					continue
				}
				if citedStore != "" && !contains(h.Cited, citedStore) {
					continue
				}
				hits = append(hits, h)
			}
			pRows.Close()

			// Also search chat_messages (content field) — these are the per-message
			// records, useful when a question wasn't recorded in the predictions table.
			msgSQL := `SELECT id, COALESCE(chat_id,'') AS chat_id,
				COALESCE(chatflowid,'') AS chatflowid,
				COALESCE(content,'') AS content,
				COALESCE(role,'') AS role,
				COALESCE(created_date, synced_at) AS dt,
				COALESCE(data,'{}') AS data
				FROM chatmessage WHERE content LIKE ?`
			msgArgs := []any{like}
			if cutoff != "" {
				msgSQL += " AND COALESCE(created_date, synced_at) >= ?"
				msgArgs = append(msgArgs, cutoff)
			}
			if chatflowID != "" {
				msgSQL += " AND chatflowid = ?"
				msgArgs = append(msgArgs, chatflowID)
			}
			msgSQL += " ORDER BY COALESCE(created_date, synced_at) DESC LIMIT ?"
			msgArgs = append(msgArgs, limit)
			mRows, mErr := db.DB().QueryContext(cmd.Context(), msgSQL, msgArgs...)
			if mErr != nil && mErr != sql.ErrNoRows {
				// non-fatal: predictions search still useful
				_ = mErr
			} else if mRows != nil {
				for mRows.Next() {
					var h hit
					h.Source = "chatmessage"
					var role, dt, raw, cfID string
					if err := mRows.Scan(&h.ID, &h.ChatID, &cfID, &h.Question, &role, &dt, &raw); err != nil {
						continue
					}
					h.SyncedAt = dt
					h.Text = ""
					if role != "" {
						h.SessionID = "role=" + role + ";cf=" + cfID
					}
					h.Raw = json.RawMessage(raw)
					extractUsedToolsAndDocs(h.Raw, &h.UsedTools, &h.Cited)
					if usedTool != "" && !contains(h.UsedTools, usedTool) {
						continue
					}
					if citedStore != "" && !contains(h.Cited, citedStore) {
						continue
					}
					hits = append(hits, h)
				}
				mRows.Close()
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, hits)
			}
			if len(hits) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No hits for %q in local cache. Run `sync` first if you haven't recently.\n", query)
				return nil
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "SOURCE\tCHAT ID\tQUESTION\tTEXT\tWHEN")
			for _, h := range hits {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					h.Source,
					truncate(h.ChatID, 20),
					truncate(h.Question, 40),
					truncate(h.Text, 40),
					h.SyncedAt)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&sinceStr, "since", "", "Time window for the search (e.g. 24h, 7d, 30d)")
	cmd.Flags().StringVar(&usedTool, "used-tool", "", "Filter to predictions where this tool was invoked (matches against usedTools in the data blob)")
	cmd.Flags().StringVar(&citedStore, "cited-store", "", "Filter to predictions whose sourceDocuments cite this store id or name")
	cmd.Flags().StringVar(&chatflowID, "chatflow", "", "Filter to predictions/messages from this chatflow id (applies to chatmessage source)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of hits to return")
	return cmd
}

// extractUsedToolsAndDocs walks the raw response blob and pulls usedTools and
// sourceDocuments names/ids into the slices.
func extractUsedToolsAndDocs(raw json.RawMessage, tools, docs *[]string) {
	var blob map[string]json.RawMessage
	if json.Unmarshal(raw, &blob) != nil {
		return
	}
	// usedTools is typically an array of objects with a "tool" field
	if ut, ok := blob["usedTools"]; ok {
		var items []map[string]any
		if json.Unmarshal(ut, &items) == nil {
			for _, it := range items {
				for _, k := range []string{"tool", "name", "toolName"} {
					if v, ok := it[k].(string); ok && v != "" {
						*tools = append(*tools, v)
						break
					}
				}
			}
		}
	}
	if sd, ok := blob["sourceDocuments"]; ok {
		var items []map[string]any
		if json.Unmarshal(sd, &items) == nil {
			for _, it := range items {
				if meta, ok := it["metadata"].(map[string]any); ok {
					for _, k := range []string{"source", "docstoreId", "storeId", "documentStoreId", "fileName"} {
						if v, ok := meta[k].(string); ok && v != "" {
							*docs = append(*docs, v)
							break
						}
					}
				}
			}
		}
	}
}

func contains(haystack []string, needle string) bool {
	nl := strings.ToLower(needle)
	for _, h := range haystack {
		if strings.Contains(strings.ToLower(h), nl) {
			return true
		}
	}
	return false
}
