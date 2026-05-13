// Copyright 2026 daniel-larson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"flowiseai-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

func newNewsletterAuditCmd(flags *rootFlags) *cobra.Command {
	var since string
	var format string
	var chatflowID string

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit report joining predictions, chat messages, and upsert history for a time window",
		Long: `Generate a per-chatId audit report for predictions recorded in the local
cache within the time window. Each row shows:

  * the chatflow name
  * the question + first 200 chars of the response text
  * how many source documents the response cited
  * which tools the chatflow invoked
  * when the upsert_history table last saw activity for that chatflow

Output is CSV by default (suitable for spreadsheet review or compliance
archives); --format json emits the same data as a JSON array.`,
		Example: "  flowiseai-pp-cli newsletter audit --since 7d --format csv > audit.csv",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cutoffTS, err := parseSinceDuration(since)
			if err != nil {
				return usageErr(err)
			}
			cutoff := cutoffTS.UTC().Format(time.RFC3339)

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("flowiseai-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			predSQL := `SELECT id, COALESCE(chat_id,''), COALESCE(question,''),
				COALESCE(text,''), COALESCE(synced_at,''),
				COALESCE(json_extract(data, '$.chatflowId'), '') AS cf,
				COALESCE(data,'{}') AS data
				FROM prediction WHERE synced_at >= ?`
			pargs := []any{cutoff}
			if chatflowID != "" {
				predSQL += " AND json_extract(data, '$.chatflowId') = ?"
				pargs = append(pargs, chatflowID)
			}
			predSQL += " ORDER BY synced_at DESC"

			rows, err := db.DB().QueryContext(cmd.Context(), predSQL, pargs...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}

			// Cache chatflow names
			cfNames := map[string]string{}
			cfRows, _ := db.DB().QueryContext(cmd.Context(), `SELECT id, COALESCE(name,'') FROM chatflows`)
			if cfRows != nil {
				for cfRows.Next() {
					var id, name string
					_ = cfRows.Scan(&id, &name)
					cfNames[id] = name
				}
				cfRows.Close()
			}

			// Cache last upsert_history per chatflow
			lastUpsert := map[string]string{}
			uRows, _ := db.DB().QueryContext(cmd.Context(), `SELECT COALESCE(chatflowid,''), MAX(date) FROM upsert_history GROUP BY chatflowid`)
			if uRows != nil {
				for uRows.Next() {
					var id, dt string
					_ = uRows.Scan(&id, &dt)
					lastUpsert[id] = dt
				}
				uRows.Close()
			}

			type auditRow struct {
				ChatID        string   `json:"chatId"`
				ChatflowID    string   `json:"chatflowId"`
				ChatflowName  string   `json:"chatflowName"`
				Question      string   `json:"question"`
				TextSnippet   string   `json:"textSnippet"`
				CitedDocs     int      `json:"citedDocCount"`
				UsedTools     []string `json:"usedTools,omitempty"`
				LastUpsertAt  string   `json:"lastUpsertAt,omitempty"`
				PredictionAt  string   `json:"predictionAt"`
			}
			var results []auditRow

			for rows.Next() {
				var id, chatID, question, text, syncedAt, cfID, raw string
				if err := rows.Scan(&id, &chatID, &question, &text, &syncedAt, &cfID, &raw); err != nil {
					rows.Close()
					return fmt.Errorf("scan: %w", err)
				}
				var tools, docs []string
				extractUsedToolsAndDocs(json.RawMessage(raw), &tools, &docs)

				r := auditRow{
					ChatID:       chatID,
					ChatflowID:   cfID,
					ChatflowName: cfNames[cfID],
					Question:     truncate(question, 200),
					TextSnippet:  truncate(text, 200),
					CitedDocs:    len(docs),
					UsedTools:    tools,
					LastUpsertAt: lastUpsert[cfID],
					PredictionAt: syncedAt,
				}
				results = append(results, r)
			}
			rows.Close()

			out := cmd.OutOrStdout()
			if strings.EqualFold(format, "json") || flags.asJSON {
				return flags.printJSON(cmd, results)
			}

			// CSV by default
			w := csv.NewWriter(out)
			defer w.Flush()
			if err := w.Write([]string{"predictionAt", "chatId", "chatflowId", "chatflowName", "question", "textSnippet", "citedDocCount", "usedTools", "lastUpsertAt"}); err != nil {
				return err
			}
			for _, r := range results {
				if err := w.Write([]string{
					r.PredictionAt,
					r.ChatID,
					r.ChatflowID,
					r.ChatflowName,
					r.Question,
					r.TextSnippet,
					fmt.Sprintf("%d", r.CitedDocs),
					strings.Join(r.UsedTools, "|"),
					r.LastUpsertAt,
				}); err != nil {
					return err
				}
			}
			if len(results) == 0 {
				fmt.Fprintf(os.Stderr, "No predictions in local cache for the window. Run `sync` first if you haven't recently.\n")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "7d", "Time window (e.g. 24h, 7d, 30d)")
	cmd.Flags().StringVar(&format, "format", "csv", "Output format: csv (default) or json")
	cmd.Flags().StringVar(&chatflowID, "chatflow", "", "Filter to a single chatflow id")
	return cmd
}
