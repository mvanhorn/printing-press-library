// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

// PATCH: feat-comments-and-handle-resolution — novel command. Chains channels.list (with forHandle or id) -> contentDetails.relatedPlaylists.uploads -> playlistItems.list, collapsing the standard two-call YouTube workflow. Threads cmd.Context() through Client.WithContext.

package cli

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/spf13/cobra"
)

type channelUpload struct {
	VideoID      string `json:"videoId"`
	Title        string `json:"title"`
	Description  string `json:"description,omitempty"`
	PublishedAt  string `json:"publishedAt"`
	WatchURL     string `json:"watchUrl"`
	EmbedURL     string `json:"embedUrl"`
	ThumbnailURL string `json:"thumbnailUrl"`
	Position     int    `json:"position"`
}

type channelUploadsResponse struct {
	Input           string          `json:"input"`
	ChannelID       string          `json:"channelId"`
	ChannelTitle    string          `json:"channelTitle"`
	UploadsPlaylist string          `json:"uploadsPlaylistId"`
	Returned        int             `json:"returned"`
	Uploads         []channelUpload `json:"uploads"`
	Warnings        []string        `json:"warnings,omitempty"`
}

func newYoutubeChannelUploadsCmd(flags *rootFlags) *cobra.Command {
	var top int

	cmd := &cobra.Command{
		Use:         "channel-uploads <@handle|channelId>",
		Short:       "List a channel's most recent uploads (resolves @handle or channelId, then walks the uploads playlist)",
		Example:     "  youtube-pp-cli youtube channel-uploads @veritasium --top 10",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			input := strings.TrimSpace(args[0])
			if input == "" {
				return usageErr(fmt.Errorf("channel handle or ID is required"))
			}
			if top <= 0 {
				return usageErr(fmt.Errorf("--top must be > 0"))
			}

			// Distinguish @handle vs channelId. YouTube channel IDs start with "UC" and are 24 chars.
			channelParam := map[string]string{"part": "snippet,contentDetails"}
			switch {
			case strings.HasPrefix(input, "@"):
				channelParam["forHandle"] = input
			case strings.HasPrefix(input, "UC") && len(input) == 24:
				channelParam["id"] = input
			default:
				// Be forgiving: if it doesn't look like a channelId, try as a bare handle.
				channelParam["forHandle"] = "@" + strings.TrimPrefix(input, "@")
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "GET /youtube/v3/channels (resolve %s)\n", input)
				fmt.Fprintln(cmd.ErrOrStderr(), "GET /youtube/v3/playlistItems (uploads playlist)")
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c = c.WithContext(cmd.Context())

			// 1. Resolve channel -> uploads playlist ID.
			chData, err := c.GetWithHeaders("/youtube/v3/channels", channelParam, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var chResp struct {
				Items []struct {
					ID      string `json:"id"`
					Snippet struct {
						Title string `json:"title"`
					} `json:"snippet"`
					ContentDetails struct {
						RelatedPlaylists struct {
							Uploads string `json:"uploads"`
						} `json:"relatedPlaylists"`
					} `json:"contentDetails"`
				} `json:"items"`
			}
			if err := json.Unmarshal(chData, &chResp); err != nil {
				return apiErr(fmt.Errorf("parse channels response: %w", err))
			}
			if len(chResp.Items) == 0 {
				return notFoundErr(fmt.Errorf("channel %q not found (tried %v)", input, channelParam))
			}
			ch := chResp.Items[0]
			uploadsID := ch.ContentDetails.RelatedPlaylists.Uploads
			if uploadsID == "" {
				return apiErr(fmt.Errorf("channel %s has no uploads playlist (uncommon — channel may be terminated)", ch.ID))
			}

			out := channelUploadsResponse{
				Input:           input,
				ChannelID:       ch.ID,
				ChannelTitle:    html.UnescapeString(ch.Snippet.Title),
				UploadsPlaylist: uploadsID,
			}

			// 2. Walk the uploads playlist; cap pagination at 5 pages (250 items).
			maxPages := (top + 49) / 50
			if maxPages > 5 {
				maxPages = 5
				out.Warnings = append(out.Warnings,
					fmt.Sprintf("--top %d capped to 250 items (5 pages of 50)", top))
			}

			pageToken := ""
			items := []channelUpload{}
		fetchLoop:
			for page := 0; page < maxPages; page++ {
				params := map[string]string{
					"playlistId": uploadsID,
					"part":       "snippet,contentDetails",
					"maxResults": "50",
				}
				if pageToken != "" {
					params["pageToken"] = pageToken
				}
				piData, err := c.GetWithHeaders("/youtube/v3/playlistItems", params, nil)
				if err != nil {
					if page == 0 {
						return classifyAPIError(err, flags)
					}
					out.Warnings = append(out.Warnings, fmt.Sprintf("page %d fetch failed: %v", page+1, err))
					break
				}
				var piResp struct {
					NextPageToken string `json:"nextPageToken"`
					Items         []struct {
						Snippet struct {
							Title       string `json:"title"`
							Description string `json:"description"`
							PublishedAt string `json:"publishedAt"`
							Position    int    `json:"position"`
							Thumbnails  map[string]struct {
								URL string `json:"url"`
							} `json:"thumbnails"`
							ResourceID struct {
								VideoID string `json:"videoId"`
							} `json:"resourceId"`
						} `json:"snippet"`
						ContentDetails struct {
							VideoPublishedAt string `json:"videoPublishedAt"`
						} `json:"contentDetails"`
					} `json:"items"`
				}
				if err := json.Unmarshal(piData, &piResp); err != nil {
					return apiErr(fmt.Errorf("parse playlistItems page %d: %w", page+1, err))
				}
				for _, it := range piResp.Items {
					vid := it.Snippet.ResourceID.VideoID
					if vid == "" {
						continue
					}
					thumb := ""
					if t, ok := it.Snippet.Thumbnails["high"]; ok {
						thumb = t.URL
					} else if t, ok := it.Snippet.Thumbnails["medium"]; ok {
						thumb = t.URL
					} else if t, ok := it.Snippet.Thumbnails["default"]; ok {
						thumb = t.URL
					}
					// Prefer videoPublishedAt (when video went live) over the
					// playlistItem PublishedAt (when added to uploads — usually
					// the same, but not always).
					pubAt := it.ContentDetails.VideoPublishedAt
					if pubAt == "" {
						pubAt = it.Snippet.PublishedAt
					}
					items = append(items, channelUpload{
						VideoID:      vid,
						Title:        html.UnescapeString(it.Snippet.Title),
						Description:  html.UnescapeString(it.Snippet.Description),
						PublishedAt:  pubAt,
						WatchURL:     fmt.Sprintf("https://www.youtube.com/watch?v=%s", vid),
						EmbedURL:     fmt.Sprintf("https://www.youtube.com/embed/%s", vid),
						ThumbnailURL: thumb,
						Position:     it.Snippet.Position,
					})
					if len(items) >= top {
						break fetchLoop
					}
				}
				if piResp.NextPageToken == "" {
					break
				}
				pageToken = piResp.NextPageToken
			}

			out.Uploads = items
			out.Returned = len(items)

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}

	cmd.Flags().IntVar(&top, "top", 10, "Number of recent uploads to return")

	return cmd
}
