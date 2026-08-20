// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 Podcasting 2.0 <podcast:transcript> adapter.

package rss

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

const adapterName = "rss"

type Adapter struct {
	Client *http.Client
}

func New() *Adapter {
	return &Adapter{Client: &http.Client{Timeout: 30 * time.Second}}
}

func (a *Adapter) Name() string          { return adapterName }
func (a *Adapter) Tier() transcript.Tier { return transcript.TierFree }

// Match accepts any URL ending in .xml/.rss or with "feed" in the path, OR a
// direct episode URL that the caller hinted at via the --feed flag (handled by
// the dispatcher). For the open-ended episode-URL case, RSS is a *secondary*
// match — the dispatcher tries it after dwarkesh/cookie tiers but before paid.
var rssShapeRE = regexp.MustCompile(`(?i)(\.xml$|/rss$|/feed$|/feed/|\.rss$|/feed\.xml$|/podcast\.xml$)`)

func (a *Adapter) Match(url string) bool {
	return rssShapeRE.MatchString(url)
}

// Feed-level Podcasting 2.0 element.
type pcTranscript struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
	Lang string `xml:"language,attr,omitempty"`
}

type rssItem struct {
	GUID        string         `xml:"guid"`
	Title       string         `xml:"title"`
	Link        string         `xml:"link"`
	PubDate     string         `xml:"pubDate"`
	Description string         `xml:"description"`
	Duration    string         `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd duration"`
	Transcripts []pcTranscript `xml:"https://podcastindex.org/namespace/1.0 transcript"`
}

type rssChannel struct {
	Title string    `xml:"title"`
	Items []rssItem `xml:"item"`
}

type rssDoc struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

// Fetch treats the URL as a feed URL and returns the most recent item with
// a <podcast:transcript> tag.
func (a *Adapter) Fetch(ctx context.Context, url string) (*transcript.Transcript, error) {
	feedURL := url
	itemHint := ""
	// If URL looks like an episode page rather than a feed, this adapter
	// doesn't apply. Match() already gated; this is the explicit signal.
	if !a.Match(feedURL) {
		return nil, &source.NotApplicableError{
			Source: adapterName,
			URL:    url,
			Reason: "URL does not look like an RSS feed (use 'feeds add' to track an RSS feed)",
		}
	}

	feed, err := a.fetchFeed(ctx, feedURL)
	if err != nil {
		return nil, err
	}

	for _, item := range feed.Channel.Items {
		if itemHint != "" && !strings.Contains(item.GUID, itemHint) && !strings.Contains(item.Link, itemHint) {
			continue
		}
		for _, tr := range item.Transcripts {
			if tr.URL == "" {
				continue
			}
			out, err := a.fetchTranscript(ctx, tr, item, feed.Channel.Title)
			if err == nil {
				return out, nil
			}
		}
	}
	return nil, &source.NotApplicableError{
		Source: adapterName,
		URL:    url,
		Reason: "no <podcast:transcript> tag found in any item in this feed",
	}
}

// FetchItem is exposed for feeds-sync.
func (a *Adapter) FetchItem(ctx context.Context, feedURL string, item rssItem, showTitle string) (*transcript.Transcript, error) {
	for _, tr := range item.Transcripts {
		if tr.URL == "" {
			continue
		}
		return a.fetchTranscript(ctx, tr, item, showTitle)
	}
	return nil, &source.NotApplicableError{Source: adapterName, URL: item.Link, Reason: "no transcript tag"}
}

func (a *Adapter) fetchFeed(ctx context.Context, feedURL string) (*rssDoc, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	req.Header.Set("User-Agent", "podcast-goat-pp-cli/0.1")
	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rss GET %s: %w", feedURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("rss GET %s: HTTP %d", feedURL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("rss read body: %w", err)
	}
	var doc rssDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("rss parse: %w", err)
	}
	return &doc, nil
}

func (a *Adapter) fetchTranscript(ctx context.Context, tr pcTranscript, item rssItem, showTitle string) (*transcript.Transcript, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", tr.URL, nil)
	req.Header.Set("User-Agent", "podcast-goat-pp-cli/0.1")
	if tr.Type != "" {
		req.Header.Set("Accept", tr.Type+",*/*")
	}
	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rss transcript GET %s: %w", tr.URL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("rss transcript GET %s: HTTP %d", tr.URL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("rss read transcript: %w", err)
	}
	mimeType := strings.ToLower(strings.TrimSpace(strings.SplitN(tr.Type, ";", 2)[0]))
	if mimeType == "" {
		mimeType = strings.ToLower(strings.TrimSpace(strings.SplitN(resp.Header.Get("Content-Type"), ";", 2)[0]))
	}
	segs, err := parseByMIME(string(body), mimeType)
	if err != nil {
		return nil, err
	}
	durSec := parseDuration(item.Duration)
	out := &transcript.Transcript{
		ID:          transcript.IDFor(item.Link),
		Source:      adapterName,
		Show:        slugify(showTitle),
		Tier:        transcript.TierFree,
		URL:         item.Link,
		Title:       cliutil.CleanText(item.Title),
		Published:   item.PubDate,
		DurationSec: durSec,
		Provider:    adapterName,
		Segments:    segs,
		FetchedAt:   time.Now().UTC(),
	}
	return out, nil
}

func parseByMIME(body, mime string) ([]transcript.Segment, error) {
	switch {
	case strings.Contains(mime, "vtt"):
		return parseVTT(body), nil
	case strings.Contains(mime, "srt") || strings.Contains(mime, "subrip"):
		return parseSRT(body), nil
	case strings.Contains(mime, "json"):
		return parsePCJSON(body)
	case strings.Contains(mime, "html"):
		return parseHTML(body), nil
	case strings.Contains(mime, "plain"), mime == "":
		return parsePlain(body), nil
	}
	return parsePlain(body), nil
}

var vttTimeRE = regexp.MustCompile(`(?m)^(\d{1,2}):(\d{2}):(\d{2})\.\d{3}\s+-->`)
var srtTimeRE = regexp.MustCompile(`(?m)^(\d{1,2}):(\d{2}):(\d{2}),\d{3}\s+-->`)
var stripTagRE = regexp.MustCompile(`<[^>]+>`)

func parseVTT(s string) []transcript.Segment {
	var out []transcript.Segment
	lines := strings.Split(s, "\n")
	curTS := -1
	var buf []string
	flush := func(speaker string) {
		txt := strings.TrimSpace(strings.Join(buf, " "))
		if txt != "" && curTS >= 0 {
			out = append(out, transcript.Segment{TsSec: curTS, Speaker: speaker, Text: txt})
		}
		buf = buf[:0]
	}
	for _, raw := range lines {
		ln := strings.TrimRight(raw, "\r")
		if strings.HasPrefix(ln, "WEBVTT") || strings.HasPrefix(ln, "NOTE") {
			continue
		}
		if m := vttTimeRE.FindStringSubmatch(ln); m != nil {
			flush("Speaker")
			h, _ := strconv.Atoi(m[1])
			mn, _ := strconv.Atoi(m[2])
			sec, _ := strconv.Atoi(m[3])
			curTS = h*3600 + mn*60 + sec
			continue
		}
		if ln == "" {
			continue
		}
		clean := strings.TrimSpace(stripTagRE.ReplaceAllString(ln, ""))
		if clean != "" {
			buf = append(buf, clean)
		}
	}
	flush("Speaker")
	return out
}

func parseSRT(s string) []transcript.Segment {
	var out []transcript.Segment
	lines := strings.Split(s, "\n")
	curTS := -1
	var buf []string
	flush := func() {
		txt := strings.TrimSpace(strings.Join(buf, " "))
		if txt != "" && curTS >= 0 {
			out = append(out, transcript.Segment{TsSec: curTS, Speaker: "Speaker", Text: txt})
		}
		buf = buf[:0]
	}
	for _, raw := range lines {
		ln := strings.TrimRight(raw, "\r")
		if m := srtTimeRE.FindStringSubmatch(ln); m != nil {
			flush()
			h, _ := strconv.Atoi(m[1])
			mn, _ := strconv.Atoi(m[2])
			sec, _ := strconv.Atoi(m[3])
			curTS = h*3600 + mn*60 + sec
			continue
		}
		if ln == "" || regexp.MustCompile(`^\d+$`).MatchString(ln) {
			continue
		}
		clean := strings.TrimSpace(stripTagRE.ReplaceAllString(ln, ""))
		if clean != "" {
			buf = append(buf, clean)
		}
	}
	flush()
	return out
}

func parsePCJSON(s string) ([]transcript.Segment, error) {
	// Podcasting 2.0 JSON transcript schema: { "segments": [{ "startTime", "speaker", "body" }]}
	var doc struct {
		Segments []struct {
			StartTime float64 `json:"startTime"`
			Speaker   string  `json:"speaker"`
			Body      string  `json:"body"`
		} `json:"segments"`
	}
	if err := json.Unmarshal([]byte(s), &doc); err != nil {
		return nil, fmt.Errorf("rss json transcript parse: %w", err)
	}
	var out []transcript.Segment
	for _, seg := range doc.Segments {
		speaker := strings.TrimSpace(seg.Speaker)
		if speaker == "" {
			speaker = "Speaker"
		}
		out = append(out, transcript.Segment{
			TsSec:   int(seg.StartTime),
			Speaker: speaker,
			Text:    strings.TrimSpace(seg.Body),
		})
	}
	return out, nil
}

func parseHTML(s string) []transcript.Segment {
	// Naive: each <p> is a segment, look for <strong>Speaker</strong> prefix.
	pRE := regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)
	strongRE := regexp.MustCompile(`(?is)<strong>(.*?)</strong>`)
	var out []transcript.Segment
	for _, m := range pRE.FindAllStringSubmatch(s, -1) {
		seg := transcript.Segment{Speaker: "Speaker"}
		if sm := strongRE.FindStringSubmatch(m[1]); sm != nil {
			seg.Speaker = cliutil.CleanText(stripTagRE.ReplaceAllString(sm[1], ""))
		}
		txt := cliutil.CleanText(stripTagRE.ReplaceAllString(m[1], ""))
		if seg.Speaker != "Speaker" {
			txt = strings.TrimPrefix(txt, seg.Speaker)
			txt = strings.TrimPrefix(txt, ":")
			txt = strings.TrimSpace(txt)
		}
		if txt == "" {
			continue
		}
		seg.Text = txt
		out = append(out, seg)
	}
	return out
}

func parsePlain(s string) []transcript.Segment {
	lines := strings.Split(s, "\n")
	var out []transcript.Segment
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		out = append(out, transcript.Segment{Speaker: "Speaker", Text: t})
	}
	return out
}

func parseDuration(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Could be "HH:MM:SS", "MM:SS", or "NNNN" seconds.
	if !strings.Contains(s, ":") {
		n, _ := strconv.Atoi(s)
		return n
	}
	parts := strings.Split(s, ":")
	var h, m, sec int
	switch len(parts) {
	case 2:
		m, _ = strconv.Atoi(parts[0])
		sec, _ = strconv.Atoi(parts[1])
	case 3:
		h, _ = strconv.Atoi(parts[0])
		m, _ = strconv.Atoi(parts[1])
		sec, _ = strconv.Atoi(parts[2])
	}
	return h*3600 + m*60 + sec
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// FetchFeedItems is exposed for `feeds sync`.
func (a *Adapter) FetchFeedItems(ctx context.Context, feedURL string) (showTitle string, items []FeedItem, err error) {
	doc, err := a.fetchFeed(ctx, feedURL)
	if err != nil {
		return "", nil, err
	}
	showTitle = doc.Channel.Title
	for _, it := range doc.Channel.Items {
		hasTranscript := false
		for _, tr := range it.Transcripts {
			if tr.URL != "" {
				hasTranscript = true
				break
			}
		}
		items = append(items, FeedItem{
			GUID:          it.GUID,
			Title:         it.Title,
			Link:          it.Link,
			PubDate:       it.PubDate,
			HasTranscript: hasTranscript,
		})
	}
	return showTitle, items, nil
}

// FeedItem is the externally-visible RSS item summary.
type FeedItem struct {
	GUID          string
	Title         string
	Link          string
	PubDate       string
	HasTranscript bool
}

var _ source.Adapter = (*Adapter)(nil)
