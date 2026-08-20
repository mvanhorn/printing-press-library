package cli

import (
	"database/sql"
	"fmt"
	"io"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drudgereport/internal/drudge"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drudgereport/internal/store"
	"github.com/spf13/cobra"
)

// newOnDateCmd returns the local historical snapshot reconstruction command.
func newOnDateCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "on-date <date_or_time>",
		Short:       "Reconstruct Drudge at a past timestamp the CLI has observed.",
		Example:     "  drudgereport-pp-cli on-date 2026-05-21T08:30 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("on-date takes exactly one <date_or_time> argument"))
			}
			if dryRunOK(flags) {
				return nil
			}

			requestedAt, err := parseOnDateTime(args[0])
			if err != nil {
				return usageErr(err)
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

			result, err := queryOnDate(cmd, s.DB(), requestedAt)
			if err != nil {
				// PATCH(greptile-2026-05-21:on-date-exit-code): "no
				// snapshots in local store" used exit 4, which is the
				// framework's auth-error code. That made scripts that
				// branch on exit code conflate "I need to sync" with
				// "your credentials are bad". Use exit 3 (not found)
				// for the no-data case so the conventional meaning is
				// preserved.
				if noData, ok := err.(*cliError); ok && noData.code == 3 {
					_ = emitDrudgeLocal(cmd.OutOrStdout(), flags, map[string]any{
						"error":   "no_snapshots",
						"message": "No Drudge snapshots exist in the local store. Run `drudgereport-pp-cli sync` or `drudgereport-pp-cli splash` first.",
					}, func(w io.Writer) error {
						fmt.Fprintln(w, "No Drudge snapshots exist in the local store. Run `drudgereport-pp-cli sync` or `drudgereport-pp-cli splash` first.")
						return nil
					})
				}
				return err
			}
			return emitDrudgeLocal(cmd.OutOrStdout(), flags, result, func(w io.Writer) error {
				fmt.Fprintf(w, "requested: %v\nsnapshot:  %v (%v)\n\n", result["requested_at"], result["snapshot_captured_at"], result["snapshot_id"])
				if splash, ok := result["splash"].(map[string]any); ok {
					fmt.Fprintf(w, "%s\n%s\n\n", bold(fmt.Sprint(splash["title"])), splash["url"])
				}
				headlines, _ := result["headlines"].([]map[string]any)
				for _, row := range headlines {
					marker := " "
					if red, _ := row["is_red"].(bool); red {
						marker = "!"
					}
					fmt.Fprintf(w, "%s [%v] %v (%v)\n", marker, row["slot"], row["title"], row["url"])
				}
				return nil
			})
		},
	}
	return cmd
}

func parseOnDateTime(raw string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 12, 0, 0, 0, time.UTC), nil
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04", raw, time.UTC); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid date %q: use YYYY-MM-DD, YYYY-MM-DDTHH:MM, or RFC3339", raw)
}

func queryOnDate(cmd *cobra.Command, db *sql.DB, requestedAt time.Time) (map[string]any, error) {
	var snapshotID, snapshotCapturedAt string
	err := db.QueryRowContext(cmd.Context(),
		`SELECT snapshot_id, captured_at
		 FROM drudge_snapshot
		 ORDER BY ABS(strftime('%s', captured_at) - strftime('%s', ?))
		 LIMIT 1`,
		requestedAt.Format(time.RFC3339Nano),
	).Scan(&snapshotID, &snapshotCapturedAt)
	if err == sql.ErrNoRows {
		// PATCH(greptile-2026-05-21:on-date-exit-code): exit 3 = "not found"
		// is the conventional code for missing data; the framework's exit 4
		// is reserved for auth errors and using it here breaks scripts that
		// branch on exit code.
		return nil, notFoundErr(fmt.Errorf("no snapshots in local store"))
	}
	if err != nil {
		return nil, fmt.Errorf("query nearest snapshot: %w", err)
	}

	rows, err := db.QueryContext(cmd.Context(),
		`SELECT title, url, slot, slot_index, is_red, has_image, image_url, outbound_domain, captured_at, story_id
		 FROM drudge_story
		 WHERE snapshot_id = ?
		 ORDER BY (CASE slot WHEN ? THEN 0 WHEN ? THEN 1 WHEN ? THEN 2 ELSE 3 END), slot_index`,
		snapshotID, string(drudge.SlotSplash), string(drudge.SlotTopLeft), string(drudge.SlotColumn1),
	)
	if err != nil {
		return nil, fmt.Errorf("query snapshot stories: %w", err)
	}
	defer rows.Close()

	headlines := make([]map[string]any, 0)
	breaking := make([]map[string]any, 0)
	var splash map[string]any
	for rows.Next() {
		var title, url, slot, outboundDomain, capturedAt, storyID string
		var slotIndex int64
		var isRed, hasImage int64
		var imageURL sql.NullString
		if err := rows.Scan(&title, &url, &slot, &slotIndex, &isRed, &hasImage, &imageURL, &outboundDomain, &capturedAt, &storyID); err != nil {
			return nil, fmt.Errorf("scan snapshot story: %w", err)
		}
		row := map[string]any{
			"title":           title,
			"url":             url,
			"slot":            slot,
			"slot_index":      slotIndex,
			"is_red":          isRed != 0,
			"has_image":       hasImage != 0,
			"image_url":       nullStringAny(imageURL),
			"outbound_domain": outboundDomain,
			"captured_at":     capturedAt,
			"story_id":        storyID,
		}
		if splash == nil && slot == string(drudge.SlotSplash) {
			splash = row
		}
		if isRed != 0 {
			breaking = append(breaking, row)
		}
		headlines = append(headlines, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate snapshot stories: %w", err)
	}

	return map[string]any{
		"requested_at":         requestedAt.Format(time.RFC3339),
		"snapshot_captured_at": snapshotCapturedAt,
		"snapshot_id":          snapshotID,
		"splash":               splash,
		"breaking":             breaking,
		"headlines":            headlines,
	}, nil
}
