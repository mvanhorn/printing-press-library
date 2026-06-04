// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: feat-comments-and-handle-resolution — HTML-unescape titles/channelTitle (mirrors videos_related.go:searchListVideos boundary handling so &#39; and &amp; don't bleed into JSON consumers); thread cmd.Context() into the client so --timeout / Ctrl+C cancel the per-term sequential search.list calls.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// bulkResult is the per-term result group.
type bulkSearchResult struct {
	VideoID      string `json:"videoId"`
	Title        string `json:"title"`
	ChannelTitle string `json:"channelTitle"`
	ChannelID    string `json:"channelId"`
	PublishedAt  string `json:"publishedAt"`
	ThumbnailURL string `json:"thumbnailUrl"`
	EmbedURL     string `json:"embedUrl"`
	WatchURL     string `json:"watchUrl"`
	Description  string `json:"description"`
}

type bulkTermGroup struct {
	Query   string             `json:"query"`
	Results []bulkSearchResult `json:"results"`
	Error   string             `json:"error,omitempty"`
}

type bulkResponse struct {
	Terms []bulkTermGroup `json:"terms"`
}

func newYoutubeSearchBulkCmd(flags *rootFlags) *cobra.Command {
	var fromStdin bool
	var top int
	var region string
	var lang string

	cmd := &cobra.Command{
		Use:   "search-bulk [terms...]",
		Short: "Search YouTube for multiple terms in one call, return top-N per term",
		Long: `Run a YouTube search for each term and aggregate the top-N results into a single JSON document.

Reads terms from positional args, or from stdin (one per line) when --stdin is set. For each term,
calls search.list with part=snippet&type=video and returns the top results with title, channel,
embed URL, thumbnail, and description.`,
		Example: `  youtube-pp-cli youtube search-bulk "sourdough scoring" "knife sharpening"
  printf "term1\nterm2\n" | youtube-pp-cli youtube search-bulk --stdin --top 3`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Collect terms
			var terms []string
			if fromStdin {
				scanner := bufio.NewScanner(cmd.InOrStdin())
				for scanner.Scan() {
					t := strings.TrimSpace(scanner.Text())
					if t != "" {
						terms = append(terms, t)
					}
				}
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
			} else {
				terms = append(terms, args...)
			}

			if len(terms) == 0 {
				return cmd.Help()
			}

			if dryRunOK(flags) {
				// Show what would be sent for the first term
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				params := map[string]string{
					"part":       "snippet",
					"q":          terms[0],
					"maxResults": fmt.Sprintf("%d", top),
					"type":       "video",
				}
				if region != "" {
					params["regionCode"] = region
				}
				if lang != "" {
					params["relevanceLanguage"] = lang
				}
				_, _ = c.GetWithHeaders("/youtube/v3/search", params, nil)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c = c.WithContext(cmd.Context())

			out := bulkResponse{Terms: make([]bulkTermGroup, 0, len(terms))}
			for _, term := range terms {
				params := map[string]string{
					"part":       "snippet",
					"q":          term,
					"maxResults": fmt.Sprintf("%d", top),
					"type":       "video",
				}
				if region != "" {
					params["regionCode"] = region
				}
				if lang != "" {
					params["relevanceLanguage"] = lang
				}

				group := bulkTermGroup{Query: term, Results: []bulkSearchResult{}}
				data, err := c.GetWithHeaders("/youtube/v3/search", params, nil)
				if err != nil {
					group.Error = err.Error()
					out.Terms = append(out.Terms, group)
					continue
				}

				var resp struct {
					Items []struct {
						ID struct {
							VideoID string `json:"videoId"`
						} `json:"id"`
						Snippet struct {
							PublishedAt  string `json:"publishedAt"`
							ChannelID    string `json:"channelId"`
							ChannelTitle string `json:"channelTitle"`
							Title        string `json:"title"`
							Description  string `json:"description"`
							Thumbnails   map[string]struct {
								URL string `json:"url"`
							} `json:"thumbnails"`
						} `json:"snippet"`
					} `json:"items"`
				}
				if err := json.Unmarshal(data, &resp); err != nil {
					group.Error = fmt.Sprintf("parse error: %v", err)
					out.Terms = append(out.Terms, group)
					continue
				}

				for _, it := range resp.Items {
					vid := it.ID.VideoID
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
					if thumb == "" {
						thumb = fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", vid)
					}
					group.Results = append(group.Results, bulkSearchResult{
						VideoID:      vid,
						Title:        html.UnescapeString(it.Snippet.Title),
						ChannelTitle: html.UnescapeString(it.Snippet.ChannelTitle),
						ChannelID:    it.Snippet.ChannelID,
						PublishedAt:  it.Snippet.PublishedAt,
						ThumbnailURL: thumb,
						EmbedURL:     fmt.Sprintf("https://www.youtube.com/embed/%s", vid),
						WatchURL:     fmt.Sprintf("https://www.youtube.com/watch?v=%s", vid),
						Description:  html.UnescapeString(it.Snippet.Description),
					})
				}
				out.Terms = append(out.Terms, group)
			}

			// Always JSON for this command (the response shape is the value).
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}

	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "Read terms from stdin (one per line) instead of args")
	cmd.Flags().IntVar(&top, "top", 5, "Top N results per term")
	cmd.Flags().StringVar(&region, "region", "US", "regionCode for search")
	cmd.Flags().StringVar(&lang, "lang", "", "relevanceLanguage filter (e.g. en, es)")

	// Silence unused import in case fromStdin is the only consumer of os.
	_ = os.Stdin

	return cmd
}
