// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 spoken.md paid adapter (demo key fallback).

package spoken

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

const (
	adapterName = "spoken"
	baseURL     = "https://spoken.md"
	demoKey     = "pt_demo"
)

// Adapter is the spoken.md paid transcripts adapter.
type Adapter struct {
	Client *http.Client
	APIKey string // resolved from env or demo
	// AllowDemo controls whether we fall back to the public demo key. The
	// dispatcher only enables this when the user explicitly opted into paid.
	AllowDemo bool
}

func New() *Adapter {
	return &Adapter{
		Client:    &http.Client{Timeout: 30 * time.Second},
		APIKey:    os.Getenv("SPOKEN_API_KEY"),
		AllowDemo: true,
	}
}

func (a *Adapter) Name() string          { return adapterName }
func (a *Adapter) Tier() transcript.Tier { return transcript.TierPaid }

var hostRE = regexp.MustCompile(`^https?://(www\.)?spoken\.md/`)

// Match accepts spoken.md URLs directly AND any HTTPS URL as a paid universal
// fallback — spoken.md resolves arbitrary episode URLs through its search
// endpoint. The dispatcher gates by tier+--paid before firing, so this isn't
// overreach: it's the brief's "any URL" promise. Non-paid runs never see this
// adapter.
func (a *Adapter) Match(url string) bool {
	if hostRE.MatchString(url) {
		return true
	}
	return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")
}

func (a *Adapter) key() string {
	if a.APIKey != "" {
		return a.APIKey
	}
	if a.AllowDemo {
		return demoKey
	}
	return ""
}

// Search resolves a non-spoken episode URL to a spoken.md transcript id.
func (a *Adapter) Search(ctx context.Context, q string) (string, error) {
	k := a.key()
	if k == "" {
		return "", &source.KeyMissingError{EnvVar: "SPOKEN_API_KEY", URL: "https://spoken.md/"}
	}
	u := baseURL + "/search?q=" + url.QueryEscape(q)
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("x-api-key", k)
	req.Header.Set("Accept", "application/json")
	resp, err := a.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("spoken search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("spoken search: HTTP %d", resp.StatusCode)
	}
	var sr struct {
		Results []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return "", fmt.Errorf("spoken search decode: %w", err)
	}
	if len(sr.Results) == 0 {
		return "", &source.NotApplicableError{Source: adapterName, URL: q, Reason: "no spoken.md results"}
	}
	return sr.Results[0].ID, nil
}

// Fetch loads a transcript by URL. If the URL is not spoken.md itself, it
// performs a search() resolution first.
func (a *Adapter) Fetch(ctx context.Context, episodeURL string) (*transcript.Transcript, error) {
	k := a.key()
	if k == "" {
		return nil, &source.KeyMissingError{EnvVar: "SPOKEN_API_KEY", URL: "https://spoken.md/"}
	}

	id := ""
	if strings.HasPrefix(episodeURL, baseURL+"/transcripts/") {
		id = strings.TrimPrefix(episodeURL, baseURL+"/transcripts/")
		id = strings.TrimSuffix(id, "/")
	} else {
		resolved, err := a.Search(ctx, episodeURL)
		if err != nil {
			return nil, err
		}
		id = resolved
	}

	u := baseURL + "/transcripts/" + id
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("x-api-key", k)
	req.Header.Set("Accept", "text/markdown,*/*")
	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("spoken GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, &source.KeyMissingError{EnvVar: "SPOKEN_API_KEY", URL: "https://spoken.md/"}
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("spoken GET %s: HTTP %d (%s)", u, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	credits := 0.0
	if v := resp.Header.Get("X-Credits-Charged"); v != "" {
		credits, _ = strconv.ParseFloat(v, 64)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("spoken read body: %w", err)
	}
	md := string(body)
	segs, sections, title := parseSpokenMarkdown(md)

	return &transcript.Transcript{
		ID:                transcript.IDFor(episodeURL),
		Source:            adapterName,
		Show:              guessShowFromTitle(title),
		Tier:              transcript.TierPaid,
		URL:               episodeURL,
		Title:             title,
		Provider:          adapterName,
		CostCredits:       credits,
		Segments:          segs,
		SectionTimestamps: sections,
		FetchedAt:         time.Now().UTC(),
	}, nil
}

// spokenSpeakerRE matches "**Speaker** (HH:MM:SS)" or "**Speaker** (MM:SS)".
var spokenSpeakerRE = regexp.MustCompile(`^\*\*([^*]+)\*\*\s*\((\d{1,2}:\d{2}(?::\d{2})?)\)\s*$`)

// spokenH2RE matches "## Section title (MM:SS)" optionally with timestamp.
var spokenH2RE = regexp.MustCompile(`^##\s+(.+?)(?:\s*\((\d{1,2}:\d{2}(?::\d{2})?)\))?\s*$`)

func parseTS(s string) int {
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 2:
		m, _ := strconv.Atoi(parts[0])
		sec, _ := strconv.Atoi(parts[1])
		return m*60 + sec
	case 3:
		h, _ := strconv.Atoi(parts[0])
		m, _ := strconv.Atoi(parts[1])
		sec, _ := strconv.Atoi(parts[2])
		return h*3600 + m*60 + sec
	}
	return 0
}

func parseSpokenMarkdown(md string) (segs []transcript.Segment, sections []transcript.SectionMark, title string) {
	lines := strings.Split(md, "\n")
	curSpeaker, curTS := "", 0
	var bodyBuf []string
	flush := func() {
		if curSpeaker == "" {
			bodyBuf = bodyBuf[:0]
			return
		}
		body := strings.TrimSpace(strings.Join(bodyBuf, "\n"))
		if body != "" {
			segs = append(segs, transcript.Segment{TsSec: curTS, Speaker: curSpeaker, Text: body})
		}
		bodyBuf = bodyBuf[:0]
	}
	for _, raw := range lines {
		ln := strings.TrimRight(raw, " \r\t")
		if title == "" && strings.HasPrefix(ln, "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(ln, "# "))
			continue
		}
		if m := spokenSpeakerRE.FindStringSubmatch(ln); m != nil {
			flush()
			curSpeaker = strings.TrimSpace(m[1])
			curTS = parseTS(m[2])
			continue
		}
		if m := spokenH2RE.FindStringSubmatch(ln); m != nil {
			flush()
			ts := 0
			if m[2] != "" {
				ts = parseTS(m[2])
			}
			sections = append(sections, transcript.SectionMark{TsSec: ts, Title: strings.TrimSpace(m[1])})
			continue
		}
		bodyBuf = append(bodyBuf, ln)
	}
	flush()
	return
}

func guessShowFromTitle(title string) string {
	if title == "" {
		return ""
	}
	// "Show Name: Episode title" -> "show-name"
	if idx := strings.Index(title, ":"); idx > 0 {
		s := strings.ToLower(strings.TrimSpace(title[:idx]))
		s = strings.ReplaceAll(s, " ", "-")
		return s
	}
	return ""
}

var _ source.Adapter = (*Adapter)(nil)
