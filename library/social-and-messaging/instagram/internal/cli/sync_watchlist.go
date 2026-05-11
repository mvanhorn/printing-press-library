// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// `instagram-pp-cli sync watchlist` — fan out per-entry syncs over the
// watchlist table. Hand-written companion to the spec-driven sync; runs
// independently so the generator-owned sync.go stays untouched.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/store"
)

func newSyncWatchlistCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "watchlist",
		Short: "Sync every entry on the watchlist (profiles, hashtags, locations)",
		Long: `Read the watchlist table and run per-entry sync logic for each row.
Profiles invoke the same cursor walk as 'user --fast-update' (no downloads —
metadata only). Hashtags invoke the spec-mirror hashtag command. Locations
are recorded as not-yet-implemented today.`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_sync": "watchlist"}, flags)
				}
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would sync watchlist (verify env)")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, closer, err := openWatchDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer closer()

			rows, err := db.Query(`SELECT id, entry_kind, entry_value FROM watchlist ORDER BY entry_kind, entry_value`)
			if err != nil {
				return err
			}
			type wlRow struct {
				ID    int64
				Kind  string
				Value string
			}
			var entries []wlRow
			for rows.Next() {
				var w wlRow
				if err := rows.Scan(&w.ID, &w.Kind, &w.Value); err != nil {
					continue
				}
				entries = append(entries, w)
			}
			rows.Close()

			started := time.Now().Unix()
			runRes, err := db.Exec(`INSERT INTO sync_runs (started_at, scope) VALUES (?, ?)`, started, "watchlist")
			runID := int64(0)
			if err == nil {
				runID, _ = runRes.LastInsertId()
			}

			added, skipped, errCount := 0, 0, 0
			results := []map[string]any{}
			for _, e := range entries {
				switch e.Kind {
				case "profile":
					n, err := walkProfileMetadata(cmd, c, e.Value)
					if err != nil {
						errCount++
						results = append(results, map[string]any{"kind": e.Kind, "value": e.Value, "error": err.Error()})
						continue
					}
					added += n
					results = append(results, map[string]any{"kind": e.Kind, "value": e.Value, "posts_seen": n})
					_, _ = db.Exec(`UPDATE watchlist SET last_synced_at = ? WHERE id = ?`, time.Now().Unix(), e.ID)
				case "hashtag":
					raw, err := c.Get(fmt.Sprintf("/api/v1/tags/%s/info/", e.Value), nil)
					if err != nil {
						errCount++
						results = append(results, map[string]any{"kind": e.Kind, "value": e.Value, "error": err.Error()})
						continue
					}
					_ = raw
					_, _ = db.Exec(`UPDATE watchlist SET last_synced_at = ? WHERE id = ?`, time.Now().Unix(), e.ID)
					results = append(results, map[string]any{"kind": e.Kind, "value": e.Value, "ok": true})
				default:
					skipped++
					results = append(results, map[string]any{"kind": e.Kind, "value": e.Value, "skipped": "kind not implemented"})
				}
			}

			if runID > 0 {
				_, _ = db.Exec(`UPDATE sync_runs SET finished_at = ?, added_count = ?, skipped_count = ?, error_count = ?
					WHERE id = ?`, time.Now().Unix(), added, skipped, errCount, runID)
			}
			summary := map[string]any{
				"entries":  len(entries),
				"added":    added,
				"skipped":  skipped,
				"errored":  errCount,
				"results":  results,
				"sync_run": runID,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"watchlist sync: %d entries, %d posts seen, %d skipped, %d errored\n",
				len(entries), added, skipped, errCount)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// walkProfileMetadata fetches the first page of a profile's user feed and
// returns the number of items seen. Mirrors what `user --info-only` would do
// without copying the whole download path.
func walkProfileMetadata(cmd *cobra.Command, c *iClient, handle string) (int, error) {
	userID, _, err := resolveUserID(c, handle)
	if err != nil {
		return 0, err
	}
	raw, err := c.Get(fmt.Sprintf("/api/v1/feed/user/%s/", userID), nil)
	if err != nil {
		return 0, err
	}
	var page struct {
		Items []json.RawMessage `json:"items"`
	}
	_ = json.Unmarshal(raw, &page)
	dbPath := defaultDBPath("instagram-pp-cli")
	st, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err == nil {
		defer st.Close()
		_ = store.ApplyExtraMigrations(cmd.Context(), st.DB())
		for _, item := range page.Items {
			var meta struct {
				Code string `json:"code"`
			}
			_ = json.Unmarshal(item, &meta)
			if meta.Code != "" {
				_, _ = st.DB().Exec(`INSERT INTO download_history (shortcode, owner_username, status, attempts, last_attempt_at, first_seen_at)
					VALUES (?, ?, 'metadata', 1, ?, ?)
					ON CONFLICT(shortcode) DO UPDATE SET last_attempt_at = excluded.last_attempt_at`,
					meta.Code, handle, time.Now().Unix(), time.Now().Unix())
			}
		}
	}
	return len(page.Items), nil
}
