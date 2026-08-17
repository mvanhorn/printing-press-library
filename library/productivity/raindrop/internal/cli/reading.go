// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newReadingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "reading", Short: "Manage local reading queue", Example: "  raindrop-pp-cli reading queue --agent", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newReadingQueueCmd(flags), newReadingDoneCmd(flags))
	return cmd
}

func newReadingQueueCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{Use: "queue", Short: "List queued bookmarks with their most recent resurfacing time", Example: "  raindrop-pp-cli reading queue --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, _, err := openNovelStore(cmd.Context(), dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		rows, err := db.DB().QueryContext(cmd.Context(), `SELECT r.data,s.last_shown_at FROM reading_state s JOIN raindrops r ON r.id=s.bookmark_id WHERE s.status='queued' ORDER BY s.last_shown_at DESC`)
		if err != nil {
			return err
		}
		defer rows.Close()
		var out []map[string]any
		for rows.Next() {
			var raw, shown string
			if err := rows.Scan(&raw, &shown); err != nil {
				return err
			}
			out = append(out, map[string]any{"bookmark": jsonObject(raw), "last_shown_at": shown})
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"items": out, "count": len(out)}, flags)
	}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}

func newReadingDoneCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{Use: "done <bookmark-id>", Short: "Mark queued bookmark completed", Example: "  raindrop-pp-cli reading done 912345678 --agent", Args: cobra.ExactArgs(1), Annotations: map[string]string{"mcp:local-write": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, _, err := openNovelStore(cmd.Context(), dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		res, err := db.DB().ExecContext(cmd.Context(), `UPDATE reading_state SET status='done',completed_at=? WHERE bookmark_id=?`, time.Now().UTC().Format(time.RFC3339), args[0])
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return fmt.Errorf("bookmark %s is not queued", args[0])
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"bookmark_id": args[0], "status": "done"}, flags)
	}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}

func jsonObject(raw string) any {
	var value any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return raw
	}
	return value
}
