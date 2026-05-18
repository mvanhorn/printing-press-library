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
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source/titlextract"
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
		APIKey:    config.Resolve("SPOKEN_API_KEY"),
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

// SearchHit carries the fields spoken.md's /search returns for one result.
type SearchHit struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Podcast string `json:"podcast"`
	URL     string `json:"url"`
	Date    string `json:"date"`
}

// Search resolves a query (URL or title text) to a spoken.md transcript id
// plus the matched result's title + podcast so Fetch() can populate the
// canonical Transcript without re-deriving from the markdown alone.
func (a *Adapter) Search(ctx context.Context, q string) (*SearchHit, error) {
	k := a.key()
	if k == "" {
		return nil, &source.KeyMissingError{EnvVar: "SPOKEN_API_KEY", URL: "https://spoken.md/"}
	}
	u := baseURL + "/search?q=" + url.QueryEscape(q)
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("x-api-key", k)
	req.Header.Set("Accept", "application/json")
	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("spoken search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("spoken search: HTTP %d", resp.StatusCode)
	}
	var sr struct {
		Results []SearchHit `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("spoken search decode: %w", err)
	}
	if len(sr.Results) == 0 {
		return nil, &source.NotApplicableError{Source: adapterName, URL: q, Reason: "no spoken.md results"}
	}
	hit := sr.Results[0]
	return &hit, nil
}

// resolveByURL is the URL → SearchHit resolution flow. It tries the raw URL
// first (works for Apple Podcasts + Spotify URLs spoken.md indexes by id);
// on no-results, fetches the publisher page and re-searches by extracted title.
func (a *Adapter) resolveByURL(ctx context.Context, episodeURL string) (*SearchHit, error) {
	hit, err := a.Search(ctx, episodeURL)
	if err == nil {
		return hit, nil
	}
	// Only retry with title-extract on no-results errors — other failures
	// (auth, network) should propagate directly.
	var notApp *source.NotApplicableError
	if !asNotApplicable(err, &notApp) {
		return nil, err
	}
	// Skip title-extract for hosts spoken.md indexes by URL identifier.
	if isSpokenIndexedHost(episodeURL) {
		return nil, err
	}
	title, terr := titlextract.Extract(ctx, episodeURL)
	if terr != nil {
		// Title-extract failed — surface the original no-results error so
		// the dispatcher's trace reflects "spoken doesn't know this URL"
		// rather than "we couldn't fetch the page".
		return nil, err
	}
	hostHints := hostHintsFromURL(episodeURL)
	hit, sErr := a.Search(ctx, title)
	if sErr == nil && hitMatchesHost(hit, hostHints) {
		return hit, nil
	}
	// Don't pipe-split-and-retry. We tried that and it returns wrong-episode-
	// of-right-show for show-name-only titles like "Vanguard | Acquired"
	// (search for "Acquired" returns whatever the latest Acquired episode is,
	// not the requested Vanguard episode). Better to fail clean than cache
	// wrong content as if it were right.
	return nil, err
}

// hostHintsFromURL extracts publisher-name hint words from a URL host. Used
// to reject spoken.md hits whose `podcast` field has nothing to do with the
// host the user pointed us at. Examples:
//   acquired.fm                 → ["acquired"]
//   www.lexfridman.com          → ["lex", "fridman"]
//   tim.blog                    → ["tim"]
//   www.hubermanlab.com         → ["huberman", "lab"]
//   open.spotify.com            → []   (skip — Spotify is a hosting platform, not the publisher)
//   podcasts.apple.com          → []   (same)
func hostHintsFromURL(u string) []string {
	if u == "" {
		return nil
	}
	parsed, err := url.Parse(u)
	if err != nil || parsed.Host == "" {
		return nil
	}
	host := strings.ToLower(parsed.Host)
	// Skip hosting platforms — their host doesn't identify the publisher.
	for _, platform := range []string{"open.spotify.com", "play.spotify.com", "podcasts.apple.com", "spoken.md", "youtube.com", "youtu.be"} {
		if strings.HasSuffix(host, platform) {
			return nil
		}
	}
	// Drop common subdomains and TLDs to extract meaningful publisher parts.
	host = strings.TrimPrefix(host, "www.")
	parts := strings.Split(host, ".")
	if len(parts) == 0 {
		return nil
	}
	// Take the leftmost label (the most-distinctive part of the host).
	primary := parts[0]
	// Split camel-cased or compound names where each component is at least 3 chars.
	hints := splitCompoundHost(primary)
	return hints
}

// splitCompoundHost splits "hubermanlab" → ["huberman", "lab"], etc., when the
// host ends in a known publisher-suffix word. Otherwise returns the full host
// as a single hint. hitMatchesHost also does space-normalized substring
// matching, which handles names like "lexfridman" / "Lex Fridman" without
// per-name splitting.
var hostSuffixWords = []string{"lab", "podcast", "show", "media", "network"}

func splitCompoundHost(host string) []string {
	for _, suffix := range hostSuffixWords {
		if strings.HasSuffix(host, suffix) && len(host) > len(suffix)+2 {
			prefix := host[:len(host)-len(suffix)]
			return []string{prefix, suffix, host} // include the un-split form too
		}
	}
	return []string{host}
}

// hostNormalizeRE strips everything that isn't a letter/digit so "Lex Fridman
// Podcast" → "lexfridmanpodcast" matches the hint "lexfridman".
var hostNormalizeRE = regexp.MustCompile(`[^a-z0-9]+`)

func normalizeForHostMatch(s string) string {
	return hostNormalizeRE.ReplaceAllString(strings.ToLower(s), "")
}

// hitMatchesHost is true when no hints are present (skip validation) OR when
// the hit's podcast/url field contains at least one hint word as a substring
// after normalization (strip spaces, dashes, lowercase).
func hitMatchesHost(hit *SearchHit, hints []string) bool {
	if len(hints) == 0 {
		return true
	}
	if hit == nil {
		return false
	}
	hay := normalizeForHostMatch(hit.Podcast + " " + hit.URL)
	for _, h := range hints {
		if h == "" {
			continue
		}
		needle := normalizeForHostMatch(h)
		if needle == "" {
			continue
		}
		if strings.Contains(hay, needle) {
			return true
		}
	}
	return false
}

// asNotApplicable is a small `errors.As` wrapper kept local to avoid pulling
// the errors package import just for one call site.
func asNotApplicable(err error, target **source.NotApplicableError) bool {
	if err == nil {
		return false
	}
	cur := err
	for cur != nil {
		if x, ok := cur.(*source.NotApplicableError); ok {
			*target = x
			return true
		}
		uw, ok := cur.(interface{ Unwrap() error })
		if !ok {
			break
		}
		cur = uw.Unwrap()
	}
	return false
}

var spokenIndexedHostRE = regexp.MustCompile(`(?i)://(www\.)?(open\.spotify\.com|play\.spotify\.com|podcasts\.apple\.com|spoken\.md)/`)

func isSpokenIndexedHost(u string) bool {
	return spokenIndexedHostRE.MatchString(u)
}

// Fetch loads a transcript by URL. If the URL is not spoken.md itself, it
// performs a search() resolution first.
func (a *Adapter) Fetch(ctx context.Context, episodeURL string) (*transcript.Transcript, error) {
	k := a.key()
	if k == "" {
		return nil, &source.KeyMissingError{EnvVar: "SPOKEN_API_KEY", URL: "https://spoken.md/"}
	}

	id := ""
	var hit *SearchHit
	if strings.HasPrefix(episodeURL, baseURL+"/transcripts/") {
		id = strings.TrimPrefix(episodeURL, baseURL+"/transcripts/")
		id = strings.TrimSuffix(id, "/")
	} else {
		resolved, err := a.resolveByURL(ctx, episodeURL)
		if err != nil {
			return nil, err
		}
		hit = resolved
		id = resolved.ID
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

	// Prefer the search-hit's authoritative title + podcast over what we can
	// guess from the markdown body. Hit may be nil when the input was already
	// a spoken.md/transcripts/<id> URL (no search needed); in that case keep
	// the markdown-derived title and infer show from it.
	resolvedTitle := title
	show := guessShowFromTitle(title)
	if hit != nil {
		if hit.Title != "" {
			resolvedTitle = hit.Title
		}
		if hit.Podcast != "" {
			show = slugifyShow(hit.Podcast)
		}
	}

	return &transcript.Transcript{
		ID:                transcript.IDFor(episodeURL),
		Source:            adapterName,
		Show:              show,
		Tier:              transcript.TierPaid,
		URL:               episodeURL,
		Title:             resolvedTitle,
		Provider:          adapterName,
		CostCredits:       credits,
		Segments:          segs,
		SectionTimestamps: sections,
		FetchedAt:         time.Now().UTC(),
	}, nil
}

// slugifyShow converts "The Tim Ferriss Show" -> "the-tim-ferriss-show" for
// the store's `show` column, matching how other adapters key shows.
func slugifyShow(s string) string {
	out := strings.ToLower(strings.TrimSpace(s))
	out = strings.ReplaceAll(out, " ", "-")
	out = strings.ReplaceAll(out, "'", "")
	out = strings.ReplaceAll(out, "—", "-")
	return out
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
