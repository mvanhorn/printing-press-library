// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 `episode quote` — phrase search with N-segment context window.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/store"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

func newEpisodeQuoteCmd(flags *rootFlags) *cobra.Command {
	var (
		flagCtx   int
		flagLimit int
		flagShow  string
	)
	cmd := &cobra.Command{
		Use:         "quote [phrase]",
		Short:       "Find a phrase in cached transcripts with N segments of context",
		Example:     `  podcast-goat-pp-cli episode quote "training compute" -C 2`,
		Annotations: map[string]string{"pp:endpoint": "episode.quote", "pp:method": "GET", "mcp:read-only": "true"},
		Args:        cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			phrase := args[0]
			ps, err := openPodcastStore(cmd.Context())
			if err != nil {
				return err
			}
			hits, err := ps.SearchSegments(cmd.Context(), phrase, flagCtx, flagLimit)
			if err != nil {
				return err
			}
			if flagShow != "" {
				filtered := map[string][]store.SegmentHit{}
				for url, segs := range hits {
					if len(segs) > 0 && segs[0].Show == flagShow {
						filtered[url] = segs
					}
				}
				hits = filtered
			}
			if flags.asJSON {
				type resultBlock struct {
					EpisodeURL   string             `json:"episode_url"`
					EpisodeTitle string             `json:"episode_title"`
					Show         string             `json:"show"`
					Segments     []store.SegmentHit `json:"segments"`
				}
				results := []resultBlock{}
				for url, segs := range hits {
					title := ""
					show := ""
					if len(segs) > 0 {
						title = segs[0].EpisodeTitle
						show = segs[0].Show
					}
					results = append(results, resultBlock{
						EpisodeURL: url, EpisodeTitle: title, Show: show, Segments: segs,
					})
				}
				out, _ := json.MarshalIndent(map[string]any{"phrase": phrase, "results": results}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			if len(hits) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no matches for %q in cached transcripts\n", phrase)
				return nil
			}
			for url, segs := range hits {
				if len(segs) == 0 {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "## %s\n%s\n\n", segs[0].EpisodeTitle, url)
				for _, s := range segs {
					marker := ""
					if strings.Contains(strings.ToLower(s.Text), strings.ToLower(phrase)) {
						marker = " <-- match"
					}
					fmt.Fprintf(cmd.OutOrStdout(),
						"**%s** (%s)%s\n\n%s\n\n",
						s.Speaker, transcript.FmtTime(s.TsSec), marker, strings.TrimSpace(s.Text),
					)
				}
				if firstSeg := segs[0]; firstSeg.EpisodeURL != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "<!-- deeplink: %s#t=%d -->\n\n", firstSeg.EpisodeURL, firstSeg.TsSec)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&flagCtx, "context", "C", 1, "Number of segments above and below the match to include")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Max distinct seed matches to surface")
	cmd.Flags().StringVar(&flagShow, "show", "", "Filter to a single show slug")
	return cmd
}
