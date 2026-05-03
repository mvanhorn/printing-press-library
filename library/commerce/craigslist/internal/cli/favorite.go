// `favorite add / list / remove` — local pinning with notes. CRUD over the
// favorites table; no API surface.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type favoriteRow struct {
	PID     int64  `json:"pid"`
	Note    string `json:"note,omitempty"`
	AddedAt int64  `json:"addedAt"`
}

func newFavoriteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "favorite",
		Short:       "Pin local listings with notes",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newFavoriteAddCmd(flags))
	cmd.AddCommand(newFavoriteListCmd(flags))
	cmd.AddCommand(newFavoriteRemoveCmd(flags))
	return cmd
}

func newFavoriteAddCmd(flags *rootFlags) *cobra.Command {
	var note string
	cmd := &cobra.Command{
		Use:         "add [pid]",
		Short:       "Pin a listing with an optional note",
		Example:     "  craigslist-pp-cli favorite add 7915891289 --note \"check this one tomorrow\"\n  craigslist-pp-cli favorite add 7915891289 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			pid, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
			if err != nil {
				return fmt.Errorf("invalid pid %q: %w", args[0], err)
			}
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			_, err = db.DB().ExecContext(ctx, `
				INSERT INTO favorites(pid, note, added_at) VALUES (?, ?, ?)
				ON CONFLICT(pid) DO UPDATE SET note = excluded.note`,
				pid, note, time.Now().Unix())
			if err != nil {
				return fmt.Errorf("save favorite: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added favorite %d\n", pid)
			return nil
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "Optional human note to attach")
	return cmd
}

func newFavoriteListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List pinned Craigslist listings (local favorites table) with optional notes, most recently added first",
		Example:     "  craigslist-pp-cli favorite list\n  craigslist-pp-cli favorite list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := loadFavorites(ctx, db.DB())
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	return cmd
}

func newFavoriteRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "remove [pid]",
		Short:       "Unpin a Craigslist listing by posting id, removing it from the local favorites table",
		Example:     "  craigslist-pp-cli favorite remove 7915891289",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			pid, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
			if err != nil {
				return fmt.Errorf("invalid pid %q: %w", args[0], err)
			}
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			_, err = db.DB().ExecContext(ctx, `DELETE FROM favorites WHERE pid = ?`, pid)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed favorite %d\n", pid)
			return nil
		},
	}
	return cmd
}

func loadFavorites(ctx context.Context, db *sql.DB) ([]favoriteRow, error) {
	rows, err := db.QueryContext(ctx, `SELECT pid, COALESCE(note,''), added_at FROM favorites ORDER BY added_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []favoriteRow
	for rows.Next() {
		var r favoriteRow
		if err := rows.Scan(&r.PID, &r.Note, &r.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
