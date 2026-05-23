// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

// PATCH(amend-20260523: novel command) — extracts HTTP(S) links from a video
// description (real videos.list call), expands known short-link redirects, and
// filters social/storefront noise. Brings the CLI to parity with the
// yt-video-mcp fetch_description tool. Calls the real Data API; redirect
// resolution follows public redirects only and is short-circuited under verify
// env so the verifier never dials out.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/cliutil"

	"github.com/spf13/cobra"
)

type descriptionLink struct {
	URL          string `json:"url"`
	Host         string `json:"host"`
	Shortener    bool   `json:"shortener"`
	FinalURL     string `json:"finalUrl,omitempty"`
	FinalHost    string `json:"finalHost,omitempty"`
	Skipped      bool   `json:"skipped,omitempty"`
	SkipReason   string `json:"skipReason,omitempty"`
	ResolveError string `json:"resolveError,omitempty"`
}

type videoLinksResponse struct {
	VideoID  string            `json:"videoId"`
	Title    string            `json:"title,omitempty"`
	Returned int               `json:"returned"`
	Links    []descriptionLink `json:"links"`
}

// urlRegex matches bare HTTP(S) URLs in free text. Trailing punctuation is
// trimmed after the match so a URL ending a sentence doesn't keep its period.
var urlRegex = regexp.MustCompile(`https?://[^\s<>"')\]]+`)

// videoIDRe matches a bare YouTube video ID: exactly 11 chars from the
// URL-safe base64 alphabet. Used to reject channel/playlist/user URLs whose
// final path segment is not a video ID.
var videoIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// knownShorteners are hosts whose links we expand to a final URL when
// --resolve is on. Match is on exact host (case-insensitive, www-stripped).
var knownShorteners = map[string]bool{
	"amzn.to":     true,
	"bit.ly":      true,
	"goo.gl":      true,
	"ow.ly":       true,
	"t.co":        true,
	"tinyurl.com": true,
	"buff.ly":     true,
}

// noisyHostSuffixes are storefront/social hosts skipped unless --include-social.
// Matched as a suffix so subdomains (e.g. m.facebook.com) are covered.
var noisyHostSuffixes = []string{
	"instagram.com",
	"twitter.com",
	"x.com",
	"tiktok.com",
	"facebook.com",
	"fb.com",
	"patreon.com",
	"threads.net",
	"snapchat.com",
	"discord.gg",
	"discord.com",
	"youtube.com",
	"youtu.be",
}

func newYoutubeVideosLinksCmd(flags *rootFlags) *cobra.Command {
	var resolve bool
	var includeSocial bool

	cmd := &cobra.Command{
		Use:         "videos-links <videoId>",
		Short:       "Extract resource links from a video's description (expands short links, skips social/storefront noise)",
		Example:     "  youtube-pp-cli youtube videos-links dQw4w9WgXcQ",
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
				fmt.Fprintf(cmd.ErrOrStderr(), "GET /youtube/v3/videos (id=%s, part=snippet)\n", videoID)
				if resolve {
					fmt.Fprintln(cmd.ErrOrStderr(), "HEAD <shortener URLs> (redirect resolution)")
				}
				return nil
			}

			title, description, err := fetchVideoSnippet(cmd.Context(), flags, videoID)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			out := videoLinksResponse{VideoID: videoID, Title: title}
			out.Links = extractDescriptionLinks(cmd.Context(), description, resolve, includeSocial)
			// Returned counts non-skipped links so consumers see the
			// resource-shaped total, not the raw match count.
			for _, l := range out.Links {
				if !l.Skipped {
					out.Returned++
				}
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}

	cmd.Flags().BoolVar(&resolve, "resolve", true, "Follow redirects for known short links to their final URL")
	cmd.Flags().BoolVar(&includeSocial, "include-social", false, "Include social/storefront links instead of skipping them")

	return cmd
}

// extractDescriptionLinks pulls URLs from the description, normalizes hosts,
// flags shorteners and noisy hosts, and optionally resolves shorteners. It
// dedupes by raw URL so a link repeated in the description appears once.
func extractDescriptionLinks(ctx context.Context, description string, resolve, includeSocial bool) []descriptionLink {
	matches := urlRegex.FindAllString(description, -1)
	links := make([]descriptionLink, 0, len(matches))
	seen := map[string]bool{}
	for _, raw := range matches {
		clean := strings.TrimRight(raw, ".,;:!?)\"'")
		if clean == "" || seen[clean] {
			continue
		}
		seen[clean] = true

		host := hostOf(clean)
		link := descriptionLink{
			URL:       clean,
			Host:      host,
			Shortener: knownShorteners[host],
		}

		if !includeSocial && isNoisyHost(host) {
			link.Skipped = true
			link.SkipReason = "social/storefront host (use --include-social to keep)"
			links = append(links, link)
			continue
		}

		// Resolve known shorteners to their final URL. Skip outbound dials
		// under verify env so the verifier never reaches the network.
		if resolve && link.Shortener && !cliutil.IsVerifyEnv() {
			final, ferr := resolveRedirect(ctx, clean)
			if ferr != nil {
				link.ResolveError = ferr.Error()
			} else if final != "" && final != clean {
				link.FinalURL = final
				link.FinalHost = hostOf(final)
			}
		}
		links = append(links, link)
	}
	return links
}

// hostOf returns the lowercased, www-stripped host of a URL, or "" on parse
// failure.
func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

// isNoisyHost reports whether host is (or is a subdomain of) a known
// social/storefront host.
func isNoisyHost(host string) bool {
	for _, suffix := range noisyHostSuffixes {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

// resolveRedirect follows redirects for a short link and returns the final
// URL. The 8s context (shared across the HEAD+GET fallback) is the sole bound,
// so a dead shortener can't hang the command.
func resolveRedirect(ctx context.Context, rawURL string) (string, error) {
	rctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	// No http.Client.Timeout: both requests run under rctx via
	// NewRequestWithContext, so a per-client timeout would be dead code.
	client := &http.Client{}
	// HEAD first (cheap); some shorteners only redirect on GET, so fall back.
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		req, err := http.NewRequestWithContext(rctx, method, rawURL, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", watchPageUserAgent)
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		final := resp.Request.URL.String()
		resp.Body.Close()
		if final != "" && final != rawURL {
			return final, nil
		}
	}
	return "", nil
}

// fetchVideoSnippet pulls the title and description for a video in one
// videos.list call.
func fetchVideoSnippet(ctx context.Context, flags *rootFlags, videoID string) (title, description string, err error) {
	c, err := flags.newClient()
	if err != nil {
		return "", "", err
	}
	c = c.WithContext(ctx)
	data, err := c.GetWithHeaders("/youtube/v3/videos", map[string]string{
		"id":   videoID,
		"part": "snippet",
	}, nil)
	if err != nil {
		return "", "", err
	}
	var resp struct {
		Items []struct {
			Snippet struct {
				Title       string `json:"title"`
				Description string `json:"description"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", "", err
	}
	if len(resp.Items) == 0 {
		return "", "", fmt.Errorf("video %s not found", videoID)
	}
	return html.UnescapeString(resp.Items[0].Snippet.Title),
		html.UnescapeString(resp.Items[0].Snippet.Description), nil
}

// parseVideoID extracts an 11-char video ID from a watch/shorts/embed/live URL
// or a bare ID. It deliberately returns "" for channel, playlist, and user URLs
// (e.g. /c/Name, /channel/UC…, /user/Name, /playlist?list=…) rather than
// guessing a non-video path segment, so the caller can report a clear
// "could not extract a video ID" error instead of a confusing "video X not
// found" after a wasted videos.list call.
func parseVideoID(in string) string {
	if in == "" {
		return ""
	}
	if strings.Contains(in, "://") {
		u, err := url.Parse(in)
		if err != nil {
			return ""
		}
		if v := u.Query().Get("v"); videoIDRe.MatchString(v) {
			return v
		}
		// Only known video-bearing shapes carry an ID in the path:
		// youtu.be/<id>, /embed/<id>, /shorts/<id>, /live/<id>.
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) > 0 {
			last := parts[len(parts)-1]
			fromShortHost := strings.EqualFold(u.Hostname(), "youtu.be")
			fromVideoPath := len(parts) >= 2 &&
				(parts[len(parts)-2] == "embed" ||
					parts[len(parts)-2] == "shorts" ||
					parts[len(parts)-2] == "live")
			if (fromShortHost || fromVideoPath) && videoIDRe.MatchString(last) {
				return last
			}
		}
		return ""
	}
	// Bare ID.
	if videoIDRe.MatchString(in) {
		return in
	}
	return ""
}
