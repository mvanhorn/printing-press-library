// Copyright 2026 bryanaaron. Licensed under Apache-2.0.
// Hand-authored Phase 3 novel analytics: followers diff + following non-mutual.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"instagram-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

// newFollowersCmd returns the `followers` top-level command.
func newFollowersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "followers",
		Short: "Follower analytics (powered by local SQLite — run sync followers first)",
	}
	cmd.AddCommand(newFollowersDiffCmd(flags))
	return cmd
}

func newFollowersDiffCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "diff <username>",
		Short: "Show who unfollowed since the previous sync (requires two or more sync followers runs)",
		Long: `Compares the two most recent followers snapshots for a username and lists
accounts that appeared in the older snapshot but not the newer one.

Run 'instagram-pp-cli sync followers <username>' at least twice (with time
between runs) to accumulate snapshots before using this command.

Exit codes:
  0  success (diff computed, may have zero results)
  1  error
  2  fewer than two snapshots exist for this username`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		Example: `  # After two sync followers runs:
  instagram-pp-cli followers diff johndoe
  instagram-pp-cli followers diff johndoe --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			username := args[0]
			if dbPath == "" {
				dbPath = defaultDBPath("instagram-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer db.Close()

			if err := ensureFollowersSnapshotTable(db); err != nil {
				return fmt.Errorf("prepare table: %w", err)
			}

			// Find the two most recent distinct snapshot timestamps for this username
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT DISTINCT snapshot_ts FROM followers_snapshots WHERE username=? ORDER BY snapshot_ts DESC LIMIT 2`,
				username)
			if err != nil {
				return fmt.Errorf("query snapshots: %w", err)
			}
			var tss []int64
			for rows.Next() {
				var ts int64
				if err := rows.Scan(&ts); err == nil {
					tss = append(tss, ts)
				}
			}
			rows.Close()

			if len(tss) < 2 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"Only %d snapshot(s) found for @%s. Run 'sync followers %s' at least twice.\n",
					len(tss), username, username)
				return &cliError{err: fmt.Errorf("insufficient snapshots"), code: 2}
			}

			newerTS, olderTS := tss[0], tss[1]

			// Unfollowers: in older snapshot but NOT in newer snapshot
			unfRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT old.pk, old.uname, old.full_name
				FROM followers_snapshots old
				WHERE old.username=? AND old.snapshot_ts=?
				AND NOT EXISTS (
					SELECT 1 FROM followers_snapshots new
					WHERE new.username=? AND new.snapshot_ts=? AND new.pk=old.pk
				)
				ORDER BY old.uname
			`, username, olderTS, username, newerTS)
			if err != nil {
				return fmt.Errorf("diff query: %w", err)
			}
			defer unfRows.Close()

			type unfollower struct {
				PK       string `json:"pk"`
				Username string `json:"username"`
				FullName string `json:"full_name"`
			}
			var unfollowers []unfollower
			for unfRows.Next() {
				var u unfollower
				if err := unfRows.Scan(&u.PK, &u.Username, &u.FullName); err == nil {
					unfollowers = append(unfollowers, u)
				}
			}

			olderTime := time.Unix(olderTS, 0).Format("2006-01-02 15:04")
			newerTime := time.Unix(newerTS, 0).Format("2006-01-02 15:04")

			if flags.asJSON {
				out := map[string]any{
					"username":   username,
					"older_snap": olderTime,
					"newer_snap": newerTime,
					"count":      len(unfollowers),
					"unfollowers": func() any {
						if unfollowers == nil {
							return []unfollower{}
						}
						return unfollowers
					}(),
				}
				raw, _ := json.MarshalIndent(out, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Follower diff for @%s\n", username)
			fmt.Fprintf(cmd.OutOrStdout(), "  Older snapshot: %s\n", olderTime)
			fmt.Fprintf(cmd.OutOrStdout(), "  Newer snapshot: %s\n", newerTime)
			fmt.Fprintf(cmd.OutOrStdout(), "  Unfollowers:    %d\n\n", len(unfollowers))

			if len(unfollowers) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No unfollowers detected between these two snapshots.")
				return nil
			}

			return flags.printTable(cmd, []string{"USERNAME", "FULL NAME", "PK"}, func() [][]string {
				var rows [][]string
				for _, u := range unfollowers {
					rows = append(rows, []string{u.Username, u.FullName, u.PK})
				}
				return rows
			}())
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/instagram-pp-cli/data.db)")
	return cmd
}

// newFollowingCmd returns the `following` top-level command.
func newFollowingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "following",
		Short: "Following analytics (powered by local SQLite — run sync following and sync followers first)",
	}
	cmd.AddCommand(newFollowingNonMutualCmd(flags))
	return cmd
}

func newFollowingNonMutualCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "non-mutual",
		Short: "List accounts you follow that don't follow you back",
		Long: `Performs a LEFT JOIN between the local following and followers tables and
returns accounts where no matching follower record exists.

Requires a prior 'sync followers <username>' and 'sync following <username>'
run for the same account. Results are offline-only — no API calls.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  instagram-pp-cli following non-mutual
  instagram-pp-cli following non-mutual --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("instagram-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					json_extract(f.data, '$.pk')       AS pk,
					json_extract(f.data, '$.username')  AS username,
					json_extract(f.data, '$.full_name') AS full_name
				FROM resources f
				WHERE f.resource_type = 'following'
				AND NOT EXISTS (
					SELECT 1 FROM resources r
					WHERE r.resource_type = 'followers'
					AND json_extract(r.data, '$.pk') = json_extract(f.data, '$.pk')
				)
				ORDER BY json_extract(f.data, '$.username')
			`)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type account struct {
				PK       string `json:"pk"`
				Username string `json:"username"`
				FullName string `json:"full_name"`
			}
			var accounts []account
			for rows.Next() {
				var a account
				if err := rows.Scan(&a.PK, &a.Username, &a.FullName); err == nil {
					accounts = append(accounts, a)
				}
			}

			if flags.asJSON {
				raw, _ := json.MarshalIndent(map[string]any{
					"count":    len(accounts),
					"accounts": func() any { if accounts == nil { return []account{} }; return accounts }(),
				}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Accounts you follow that don't follow back: %d\n\n", len(accounts))
			if len(accounts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Everyone you follow follows you back. (Or run sync following + sync followers first.)")
				return nil
			}
			return flags.printTable(cmd, []string{"USERNAME", "FULL NAME", "PK"}, func() [][]string {
				var rows [][]string
				for _, a := range accounts {
					rows = append(rows, []string{a.Username, a.FullName, a.PK})
				}
				return rows
			}())
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/instagram-pp-cli/data.db)")
	return cmd
}
