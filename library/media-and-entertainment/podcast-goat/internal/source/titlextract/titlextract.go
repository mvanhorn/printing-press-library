// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// Generic OpenGraph + <title> scraper used by paid adapters (spoken.md, Taddy)
// to convert a publisher-page URL into an episode-distinctive search query.
//
// The shape spoken.md's `/search?q=...` endpoint expects is the episode TITLE
// text, not the URL. Publisher pages (tim.blog, lexfridman.com, acquired.fm,
// hubermanlab.com) reliably advertise the title in three places we try in
// priority order:
//   1. <meta property="og:title" content="...">     (drives Apple/Spotify previews)
//   2. <meta name="twitter:title" content="...">    (drives X previews)
//   3. <title>...</title>                           (fallback)
//
// All three commonly carry a publisher suffix (" - Tim Ferriss Blog",
// " | The Acquired Podcast") that hurts search ranking. We strip the suffix
// when one of a small known set of separators appears AND the split is
// unambiguous (longer side wins; ties go to the first side which is
// conventionally the episode-distinctive part).

package titlextract

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Extract fetches the URL and returns the best-effort episode title with any
// trailing publisher suffix stripped. Returns an error when the page is
// unreachable, the response is too large, or no title source is present.
func Extract(ctx context.Context, pageURL string) (string, error) {
	return defaultExtractor.Extract(ctx, pageURL)
}

// Extractor exposes Extract for tests that need to substitute the HTTP client.
type Extractor struct {
	Client *http.Client
	// MaxBytes caps the page read so a runaway HTML page can't stall the
	// dispatcher. 512KB covers every podcast page we care about.
	MaxBytes int64
}

func NewExtractor() *Extractor {
	return &Extractor{
		Client:   &http.Client{Timeout: 10 * time.Second},
		MaxBytes: 512 << 10,
	}
}

var defaultExtractor = NewExtractor()

func (e *Extractor) Extract(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return "", fmt.Errorf("titlextract build request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	resp, err := e.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("titlextract fetch %s: %w", pageURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("titlextract %s: HTTP %d", pageURL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, e.MaxBytes))
	if err != nil {
		return "", fmt.Errorf("titlextract read %s: %w", pageURL, err)
	}
	return ExtractFromHTML(string(body))
}

// ExtractFromHTML runs the title-scraping logic against a literal HTML body.
// Exposed for the test suite — no network access.
func ExtractFromHTML(htmlBody string) (string, error) {
	if t := extractOG(htmlBody); t != "" {
		return Sanitize(t), nil
	}
	if t := matchFirst(twitterTitleRE, htmlBody); t != "" {
		return Sanitize(t), nil
	}
	if t := matchFirst(titleTagRE, htmlBody); t != "" {
		return Sanitize(t), nil
	}
	return "", fmt.Errorf("titlextract: no og:title, twitter:title, or <title> found")
}

// Sanitize cleans extracted title text and strips publisher-name suffixes.
//
// Two phases:
//  1. HTML-entity decode + whitespace normalize.
//  2. Up to 3 publisher-strip passes. Each pass picks one separator and drops
//     the publisher side. Passes stop when no publisher-word side is found,
//     so structural in-title delimiters (e.g., "Guest Name — Episode") are
//     preserved — we only strip what looks like a publisher tail.
//
// Pass rules (in order):
//   a. If exactly one side contains a "publisher word" (podcast, show, blog,
//      network, studio, channel), drop that side.
//   b. If neither side has a publisher word but one side is very short (<4
//      chars) and the other is real, drop the short side (catches "EP - real
//      title" style prefixes).
//   c. Otherwise stop — splitting further would chop legitimate episode
//      structure.
//
// Exposed for tests.
func Sanitize(raw string) string {
	out := strings.TrimSpace(html.UnescapeString(raw))
	out = whitespaceRE.ReplaceAllString(out, " ")

	for pass := 0; pass < 3; pass++ {
		next, changed := stripPublisherOnce(out)
		if !changed {
			return out
		}
		out = next
	}
	return out
}

// stripPublisherOnce splits on the first separator that yields a clear
// publisher side and drops that side. Returns (new, true) on a successful
// strip; (s, false) when none of the separators produces a clear publisher
// drop.
func stripPublisherOnce(s string) (string, bool) {
	for _, sep := range []string{" | ", " — ", " – ", " - "} {
		if !strings.Contains(s, sep) {
			continue
		}
		parts := strings.SplitN(s, sep, 2)
		if len(parts) != 2 {
			continue
		}
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])

		leftPub := hasPublisherWord(left)
		rightPub := hasPublisherWord(right)

		// Rule (a): exactly one side has a publisher word -> drop it.
		if leftPub && !rightPub {
			return right, true
		}
		if rightPub && !leftPub {
			return left, true
		}

		// Rule (b): one side is a 1-3 char prefix/suffix marker. Drop it.
		if len(left) < 4 && len(right) >= 8 {
			return right, true
		}
		if len(right) < 4 && len(left) >= 8 {
			return left, true
		}

		// Otherwise this separator doesn't look like a publisher split.
		// Try the next separator before giving up.
		continue
	}
	return s, false
}

var publisherWordRE = regexp.MustCompile(`(?i)\b(podcast|show|blog|network|studio|channel)\b`)

func hasPublisherWord(s string) bool {
	return publisherWordRE.MatchString(s)
}

func matchFirst(re *regexp.Regexp, body string) string {
	m := re.FindStringSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

var (
	// ogTitleRE matches <meta property="og:title" content="...">. The
	// `content` and `property` attributes can appear in either order, so we
	// match both shapes via alternation. Quotes can be either flavor.
	ogTitleRE = regexp.MustCompile(`(?is)<meta[^>]+(?:property|name)\s*=\s*["']og:title["'][^>]*content\s*=\s*["']([^"']+)["']`)
	// Spec-compliant alternative form: content first, property after.
	ogTitleAltRE = regexp.MustCompile(`(?is)<meta[^>]+content\s*=\s*["']([^"']+)["'][^>]*(?:property|name)\s*=\s*["']og:title["']`)
	// twitter:title matches Twitter card meta.
	twitterTitleRE = regexp.MustCompile(`(?is)<meta[^>]+name\s*=\s*["']twitter:title["'][^>]*content\s*=\s*["']([^"']+)["']`)
	titleTagRE     = regexp.MustCompile(`(?is)<title[^>]*>([^<]+)</title>`)
	whitespaceRE   = regexp.MustCompile(`\s+`)
)

// Override matchFirst for og:title to try both attribute orders.
func extractOG(body string) string {
	if v := matchFirst(ogTitleRE, body); v != "" {
		return v
	}
	return matchFirst(ogTitleAltRE, body)
}
