package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drudgereport/internal/drudge"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drudgereport/internal/store"
	"github.com/spf13/cobra"
)

// newSplashCmd returns the current center-slot splash headline command.
func newSplashCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "splash",
		Short:       "Just the current center-slot splash headline with image, outbound URL, red flag, and tenure.",
		Long:        "Fetch the live Drudge page, persist a local snapshot, and print the current center-slot splash headline with image, outbound URL, red flag, and tenure.",
		Example:     "  drudgereport-pp-cli splash --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if limit < 0 {
				return usageErr(fmt.Errorf("--limit must be non-negative"))
			}

			_, stories, _, err := fetchDrudge(cmd.Context())
			if err != nil {
				return err
			}

			splashStories := make([]drudge.Story, 0)
			for _, story := range stories {
				if story.Slot == drudge.SlotSplash {
					splashStories = append(splashStories, story)
				}
			}
			if len(splashStories) == 0 {
				if wantsHumanTable(cmd.OutOrStdout(), flags) {
					fmt.Fprintln(cmd.OutOrStdout(), "no splash item found")
					return nil
				}
				raw, err := json.Marshal(map[string]any{"message": "no splash item found"})
				if err != nil {
					return err
				}
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}

			if limit == 0 || limit > len(splashStories) {
				limit = len(splashStories)
			}

			now := time.Now().UTC()
			results := make([]map[string]any, 0, limit)
			for _, story := range splashStories[:limit] {
				tenureSeconds, err := splashTenureSeconds(cmd.Context(), story.StoryID, now)
				if err != nil {
					return err
				}
				results = append(results, drudgeStoryResult(story, map[string]any{
					"splash_tenure_seconds": tenureSeconds,
				}))
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				for _, story := range results {
					fmt.Fprintln(cmd.OutOrStdout(), bold(fmt.Sprint(story["title"])))
					fmt.Fprintf(cmd.OutOrStdout(), "%s  tenure: %ds\n", story["url"], story["splash_tenure_seconds"])
				}
				return nil
			}

			var payload any = results
			if limit == 1 {
				payload = results[0]
			}
			raw, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 1, "Maximum number of splash items to show")
	return cmd
}

func splashTenureSeconds(ctx context.Context, storyID string, now time.Time) (int64, error) {
	s, err := store.OpenWithContext(ctx, defaultDBPath("drudgereport-pp-cli"))
	if err != nil {
		return 0, fmt.Errorf("open store for splash tenure: %w", err)
	}
	defer s.Close()
	if err := store.EnsureDrudgeSchema(ctx, s.DB()); err != nil {
		return 0, fmt.Errorf("ensure drudge schema for splash tenure: %w", err)
	}

	// PATCH(greptile-2026-05-21:splash-tenure-slot-filter): tenure must be
	// measured from the story's first appearance on the SPLASH slot, not its
	// first appearance in any slot. A story promoted to splash from a column
	// previously reported inflated tenure (time since first column appearance
	// instead of time since promotion). Use the contiguous-run pattern from
	// tenure.go: earliest splash row strictly after the most recent non-splash
	// row for this story, or the all-time MIN(captured_at AND slot='splash')
	// if the story has never been off splash.
	var raw sql.NullString
	err = s.DB().QueryRowContext(ctx,
		`SELECT MIN(captured_at) FROM drudge_story
		 WHERE story_id = ?
		   AND slot = ?
		   AND captured_at > COALESCE((
		     SELECT MAX(captured_at) FROM drudge_story
		     WHERE story_id = ? AND slot != ?
		   ), '1970-01-01T00:00:00Z')`,
		storyID, string(drudge.SlotSplash), storyID, string(drudge.SlotSplash),
	).Scan(&raw)
	if err != nil {
		return 0, fmt.Errorf("query splash tenure: %w", err)
	}
	if !raw.Valid || raw.String == "" {
		return 0, nil
	}

	earliest, err := time.Parse(time.RFC3339Nano, raw.String)
	if err != nil {
		return 0, fmt.Errorf("parse splash tenure timestamp: %w", err)
	}
	seconds := int64(now.Sub(earliest.UTC()).Seconds())
	if seconds < 0 {
		return 0, nil
	}
	return seconds, nil
}

func drudgeStoryResult(story drudge.Story, extra map[string]any) map[string]any {
	result := map[string]any{
		"title":           story.Title,
		"url":             story.URL,
		"slot":            string(story.Slot),
		"slot_index":      story.SlotIndex,
		"is_red":          story.IsRed,
		"has_image":       story.HasImage,
		"image_url":       story.ImageURL,
		"outbound_domain": story.OutboundDomain,
		"captured_at":     story.CapturedAt.UTC().Format(time.RFC3339),
		"story_id":        story.StoryID,
	}
	for key, value := range extra {
		result[key] = value
	}
	return result
}
