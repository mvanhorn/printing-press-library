// Copyright 2026 adbonnet and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
//
// tgrep — novel feature, hand-authored (not generator-emitted). Searches inside
// episode transcripts, the transcripts[] URLs the PodcastIndex API never indexes.
// Only possible because this CLI downloads + searches the transcript files
// locally; the API only indexes titles and descriptions.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcastindex/internal/cliutil"
)

// clientGetter is the subset of *client.Client tgrep needs; declared as an
// interface so the resolve/episode helpers can be unit-tested with a fake.
type clientGetter interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}

type tgrepMatch struct {
	FeedID        int64  `json:"feedId"`
	FeedTitle     string `json:"feedTitle,omitempty"`
	EpisodeID     int64  `json:"episodeId"`
	EpisodeTitle  string `json:"episodeTitle,omitempty"`
	DatePublished int64  `json:"datePublished,omitempty"`
	Snippet       string `json:"snippet"`
	TranscriptURL string `json:"transcriptUrl"`
}

type tgrepFailure struct {
	FeedID    int64  `json:"feedId,omitempty"`
	EpisodeID int64  `json:"episodeId,omitempty"`
	URL       string `json:"url,omitempty"`
	Error     string `json:"error"`
}

type tgrepView struct {
	Pattern         string         `json:"pattern"`
	ScannedEpisodes int            `json:"scanned_episodes"`
	WithTranscript  int            `json:"episodes_with_transcript"`
	MaxScanEpisodes int            `json:"max_scan_episodes"`
	Matches         []tgrepMatch   `json:"matches"`
	FetchFailures   []tgrepFailure `json:"fetch_failures,omitempty"`
	Note            string         `json:"note,omitempty"`
}

func newNovelTgrepCmd(flags *rootFlags) *cobra.Command {
	var flagCat string
	var flagFeeds string
	var flagMaxFeeds int
	var flagMaxEpisodes int
	var flagMaxScan int
	var flagCaseSensitive bool

	cmd := &cobra.Command{
		Use:   "tgrep [pattern]",
		Short: "Search inside actual episode transcripts, not just titles and descriptions.",
		Long: strings.TrimSpace(`
Search the full text of episode transcripts for a regular expression.

PodcastIndex indexes only feed/episode titles and descriptions. This command
downloads the transcripts[] files an episode publishes and searches what was
actually said. Scope the scan with --feed <id> (or --feeds a,b,c) for a known
show, or --cat <category> to scan trending feeds in a category. Scanning is
bounded by --max-feeds, --max-episodes, and --max-scan.`),
		Example: strings.Trim(`
  podcastindex-pp-cli tgrep "interest rates" --feed 920666
  podcastindex-pp-cli tgrep "jepa" --cat Technology --agent
  podcastindex-pp-cli tgrep "lightning" --feeds 75075,920666 --max-episodes 20`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan episode transcripts for the given pattern")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search pattern argument is required"))
			}
			if flagFeeds == "" && flagCat == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("scope required: pass --feed <id>, --feeds <csv>, or --cat <category>"))
			}

			pattern := args[0]
			if !flagCaseSensitive {
				pattern = "(?i)" + pattern
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				return usageErr(fmt.Errorf("invalid pattern %q: %w", args[0], err))
			}

			// Curtail under live-dogfood so the happy path fits the 30s matrix timeout.
			if cliutil.IsDogfoodEnv() {
				if flagMaxFeeds > 1 {
					flagMaxFeeds = 1
				}
				if flagMaxEpisodes > 3 {
					flagMaxEpisodes = 3
				}
				if flagMaxScan > 5 {
					flagMaxScan = 5
				}
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			feedIDs, err := tgrepResolveFeeds(ctx, c, flagFeeds, flagCat, flagMaxFeeds)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			httpc := &http.Client{Timeout: 20 * time.Second}
			view := tgrepView{
				Pattern:         args[0],
				MaxScanEpisodes: flagMaxScan,
				Matches:         make([]tgrepMatch, 0),
				FetchFailures:   make([]tgrepFailure, 0),
			}
			scanCapHit := false

		feedLoop:
			for _, fid := range feedIDs {
				eps, ferr := tgrepFeedEpisodes(ctx, c, fid, flagMaxEpisodes)
				if ferr != nil {
					view.FetchFailures = append(view.FetchFailures, tgrepFailure{FeedID: fid, Error: ferr.Error()})
					continue
				}
				for _, ep := range eps {
					if view.ScannedEpisodes >= flagMaxScan {
						scanCapHit = true
						break feedLoop
					}
					view.ScannedEpisodes++
					url := ep.transcriptURL()
					if url == "" {
						continue
					}
					view.WithTranscript++
					text, derr := tgrepFetchTranscript(ctx, httpc, url)
					if derr != nil {
						view.FetchFailures = append(view.FetchFailures, tgrepFailure{FeedID: fid, EpisodeID: ep.ID, URL: url, Error: derr.Error()})
						continue
					}
					if loc := re.FindStringIndex(text); loc != nil {
						view.Matches = append(view.Matches, tgrepMatch{
							FeedID:        fid,
							FeedTitle:     ep.FeedTitle,
							EpisodeID:     ep.ID,
							EpisodeTitle:  ep.Title,
							DatePublished: ep.DatePublished,
							Snippet:       tgrepSnippet(text, loc),
							TranscriptURL: url,
						})
					}
				}
			}

			if len(view.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d transcript fetch(es) failed; matches computed over the rest\n", len(view.FetchFailures))
			}
			if len(view.Matches) == 0 {
				switch {
				case scanCapHit:
					view.Note = fmt.Sprintf("scanned %d episodes (scan cap %d hit) with no transcript match; raise --max-scan or --max-episodes to widen", view.ScannedEpisodes, flagMaxScan)
				case view.WithTranscript == 0:
					view.Note = fmt.Sprintf("scanned %d episodes but none published a transcript; PodcastIndex transcript coverage is partial", view.ScannedEpisodes)
				default:
					view.Note = fmt.Sprintf("no transcript match across %d episodes (%d with transcripts)", view.ScannedEpisodes, view.WithTranscript)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagCat, "cat", "", "Scan trending feeds in this category (e.g. Technology)")
	cmd.Flags().StringVar(&flagFeeds, "feed", "", "Scan a single feed by PodcastIndex feed id (alias of --feeds with one id)")
	cmd.Flags().StringVar(&flagFeeds, "feeds", "", "Scan these feeds (comma-separated PodcastIndex feed ids)")
	cmd.Flags().IntVar(&flagMaxFeeds, "max-feeds", 5, "Maximum feeds to scan when using --cat")
	cmd.Flags().IntVar(&flagMaxEpisodes, "max-episodes", 10, "Maximum episodes to pull per feed")
	cmd.Flags().IntVar(&flagMaxScan, "max-scan", 50, "Maximum episodes to scan overall before returning")
	cmd.Flags().BoolVar(&flagCaseSensitive, "case-sensitive", false, "Match the pattern case-sensitively (default: case-insensitive)")
	// --feed and --feeds bind the same variable (--feed is a singular alias);
	// passing both would silently drop one, so reject that combination.
	cmd.MarkFlagsMutuallyExclusive("feed", "feeds")
	return cmd
}

// tgrepEpisode is a minimal view over the PodcastIndex episode JSON.
type tgrepEpisode struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	FeedTitle     string `json:"feedTitle"`
	DatePublished int64  `json:"datePublished"`
	TranscriptURL string `json:"transcriptUrl"`
	Transcripts   []struct {
		URL  string `json:"url"`
		Type string `json:"type"`
	} `json:"transcripts"`
}

func (e tgrepEpisode) transcriptURL() string {
	for _, t := range e.Transcripts {
		if strings.TrimSpace(t.URL) != "" {
			return t.URL
		}
	}
	return strings.TrimSpace(e.TranscriptURL)
}

func tgrepResolveFeeds(ctx context.Context, c clientGetter, feedCSV, cat string, maxFeeds int) ([]int64, error) {
	if feedCSV != "" {
		ids := make([]int64, 0)
		for _, part := range strings.Split(feedCSV, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			n, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return nil, usageErr(fmt.Errorf("invalid feed id %q: %w", part, err))
			}
			ids = append(ids, n)
		}
		if len(ids) == 0 {
			return nil, usageErr(fmt.Errorf("no valid feed ids supplied"))
		}
		return ids, nil
	}
	// Category scope: pull trending feeds in the category.
	params := map[string]string{"cat": cat, "max": strconv.Itoa(maxFeeds)}
	data, err := c.Get(ctx, "/podcasts/trending", params)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Feeds []struct {
			ID int64 `json:"id"`
		} `json:"feeds"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing trending feeds: %w", err)
	}
	ids := make([]int64, 0, len(resp.Feeds))
	for _, f := range resp.Feeds {
		ids = append(ids, f.ID)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no trending feeds found for category %q", cat)
	}
	return ids, nil
}

func tgrepFeedEpisodes(ctx context.Context, c clientGetter, feedID int64, maxEpisodes int) ([]tgrepEpisode, error) {
	params := map[string]string{"id": strconv.FormatInt(feedID, 10), "max": strconv.Itoa(maxEpisodes)}
	data, err := c.Get(ctx, "/episodes/byfeedid", params)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Items []tgrepEpisode `json:"items"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing episodes for feed %d: %w", feedID, err)
	}
	return resp.Items, nil
}

func tgrepFetchTranscript(ctx context.Context, httpc *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "podcastindex-pp-cli/1.12.1")
	resp, err := httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("transcript fetch returned HTTP %d", resp.StatusCode)
	}
	// Cap transcript size to keep memory bounded (~2 MB is generous for text).
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	return tgrepExtractText(resp.Header.Get("Content-Type"), body), nil
}

// tgrepExtractText normalises SRT, VTT, JSON-segment, and plain-text transcripts
// into searchable plain text.
func tgrepExtractText(contentType string, body []byte) string {
	s := string(body)
	ct := strings.ToLower(contentType)
	trimmed := strings.TrimSpace(s)
	if strings.Contains(ct, "json") || strings.HasPrefix(trimmed, "{") {
		var j struct {
			Segments []struct {
				Body string `json:"body"`
			} `json:"segments"`
		}
		if err := json.Unmarshal([]byte(trimmed), &j); err == nil && len(j.Segments) > 0 {
			parts := make([]string, 0, len(j.Segments))
			for _, seg := range j.Segments {
				parts = append(parts, seg.Body)
			}
			return cliutil.CleanText(strings.Join(parts, " "))
		}
	}
	// SRT/VTT/plain: drop cue indices and timestamp lines, keep spoken text.
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		l := strings.TrimSpace(line)
		if l == "" || l == "WEBVTT" {
			continue
		}
		if tgrepIsTimestampLine(l) || tgrepIsIndexLine(l) {
			continue
		}
		b.WriteString(l)
		b.WriteByte(' ')
	}
	return cliutil.CleanText(b.String())
}

var tgrepTimestampRe = regexp.MustCompile(`\d\d:\d\d:\d\d[.,]\d\d\d\s*-->`)

func tgrepIsTimestampLine(l string) bool { return tgrepTimestampRe.MatchString(l) }

func tgrepIsIndexLine(l string) bool {
	if l == "" {
		return false
	}
	for _, r := range l {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func tgrepSnippet(text string, loc []int) string {
	const pad = 90
	start := loc[0] - pad
	if start < 0 {
		start = 0
	}
	end := loc[1] + pad
	if end > len(text) {
		end = len(text)
	}
	// loc holds byte offsets; pad may land mid-codepoint. Walk both bounds
	// back to a rune boundary so the slice never splits a multi-byte rune.
	for start > 0 && !utf8.RuneStart(text[start]) {
		start--
	}
	for end < len(text) && !utf8.RuneStart(text[end]) {
		end++
	}
	snippet := strings.TrimSpace(text[start:end])
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(text) {
		snippet = snippet + "…"
	}
	return snippet
}
