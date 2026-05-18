// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// v0.1: Spotify transcript-read-along scraper. Captured from a logged-in
// Premium session on 2026-05-17 against
// `https://spclient.wg.spotify.com/transcript-read-along/v2/episode/{id}?format=json&maxSentenceLength=500&excludeCC=true`.
//
// Auth model in v0.1: user supplies a fresh Bearer token via SPOTIFY_BEARER
// env var (copy from DevTools Network panel of any open.spotify.com tab; TTL
// ~1 hour). v0.2 will automate the TOTP-signed bootstrap from sp_dc cookie
// captured via `auth login-service --service spotify`.

package spotify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

const (
	adapterName   = "spotify"
	service       = "spotify"
	publicHost    = "open.spotify.com"
	transcriptAPI = "https://spclient.wg.spotify.com/transcript-read-along/v2/episode/%s?format=json&maxSentenceLength=500&excludeCC=true"
)

// Adapter is the Spotify Premium auto-generated transcript adapter.
type Adapter struct {
	Client    *http.Client
	BearerEnv string

	// cache holds an in-memory bootstrapped bearer keyed by sp_dc. Guarded
	// by mu because the dispatcher registers Adapter as a process-singleton
	// (see internal/dispatch/dispatch.go) and `episode batch` plus the MCP
	// server can hit resolveBearer concurrently. Without the mutex, the
	// check-then-set against a.cache races (flagged by Greptile P1 + Go
	// race detector).
	mu    sync.Mutex
	cache bearerCache
}

// New returns an Adapter that auto-bootstraps a bearer from the user's
// captured sp_dc cookie, with SPOTIFY_BEARER as a manual override.
func New() *Adapter {
	return &Adapter{
		Client:    &http.Client{Timeout: 30 * time.Second},
		BearerEnv: "SPOTIFY_BEARER",
	}
}

func (a *Adapter) Name() string          { return adapterName }
func (a *Adapter) Tier() transcript.Tier { return transcript.TierCookie }

// episodeIDRE captures the 22-char base62 episode id from a Spotify URL.
// Spotify episode IDs are exactly 22 chars from the base62 alphabet.
var episodeIDRE = regexp.MustCompile(`(?i)^https?://(?:open|play)\.spotify\.com/(?:embed/)?episode/([A-Za-z0-9]{22})`)

func (a *Adapter) Match(url string) bool { return episodeIDRE.MatchString(url) }

// extractEpisodeID pulls the 22-char base62 id from a Spotify URL.
func extractEpisodeID(url string) string {
	m := episodeIDRE.FindStringSubmatch(url)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// Fetch retrieves the transcript-read-along JSON for the URL's episode and
// normalizes it to the canonical Transcript shape.
func (a *Adapter) Fetch(ctx context.Context, url string) (*transcript.Transcript, error) {
	epID := extractEpisodeID(url)
	if epID == "" {
		return nil, &source.NotApplicableError{
			Source: adapterName,
			URL:    url,
			Reason: "URL does not match Spotify episode pattern",
		}
	}

	bearer, err := a.resolveBearer(ctx)
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf(transcriptAPI, epID)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build spotify request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("App-Platform", "WebPlayer")
	req.Header.Set("Spotify-App-Version", "1.2.71")

	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("spotify transcript fetch: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read spotify body: %w", err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
		// fall through
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, fmt.Errorf("spotify auth rejected (HTTP %d): SPOTIFY_BEARER expired or invalid. Capture a fresh one from DevTools (TTL ~1h)", resp.StatusCode)
	case http.StatusNotFound:
		return nil, fmt.Errorf("spotify HTTP 404: episode %s has no auto-generated transcript (most non-music podcasts do, but not all)", epID)
	default:
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return nil, fmt.Errorf("spotify HTTP %d: %s", resp.StatusCode, preview)
	}

	return parseTranscriptJSON(body, url, epID)
}

// transcriptJSON mirrors the on-wire response captured 2026-05-17 from
// Spotify Premium web player.
type transcriptJSON struct {
	Version          string            `json:"version"`
	TranscriptURI    string            `json:"transcriptUri"`
	PublishedAt      string            `json:"publishedAt"`
	Language         string            `json:"language"`
	Section          []transcriptEntry `json:"section"`
	ShowName         string            `json:"showName"`
	EpisodeName      string            `json:"episodeName"`
	Shareable        bool              `json:"shareable"`
	TimeSyncedStatus string            `json:"timeSyncedStatus"`
}

type transcriptEntry struct {
	StartMs  int64           `json:"startMs"`
	Title    *titleNode      `json:"title,omitempty"`
	Text     *textNode       `json:"text,omitempty"`
	Fallback json.RawMessage `json:"fallback,omitempty"`
}

type titleNode struct {
	Title string `json:"title"`
}

type textNode struct {
	Sentence sentenceNode `json:"sentence"`
}

type sentenceNode struct {
	StartMs int64  `json:"startMs"`
	Text    string `json:"text"`
}

// speakerLabelRE detects Spotify's auto-generated speaker labels.
// Real-name diarization sometimes appears (e.g., "Ben Gilbert") but the
// auto path emits "Speaker 1", "Speaker 2", ...
var speakerLabelRE = regexp.MustCompile(`^Speaker\s+\d+$`)

func parseTranscriptJSON(body []byte, url, epID string) (*transcript.Transcript, error) {
	var raw transcriptJSON
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse spotify json: %w", err)
	}

	idSum := sha256.Sum256([]byte(url))
	id := hex.EncodeToString(idSum[:])

	t := &transcript.Transcript{
		ID:          id,
		Source:      adapterName,
		Show:        slugify(raw.ShowName),
		Tier:        transcript.TierCookie,
		URL:         url,
		Title:       raw.EpisodeName,
		Host:        raw.ShowName,
		Provider:    "spotify-readalong",
		CostCredits: 0,
		Published:   raw.PublishedAt,
		FetchedAt:   time.Now().UTC(),
	}

	var (
		curSpeaker      = "Speaker 1"
		curSpeakerSet   = false
		curSentenceTs   int64
		curSentenceBody strings.Builder
	)

	flush := func() {
		body := strings.TrimSpace(curSentenceBody.String())
		if body == "" {
			return
		}
		t.Segments = append(t.Segments, transcript.Segment{
			TsSec:   int(curSentenceTs / 1000),
			Speaker: curSpeaker,
			Text:    body,
		})
		curSentenceBody.Reset()
	}

	for _, s := range raw.Section {
		switch {
		case s.Title != nil:
			title := strings.TrimSpace(s.Title.Title)
			if title == "" {
				continue
			}
			if speakerLabelRE.MatchString(title) {
				// Speaker turn boundary: flush prior sentence first.
				flush()
				curSpeaker = title
				curSpeakerSet = true
				curSentenceTs = s.StartMs
			} else {
				// Section header (chapter title). Flush prior, then emit
				// the section marker.
				flush()
				t.SectionTimestamps = append(t.SectionTimestamps, transcript.SectionMark{
					TsSec: int(s.StartMs / 1000),
					Title: title,
				})
				if !curSpeakerSet {
					curSentenceTs = s.StartMs
				}
			}
		case s.Text != nil:
			line := strings.TrimSpace(s.Text.Sentence.Text)
			if line == "" {
				continue
			}
			// Group consecutive sentences under the active speaker into a
			// single segment paragraph: matches the brief's "multiple
			// paragraphs from one speaker stay together until the next
			// speaker" rule.
			if curSentenceBody.Len() == 0 {
				curSentenceTs = s.Text.Sentence.StartMs
			} else {
				curSentenceBody.WriteByte(' ')
			}
			curSentenceBody.WriteString(line)
		default:
			if s.Fallback != nil {
				flush()
				t.Segments = append(t.Segments, transcript.Segment{
					TsSec:   int(s.StartMs / 1000),
					Speaker: curSpeaker,
					Text:    "[music]",
				})
			}
		}
	}
	flush()

	if len(t.Segments) == 0 {
		return nil, fmt.Errorf("spotify transcript for %s parsed to zero segments", epID)
	}

	return t, nil
}

// slugify converts "Acquired" → "acquired" for the store's `show` column,
// matching how other adapters key shows.
func slugify(s string) string {
	out := strings.ToLower(strings.TrimSpace(s))
	out = strings.ReplaceAll(out, " ", "-")
	out = strings.ReplaceAll(out, "'", "")
	out = strings.ReplaceAll(out, "—", "-")
	return out
}

// resolveBearer returns a valid Bearer token. Resolution order:
//  1. SPOTIFY_BEARER env var (manual override, useful for testing)
//  2. In-memory cache from a prior call in this process
//  3. Auto-bootstrap from the sp_dc cookie captured via `auth login-service --service spotify`
//
// If none work, returns the most actionable error (CookieMissingError when no
// cookie file exists; otherwise the bootstrap's underlying failure).
func (a *Adapter) resolveBearer(ctx context.Context) (string, error) {
	if v := os.Getenv(a.BearerEnv); v != "" {
		return strings.TrimSpace(strings.TrimPrefix(v, "Bearer ")), nil
	}
	spDC, err := readSpDC()
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	if a.cache.valid(spDC) {
		tok := a.cache.token
		a.mu.Unlock()
		return tok, nil
	}
	a.mu.Unlock()

	// Disk-cache lookup before TOTP bootstrap. Survives across CLI invocations.
	// The HTTP call to spclient + disk read happen outside the mutex so we
	// don't serialize concurrent fetches behind one mutex hold.
	if tok, exp, hit := LoadDiskCache(spDC); hit {
		a.mu.Lock()
		a.cache.token = tok
		a.cache.spDC = spDC
		a.cache.expiresAt = exp
		a.mu.Unlock()
		return tok, nil
	}

	token, expMs, err := bootstrapBearer(ctx, a.Client, spDC)
	if err != nil {
		return "", fmt.Errorf("spotify auto-bootstrap failed: %w. Workaround: set %s manually from DevTools (TTL ~1h)", err, a.BearerEnv)
	}

	a.mu.Lock()
	a.cache.token = token
	a.cache.spDC = spDC
	a.cache.expiresAt = time.Unix(0, expMs*int64(time.Millisecond))
	a.mu.Unlock()

	// Best-effort disk persist; failures don't break the in-memory path.
	_ = SaveDiskCache(spDC, token, expMs)
	return token, nil
}

// readSpDC pulls the sp_dc cookie value from the locally captured cookies
// file for the spotify service. Returns CookieMissingError when none is
// captured.
func readSpDC() (string, error) {
	if !source.HasCookie(service) {
		return "", &source.CookieMissingError{
			Service: service,
			Hint:    "run `auth login-service --service spotify` after logging in at open.spotify.com (Premium required for auto-transcripts). The bearer is then auto-bootstrapped via TOTP-signed bootstrap on each `episode get`.",
		}
	}
	raw, err := os.ReadFile(source.CookieFile(service))
	if err != nil {
		return "", fmt.Errorf("read spotify cookie: %w", err)
	}
	cookies, err := source.ParseCookieJSON(raw)
	if err != nil {
		return "", fmt.Errorf("parse spotify cookie: %w", err)
	}
	for _, c := range cookies {
		if c.Name == "sp_dc" && c.Value != "" {
			return c.Value, nil
		}
	}
	return "", fmt.Errorf("spotify cookie file has no sp_dc entry — re-run `auth login-service --service spotify` while logged into open.spotify.com")
}

var _ source.Adapter = (*Adapter)(nil)
