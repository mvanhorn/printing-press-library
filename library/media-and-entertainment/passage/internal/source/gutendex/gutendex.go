// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Gutendex is passage's public-domain full-text source (a JSON API over
// Project Gutenberg). Open Library gives metadata; Gutendex gives the actual
// text you sit with. Keyless — but Cloudflare-fronted, so a real User-Agent is
// required. The plain-text file lives on gutenberg.org, a second fetch.
package gutendex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	baseURL   = "https://gutendex.com"
	userAgent = "passage/1.0 (+https://github.com/justinwfu/passage)"
)

type Author struct {
	Name  string `json:"name"`
	Birth *int   `json:"birth_year"`
	Death *int   `json:"death_year"`
}

type Book struct {
	ID        int               `json:"id"`
	Title     string            `json:"title"`
	Authors   []Author          `json:"authors"`
	Subjects  []string          `json:"subjects"`
	Languages []string          `json:"languages"`
	Downloads int               `json:"download_count"`
	Formats   map[string]string `json:"formats"`
}

// AuthorLine joins author names into a display string.
func (b Book) AuthorLine() string {
	names := make([]string, 0, len(b.Authors))
	for _, a := range b.Authors {
		names = append(names, a.Name)
	}
	if len(names) == 0 {
		return "Unknown"
	}
	return strings.Join(names, ", ")
}

// TextURL returns the best plain-text download URL, or "" if none.
func (b Book) TextURL() string {
	prefer := []string{"text/plain; charset=utf-8", "text/plain; charset=us-ascii"}
	for _, k := range prefer {
		if u, ok := b.Formats[k]; ok && !strings.HasSuffix(u, ".zip") {
			return u
		}
	}
	// Any other text/plain variant.
	for k, u := range b.Formats {
		if strings.HasPrefix(k, "text/plain") && !strings.HasSuffix(u, ".zip") {
			return u
		}
	}
	return ""
}

type Client struct{ hc *http.Client }

func New() *Client { return &Client{hc: &http.Client{Timeout: 30 * time.Second}} }

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("gutendex rate-limited (429) — slow down and retry")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gutendex %s -> HTTP %d", url, resp.StatusCode)
	}
	// Cap the read so a huge Gutenberg text can't blow memory.
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}

type listResp struct {
	Count   int    `json:"count"`
	Results []Book `json:"results"`
}

// Search returns Gutenberg books matching query. When textOnly, only books that
// have a plain-text format are returned.
func (c *Client) Search(ctx context.Context, query string, textOnly bool) ([]Book, error) {
	url := baseURL + "/books?search=" + urlQuery(query) + "&languages=en"
	if textOnly {
		url += "&mime_type=" + urlQuery("text/plain")
	}
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var lr listResp
	if err := json.Unmarshal(body, &lr); err != nil {
		return nil, fmt.Errorf("parsing gutendex search: %w", err)
	}
	// Most-downloaded first — a sane relevance proxy.
	sort.SliceStable(lr.Results, func(i, j int) bool { return lr.Results[i].Downloads > lr.Results[j].Downloads })
	return lr.Results, nil
}

// Get fetches a single book by its Gutenberg id.
func (c *Client) Get(ctx context.Context, id int) (Book, error) {
	body, err := c.get(ctx, fmt.Sprintf("%s/books/%d", baseURL, id))
	if err != nil {
		return Book{}, err
	}
	var b Book
	if err := json.Unmarshal(body, &b); err != nil {
		return Book{}, fmt.Errorf("parsing gutendex book %d: %w", id, err)
	}
	if b.ID == 0 {
		return Book{}, fmt.Errorf("no Gutenberg book with id %d", id)
	}
	return b, nil
}

// Excerpt fetches a book's plain text and returns a cleaned passage of about
// maxRunes runes, skipping the Project Gutenberg license header.
func (c *Client) Excerpt(ctx context.Context, b Book, maxRunes int) (string, error) {
	u := b.TextURL()
	if u == "" {
		return "", fmt.Errorf("%q has no plain-text edition on Project Gutenberg", b.Title)
	}
	body, err := c.get(ctx, u)
	if err != nil {
		return "", err
	}
	return CleanExcerpt(string(body), maxRunes), nil
}

var frontMatterRe = regexp.MustCompile(`(?i)illustration|^contents$|publisher|copyright|transcriber|produced by|gutenberg|^\s*chapter\b|^\s*book\b|charing cross|all rights reserved|etext|^\s*\*`)

// workStartRe matches the heading where the actual work begins ("CHAPTER I",
// "BOOK ONE", "THE FIRST BOOK", "PART I") so the excerpt skips a translator's
// introduction or preface rather than opening on a biography.
var workStartRe = []*regexp.Regexp{
	regexp.MustCompile(`(?im)^[ \t]*(chapter|book|part|canto|letter|act|scene|adventure|story|tale)[ \t]+(the\s+first|first|one|i|1|1st)\b`),
	regexp.MustCompile(`(?im)^[ \t]*(the\s+)?(first|1st|i)[ \t]+(book|part|canto)\b`),
}

// findWorkStart returns the byte offset of the first chapter/book heading that
// is followed by real prose (so a table-of-contents entry doesn't count), or -1.
func findWorkStart(text string) int {
	best := -1
	for _, re := range workStartRe {
		for _, loc := range re.FindAllStringIndex(text, -1) {
			lineEnd := len(text)
			if nl := strings.IndexByte(text[loc[0]:], '\n'); nl >= 0 {
				lineEnd = loc[0] + nl
			}
			// The match must be a standalone heading line ("THE FIRST BOOK"),
			// not a sentence that merely begins with those words ("First Book he
			// sets down to account all the debts…" in a translator's intro).
			if len(strings.TrimSpace(text[loc[0]:lineEnd])) > 40 {
				continue
			}
			// The real heading is immediately followed by prose; a table-of-
			// contents entry ("FIRST BOOK") is followed by another heading
			// ("SECOND BOOK"). Only inspect the first couple of paragraphs.
			if firstParasHaveProse(text[lineEnd:], 2) && (best == -1 || loc[0] < best) {
				best = loc[0]
			}
		}
	}
	return best
}

// firstParasHaveProse reports whether any of the first n non-empty paragraphs of
// s read as body text — so a heading immediately followed by real prose (or a
// short chapter title then prose) qualifies, but a table-of-contents run of
// headings does not.
func firstParasHaveProse(s string, n int) bool {
	seen := 0
	for _, para := range strings.Split(s, "\n\n") {
		p := strings.TrimSpace(para)
		if p == "" {
			continue
		}
		if isProse(p) {
			return true
		}
		if seen++; seen >= n {
			break
		}
	}
	return false
}

// CleanExcerpt strips the Gutenberg header + front matter and returns the first
// runs of real prose, up to maxRunes runes. Exported for testing.
func CleanExcerpt(raw string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = 1000
	}
	// Skip the Project Gutenberg license header.
	if i := strings.Index(raw, "*** START OF"); i >= 0 {
		if nl := strings.IndexByte(raw[i:], '\n'); nl >= 0 {
			raw = raw[i+nl+1:]
		}
	}
	if i := strings.Index(raw, "*** END OF"); i >= 0 {
		raw = raw[:i]
	}
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	// Skip a translator's introduction/preface to where the work actually begins.
	if i := findWorkStart(raw); i >= 0 {
		raw = raw[i:]
	}

	// Walk paragraphs; keep the first that read as real prose (sentence-like,
	// long enough, not a TOC/illustration/publisher block).
	paras := strings.Split(raw, "\n\n")
	var b strings.Builder
	for _, p := range paras {
		p = strings.TrimSpace(p)
		if !isProse(p) {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(strings.Join(strings.Fields(p), " "))
		if len([]rune(b.String())) >= maxRunes {
			break
		}
	}
	out := b.String()
	if out == "" {
		out = fallbackExcerpt(raw)
	}
	r := []rune(out)
	if len(r) <= maxRunes {
		return strings.TrimRight(out, " \n\t") // complete: no truncation ellipsis
	}
	return strings.TrimRight(string(r[:maxRunes]), " \n\t") + "…"
}

// fallbackExcerpt preserves verse and other short-line works without returning
// the unfiltered Gutenberg front matter when no paragraph passes isProse.
func fallbackExcerpt(raw string) string {
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if len([]rune(line)) < 20 || frontMatterRe.MatchString(line) || mostlyUpper(line) {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func mostlyUpper(s string) bool {
	letters, upper := 0, 0
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			upper++
			letters++
		} else if r >= 'a' && r <= 'z' {
			letters++
		}
	}
	return letters > 0 && float64(upper)/float64(letters) > 0.5
}

// isProse reports whether a paragraph reads as real body text rather than
// front matter (title page, table of contents, illustration caption).
func isProse(p string) bool {
	if len([]rune(p)) < 50 {
		return false
	}
	// Only treat a SHORT paragraph as front matter — a long body paragraph that
	// merely mentions "publisher"/"gutenberg" (e.g. "the publisher rejected…")
	// is still real prose and must not be dropped.
	if len([]rune(p)) < 300 && frontMatterRe.MatchString(p) {
		return false
	}
	// Mostly-uppercase blocks are headings/title pages.
	if mostlyUpper(p) {
		return false
	}
	// A run with several roman/number list markers is a table of contents, not
	// prose ("I. A Scandal in Bohemia II. The Red-Headed League III. …").
	if len(tocMarkerRe.FindAllString(p, 4)) >= 3 {
		return false
	}
	// Real prose contains sentence punctuation.
	return strings.ContainsAny(p, ".?!")
}

// tocMarkerRe matches an enumerator like "II. " or "3. " (roman or arabic).
var tocMarkerRe = regexp.MustCompile(`(?:^|\s)([IVXLC]{1,5}|\d{1,3})\.\s`)

func urlQuery(s string) string {
	return url.QueryEscape(strings.TrimSpace(s))
}
