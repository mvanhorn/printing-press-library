// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newWorkflowTriageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "triage", Short: "Run crash-safe bounded bookmark queues", Example: "  raindrop-pp-cli workflow triage status --agent", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newTriageCreateCmd(flags), newTriageNextCmd(flags), newTriageStatusCmd(flags), newTriageRetryCmd(flags))
	return cmd
}

func newTriageCreateCmd(flags *rootFlags) *cobra.Command {
	var query, dbPath string
	var batch int
	cmd := &cobra.Command{Use: "create <name>", Args: cobra.ExactArgs(1), Short: "Create a durable triage queue from the local mirror", Example: "  raindrop-pp-cli workflow triage create weekly --query go --batch 10 --agent", RunE: func(cmd *cobra.Command, args []string) error {
		db, _, err := openNovelStore(cmd.Context(), dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		res, err := db.DB().ExecContext(cmd.Context(), `INSERT INTO triage_workflows(name,query,batch_size) VALUES(?,?,?)`, args[0], query, batch)
		if err != nil {
			return fmt.Errorf("creating workflow: %w", err)
		}
		workflowID, _ := res.LastInsertId()
		bookmarks, err := loadLocalBookmarks(db)
		if err != nil {
			return err
		}
		needle := strings.ToLower(strings.TrimSpace(query))
		queued := 0
		for _, bookmark := range bookmarks {
			haystack := strings.ToLower(bookmark.Title + " " + bookmark.Link + " " + bookmark.Excerpt + " " + bookmark.Note + " " + strings.Join(bookmark.Tags, " "))
			if needle != "" && !strings.Contains(haystack, needle) {
				continue
			}
			if _, err := db.DB().ExecContext(cmd.Context(), `INSERT OR IGNORE INTO triage_items(workflow_id,bookmark_id) VALUES(?,?)`, workflowID, bookmark.ID); err != nil {
				return err
			}
			queued++
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"id": workflowID, "name": args[0], "query": query, "batch_size": batch, "queued": queued}, flags)
	}}
	cmd.Flags().StringVar(&query, "query", "", "Text required in bookmark title, URL, excerpt, note, or tags")
	cmd.Flags().IntVar(&batch, "batch", 10, "Maximum items claimed by each next call")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}

func newTriageNextCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{Use: "next <name>", Args: cobra.ExactArgs(1), Short: "Atomically claim the next triage batch", Example: "  raindrop-pp-cli workflow triage next weekly --agent", RunE: func(cmd *cobra.Command, args []string) error {
		db, _, err := openNovelStore(cmd.Context(), dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		tx, err := db.DB().BeginTx(cmd.Context(), nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		var workflowID int64
		var batch int
		if err := tx.QueryRowContext(cmd.Context(), `SELECT id,batch_size FROM triage_workflows WHERE name=?`, args[0]).Scan(&workflowID, &batch); err != nil {
			return fmt.Errorf("loading workflow: %w", err)
		}
		rows, err := tx.QueryContext(cmd.Context(), `SELECT bookmark_id FROM triage_items WHERE workflow_id=? AND state IN ('queued','retry') ORDER BY updated_at,bookmark_id LIMIT ?`, workflowID, batch)
		if err != nil {
			return err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := tx.ExecContext(cmd.Context(), `UPDATE triage_items SET state='inflight',attempts=attempts+1,updated_at=CURRENT_TIMESTAMP WHERE workflow_id=? AND bookmark_id=? AND state IN ('queued','retry')`, workflowID, id); err != nil {
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		items := make([]any, 0, len(ids))
		for _, id := range ids {
			raw, err := db.Get("raindrops", id)
			if err == nil {
				var obj any
				_ = json.Unmarshal(raw, &obj)
				items = append(items, obj)
			}
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"workflow": args[0], "count": len(items), "items": items}, flags)
	}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}

func newTriageStatusCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{Use: "status [name]", Args: cobra.MaximumNArgs(1), Short: "Show queue lifecycle counts", Example: "  raindrop-pp-cli workflow triage status --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, _, err := openNovelStore(cmd.Context(), dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		query := `SELECT w.name,i.state,count(*) FROM triage_workflows w LEFT JOIN triage_items i ON i.workflow_id=w.id`
		params := []any{}
		if len(args) == 1 {
			query += ` WHERE w.name=?`
			params = append(params, args[0])
		}
		query += ` GROUP BY w.name,i.state ORDER BY w.name,i.state`
		rows, err := db.DB().QueryContext(cmd.Context(), query, params...)
		if err != nil {
			return err
		}
		defer rows.Close()
		status := map[string]map[string]int{}
		for rows.Next() {
			var name string
			var state sql.NullString
			var count int
			if err := rows.Scan(&name, &state, &count); err != nil {
				return err
			}
			if status[name] == nil {
				status[name] = map[string]int{}
			}
			if state.Valid {
				status[name][state.String] = count
			}
		}
		return printJSONFiltered(cmd.OutOrStdout(), status, flags)
	}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}

func newTriageRetryCmd(flags *rootFlags) *cobra.Command {
	var dbPath, bookmarkID string
	cmd := &cobra.Command{Use: "retry <name>", Args: cobra.ExactArgs(1), Short: "Return failed or inflight items to the retry queue", Example: "  raindrop-pp-cli workflow triage retry weekly --agent", Annotations: map[string]string{"mcp:local-write": "true", "pp:no-error-path-probe": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, _, err := openNovelStore(cmd.Context(), dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		query := `UPDATE triage_items SET state='retry',updated_at=CURRENT_TIMESTAMP WHERE workflow_id=(SELECT id FROM triage_workflows WHERE name=?) AND state IN ('failed','manual','inflight')`
		params := []any{args[0]}
		if bookmarkID != "" {
			query += ` AND bookmark_id=?`
			params = append(params, bookmarkID)
		}
		res, err := db.DB().ExecContext(cmd.Context(), query, params...)
		if err != nil {
			return err
		}
		count, _ := res.RowsAffected()
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"workflow": args[0], "retried": count}, flags)
	}}
	cmd.Flags().StringVar(&bookmarkID, "bookmark", "", "Retry only this bookmark ID")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}
