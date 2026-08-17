// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Hand-authored body.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

// pp:data-source local
//
// The watchlist lives entirely in the local SQLite store: add/list/rm never
// touch the network. `watch digest` is likewise local-only by design — see
// watch_digest.go for why fetching per watched app would be wrong.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/sensortower/internal/store"
	"github.com/spf13/cobra"
)

const watchlistResource = "watchlist"

type watchlistEntry struct {
	AppID   string `json:"app_id"`
	OS      string `json:"os"`
	AddedAt string `json:"added_at"`
	Label   string `json:"label,omitempty"`
}

func watchlistID(osName, appID string) string { return osName + ":" + appID }

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Track a set of apps locally and digest their rank moves from stored snapshots.",
		Long: "Maintain a local watchlist of apps and report their rank deltas.\n\n" +
			"The watchlist is local state only; nothing is sent to Sensor Tower.\n" +
			"`watch digest` reads the rank snapshots that `movers` stores rather than fetching\n" +
			"per app, which keeps a large watchlist inside the API's tight request budget.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelWatchAddCmd(flags))
	cmd.AddCommand(newNovelWatchListCmd(flags))
	cmd.AddCommand(newNovelWatchRmCmd(flags))
	cmd.AddCommand(newNovelWatchDigestCmd(flags))
	return cmd
}

func newNovelWatchAddCmd(flags *rootFlags) *cobra.Command {
	var flagOS string
	var flagLabel string

	cmd := &cobra.Command{
		Use:         "add <app-id>",
		Short:       "Add an app to the local watchlist.",
		Example:     "  sensortower-pp-cli watch add 460177396 --os ios --label twitch",
		Annotations: map[string]string{"mcp:local-write": "true", "pp:happy-args": "<app-id>=460177396;--os=ios", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				if wantsMachineOutput(flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_add": true, "app_id": firstArg(args), "os": flagOS}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "would add an app to the local watchlist")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<app-id> is required (an iOS numeric id such as 460177396, or an Android package such as tv.twitch.android.app)"))
			}
			if err := validateChoice("os", flagOS, "ios", "android"); err != nil {
				return err
			}
			entry := watchlistEntry{
				AppID:   args[0],
				OS:      flagOS,
				AddedAt: time.Now().UTC().Format(time.RFC3339),
				Label:   flagLabel,
			}
			payload, err := json.Marshal(entry)
			if err != nil {
				return err
			}
			db, err := store.OpenWithContext(ctx, defaultDBPath("sensortower-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.Upsert(watchlistResource, watchlistID(entry.OS, entry.AppID), payload); err != nil {
				return err
			}
			if wantsMachineOutput(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"added": entry}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "watching %s app %s\n", entry.OS, entry.AppID)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagOS, "os", "ios", "Platform the app id belongs to (one of: ios, android)")
	cmd.Flags().StringVar(&flagLabel, "label", "", "Optional human label to show in the digest")
	return cmd
}

func newNovelWatchListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List the apps on the local watchlist.",
		Example:     "  sensortower-pp-cli watch list --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list the local watchlist")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			entries, err := readWatchlist(ctx, cmd)
			if err != nil {
				return err
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) && len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "watchlist is empty. Add an app with 'sensortower-pp-cli watch add <app-id>'.")
				return nil
			}
			// Everyone else gets JSON — including a plain pipe, which is neither
			// a human table nor an explicit machine mode, and which used to fall
			// through the gap and print nothing at all on a cold store.
			return emitNovelResult(cmd, flags, entries, "")
		},
	}
	return cmd
}

func newNovelWatchRmCmd(flags *rootFlags) *cobra.Command {
	var flagOS string

	cmd := &cobra.Command{
		Use:         "rm <app-id>",
		Short:       "Remove an app from the local watchlist.",
		Example:     "  sensortower-pp-cli watch rm 460177396 --os ios",
		Annotations: map[string]string{"mcp:local-write": "true", "pp:happy-args": "<app-id>=460177396;--os=ios", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				if wantsMachineOutput(flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_remove": true, "app_id": firstArg(args), "os": flagOS}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "would remove an app from the local watchlist")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<app-id> is required"))
			}
			if err := validateChoice("os", flagOS, "ios", "android"); err != nil {
				return err
			}
			dbPath := defaultDBPath("sensortower-pp-cli")
			// Removal is idempotent: an app that is not on the watchlist (or no
			// watchlist at all) means the desired post-condition already holds, so
			// report success with removed=false rather than a not-found error. This
			// mirrors `rm -f` / `kubectl delete --ignore-not-found` and keeps agent
			// cleanup flows from tripping on a spurious exit code.
			if !novelFileExists(dbPath) {
				if wantsMachineOutput(flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"removed": false, "app_id": args[0], "os": flagOS, "note": "no local watchlist yet; nothing to remove"}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s app %s was not on the watchlist; nothing to remove\n", flagOS, args[0])
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			res, err := db.DB().ExecContext(ctx,
				`DELETE FROM resources WHERE resource_type = ? AND id = ?`,
				watchlistResource, watchlistID(flagOS, args[0]),
			)
			if err != nil {
				return err
			}
			removed, err := res.RowsAffected()
			if err != nil {
				return err
			}
			if removed == 0 {
				if wantsMachineOutput(flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"removed": false, "app_id": args[0], "os": flagOS, "note": "not on the watchlist; nothing to remove"}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s app %s was not on the watchlist; nothing to remove\n", flagOS, args[0])
				return nil
			}
			if wantsMachineOutput(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"removed": true, "app_id": args[0], "os": flagOS}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "stopped watching %s app %s\n", flagOS, args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&flagOS, "os", "ios", "Platform the app id belongs to (one of: ios, android)")
	return cmd
}

// readWatchlist loads the watchlist rows, drain-first.
//
// Missing-mirror guard: a store that was never written holds no watchlist, which
// is an empty watchlist rather than an error, so it hints on stderr and returns
// no rows. It deliberately renders nothing itself: a reader that printed on its
// callers' behalf would have to guess their output shape, and it guessed wrong —
// `watch digest` used to emit the watchlist's bare `[]` on a cold store instead
// of its own `{"apps":[],"note":...}` envelope, so the same command's payload
// changed type with store state and an agent reading `.apps` hit a type error.
// Callers render their own shape; the empty result flows through the exact path
// an empty-but-warm store already takes.
func readWatchlist(ctx context.Context, cmd *cobra.Command) ([]watchlistEntry, error) {
	dbPath := defaultDBPath("sensortower-pp-cli")
	if !novelFileExists(dbPath) {
		fmt.Fprintln(cmd.ErrOrStderr(), "hint: no local store yet. Add an app with 'sensortower-pp-cli watch add <app-id>', then run 'sensortower-pp-cli movers <category>' to record rank snapshots.")
		return []watchlistEntry{}, nil
	}
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.DB().QueryContext(ctx,
		`SELECT data FROM resources WHERE resource_type = ? ORDER BY id`, watchlistResource)
	if err != nil {
		if syncHintMissingTable(err) {
			return []watchlistEntry{}, nil
		}
		return nil, err
	}
	// Drain fully, then close, before any follow-up query runs.
	entries := []watchlistEntry{}
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if !raw.Valid {
			continue
		}
		var e watchlistEntry
		if err := json.Unmarshal([]byte(raw.String), &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].OS != entries[j].OS {
			return entries[i].OS < entries[j].OS
		}
		return entries[i].AppID < entries[j].AppID
	})
	return entries, nil
}

// firstArg returns the first positional argument, or an empty string when the
// command was invoked with none (e.g. a --dry-run probe with only flags set).
func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
