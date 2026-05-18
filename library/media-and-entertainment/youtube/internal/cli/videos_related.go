// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

// PATCH: polish-html-unescape-and-ranking — HTML-unescape titles/channelTitle in searchListVideos so &#39; renders as ' not &#39; in cached values and downstream JSON. Lowered same_channel score to 1 and raised shared_topic to 3 so cross-channel topic-shared videos outrank channel-mate fillers. Added stderr warning when results are 100% same-channel as the input video. Threaded cmd.Context() into the client so --timeout / Ctrl+C cancel the up-to-three search.list calls.

package cli

import (
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type relatedVideo struct {
	VideoID      string   `json:"videoId"`
	Title        string   `json:"title"`
	ChannelTitle string   `json:"channelTitle"`
	ChannelID    string   `json:"channelId"`
	EmbedURL     string   `json:"embedUrl"`
	WatchURL     string   `json:"watchUrl"`
	ThumbnailURL string   `json:"thumbnailUrl"`
	PublishedAt  string   `json:"publishedAt"`
	Score        int      `json:"score"`
	Signals      []string `json:"signals"`
}

type relatedResponse struct {
	Input    string         `json:"input"`
	Results  []relatedVideo `json:"results"`
	Warnings []string       `json:"warnings,omitempty"`
}

func newYoutubeVideosRelatedCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var sameChannelOnly bool

	cmd := &cobra.Command{
		Use:         "videos-related <videoId>",
		Short:       "Find related videos via topic + channel + tag overlap (heuristic; replaces deprecated relatedToVideoId)",
		Example:     "  youtube-pp-cli youtube videos-related dQw4w9WgXcQ --limit 10",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			videoID := args[0]

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "GET /youtube/v3/videos?id=%s&part=snippet,topicDetails\n", videoID)
				fmt.Fprintln(cmd.ErrOrStderr(), "(dry run - up to 3 follow-on search.list calls would run)")
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c = c.WithContext(cmd.Context())

			// 1. Fetch the input video to extract channelId, tags, topicIds, title.
			data, err := c.GetWithHeaders("/youtube/v3/videos", map[string]string{
				"id":   videoID,
				"part": "snippet,topicDetails",
			}, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var srcResp struct {
				Items []struct {
					ID      string `json:"id"`
					Snippet struct {
						Title        string   `json:"title"`
						ChannelID    string   `json:"channelId"`
						ChannelTitle string   `json:"channelTitle"`
						Tags         []string `json:"tags"`
					} `json:"snippet"`
					TopicDetails struct {
						TopicIds []string `json:"topicIds"`
					} `json:"topicDetails"`
				} `json:"items"`
			}
			if err := json.Unmarshal(data, &srcResp); err != nil {
				return apiErr(fmt.Errorf("parse input video: %w", err))
			}
			if len(srcResp.Items) == 0 {
				return notFoundErr(fmt.Errorf("video %s not found", videoID))
			}
			src := srcResp.Items[0]

			out := relatedResponse{Input: videoID}

			// Collect candidate sets with their signal contribution
			type candidate struct {
				item    searchHit
				signals []string
				score   int
			}
			candidates := map[string]*candidate{}

			addHits := func(hits []searchHit, signal string, points int) {
				for _, h := range hits {
					if h.VideoID == "" || h.VideoID == videoID {
						continue
					}
					existing, ok := candidates[h.VideoID]
					if !ok {
						candidates[h.VideoID] = &candidate{
							item:    h,
							signals: []string{signal},
							score:   points,
						}
						continue
					}
					existing.signals = append(existing.signals, signal)
					existing.score += points
				}
			}

			// 2a. Same-channel search
			sameChannelHits, err := searchListVideos(c, map[string]string{
				"channelId":  src.Snippet.ChannelID,
				"type":       "video",
				"part":       "snippet",
				"order":      "relevance",
				"maxResults": "20",
			})
			if err != nil {
				out.Warnings = append(out.Warnings, fmt.Sprintf("same-channel search failed: %v", err))
			} else {
				// same_channel is a weak topical signal — same uploader does not
				// imply same topic. Score it below tag_match (1) and topic_match
				// (2) so cross-channel topical hits outrank channel-mate fillers
				// when both exist; same-channel only dominates when no other
				// signals fire.
				addHits(sameChannelHits, "same_channel", 1)
			}

			if !sameChannelOnly {
				// 2b. Topic-based search (best-effort)
				if len(src.TopicDetails.TopicIds) > 0 {
					topicID := src.TopicDetails.TopicIds[0]
					topicHits, terr := searchListVideos(c, map[string]string{
						"topicId":    topicID,
						"type":       "video",
						"part":       "snippet",
						"maxResults": "20",
					})
					if terr != nil {
						out.Warnings = append(out.Warnings, fmt.Sprintf("topic search failed: %v", terr))
					} else {
						// Shared topic is the strongest cross-channel signal:
						// outranks tag_match (1) and same_channel (1) so a
						// topic-shared video from a different channel wins.
						addHits(topicHits, "shared_topic_"+topicID, 3)
					}
				}

				// 2c. Tag-keyword search
				if len(src.Snippet.Tags) > 0 {
					topTags := src.Snippet.Tags
					if len(topTags) > 3 {
						topTags = topTags[:3]
					}
					q := strings.Join(topTags, " ")
					tagHits, terr := searchListVideos(c, map[string]string{
						"q":          q,
						"type":       "video",
						"part":       "snippet",
						"maxResults": "20",
					})
					if terr != nil {
						out.Warnings = append(out.Warnings, fmt.Sprintf("tag-keyword search failed: %v", terr))
					} else {
						// Per-result: only score if title shares a tag word
						titleScore := func(title string) (int, bool) {
							lower := strings.ToLower(title)
							for _, tag := range topTags {
								if strings.Contains(lower, strings.ToLower(tag)) {
									return 1, true
								}
							}
							return 0, false
						}
						for _, h := range tagHits {
							if h.VideoID == "" || h.VideoID == videoID {
								continue
							}
							pts, match := titleScore(h.Title)
							if !match {
								// still register so signal context is available, but no score bump
								if _, ok := candidates[h.VideoID]; !ok {
									candidates[h.VideoID] = &candidate{item: h, signals: []string{"tag_search_pool"}, score: 0}
								}
								continue
							}
							existing, ok := candidates[h.VideoID]
							if !ok {
								candidates[h.VideoID] = &candidate{
									item:    h,
									signals: []string{"tag_match"},
									score:   pts,
								}
								continue
							}
							existing.signals = append(existing.signals, "tag_match")
							existing.score += pts
						}
					}
				}
			}

			// 3. Sort by score desc, take top N
			all := make([]*candidate, 0, len(candidates))
			for _, c := range candidates {
				all = append(all, c)
			}
			sort.SliceStable(all, func(i, j int) bool { return all[i].score > all[j].score })

			n := limit
			if n > len(all) {
				n = len(all)
			}

			results := make([]relatedVideo, 0, n)
			sameChannelCount := 0
			for i := 0; i < n; i++ {
				c := all[i]
				h := c.item
				thumb := h.ThumbnailURL
				if thumb == "" {
					thumb = fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", h.VideoID)
				}
				if h.ChannelID == src.Snippet.ChannelID {
					sameChannelCount++
				}
				results = append(results, relatedVideo{
					VideoID:      h.VideoID,
					Title:        h.Title,
					ChannelTitle: h.ChannelTitle,
					ChannelID:    h.ChannelID,
					EmbedURL:     fmt.Sprintf("https://www.youtube.com/embed/%s", h.VideoID),
					WatchURL:     fmt.Sprintf("https://www.youtube.com/watch?v=%s", h.VideoID),
					ThumbnailURL: thumb,
					PublishedAt:  h.PublishedAt,
					Score:        c.score,
					Signals:      c.signals,
				})
			}
			out.Results = results
			// Surface a hint when results are dominated by the input video's
			// own channel — agents can decide whether to widen the sync, or
			// re-run with a different anchor video.
			if !sameChannelOnly && n > 0 && sameChannelCount == n {
				out.Warnings = append(out.Warnings,
					fmt.Sprintf("all %d results are from the input video's own channel (no cross-channel topic or tag matches found); try a video with richer topicDetails or run sync to populate more candidates", n))
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Number of related videos to return")
	cmd.Flags().BoolVar(&sameChannelOnly, "same-channel-only", false, "Skip topic/tag searches, only same-channel results")

	return cmd
}

// searchHit is the subset of search.list result fields we use.
type searchHit struct {
	VideoID      string `json:"videoId"`
	Title        string `json:"title"`
	ChannelTitle string `json:"channelTitle"`
	ChannelID    string `json:"channelId"`
	PublishedAt  string `json:"publishedAt"`
	ThumbnailURL string `json:"thumbnailUrl"`
}

func searchListVideos(c interface {
	GetWithHeaders(path string, params map[string]string, headers map[string]string) (json.RawMessage, error)
}, params map[string]string) ([]searchHit, error) {
	data, err := c.GetWithHeaders("/youtube/v3/search", params, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
			Snippet struct {
				Title        string `json:"title"`
				ChannelID    string `json:"channelId"`
				ChannelTitle string `json:"channelTitle"`
				PublishedAt  string `json:"publishedAt"`
				Thumbnails   map[string]struct {
					URL string `json:"url"`
				} `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	out := make([]searchHit, 0, len(resp.Items))
	for _, it := range resp.Items {
		thumb := ""
		if t, ok := it.Snippet.Thumbnails["high"]; ok {
			thumb = t.URL
		} else if t, ok := it.Snippet.Thumbnails["medium"]; ok {
			thumb = t.URL
		} else if t, ok := it.Snippet.Thumbnails["default"]; ok {
			thumb = t.URL
		}
		out = append(out, searchHit{
			VideoID: it.ID.VideoID,
			// search.list returns titles with HTML entities like &#39;
			// (apostrophe). Unescape once at the API boundary so cached
			// values and downstream JSON consumers see plain text.
			Title:        html.UnescapeString(it.Snippet.Title),
			ChannelID:    it.Snippet.ChannelID,
			ChannelTitle: html.UnescapeString(it.Snippet.ChannelTitle),
			PublishedAt:  it.Snippet.PublishedAt,
			ThumbnailURL: thumb,
		})
	}
	return out, nil
}
