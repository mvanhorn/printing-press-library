package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drudgereport/internal/store"
	"github.com/spf13/cobra"
)

// newTailCmd returns the local slot-transition tail command.
func newTailCmd(flags *rootFlags) *cobra.Command {
	var since time.Duration
	var limit int
	cmd := &cobra.Command{
		Use:         "tail",
		Short:       "Slot transitions and color changes since the last fetch or within a window.",
		Example:     "  drudgereport-pp-cli tail --since 6h --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if since < 0 {
				return usageErr(fmt.Errorf("--since must be non-negative"))
			}
			if limit < 0 {
				return usageErr(fmt.Errorf("--limit must be non-negative"))
			}

			ctx := cmd.Context()
			s, err := store.OpenWithContext(ctx, defaultDBPath("drudgereport-pp-cli"))
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer s.Close()
			if err := store.EnsureDrudgeSchema(ctx, s.DB()); err != nil {
				return fmt.Errorf("ensure drudge schema: %w", err)
			}

			empty, err := drudgeStoreEmpty(cmd, s.DB())
			if err != nil {
				return err
			}
			if empty {
				return emitDrudgeNoData(cmd.OutOrStdout(), flags)
			}

			// PATCH(greptile-2026-05-21:tail-n-plus-one): join the story
			// title/url into the same query that pulls slot events. The
			// previous implementation issued one SELECT per event row,
			// which becomes a bottleneck as snapshot history grows.
			results := make([]map[string]any, 0)
			if since > 0 {
				cutoff := time.Now().UTC().Add(-since).Format(time.RFC3339Nano)
				query := `SELECT e.event_id, e.snapshot_id, e.story_id, e.event_type, e.from_slot, e.to_slot, e.captured_at,
					COALESCE(s.title, ''), COALESCE(s.url, '')
					FROM drudge_slot_event e
					LEFT JOIN (
						SELECT story_id, title, url,
							ROW_NUMBER() OVER (PARTITION BY story_id ORDER BY captured_at DESC) AS rn
						FROM drudge_story
					) s ON s.story_id = e.story_id AND s.rn = 1
					WHERE e.captured_at >= ?
					ORDER BY e.captured_at DESC, e.event_id`
				if limit > 0 {
					query += " LIMIT ?"
					results, err = queryTailEvents(cmd, s.DB(), query, cutoff, limit)
				} else {
					results, err = queryTailEvents(cmd, s.DB(), query, cutoff)
				}
			} else {
				var latestSnapshotID string
				err = s.DB().QueryRowContext(ctx, `SELECT snapshot_id FROM drudge_snapshot ORDER BY captured_at DESC LIMIT 1`).Scan(&latestSnapshotID)
				if err == sql.ErrNoRows {
					err = nil
				} else if err == nil {
					query := `SELECT e.event_id, e.snapshot_id, e.story_id, e.event_type, e.from_slot, e.to_slot, e.captured_at,
						COALESCE(s.title, ''), COALESCE(s.url, '')
						FROM drudge_slot_event e
						LEFT JOIN (
							SELECT story_id, title, url,
								ROW_NUMBER() OVER (PARTITION BY story_id ORDER BY captured_at DESC) AS rn
							FROM drudge_story
						) s ON s.story_id = e.story_id AND s.rn = 1
						WHERE e.snapshot_id = ?
						ORDER BY e.event_type, e.captured_at`
					if limit > 0 {
						query += " LIMIT ?"
						results, err = queryTailEvents(cmd, s.DB(), query, latestSnapshotID, limit)
					} else {
						results, err = queryTailEvents(cmd, s.DB(), query, latestSnapshotID)
					}
				}
			}
			if err != nil {
				return err
			}

			if len(results) == 0 && !drudgeLocalJSON(cmd.OutOrStdout(), flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "No slot events recorded yet. Run `drudgereport-pp-cli sync` (or `splash`/`headlines`) twice with some time between to populate snapshot history.")
				return nil
			}
			return emitDrudgeLocal(cmd.OutOrStdout(), flags, results, func(w io.Writer) error {
				tw := newTabWriter(w)
				fmt.Fprintln(tw, "TIME\tEVENT\tSTORY\tFROM\tTO\tTITLE")
				for _, row := range results {
					fmt.Fprintf(tw, "%v\t%v\t%v\t%v\t%v\t%v\n", row["captured_at"], row["event_type"], row["story_id"], row["from_slot"], row["to_slot"], row["title"])
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().DurationVar(&since, "since", 0, `Duration window to inspect (0 = latest snapshot only)`)
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of events to return (0 = no cap)")
	return cmd
}

// queryTailEvents scans slot events and their joined story title/url in one
// query. The caller's query must select these columns in this exact order:
// event_id, snapshot_id, story_id, event_type, from_slot, to_slot, captured_at, title, url.
// PATCH(greptile-2026-05-21:tail-n-plus-one): previously the loop ran one
// SELECT per event row to fetch title/url; the join eliminates that.
func queryTailEvents(cmd *cobra.Command, db *sql.DB, query string, args ...any) ([]map[string]any, error) {
	rows, err := db.QueryContext(cmd.Context(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("query slot events: %w", err)
	}
	defer rows.Close()

	results := make([]map[string]any, 0)
	for rows.Next() {
		var eventID, snapshotID, storyID, eventType, capturedAt, title, url string
		var fromSlot, toSlot sql.NullString
		if err := rows.Scan(&eventID, &snapshotID, &storyID, &eventType, &fromSlot, &toSlot, &capturedAt, &title, &url); err != nil {
			return nil, fmt.Errorf("scan slot event: %w", err)
		}
		results = append(results, map[string]any{
			"event_type":  eventType,
			"story_id":    storyID,
			"title":       title,
			"url":         url,
			"from_slot":   nullStringAny(fromSlot),
			"to_slot":     nullStringAny(toSlot),
			"captured_at": capturedAt,
			"snapshot_id": snapshotID,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate slot events: %w", err)
	}
	return results, nil
}

func drudgeLocalJSON(w io.Writer, flags *rootFlags) bool {
	return flags != nil && flags.asJSON || !isTerminal(w)
}

// drudgeStoreEmpty returns true when the local snapshot store has no
// drudge_story rows. Used by aggregator commands (sources, tenure,
// bent, tail) to distinguish "no data in window" (empty array result)
// from "no data ever" (envelope with no_data hint pointing at sync).
func drudgeStoreEmpty(cmd *cobra.Command, db *sql.DB) (bool, error) {
	var total sql.NullInt64
	if err := db.QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM drudge_story`).Scan(&total); err != nil {
		return false, fmt.Errorf("query local data count: %w", err)
	}
	return !total.Valid || total.Int64 == 0, nil
}

// emitDrudgeNoData mirrors digest's no_data envelope so JSON callers
// across aggregator commands get a uniform "run sync/splash" hint when
// the local store is empty.
func emitDrudgeNoData(w io.Writer, flags *rootFlags) error {
	payload := map[string]any{
		"error":   "no_data",
		"message": "Run drudgereport-pp-cli sync or splash to populate snapshot history.",
	}
	return emitDrudgeLocal(w, flags, payload, func(out io.Writer) error {
		fmt.Fprintln(out, "No snapshot history yet. Run `drudgereport-pp-cli sync` (or `splash`/`headlines`) to populate the local store.")
		return nil
	})
}

func emitDrudgeLocal(w io.Writer, flags *rootFlags, payload any, human func(io.Writer) error) error {
	if drudgeLocalJSON(w, flags) {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return printOutputWithFlags(w, raw, flags)
	}
	return human(w)
}

func nullStringAny(v sql.NullString) any {
	if !v.Valid {
		return nil
	}
	return v.String
}

func nullStringText(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func secondsBetween(firstRaw, lastRaw string) (int64, error) {
	first, err := time.Parse(time.RFC3339Nano, firstRaw)
	if err != nil {
		return 0, fmt.Errorf("parse first timestamp %q: %w", firstRaw, err)
	}
	last, err := time.Parse(time.RFC3339Nano, lastRaw)
	if err != nil {
		return 0, fmt.Errorf("parse last timestamp %q: %w", lastRaw, err)
	}
	seconds := int64(last.Sub(first).Seconds())
	if seconds < 0 {
		return 0, nil
	}
	return seconds, nil
}

func secondsSince(raw string, now time.Time) (int64, error) {
	first, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return 0, fmt.Errorf("parse timestamp %q: %w", raw, err)
	}
	seconds := int64(now.Sub(first.UTC()).Seconds())
	if seconds < 0 {
		return 0, nil
	}
	return seconds, nil
}
