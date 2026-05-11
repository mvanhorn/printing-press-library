// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// `instagram-pp-cli watch add|rm|list` — manage the watchlist that drives
// `sync --watchlist`. Entries: bare username (profile), #hashtag, or
// %location_id.

package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/store"
)

// classifyEntry maps a user-typed watchlist entry to (kind, value).
// "@nasa" / "nasa" -> profile, "#nature" -> hashtag, "%213385402" -> location.
func classifyEntry(s string) (kind, value string) {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "#"):
		return "hashtag", strings.TrimPrefix(s, "#")
	case strings.HasPrefix(s, "%"):
		return "location", strings.TrimPrefix(s, "%")
	default:
		return "profile", strings.TrimPrefix(s, "@")
	}
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Manage the watchlist (profiles, hashtags, locations) used by sync",
	}
	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchRmCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	return cmd
}

func openWatchDB(cmd *cobra.Command, dbPath string) (*sql.DB, func(), error) {
	if dbPath == "" {
		dbPath = defaultDBPath("instagram-pp-cli")
	}
	st, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, func() {}, err
	}
	if err := store.ApplyExtraMigrations(cmd.Context(), st.DB()); err != nil {
		st.Close()
		return nil, func() {}, err
	}
	return st.DB(), func() { st.Close() }, nil
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "add [entry...]",
		Short: "Add one or more entries to the watchlist",
		Example: `  instagram-pp-cli watch add nasa
  instagram-pp-cli watch add nasa "#nature" %213385402`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, closer, err := openWatchDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer closer()
			now := time.Now().Unix()
			added := []map[string]string{}
			for _, raw := range args {
				kind, value := classifyEntry(raw)
				if value == "" {
					continue
				}
				_, err := db.Exec(`INSERT INTO watchlist (entry_kind, entry_value, added_at)
					VALUES (?, ?, ?) ON CONFLICT(entry_kind, entry_value) DO NOTHING`,
					kind, value, now)
				if err != nil {
					return err
				}
				added = append(added, map[string]string{"kind": kind, "value": value})
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), added, flags)
			}
			for _, a := range added {
				fmt.Fprintf(cmd.OutOrStdout(), "added %s:%s\n", a["kind"], a["value"])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWatchRmCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "rm [entry...]",
		Short: "Remove one or more entries from the watchlist",
		Example: `  instagram-pp-cli watch rm nasa
  instagram-pp-cli watch rm nasa "#nature" %213385402`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, closer, err := openWatchDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer closer()
			removed := []map[string]string{}
			for _, raw := range args {
				kind, value := classifyEntry(raw)
				res, err := db.Exec(`DELETE FROM watchlist WHERE entry_kind = ? AND entry_value = ?`, kind, value)
				if err != nil {
					return err
				}
				n, _ := res.RowsAffected()
				if n > 0 {
					removed = append(removed, map[string]string{"kind": kind, "value": value})
				}
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), removed, flags)
			}
			for _, r := range removed {
				fmt.Fprintf(cmd.OutOrStdout(), "removed %s:%s\n", r["kind"], r["value"])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List watchlist entries (profiles, hashtags, locations) with kind, value, and last-synced timestamp",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), []any{}, flags)
				}
				return nil
			}
			db, closer, err := openWatchDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer closer()
			rows, err := db.Query(`SELECT entry_kind, entry_value, added_at, COALESCE(last_synced_at, 0)
				FROM watchlist ORDER BY entry_kind, entry_value`)
			if err != nil {
				return err
			}
			defer rows.Close()
			type entry struct {
				Kind         string `json:"kind"`
				Value        string `json:"value"`
				AddedAt      int64  `json:"added_at"`
				LastSyncedAt int64  `json:"last_synced_at"`
			}
			var out []entry
			for rows.Next() {
				var e entry
				if err := rows.Scan(&e.Kind, &e.Value, &e.AddedAt, &e.LastSyncedAt); err != nil {
					continue
				}
				out = append(out, e)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			for _, e := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", e.Kind, e.Value)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
