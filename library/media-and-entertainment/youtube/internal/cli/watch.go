// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: competitor watchlist. Implemented body — regeneration preserves this file.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "watch",
		Short:   "Register the competitor channels your monitoring machine tracks, in a typed watchlist table you own.",
		Long:    "Use this command to manage the set of channels the monitoring machine tracks.\nDo NOT use it to fetch channel data; use 'youtube channels-list' instead.",
		Example: "  youtube-pp-cli watch add @mkbhd --json\n  youtube-pp-cli watch list --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:parent-group":     "true",
			"pp:happy-args":       "action=list",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchRemoveCmd(flags))
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List watched channels with last-monitored times",
		Example:     "  youtube-pp-cli watch list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watch list")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, _, err := openAnalystStore(ctx, dbPath)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			entries, err := watchlistEntries(ctx, db)
			if err != nil {
				return apiErr(err)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Watchlist is empty. Add competitors with: youtube-pp-cli watch add @handle")
				return nil
			}
			for _, e := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\tlast monitored: %s\n", e.ChannelID, e.Handle, e.Title, orDash(e.LastMonitoredAt))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: active workspace databank)")
	return cmd
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var dbPath, note string
	var rotate bool
	cmd := &cobra.Command{
		Use:         "add [@handle|channelId]",
		Short:       "Resolve a channel live and put it on the watchlist (1 quota unit)",
		Example:     "  youtube-pp-cli watch add @mkbhd --json",
		Annotations: map[string]string{"pp:happy-args": "channel=@GoogleDevelopers", "pp:typed-exit-codes": "0,2,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watch add")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("channel handle or id is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			g, err := newRotatingGetter(flags, rotate)
			if err != nil {
				return err
			}
			ch, err := resolveChannelInput(ctx, g, args[0])
			if err != nil {
				return classifyAPIError(cmd.ErrOrStderr(), err, flags)
			}
			db, _, err := openAnalystStore(ctx, dbPath)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			now := nowUTC()
			if _, err := db.DB().ExecContext(ctx, `INSERT INTO yt_watchlist (channel_id, handle, title, note, added_at)
				VALUES (?,?,?,?,?)
				ON CONFLICT(channel_id) DO UPDATE SET handle=excluded.handle, title=excluded.title,
					note=CASE WHEN excluded.note != '' THEN excluded.note ELSE yt_watchlist.note END`,
				ch.ChannelID, ch.Handle, ch.Title, note, now); err != nil {
				return apiErr(err)
			}
			if err := writeChannelSnapshot(ctx, db, ch, now); err != nil {
				return apiErr(err)
			}
			out := struct {
				Added    *resolvedChannel `json:"added"`
				Hint     string           `json:"hint"`
				Watchers int              `json:"watchlistSize"`
			}{Added: ch, Hint: fmt.Sprintf("seed its history with: youtube-pp-cli backfill %s", ch.ChannelID)}
			entries, err := watchlistEntries(ctx, db)
			if err == nil {
				out.Watchers = len(entries)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: active workspace databank)")
	cmd.Flags().StringVar(&note, "note", "", "Free-form note stored with the watchlist entry")
	cmd.Flags().BoolVar(&rotate, "rotate", false, "Fail over to the next stored API key on quota exhaustion")
	return cmd
}

func newWatchRemoveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "remove [@handle|channelId]",
		Short:       "Remove a channel from the watchlist (collected history stays in the databank)",
		Example:     "  youtube-pp-cli watch remove @mkbhd",
		Annotations: map[string]string{"pp:happy-args": "channel=@GoogleDevelopers;--dry-run", "pp:typed-exit-codes": "0,2,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watch remove")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("channel handle or id is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, _, err := openAnalystStore(ctx, dbPath)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			in := args[0]
			res, err := db.DB().ExecContext(ctx,
				`DELETE FROM yt_watchlist WHERE channel_id = ? OR handle = ? COLLATE NOCASE OR handle = ? COLLATE NOCASE`,
				in, in, "@"+in)
			if err != nil {
				return apiErr(err)
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				return notFoundErr(fmt.Errorf("%q is not on the watchlist", in))
			}
			return printJSONFiltered(cmd.OutOrStdout(), struct {
				Removed string `json:"removed"`
			}{in}, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: active workspace databank)")
	return cmd
}
