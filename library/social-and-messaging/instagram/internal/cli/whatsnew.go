// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// `instagram-pp-cli whatsnew` — list shortcodes added since the prior sync run.

package cli

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/store"
)

func newWhatsnewCmd(flags *rootFlags) *cobra.Command {
	var (
		since  string
		owner  string
		limit  int
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "whatsnew",
		Short: "List posts added to the local catalog since the last sync run",
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

			cutoff, err := resolveSinceCutoff(st.DB(), since)
			if err != nil {
				return err
			}

			where := "downloaded_at > ?"
			argsSQL := []any{cutoff}
			if owner != "" {
				where += " AND owner_username = ?"
				argsSQL = append(argsSQL, owner)
			}
			argsSQL = append(argsSQL, limit)
			q := fmt.Sprintf(`SELECT shortcode, COALESCE(owner_username,''),
				COALESCE(SUBSTR(caption,1,120),''), COALESCE(taken_at,0), COALESCE(downloaded_at,0)
				FROM media_files WHERE %s ORDER BY downloaded_at DESC LIMIT ?`, where)

			rows, err := st.DB().Query(q, argsSQL...)
			if err != nil {
				return err
			}
			defer rows.Close()
			type row struct {
				Shortcode      string `json:"shortcode"`
				OwnerUsername  string `json:"owner_username"`
				CaptionSnippet string `json:"caption_snippet"`
				TakenAt        int64  `json:"taken_at"`
				DownloadedAt   int64  `json:"downloaded_at"`
			}
			var out []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Shortcode, &r.OwnerUsername, &r.CaptionSnippet, &r.TakenAt, &r.DownloadedAt); err != nil {
					continue
				}
				out = append(out, r)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", r.Shortcode, r.OwnerUsername, r.CaptionSnippet)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "last-run", `"last-run", a duration like "7d", or RFC3339 timestamp`)
	cmd.Flags().StringVar(&owner, "owner", "", "Restrict to posts by this owner")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// resolveSinceCutoff turns the user's --since string into a Unix timestamp.
// "last-run" reads sync_runs.finished_at of the most-recent completed run;
// if no run has finished yet, falls back to one hour before the most recent
// download (so a fresh database still produces a useful "what's new" list).
func resolveSinceCutoff(db *sql.DB, since string) (int64, error) {
	if since == "" || since == "last-run" {
		var ts sql.NullInt64
		err := db.QueryRow(`SELECT MAX(finished_at) FROM sync_runs WHERE finished_at IS NOT NULL`).Scan(&ts)
		if err == nil && ts.Valid && ts.Int64 > 0 {
			return ts.Int64, nil
		}
		var maxDl sql.NullInt64
		err = db.QueryRow(`SELECT MAX(downloaded_at) FROM media_files`).Scan(&maxDl)
		if err == nil && maxDl.Valid && maxDl.Int64 > 0 {
			return maxDl.Int64 - 3600, nil
		}
		return 0, nil
	}
	if t, err := time.Parse(time.RFC3339, since); err == nil {
		return t.Unix(), nil
	}
	t, err := parseSinceDuration(since)
	if err != nil {
		return 0, fmt.Errorf("invalid --since %q: %w", since, err)
	}
	return t.Unix(), nil
}
