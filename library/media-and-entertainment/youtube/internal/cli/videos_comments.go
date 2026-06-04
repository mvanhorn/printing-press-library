// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH: feat-comments-and-handle-resolution — novel command. commentThreads.list returns by relevance or time, never by likeCount. Local sort surfaces audience-validated comments that the API ordering buries. Threads cmd.Context() through Client.WithContext so Ctrl+C cancels in-flight pages.

package cli

import (
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type videoComment struct {
	CommentID   string `json:"commentId"`
	Author      string `json:"author"`
	AuthorURL   string `json:"authorChannelUrl,omitempty"`
	Text        string `json:"text"`
	LikeCount   int    `json:"likeCount"`
	PublishedAt string `json:"publishedAt"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
	ReplyCount  int    `json:"replyCount"`
}

type videoCommentsResponse struct {
	VideoID      string         `json:"videoId"`
	Returned     int            `json:"returned"`
	FetchedPages int            `json:"fetchedPages"`
	Order        string         `json:"order"`
	Comments     []videoComment `json:"comments"`
	Warnings     []string       `json:"warnings,omitempty"`
}

func newYoutubeVideosCommentsCmd(flags *rootFlags) *cobra.Command {
	var top int
	var order string

	cmd := &cobra.Command{
		Use:         "videos-comments <videoId|url>",
		Short:       "Fetch top comments on a video, ranked by likeCount (uses commentThreads.list, public read-only)",
		Example:     "  youtube-pp-cli youtube videos-comments dQw4w9WgXcQ --top 10\n  youtube-pp-cli youtube videos-comments 'https://www.youtube.com/watch?v=dQw4w9WgXcQ' --top 10",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			videoID := parseVideoID(strings.TrimSpace(args[0]))
			if videoID == "" {
				return usageErr(fmt.Errorf("could not extract a video ID from %q", args[0]))
			}

			if top <= 0 {
				return usageErr(fmt.Errorf("--top must be > 0"))
			}
			if order != "relevance" && order != "time" {
				return usageErr(fmt.Errorf("--order must be 'relevance' or 'time' (got %q)", order))
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "GET /youtube/v3/commentThreads?videoId=%s&part=snippet&order=%s&maxResults=100\n", videoID, order)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c = c.WithContext(cmd.Context())

			out := videoCommentsResponse{VideoID: videoID, Order: order}

			// Fetch up to ceil(top / 100) pages so --top above 100 still works.
			// Cap pagination at 5 pages (500 comments) to avoid runaway quota use
			// on extremely-commented videos; any --top above that gets a warning.
			maxPages := (top + 99) / 100
			if maxPages > 5 {
				maxPages = 5
				out.Warnings = append(out.Warnings,
					fmt.Sprintf("--top %d capped to 500 candidate comments (5 pages of 100); ranking is over those", top))
			}

			pageToken := ""
			collected := []videoComment{}
			for page := 0; page < maxPages; page++ {
				params := map[string]string{
					"videoId":    videoID,
					"part":       "snippet",
					"order":      order,
					"maxResults": "100",
					"textFormat": "plainText",
				}
				if pageToken != "" {
					params["pageToken"] = pageToken
				}
				data, err := c.GetWithHeaders("/youtube/v3/commentThreads", params, nil)
				if err != nil {
					if page == 0 {
						return classifyAPIError(err, flags)
					}
					out.Warnings = append(out.Warnings, fmt.Sprintf("page %d fetch failed: %v", page+1, err))
					break
				}
				out.FetchedPages++

				var resp struct {
					NextPageToken string `json:"nextPageToken"`
					Items         []struct {
						ID      string `json:"id"`
						Snippet struct {
							TotalReplyCount int  `json:"totalReplyCount"`
							CanReply        bool `json:"canReply"`
							TopLevelComment struct {
								ID      string `json:"id"`
								Snippet struct {
									TextDisplay       string `json:"textDisplay"`
									TextOriginal      string `json:"textOriginal"`
									AuthorDisplayName string `json:"authorDisplayName"`
									AuthorChannelURL  string `json:"authorChannelUrl"`
									LikeCount         int    `json:"likeCount"`
									PublishedAt       string `json:"publishedAt"`
									UpdatedAt         string `json:"updatedAt"`
								} `json:"snippet"`
							} `json:"topLevelComment"`
						} `json:"snippet"`
					} `json:"items"`
				}
				if err := json.Unmarshal(data, &resp); err != nil {
					return apiErr(fmt.Errorf("parse commentThreads page %d: %w", page+1, err))
				}
				for _, it := range resp.Items {
					tl := it.Snippet.TopLevelComment
					text := tl.Snippet.TextOriginal
					if text == "" {
						text = html.UnescapeString(tl.Snippet.TextDisplay)
					}
					collected = append(collected, videoComment{
						CommentID:   tl.ID,
						Author:      tl.Snippet.AuthorDisplayName,
						AuthorURL:   tl.Snippet.AuthorChannelURL,
						Text:        text,
						LikeCount:   tl.Snippet.LikeCount,
						PublishedAt: tl.Snippet.PublishedAt,
						UpdatedAt:   tl.Snippet.UpdatedAt,
						ReplyCount:  it.Snippet.TotalReplyCount,
					})
				}
				if resp.NextPageToken == "" {
					break
				}
				pageToken = resp.NextPageToken
			}

			// Always rank by likeCount desc; --order steers what API returns,
			// not what we sort the local result by. Stable sort preserves API
			// order for ties (so 'time' order shows newer first within ties).
			sort.SliceStable(collected, func(i, j int) bool {
				return collected[i].LikeCount > collected[j].LikeCount
			})
			if len(collected) > top {
				collected = collected[:top]
			}
			out.Comments = collected
			out.Returned = len(collected)

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}

	cmd.Flags().IntVar(&top, "top", 10, "Number of top comments to return, ranked by likeCount")
	cmd.Flags().StringVar(&order, "order", "relevance", "Order to fetch from API before like-count ranking: 'relevance' or 'time'")

	return cmd
}
