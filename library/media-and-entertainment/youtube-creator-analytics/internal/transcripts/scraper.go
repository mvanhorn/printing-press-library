// Package transcripts fetches caption tracks for any public YouTube video
// without OAuth, by scraping the timedtext/InnerTube endpoints the web player
// uses. This is the only path to bulk transcripts for competitors' channels.
package transcripts

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Segment struct {
	Start    float64 `json:"start"`
	Duration float64 `json:"duration"`
	Text     string  `json:"text"`
}

type Transcript struct {
	VideoID  string    `json:"video_id"`
	Language string    `json:"language"`
	Segments []Segment `json:"segments"`
	Source   string    `json:"source"`
}

var defaultClient = &http.Client{Timeout: 20 * time.Second}

// limiter paces transcript scrapes to avoid tripping YouTube's anti-bot
// throttle. Starts at 2 req/s and adapts down on observed throttling.
var limiter = newScrapeLimiter(2.0)

// playerCaptionsRe matches the captionTracks JSON blob inside the watch page.
var playerCaptionsRe = regexp.MustCompile(`"captionTracks":(\[.*?\])`)

type captionTrack struct {
	BaseURL      string `json:"baseUrl"`
	LanguageCode string `json:"languageCode"`
}

// Fetch returns the transcript for videoID in the preferred language; falls
// back to any available track. Returns ErrNoTranscript when none exist.
func Fetch(ctx context.Context, videoID, lang string) (*Transcript, error) {
	tracks, err := discoverTracks(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if len(tracks) == 0 {
		return nil, ErrNoTranscript
	}
	track := pickTrack(tracks, lang)
	segs, err := fetchTimedText(ctx, track.BaseURL)
	if err != nil {
		return nil, err
	}
	return &Transcript{
		VideoID:  videoID,
		Language: track.LanguageCode,
		Segments: segs,
		Source:   "timedtext",
	}, nil
}

// ErrNoTranscript signals that the video has no caption tracks at all.
var ErrNoTranscript = fmt.Errorf("no caption tracks available")

func discoverTracks(ctx context.Context, videoID string) ([]captionTrack, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://www.youtube.com/watch?v="+url.QueryEscape(videoID), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; youtube-creator-analytics-pp-cli)")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	limiter.Wait()
	resp, err := defaultClient.Do(req)
	if resp != nil && resp.StatusCode == 429 {
		limiter.onThrottle()
		return nil, &RateLimitError{Source: "youtube-timedtext", After: 30 * time.Second}
	}
	if err != nil {
		return nil, fmt.Errorf("fetch watch page: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, err
	}
	m := playerCaptionsRe.FindSubmatch(body)
	if len(m) < 2 {
		return nil, nil
	}
	var tracks []captionTrack
	if err := json.Unmarshal(m[1], &tracks); err != nil {
		return nil, fmt.Errorf("parse captionTracks: %w", err)
	}
	return tracks, nil
}

func pickTrack(tracks []captionTrack, lang string) captionTrack {
	if lang != "" {
		for _, t := range tracks {
			if strings.EqualFold(t.LanguageCode, lang) {
				return t
			}
		}
		for _, t := range tracks {
			if strings.HasPrefix(strings.ToLower(t.LanguageCode), strings.ToLower(lang)) {
				return t
			}
		}
	}
	return tracks[0]
}

type ttXML struct {
	XMLName xml.Name  `xml:"transcript"`
	Texts   []ttEntry `xml:"text"`
}

type ttEntry struct {
	Start float64 `xml:"start,attr"`
	Dur   float64 `xml:"dur,attr"`
	Text  string  `xml:",chardata"`
}

var htmlEntity = strings.NewReplacer("&amp;", "&", "&#39;", "'", "&quot;", `"`, "&lt;", "<", "&gt;", ">")

func fetchTimedText(ctx context.Context, baseURL string) ([]Segment, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	limiter.Wait()
	resp, err := defaultClient.Do(req)
	if resp != nil && resp.StatusCode == 429 {
		limiter.onThrottle()
		return nil, &RateLimitError{Source: "youtube-timedtext", After: 30 * time.Second}
	}
	if err != nil {
		return nil, fmt.Errorf("fetch timedtext: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, err
	}
	var doc ttXML
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse timedtext xml: %w", err)
	}
	out := make([]Segment, 0, len(doc.Texts))
	for _, e := range doc.Texts {
		txt := strings.TrimSpace(htmlEntity.Replace(e.Text))
		if txt == "" {
			continue
		}
		out = append(out, Segment{Start: e.Start, Duration: e.Dur, Text: txt})
	}
	return out, nil
}

// PlainText flattens segments into a single space-separated string for FTS.
func (t *Transcript) PlainText() string {
	parts := make([]string, 0, len(t.Segments))
	for _, s := range t.Segments {
		parts = append(parts, s.Text)
	}
	return strings.Join(parts, " ")
}
