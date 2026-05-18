// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: feat-comments-and-handle-resolution — HTML-unescape the title returned by fetchVideoTitle so --with-title in markdown/iframe formats doesn't render as Don&#39;t Look Up (raw entity in markdown alt-text) or &amp;#39; (double-escape in HTML figcaption via escapeHTML). Boundary handling matches search_bulk and videos_related. Also threads cmd.Context() into fetchVideoTitle via Client.WithContext so --timeout / Ctrl+C honor the in-flight videos.list call, matching the other novel commands.

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/spf13/cobra"
)

type embedSnippet struct {
	VideoID      string `json:"videoId"`
	Title        string `json:"title,omitempty"`
	EmbedURL     string `json:"embedUrl"`
	WatchURL     string `json:"watchUrl"`
	ThumbnailURL string `json:"thumbnailUrl"`
	IframeHTML   string `json:"iframeHtml"`
	Markdown     string `json:"markdown"`
	HTML         string `json:"html"`
	Format       string `json:"format"`
	Output       string `json:"output"`
}

func newYoutubeVideosEmbedCmd(flags *rootFlags) *cobra.Command {
	var format string
	var width int
	var height int
	var withTitle bool

	cmd := &cobra.Command{
		Use:         "videos-embed <videoId>",
		Short:       "Print embed HTML, iframe, or markdown snippet for a video",
		Example:     "  youtube-pp-cli youtube videos-embed dQw4w9WgXcQ --format markdown",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			videoID := args[0]

			validFormats := map[string]bool{"url": true, "iframe": true, "markdown": true, "html": true}
			if !validFormats[format] {
				return usageErr(fmt.Errorf("invalid --format %q: must be one of url, iframe, markdown, html", format))
			}

			if dryRunOK(flags) {
				return nil
			}

			title := fmt.Sprintf("YouTube video %s", videoID)
			if withTitle && format != "url" && !dryRunOK(flags) {
				if t, err := fetchVideoTitle(cmd.Context(), flags, videoID); err == nil && t != "" {
					title = t
				}
			}

			embedURL := fmt.Sprintf("https://www.youtube.com/embed/%s", videoID)
			watchURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
			thumbURL := fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", videoID)
			iframeHTML := fmt.Sprintf(`<iframe width="%d" height="%d" src="%s" frameborder="0" allowfullscreen></iframe>`, width, height, embedURL)
			markdown := fmt.Sprintf("[![%s](%s)](%s)", title, thumbURL, watchURL)

			var htmlOut string
			if withTitle {
				htmlOut = fmt.Sprintf("<figure>\n  %s\n  <figcaption>%s</figcaption>\n</figure>", iframeHTML, escapeHTML(title))
			} else {
				htmlOut = iframeHTML
			}

			var output string
			switch format {
			case "url":
				output = embedURL
			case "iframe":
				output = iframeHTML
			case "markdown":
				output = markdown
			case "html":
				output = htmlOut
			}

			if flags.asJSON {
				snippet := embedSnippet{
					VideoID:      videoID,
					EmbedURL:     embedURL,
					WatchURL:     watchURL,
					ThumbnailURL: thumbURL,
					IframeHTML:   iframeHTML,
					Markdown:     markdown,
					HTML:         htmlOut,
					Format:       format,
					Output:       output,
				}
				if withTitle {
					snippet.Title = title
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(snippet)
			}

			fmt.Fprintln(cmd.OutOrStdout(), output)
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "url", "Output format: url, iframe, markdown, html")
	cmd.Flags().IntVar(&width, "width", 560, "Width for iframe")
	cmd.Flags().IntVar(&height, "height", 315, "Height for iframe")
	cmd.Flags().BoolVar(&withTitle, "with-title", false, "For markdown and iframe formats, fetch and include the video title (extra API call)")

	return cmd
}

// escapeHTML provides minimal HTML escaping for title text in a figcaption.
func escapeHTML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	)
	return r.Replace(s)
}

// fetchVideoTitle calls videos.list?part=snippet&id=<id> to pull just the title.
func fetchVideoTitle(ctx context.Context, flags *rootFlags, videoID string) (string, error) {
	c, err := flags.newClient()
	if err != nil {
		return "", err
	}
	c = c.WithContext(ctx)
	data, err := c.GetWithHeaders("/youtube/v3/videos", map[string]string{
		"id":   videoID,
		"part": "snippet",
	}, nil)
	if err != nil {
		return "", err
	}
	var resp struct {
		Items []struct {
			Snippet struct {
				Title string `json:"title"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	if len(resp.Items) == 0 {
		return "", fmt.Errorf("video %s not found", videoID)
	}
	return html.UnescapeString(resp.Items[0].Snippet.Title), nil
}
