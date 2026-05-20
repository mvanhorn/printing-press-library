// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 dwarkesh.com Substack /p/<slug> HTML scraper.

package dwarkesh

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

const adapterName = "dwarkesh"

// Adapter is the dwarkesh.com Substack post scraper.
type Adapter struct {
	Client *http.Client
}

// New returns a default Adapter.
func New() *Adapter {
	return &Adapter{Client: &http.Client{Timeout: 30 * time.Second}}
}

func (a *Adapter) Name() string             { return adapterName }
func (a *Adapter) Tier() transcript.Tier    { return transcript.TierFree }

var hostRE = regexp.MustCompile(`^https?://(www\.)?dwarkesh(patel)?\.com/p/`)

func (a *Adapter) Match(url string) bool {
	return hostRE.MatchString(url)
}

// stripTags removes HTML tags from a fragment, preserving inline text.
var tagRE = regexp.MustCompile(`<[^>]+>`)

func stripTags(s string) string {
	return cliutil.CleanText(tagRE.ReplaceAllString(s, ""))
}

// headerTSRE captures HH:MM:SS or MM:SS at the tail of an h2 header.
var headerTSRE = regexp.MustCompile(`\(((?:\d{1,2}:)?\d{1,2}:\d{2})\)\s*$`)

// titleRE picks the <title> tag.
var titleRE = regexp.MustCompile(`(?is)<title>(.*?)</title>`)

// metaPubRE picks the article:published_time meta tag.
var metaPubRE = regexp.MustCompile(`(?i)<meta[^>]+property=["']article:published_time["'][^>]+content=["']([^"']+)["']`)

// strongRE captures bold-wrapped speaker labels.
var strongRE = regexp.MustCompile(`(?is)<strong>(.*?)</strong>`)

// pRE matches a <p> block.
var pRE = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)

func parseTS(s string) int {
	parts := strings.Split(strings.TrimSpace(s), ":")
	var h, m, sec int
	switch len(parts) {
	case 2:
		m, _ = strconv.Atoi(parts[0])
		sec, _ = strconv.Atoi(parts[1])
	case 3:
		h, _ = strconv.Atoi(parts[0])
		m, _ = strconv.Atoi(parts[1])
		sec, _ = strconv.Atoi(parts[2])
	default:
		return 0
	}
	return h*3600 + m*60 + sec
}

// extractFirstStrong returns the first <strong>name</strong> in fragment and the
// remaining body. Returns "", original if none.
func extractFirstStrong(p string) (speaker, body string) {
	loc := strongRE.FindStringSubmatchIndex(p)
	if loc == nil {
		return "", p
	}
	speaker = stripTags(p[loc[2]:loc[3]])
	// strip trailing colon
	speaker = strings.TrimSuffix(strings.TrimSpace(speaker), ":")
	// rest = everything after the strong tag
	body = p[loc[1]:]
	return speaker, body
}

func (a *Adapter) Fetch(ctx context.Context, url string) (*transcript.Transcript, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "podcast-goat-pp-cli/0.1 (+https://github.com/mvanhorn/printing-press-library)")
	req.Header.Set("Accept", "text/html,*/*")
	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dwarkesh GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("dwarkesh GET %s: HTTP %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("dwarkesh read body: %w", err)
	}
	html := string(body)

	title := ""
	if m := titleRE.FindStringSubmatch(html); m != nil {
		title = stripTags(m[1])
		// Substack title pattern: "Episode Title - Dwarkesh Podcast"
		title = strings.TrimSuffix(title, " - Dwarkesh Podcast")
	}
	pubDate := ""
	if m := metaPubRE.FindStringSubmatch(html); m != nil {
		pubDate = m[1]
	}

	// Walk H2 and P elements in document order to interleave section marks
	// with segments while still tagging the current speaker. Go's RE2 has no
	// backreferences, so we scan h2 and p separately and merge by starting
	// offset to preserve document order.
	type token struct {
		kind string // "h2" | "p"
		raw  string
	}
	var tokens []token
	h2ScanRE := regexp.MustCompile(`(?is)<h2[^>]*>(.*?)</h2>`)
	pScanRE := regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)
	type located struct {
		start int
		tok   token
	}
	var found []located
	for _, idxs := range h2ScanRE.FindAllStringSubmatchIndex(html, -1) {
		found = append(found, located{start: idxs[0], tok: token{kind: "h2", raw: html[idxs[2]:idxs[3]]}})
	}
	for _, idxs := range pScanRE.FindAllStringSubmatchIndex(html, -1) {
		found = append(found, located{start: idxs[0], tok: token{kind: "p", raw: html[idxs[2]:idxs[3]]}})
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].start < found[j].start })
	for _, f := range found {
		tokens = append(tokens, f.tok)
	}

	var (
		segs       []transcript.Segment
		sections   []transcript.SectionMark
		curSpeaker = ""
		curTS      = 0
		host       = "Dwarkesh Patel"
		guests     []string
		seenGuest  = map[string]bool{}
	)

	for _, t := range tokens {
		if t.kind == "h2" {
			text := stripTags(t.raw)
			ts := 0
			if m := headerTSRE.FindStringSubmatch(text); m != nil {
				ts = parseTS(m[1])
				text = strings.TrimSpace(headerTSRE.ReplaceAllString(text, ""))
			}
			if text != "" {
				sections = append(sections, transcript.SectionMark{TsSec: ts, Title: text})
				curTS = ts
			}
			continue
		}
		// p block
		speaker, rest := extractFirstStrong(t.raw)
		if speaker != "" {
			curSpeaker = speaker
			if curSpeaker != host && !seenGuest[curSpeaker] {
				seenGuest[curSpeaker] = true
				guests = append(guests, curSpeaker)
			}
		}
		text := stripTags(rest)
		if text == "" {
			continue
		}
		if curSpeaker == "" {
			// Pre-transcript prose (intro paragraph) — skip.
			continue
		}
		segs = append(segs, transcript.Segment{
			TsSec:   curTS,
			Speaker: curSpeaker,
			Text:    text,
		})
	}

	if len(segs) == 0 {
		return nil, fmt.Errorf("dwarkesh: no speaker segments found at %s (page shape may have changed)", url)
	}

	return &transcript.Transcript{
		ID:                transcript.IDFor(url),
		Source:            adapterName,
		Show:              "dwarkesh-podcast",
		Tier:              transcript.TierFree,
		URL:               url,
		Title:             title,
		Host:              host,
		Guests:            guests,
		Published:         pubDate,
		Provider:          adapterName,
		Segments:          segs,
		SectionTimestamps: sections,
		FetchedAt:         time.Now().UTC(),
	}, nil
}

// guard against unused-import warnings if rewriting.
var _ source.Adapter = (*Adapter)(nil)
