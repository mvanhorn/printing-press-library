package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/localstore"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/zoomurl"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/store"
)

// newZoomSavedCmd: `zoom-pp-cli saved {add | list | join | rm | edit | add-from-url}`
func newZoomSavedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "saved",
		Short: "Manage local Zoom meeting bookmarks",
		Long: "User-named bookmarks for Zoom meetings, stored in a local SQLite table. " +
			"Survives across machines via standard config sync. No Zoom account needed. " +
			"Use `saved join <name>` to launch one with the desktop URL scheme.",
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	cmd.AddCommand(newSavedAddCmd(flags))
	cmd.AddCommand(newSavedAddFromURLCmd(flags))
	cmd.AddCommand(newSavedListCmd(flags))
	cmd.AddCommand(newSavedJoinCmd(flags))
	cmd.AddCommand(newSavedRmCmd(flags))
	cmd.AddCommand(newSavedGetCmd(flags))
	return cmd
}

func newSavedAddCmd(flags *rootFlags) *cobra.Command {
	var (
		pwd   string
		notes string
		url   string
	)
	cmd := &cobra.Command{
		Use:   "add [name] [meeting-id]",
		Short: "Save a meeting bookmark by name",
		Example: `  zoom-pp-cli saved add team-standup 85123456789 --pwd 123456 --notes "weekly Wed 10am"
  zoom-pp-cli saved add q3-planning 87654321987 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			name, meetingID := args[0], args[1]
			sm := localstore.SavedMeeting{Name: name, MeetingID: meetingID, Pwd: pwd, Notes: notes, URL: url}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return flags.printJSON(cmd, map[string]any{"would_save": sm})
			}
			db, closer, err := openLocalDB(cmd.Context())
			if err != nil {
				return err
			}
			defer closer()
			if err := localstore.SaveBookmark(cmd.Context(), db, sm); err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{"status": "saved", "bookmark": sm})
		},
	}
	cmd.Flags().StringVar(&pwd, "pwd", "", "Meeting password (raw, not encrypted)")
	cmd.Flags().StringVar(&notes, "notes", "", "Free-form notes")
	cmd.Flags().StringVar(&url, "url", "", "Original join URL (preserved for reference)")
	return cmd
}

func newSavedAddFromURLCmd(flags *rootFlags) *cobra.Command {
	var notes string
	cmd := &cobra.Command{
		Use:   "add-from-url [name] [url]",
		Short: "Save a bookmark by parsing any Zoom URL (https, zoommtg://, calendar-invite)",
		Long: "Recognises every URL shape: https://*.zoom.us/j/<id>?pwd=..., zoommtg://zoom.us/join?confno=..., " +
			"bare numeric meeting IDs. If the URL contains an encrypted_password, the bookmark is saved with the " +
			"URL preserved but pwd left empty (the URL scheme would refuse to launch); a warning is emitted.",
		Example: `  zoom-pp-cli saved add-from-url team-standup "https://us02web.zoom.us/j/85123456789?pwd=abc"
  zoom-pp-cli saved add-from-url quick-call "zoommtg://zoom.us/join?confno=85123456789&pwd=123456" --json`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			name, raw := args[0], args[1]
			p, err := zoomurl.Parse(raw)
			if err != nil {
				return fmt.Errorf("saved add-from-url: %w", err)
			}
			sm := localstore.SavedMeeting{
				Name:      name,
				MeetingID: p.ConfNo,
				URL:       raw,
				Notes:     notes,
			}
			warning := ""
			if p.Encrypted {
				warning = "URL contained an encrypted password; pwd not stored — the URL scheme would refuse to launch. Re-add with --pwd <raw-password> to enable `saved join`."
			} else {
				sm.Pwd = p.Pwd
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				out := map[string]any{"would_save": sm, "parsed": p}
				if warning != "" {
					out["warning"] = warning
				}
				return flags.printJSON(cmd, out)
			}
			db, closer, err := openLocalDB(cmd.Context())
			if err != nil {
				return err
			}
			defer closer()
			if err := localstore.SaveBookmark(cmd.Context(), db, sm); err != nil {
				return err
			}
			out := map[string]any{"status": "saved", "bookmark": sm, "parsed": p}
			if warning != "" {
				out["warning"] = warning
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&notes, "notes", "", "Free-form notes")
	return cmd
}

func newSavedListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List saved meeting bookmarks",
		Example: `  zoom-pp-cli saved list --json
  zoom-pp-cli saved list --select name,meeting_id --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, closer, err := openLocalDB(cmd.Context())
			if err != nil {
				return err
			}
			defer closer()
			rows, err := localstore.ListBookmarks(cmd.Context(), db)
			if err != nil {
				return err
			}
			if rows == nil {
				rows = []localstore.SavedMeeting{}
			}
			return flags.printJSON(cmd, rows)
		},
	}
}

func newSavedGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "get [name]",
		Short:   "Print one saved bookmark",
		Example: `  zoom-pp-cli saved get team-standup --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			db, closer, err := openLocalDB(cmd.Context())
			if err != nil {
				return err
			}
			defer closer()
			sm, err := localstore.GetBookmark(cmd.Context(), db, args[0])
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("saved: no bookmark named %q", args[0])
				}
				return err
			}
			return flags.printJSON(cmd, sm)
		},
	}
}

func newSavedJoinCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "join [name]",
		Short: "Launch a saved bookmark via the desktop URL scheme",
		Example: `  zoom-pp-cli saved join team-standup --json
  zoom-pp-cli saved join team-standup --dry-run`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			db, closer, err := openLocalDB(cmd.Context())
			if err != nil {
				return err
			}
			defer closer()
			sm, err := localstore.GetBookmark(cmd.Context(), db, args[0])
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("saved join: no bookmark named %q", args[0])
				}
				return err
			}
			p := zoomurl.Params{ConfNo: sm.MeetingID, Pwd: sm.Pwd, Action: zoomurl.ActionJoin}
			url, err := zoomurl.Build(p)
			if err != nil {
				return err
			}
			payload := map[string]any{
				"bookmark": sm,
				"url":      url,
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				payload["status"] = "would_launch"
				if !flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "would launch:", url)
					return nil
				}
				return flags.printJSON(cmd, payload)
			}
			if err := openURL(cmd.Context(), url); err != nil {
				return err
			}
			payload["status"] = "launched"
			return flags.printJSON(cmd, payload)
		},
	}
}

func newSavedRmCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "rm [name]",
		Short: "Delete a saved bookmark",
		Example: `  zoom-pp-cli saved rm team-standup --json
  zoom-pp-cli saved rm team-standup --dry-run`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return flags.printJSON(cmd, map[string]any{"would_delete": args[0]})
			}
			db, closer, err := openLocalDB(cmd.Context())
			if err != nil {
				return err
			}
			defer closer()
			ok, err := localstore.DeleteBookmark(cmd.Context(), db, args[0])
			if err != nil {
				return err
			}
			status := "not_found"
			if ok {
				status = "deleted"
			}
			return flags.printJSON(cmd, map[string]any{"status": status, "name": args[0]})
		},
	}
}

// openLocalDB centralises the open + cleanup for hand-coded commands.
// Wraps store.OpenWithContext using the standard defaultDBPath.
func openLocalDB(ctx context.Context) (*sql.DB, func(), error) {
	path := defaultDBPath("zoom-pp-cli")
	s, err := store.OpenWithContext(ctx, path)
	if err != nil {
		return nil, nil, fmt.Errorf("opening local database (run `zoom-pp-cli sync` or any command once to bootstrap): %w", err)
	}
	cleanup := func() {
		_ = s.Close()
	}
	return s.DB(), cleanup, nil
}

// detectURLEncrypted is exported so other places can warn consistently.
func detectURLEncrypted(p zoomurl.Params) bool {
	return p.Encrypted && strings.TrimSpace(p.Pwd) != ""
}
