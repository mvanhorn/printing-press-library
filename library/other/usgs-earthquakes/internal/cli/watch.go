// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/usgs-earthquakes/internal/cliutil"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var (
		feed      string
		interval  time.Duration
		minMag    float64
		notify    string
		maxEvents int
	)
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Long-running poll of a USGS summary feed with deduplication against the local store",
		Long: `Long-running poll of a USGS GeoJSON summary feed. Deduplicates
against the local SQLite store (event IDs already seen are skipped) and prints
each new event as it appears. Optionally invokes a shell hook per new event for
custom alerting.

The shell hook receives the event ID as the first argument and a JSON-encoded
event feature on stdin. Placeholders {id}, {place}, {mag}, {alert} are substituted
into the --notify command before exec.

Curtails work under printing-press dogfood (single poll, then exit) and skips
the shell hook under printing-press verify.`,
		Example: strings.Trim(`
  # Print every new M4.5+ event from the past-day feed every 60s
  usgs-earthquakes-pp-cli watch --feed 4.5_day --min-magnitude 4.5

  # With a shell hook (placeholders {id} {place} {mag} {alert})
  usgs-earthquakes-pp-cli watch --feed all_hour --notify "echo {id}: {mag} {place}"

  # Significant events only, longer interval
  usgs-earthquakes-pp-cli watch --feed significant_day --interval 5m
`, "\n"),
		Annotations: map[string]string{}, // intentionally not mcp:read-only (invokes shell)
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if !validFeedName(feed) {
				return usageErr(fmt.Errorf("invalid feed name %q (run `usgs-earthquakes-pp-cli feed-list` to see the 20 valid names)", feed))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			feedURL := "https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/" + feed + ".geojson"

			seen, err := loadSeenEventIDs(ctx)
			if err != nil {
				// Continue without dedup if local store is unavailable.
				seen = make(map[string]bool)
			}

			poll := func() (int, error) {
				data, err := c.Get(feedURL, nil)
				if err != nil {
					return 0, err
				}
				var fc struct {
					Features []json.RawMessage `json:"features"`
				}
				if err := json.Unmarshal(data, &fc); err != nil {
					return 0, fmt.Errorf("parse feed: %w", err)
				}
				newCount := 0
				// Batch the events we'll persist to SQLite so that on the
				// next watch invocation, loadSeenEventIDs picks them up and
				// we don't re-emit them as "new" — preserving the documented
				// "deduplicates against the local SQLite store" contract.
				type seenRow struct {
					id  string
					raw json.RawMessage
				}
				var toPersist []seenRow
				for _, raw := range fc.Features {
					var f map[string]any
					if json.Unmarshal(raw, &f) != nil {
						continue
					}
					id, _ := f["id"].(string)
					if id == "" || seen[id] {
						continue
					}
					props, _ := f["properties"].(map[string]any)
					if props == nil {
						continue
					}
					mag, _ := props["mag"].(float64)
					if mag < minMag {
						continue
					}
					seen[id] = true
					newCount++
					toPersist = append(toPersist, seenRow{id: id, raw: raw})
					emitWatchEvent(cmd, flags, raw, f)
					if notify != "" && !cliutil.IsVerifyEnv() {
						runNotifyHook(notify, id, f, raw)
					}
					if maxEvents > 0 && newCount >= maxEvents {
						break
					}
				}
				if len(toPersist) > 0 {
					persisted := make([]persistRow, len(toPersist))
					for i, r := range toPersist {
						persisted[i] = persistRow{id: r.id, raw: r.raw}
					}
					if err := persistSeenEvents(ctx, persisted); err != nil {
						// Best-effort; a write failure should not stop the watch
						// loop, but surface it on stderr so users can spot a
						// chronically full disk or locked DB.
						fmt.Fprintf(cmd.ErrOrStderr(), "watch: persist seen events failed: %v\n", err)
					}
				}
				return newCount, nil
			}

			// Initial poll always runs.
			if _, err := poll(); err != nil {
				return classifyAPIError(err, flags)
			}

			// Under live-dogfood, exit after a single poll (curtail per
			// AGENTS.md "Long-running commands under live-dogfood" rule).
			if cliutil.IsDogfoodEnv() {
				return nil
			}

			// Under verify, also exit after one poll — the notify hook is
			// already gated above, but spending verify time in a 60s loop is
			// wasteful.
			if cliutil.IsVerifyEnv() {
				return nil
			}

			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticker.C:
					if _, err := poll(); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "poll error: %v\n", err)
					}
				}
			}
		},
	}
	cmd.Flags().StringVar(&feed, "feed", "all_hour", "Feed name to poll (run feed-list for the 20 valid names)")
	cmd.Flags().DurationVar(&interval, "interval", 60*time.Second, "Polling interval between feed fetches")
	cmd.Flags().Float64Var(&minMag, "min-magnitude", 0, "Skip events below this magnitude")
	cmd.Flags().StringVar(&notify, "notify", "", "Shell command to run per new event; placeholders {id} {place} {mag} {alert}")
	cmd.Flags().IntVar(&maxEvents, "max-events", 0, "Stop after N new events have been emitted (0 = unlimited)")
	return cmd
}

func emitWatchEvent(cmd *cobra.Command, flags *rootFlags, raw json.RawMessage, f map[string]any) {
	if flags.asJSON {
		fmt.Fprintln(cmd.OutOrStdout(), string(raw))
		return
	}
	props, _ := f["properties"].(map[string]any)
	id, _ := f["id"].(string)
	mag, _ := props["mag"].(float64)
	place, _ := props["place"].(string)
	alert, _ := props["alert"].(string)
	tMs, _ := props["time"].(float64)
	ts := time.Unix(int64(tMs)/1000, 0).UTC().Format(time.RFC3339)
	if alert == "" {
		alert = "-"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\tM%.1f\t%s\t[%s]\n", ts, id, mag, place, alert)
}

func runNotifyHook(template, id string, f map[string]any, raw json.RawMessage) {
	props, _ := f["properties"].(map[string]any)
	mag, _ := props["mag"].(float64)
	place, _ := props["place"].(string)
	alert, _ := props["alert"].(string)
	cmdStr := template
	cmdStr = strings.ReplaceAll(cmdStr, "{id}", shellQuote(id))
	cmdStr = strings.ReplaceAll(cmdStr, "{place}", shellQuote(place))
	cmdStr = strings.ReplaceAll(cmdStr, "{mag}", shellQuote(strconv.FormatFloat(mag, 'f', 1, 64)))
	cmdStr = strings.ReplaceAll(cmdStr, "{alert}", shellQuote(alert))
	c := exec.Command("sh", "-c", cmdStr)
	c.Stdin = strings.NewReader(string(raw))
	_ = c.Run() // best-effort; ignore exit
}

// shellQuote single-quotes a string for safe substitution into a `sh -c` command.
// Single quotes inside the value are escaped via the standard '"'"' idiom so the
// substituted value can never break out of the surrounding quotes to inject
// shell metacharacters from API-supplied place names, IDs, or alert levels.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func loadSeenEventIDs(ctx context.Context) (map[string]bool, error) {
	db, err := openLocalStore(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.DB().QueryContext(ctx, `SELECT id FROM resources WHERE resource_type='events'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := make(map[string]bool)
	for rows.Next() {
		var id sql.NullString
		if rows.Scan(&id) == nil && id.Valid {
			seen[id.String] = true
		}
	}
	return seen, nil
}

// persistRow carries the minimum fields needed to upsert a watched event
// into the local resources table.
type persistRow struct {
	id  string
	raw json.RawMessage
}

// persistSeenEvents writes each newly-observed event from a watch poll back
// to the local SQLite store so that a subsequent watch invocation's
// loadSeenEventIDs call dedups against it. Without this, IDs accumulated
// during a watch session evaporate on exit and the same events are
// re-emitted as "new" on the next run.
//
// Best-effort: callers log a warning on failure rather than aborting the
// poll loop. Uses INSERT OR REPLACE so a subsequent sync that fetches the
// same event with richer fields (products, updated_at) overwrites cleanly.
func persistSeenEvents(ctx context.Context, rows []persistRow) error {
	if len(rows) == 0 {
		return nil
	}
	db, err := openLocalStore(ctx)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }() // no-op after Commit
	stmt, err := tx.PrepareContext(ctx,
		`INSERT OR REPLACE INTO resources (id, resource_type, data) VALUES (?, 'events', ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.ExecContext(ctx, r.id, string(r.raw)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
