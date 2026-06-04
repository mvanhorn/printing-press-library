package cli

import (
	"database/sql"
	"fmt"
	"io"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drudgereport/internal/store"
	"github.com/spf13/cobra"
)

// newStoryCmd returns the local story event-history command.
func newStoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "story <story_id>",
		Short:       "Every slot_event for one story_id ordered by timestamp + total tenure.",
		Example:     "  drudgereport-pp-cli story be5c09c496542abd --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("story takes exactly one <story_id> argument"))
			}
			if dryRunOK(flags) {
				return nil
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

			result, err := queryStory(cmd, s.DB(), args[0])
			if err != nil {
				_ = emitDrudgeLocal(cmd.OutOrStdout(), flags, map[string]any{"error": "not_found", "story_id": args[0]}, func(w io.Writer) error {
					fmt.Fprintf(w, "story %s not found\n", args[0])
					return nil
				})
				return err
			}
			return emitDrudgeLocal(cmd.OutOrStdout(), flags, result, func(w io.Writer) error {
				fmt.Fprintf(w, "%s\n%s\nfirst seen: %v  last seen: %v  snapshots: %v  tenure: %vs\n\n", bold(fmt.Sprint(result["title"])), result["url"], result["first_seen_at"], result["last_seen_at"], result["snapshot_count"], result["total_tenure_seconds"])
				events, _ := result["events"].([]map[string]any)
				for _, event := range events {
					fmt.Fprintf(w, "%v  %v  %v -> %v\n", event["captured_at"], event["event_type"], event["from_slot"], event["to_slot"])
				}
				return nil
			})
		},
	}
	return cmd
}

func queryStory(cmd *cobra.Command, db *sql.DB, storyID string) (map[string]any, error) {
	var title, url string
	err := db.QueryRowContext(cmd.Context(),
		`SELECT title, url FROM drudge_story WHERE story_id = ? ORDER BY captured_at DESC LIMIT 1`,
		storyID,
	).Scan(&title, &url)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("story_id %s not found", storyID)
	}
	if err != nil {
		return nil, fmt.Errorf("query story: %w", err)
	}

	events := make([]map[string]any, 0)
	rows, err := db.QueryContext(cmd.Context(),
		`SELECT event_type, from_slot, to_slot, captured_at, snapshot_id
		 FROM drudge_slot_event
		 WHERE story_id = ?
		 ORDER BY captured_at`,
		storyID,
	)
	if err != nil {
		return nil, fmt.Errorf("query story events: %w", err)
	}
	for rows.Next() {
		var eventType, capturedAt, snapshotID string
		var fromSlot, toSlot sql.NullString
		if err := rows.Scan(&eventType, &fromSlot, &toSlot, &capturedAt, &snapshotID); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan story event: %w", err)
		}
		events = append(events, map[string]any{
			"event_type":  eventType,
			"from_slot":   nullStringAny(fromSlot),
			"to_slot":     nullStringAny(toSlot),
			"captured_at": capturedAt,
			"snapshot_id": snapshotID,
		})
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close story events: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate story events: %w", err)
	}

	var firstSeen, lastSeen sql.NullString
	var snapshotCount sql.NullInt64
	if err := db.QueryRowContext(cmd.Context(),
		`SELECT MIN(captured_at), MAX(captured_at), COUNT(DISTINCT snapshot_id)
		 FROM drudge_story
		 WHERE story_id = ?`,
		storyID,
	).Scan(&firstSeen, &lastSeen, &snapshotCount); err != nil {
		return nil, fmt.Errorf("query story tenure: %w", err)
	}
	tenureSeconds := int64(0)
	if firstSeen.Valid && lastSeen.Valid {
		var err error
		tenureSeconds, err = secondsBetween(firstSeen.String, lastSeen.String)
		if err != nil {
			return nil, err
		}
	}
	count := int64(0)
	if snapshotCount.Valid {
		count = snapshotCount.Int64
	}
	return map[string]any{
		"story_id":             storyID,
		"title":                title,
		"url":                  url,
		"first_seen_at":        nullStringText(firstSeen),
		"last_seen_at":         nullStringText(lastSeen),
		"snapshot_count":       count,
		"total_tenure_seconds": tenureSeconds,
		"events":               events,
	}, nil
}
