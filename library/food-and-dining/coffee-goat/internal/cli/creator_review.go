// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

type creatorReviewClip struct {
	Creator     string `json:"creator"`
	VideoTitle  string `json:"video_title"`
	VideoID     string `json:"video_id"`
	PublishedAt string `json:"published_at"`
	Excerpt     string `json:"transcript_excerpt,omitempty"`
}

func newCreatorReviewCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "creator-review <bean-or-roaster-slug>",
		Short: "Lookup Hoffmann/Hedrick YouTube clips mentioning a bean or roaster",
		Example: `  coffee-goat-pp-cli creator-review onyx --agent
  coffee-goat-pp-cli creator-review sey-banko-gotiti --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			needle := strings.ToLower(strings.TrimSpace(args[0]))
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.DB().Query(
				`SELECT creator, video_title, video_id, COALESCE(video_published_at,''),
				        COALESCE(transcript_text,''), COALESCE(mentioned_roaster_slugs_json,'')
				 FROM youtube_reviews
				 ORDER BY video_published_at DESC`,
			)
			if err != nil {
				return err
			}
			defer rows.Close()
			var clips []creatorReviewClip
			for rows.Next() {
				var creator, title, vid, pub, transcript, slugsJSON string
				if err := rows.Scan(&creator, &title, &vid, &pub, &transcript, &slugsJSON); err != nil {
					return err
				}
				matchedByMention := slugsJSON != "" && strings.Contains(strings.ToLower(slugsJSON), needle)
				matchedByTitle := strings.Contains(strings.ToLower(title), needle)
				matchedByTranscript := strings.Contains(strings.ToLower(transcript), needle)
				if !matchedByMention && !matchedByTitle && !matchedByTranscript {
					continue
				}
				clip := creatorReviewClip{
					Creator:     creator,
					VideoTitle:  title,
					VideoID:     vid,
					PublishedAt: pub,
				}
				if matchedByTranscript {
					clip.Excerpt = excerptAround(transcript, needle, 200)
				}
				clips = append(clips, clip)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate youtube_reviews rows: %w", err)
			}
			if len(clips) == 0 {
				if flags.asJSON {
					_ = printJSONFiltered(cmd.OutOrStdout(), clips, flags)
				} else {
					fmt.Fprintln(cmd.ErrOrStderr(), "no clips matched")
				}
				return notFoundErr(fmt.Errorf("no clips matched %q", needle))
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), clips, flags)
			}
			for _, c := range clips {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s — %s (%s)\n", c.Creator, c.VideoTitle, c.VideoID)
				if c.Excerpt != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", c.Excerpt)
				}
			}
			return nil
		},
	}
	return cmd
}

// excerptAround returns the substring of haystack centered on needle
// with [match] markers, total length ~radius*2.
func excerptAround(haystack, needle string, radius int) string {
	lowerH := strings.ToLower(haystack)
	idx := strings.Index(lowerH, needle)
	if idx < 0 {
		return ""
	}
	start := idx - radius
	if start < 0 {
		start = 0
	}
	end := idx + len(needle) + radius
	if end > len(haystack) {
		end = len(haystack)
	}
	prefix := haystack[start:idx]
	matched := haystack[idx : idx+len(needle)]
	suffix := haystack[idx+len(needle) : end]
	return strings.TrimSpace(prefix + "[match]" + matched + "[/match]" + suffix)
}
