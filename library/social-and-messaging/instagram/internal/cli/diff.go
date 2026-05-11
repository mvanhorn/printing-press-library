// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// `instagram-pp-cli diff <handle>` — set-diff between IG's current cursor
// walk and the local download_history. Reports new posts (added) and
// posts deleted upstream (deleted) since the local catalog last saw them.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/store"
)

func newDiffCmd(flags *rootFlags) *cobra.Command {
	var (
		depth  int
		owner  string
		since  string
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "diff [handle]",
		Short: "Show added/deleted posts on a profile vs. the local catalog",
		Example: `  instagram-pp-cli diff nasa
  instagram-pp-cli diff nasa --depth 5 --since 7d
  instagram-pp-cli diff nasa --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			handle := args[0]
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"added": []string{}, "deleted": []string{}}, flags)
				}
				return nil
			}
			if cliutil.IsVerifyEnv() {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"added": []string{}, "deleted": []string{}}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "diff: verify env, skipping live walk")
				return nil
			}
			if depth <= 0 {
				depth = 3
			}
			if depth > 10 {
				depth = 10
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			userID, username, err := resolveUserID(c, handle)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			localOwner := owner
			if localOwner == "" {
				localOwner = username
			}

			if dbPath == "" {
				dbPath = defaultDBPath("instagram-pp-cli")
			}
			st, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			if err := store.ApplyExtraMigrations(cmd.Context(), st.DB()); err != nil {
				return err
			}

			// Walk a bounded number of pages to collect remote shortcodes.
			remote := map[string]bool{}
			cursor := ""
			for i := 0; i < depth; i++ {
				params := map[string]string{}
				if cursor != "" {
					params["max_id"] = cursor
				}
				raw, err := c.Get(fmt.Sprintf("/api/v1/feed/user/%s/", userID), params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var page struct {
					Items         []json.RawMessage `json:"items"`
					MoreAvailable bool              `json:"more_available"`
					NextMaxID     string            `json:"next_max_id"`
				}
				if err := json.Unmarshal(raw, &page); err != nil {
					return err
				}
				for _, item := range page.Items {
					var meta struct {
						Code string `json:"code"`
					}
					_ = json.Unmarshal(item, &meta)
					if meta.Code != "" {
						remote[meta.Code] = true
					}
				}
				if !page.MoreAvailable || page.NextMaxID == "" {
					break
				}
				cursor = page.NextMaxID
			}

			// Build the local set bounded by --since.
			cutoff := int64(0)
			if since != "" {
				if t, perr := parseSinceDuration(since); perr == nil {
					cutoff = t.Unix()
				}
			} else {
				cutoff = time.Now().Add(-30 * 24 * time.Hour).Unix()
			}
			rows, err := st.DB().Query(
				`SELECT shortcode FROM download_history WHERE owner_username = ? AND first_seen_at >= ?`,
				localOwner, cutoff,
			)
			if err != nil {
				return err
			}
			local := map[string]bool{}
			for rows.Next() {
				var sc string
				if rows.Scan(&sc) == nil {
					local[sc] = true
				}
			}
			rows.Close()

			added := []string{}
			deleted := []string{}
			for sc := range remote {
				if !local[sc] {
					added = append(added, sc)
				}
			}
			for sc := range local {
				if !remote[sc] {
					deleted = append(deleted, sc)
				}
			}
			result := map[string]any{
				"profile": username,
				"added":   added,
				"deleted": deleted,
				"remote":  len(remote),
				"local":   len(local),
				"depth":   depth,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %d added, %d deleted (%d remote, %d local)\n",
				username, len(added), len(deleted), len(remote), len(local))
			return nil
		},
	}
	cmd.Flags().IntVar(&depth, "depth", 3, "Pages of feed to walk (1-10)")
	cmd.Flags().StringVar(&owner, "owner", "", "Override the owner column to query (default: the resolved username)")
	cmd.Flags().StringVar(&since, "since", "30d", "Only consider local rows newer than this for the deleted set")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
