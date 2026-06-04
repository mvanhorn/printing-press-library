// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

// Package youtube is the source adapter for tracked YouTube creator
// channels (James Hoffmann and Lance Hedrick). Discovery uses the
// public RSS feed at /feeds/videos.xml?channel_id=<id>; transcripts
// are fetched via a youtube-pp-cli subprocess so we don't have to
// re-implement transcript scraping in Go.
package youtube

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/roasters"
)

// ErrYoutubeCliMissing signals that the optional youtube-pp-cli
// binary is not on PATH. Callers (sync orchestrator) handle this
// gracefully — RSS discovery still works, transcripts are skipped.
var ErrYoutubeCliMissing = errors.New("youtube-pp-cli not on PATH; install it to enable transcript ingest")

// Creator is one of the tracked YouTube creators.
type Creator struct {
	Slug      string
	Name      string
	ChannelID string
}

// TrackedCreators is the curated list this CLI ingests.
var TrackedCreators = []Creator{
	{Slug: "hoffmann", Name: "James Hoffmann", ChannelID: "UCMb0O2CdPBNi-QqPk5T3gsQ"},
	{Slug: "hedrick", Name: "Lance Hedrick", ChannelID: "UCvNpZQzurSNZQ8e2QNGNXsA"},
}

// VideoReview is the unified shape this adapter emits. Stored in
// youtube_reviews.
type VideoReview struct {
	VideoID                   string
	Creator                   string
	ChannelID                 string
	VideoTitle                string
	VideoPublishedAt          string
	TranscriptText            string
	MentionedRoasterSlugsJSON string
	MentionedBeanHandlesJSON  string
}

// Fetcher is the adapter entrypoint.
type Fetcher struct {
	HTTP    *http.Client
	Limiter *cliutil.AdaptiveLimiter
}

// New returns a Fetcher with sensible defaults.
func New() *Fetcher {
	return &Fetcher{
		HTTP:    &http.Client{Timeout: 20 * time.Second},
		Limiter: cliutil.NewAdaptiveLimiter(1.0),
	}
}

// rssFeed mirrors the slice of the YouTube channel RSS we consume.
type rssFeed struct {
	XMLName xml.Name  `xml:"feed"`
	Entries []rssItem `xml:"entry"`
}

type rssItem struct {
	VideoID   string `xml:"videoId"`
	Title     string `xml:"title"`
	Published string `xml:"published"`
}

// Fetch pulls latest videos for one creator. Returns ErrYoutubeCliMissing
// when the youtube-pp-cli binary isn't on PATH (callers should
// downgrade to a warning, not abort the sync).
func (f *Fetcher) Fetch(ctx context.Context, creator Creator, lastSyncedAt time.Time) ([]VideoReview, error) {
	youtubeCliPath, err := exec.LookPath("youtube-pp-cli")
	if err != nil {
		return nil, ErrYoutubeCliMissing
	}

	feed, err := f.fetchRSSFeed(ctx, creator)
	if err != nil {
		return nil, err
	}

	var out []VideoReview
	for _, e := range feed.Entries {
		pub, _ := time.Parse(time.RFC3339, e.Published)
		if !lastSyncedAt.IsZero() && pub.Before(lastSyncedAt) {
			continue
		}
		v := VideoReview{
			VideoID:          e.VideoID,
			Creator:          creator.Slug,
			ChannelID:        creator.ChannelID,
			VideoTitle:       e.Title,
			VideoPublishedAt: e.Published,
		}
		// Bound transcript work under dogfood / verify.
		if cliutil.IsVerifyEnv() {
			out = append(out, v)
			continue
		}
		transcript, terr := fetchTranscript(ctx, youtubeCliPath, e.VideoID)
		if terr != nil {
			// Per-video transcript failure is non-fatal; we still keep
			// the video header so creator-review can surface the title.
			out = append(out, v)
			continue
		}
		v.TranscriptText = transcript

		// Mention extraction: case-insensitive substring scan against
		// every registered roaster's name + slug.
		slugs, handles := scanMentions(transcript, roasters.All())
		if len(slugs) > 0 {
			if b, err := json.Marshal(slugs); err == nil {
				v.MentionedRoasterSlugsJSON = string(b)
			}
		}
		if len(handles) > 0 {
			if b, err := json.Marshal(handles); err == nil {
				v.MentionedBeanHandlesJSON = string(b)
			}
		}
		out = append(out, v)

		if cliutil.IsDogfoodEnv() && len(out) >= 3 {
			break
		}
	}
	return out, nil
}

func (f *Fetcher) fetchRSSFeed(ctx context.Context, creator Creator) (*rssFeed, error) {
	f.Limiter.Wait()
	url := fmt.Sprintf("https://www.youtube.com/feeds/videos.xml?channel_id=%s", creator.ChannelID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("youtube.Fetch rss: %w", err)
	}
	req.Header.Set("User-Agent", "coffee-goat-pp-cli (+specialty-coffee aggregator)")
	resp, err := f.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("youtube.Fetch rss: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		f.Limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{
			URL:        url,
			RetryAfter: cliutil.RetryAfter(resp),
		}
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("youtube.Fetch rss: HTTP %d: %s", resp.StatusCode, string(body))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, fmt.Errorf("youtube.Fetch rss read: %w", err)
	}
	f.Limiter.OnSuccess()
	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("youtube.Fetch rss decode: %w", err)
	}
	return &feed, nil
}

// fetchTranscript shells out to youtube-pp-cli to get one video's
// transcript. stderr is captured separately so any complaint from
// the helper doesn't pollute the JSON parse.
func fetchTranscript(ctx context.Context, binPath, videoID string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, binPath, "youtube", "videos-transcript", videoID, "--json")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("youtube-pp-cli transcript %s: %w (stderr: %s)", videoID, err, stderr.String())
	}
	// The transcript helper emits a JSON object with a "text" field
	// holding the joined transcript. Parse and return the text.
	var resp struct {
		Text       string `json:"text"`
		Transcript string `json:"transcript"`
	}
	if jerr := json.Unmarshal(out, &resp); jerr == nil {
		if resp.Text != "" {
			return resp.Text, nil
		}
		if resp.Transcript != "" {
			return resp.Transcript, nil
		}
	}
	// Fallback: treat the whole stdout as the transcript.
	return string(out), nil
}

// scanMentions returns lists of mentioned roaster slugs and bean
// handles (currently bean handle detection is a stub: matching on
// roaster name alone) found in transcript. Case-insensitive.
func scanMentions(transcript string, rs []roasters.Roaster) (slugs, handles []string) {
	lower := strings.ToLower(transcript)
	seen := map[string]bool{}
	for _, r := range rs {
		if r.Slug == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(r.Name)) || strings.Contains(lower, strings.ToLower(r.Slug)) {
			if !seen[r.Slug] {
				slugs = append(slugs, r.Slug)
				seen[r.Slug] = true
			}
		}
	}
	return slugs, handles
}
