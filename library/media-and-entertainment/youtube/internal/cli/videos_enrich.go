// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

// PATCH(amend-20260523: novel command) — single-video analog of
// playlist-enrich: one call returns a video's metadata + transcript +
// description in the same enrichedVideo shape. Reuses fetchVideoMetadata
// (videos.list) and fetchTranscriptForVideo (InnerTube); no hand-rolled
// payloads. For the per-playlist version use playlist-enrich.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newYoutubeVideosEnrichCmd(flags *rootFlags) *cobra.Command {
	var lang string
	var noTranscript bool

	cmd := &cobra.Command{
		Use:         "videos-enrich <videoId|url>",
		Short:       "Resolve a single video to metadata + transcript + description in one call",
		Example:     "  youtube-pp-cli youtube videos-enrich dQw4w9WgXcQ",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			videoID := parseVideoID(strings.TrimSpace(args[0]))
			if videoID == "" {
				return usageErr(fmt.Errorf("could not extract a video ID from %q", args[0]))
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "GET /youtube/v3/videos (id=%s, part=snippet,contentDetails,statistics)\n", videoID)
				if !noTranscript {
					fmt.Fprintf(cmd.ErrOrStderr(), "POST https://www.youtube.com/youtubei/v1/player (videoId=%s)\n", videoID)
				}
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c = c.WithContext(cmd.Context())

			// Position is playlist-only; it stays 0 for a standalone video.
			row := enrichedVideo{
				VideoID:  videoID,
				Title:    videoID,
				WatchURL: fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID),
				EmbedURL: fmt.Sprintf("https://www.youtube.com/embed/%s", videoID),
			}

			// Metadata + description via videos.list (single-element batch).
			meta, metaWarns := fetchVideoMetadata(c, []playlistEntry{{videoID: videoID}})
			for _, w := range metaWarns {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
			}
			if m, ok := meta[videoID]; ok {
				row.Title = m.title
				row.Description = m.description
				row.ChannelTitle = m.channelTitle
				row.PublishedAt = m.publishedAt
				row.Duration = m.duration
				row.ViewCount = m.viewCount
				row.LikeCount = m.likeCount
				row.ThumbnailURL = m.thumbnailURL
			} else {
				row.MetadataError = "metadata not returned by videos.list (video may be private, deleted, or the ID is wrong)"
			}

			if !noTranscript {
				tctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
				defer cancel()
				tr, terr := fetchTranscriptForVideo(tctx, videoID, lang)
				if terr != nil {
					row.TranscriptError = terr.Error()
				} else {
					row.Transcript = tr
				}
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(row)
		},
	}

	cmd.Flags().StringVar(&lang, "lang", "en", "Caption language code for the transcript")
	cmd.Flags().BoolVar(&noTranscript, "no-transcript", false, "Skip transcript fetch (metadata + description only)")
	return cmd
}
