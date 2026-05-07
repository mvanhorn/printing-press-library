// Package nyt36hours scrapes NYT "36 Hours in <city>" travel articles. NYT
// pages are partly paywalled; this client extracts whatever is accessible
// (title, subtitle, byline, date, body text, and bolded place mentions) and
// returns it without trying to bypass the paywall.
package nyt36hours

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	defaultUserAgent = "wanderlust-goat-pp-cli/0.1 (+https://github.com/mvanhorn/printing-press-library)"
	defaultTimeout   = 10 * time.Second
)

// Article is a flattened "36 Hours" article view.
type Article struct {
	Title    string    `json:"title"`
	Subtitle string    `json:"subtitle,omitempty"`
	Author   string    `json:"author,omitempty"`
	Date     string    `json:"date,omitempty"`
	Mentions []Mention `json:"mentions,omitempty"`
	Body     string    `json:"body,omitempty"`
	URL      string    `json:"url"`
}

// Mention is a bolded place name + the description sentence(s) that follow it.
type Mention struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Client is a no-auth NYT scraper.
type Client struct {
	HTTPClient *http.Client
	UserAgent  string
}

// New constructs a Client. nil http client gets a 10-second default; empty UA
// gets the project default.
func New(httpClient *http.Client, ua string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	if ua == "" {
		ua = defaultUserAgent
	}
	return &Client{HTTPClient: httpClient, UserAgent: ua}
}

// Article fetches and parses an NYT "36 Hours" article.
func (c *Client) Article(ctx context.Context, fullURL string) (*Article, error) {
	if strings.TrimSpace(fullURL) == "" {
		return nil, fmt.Errorf("nyt36hours: empty url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nyt36hours request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("nyt36hours read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %d", fullURL, resp.StatusCode)
	}

	a := parseArticle(string(body))
	a.URL = fullURL
	return a, nil
}

var (
	jsonLDRe = regexp.MustCompile(`(?is)<script[^>]+type=["']application/ld\+json["'][^>]*>([\s\S]+?)</script>`)

	titleTagRe   = regexp.MustCompile(`(?is)<title>([\s\S]+?)</title>`)
	h1Re         = regexp.MustCompile(`(?is)<h1[^>]*>([\s\S]+?)</h1>`)
	h2Re         = regexp.MustCompile(`(?is)<h2[^>]*>([\s\S]+?)</h2>`)
	bylineRe     = regexp.MustCompile(`(?is)<(?:p|span|address|a)[^>]+(?:class|rel)=["'][^"']*(?:byline|author)[^"']*["'][^>]*>([\s\S]+?)</(?:p|span|address|a)>`)
	timeRe       = regexp.MustCompile(`(?is)<time[^>]*(?:datetime=["']([^"']+)["'])?[^>]*>([\s\S]+?)</time>`)
	paragraphRe  = regexp.MustCompile(`(?is)<p[^>]*>([\s\S]+?)</p>`)
	strongRe     = regexp.MustCompile(`(?is)<strong[^>]*>([\s\S]+?)</strong>([\s\S]*?)(?:<strong[^>]*>|</p>|$)`)
	tagStripRe   = regexp.MustCompile(`<[^>]+>`)
	whitespaceRe = regexp.MustCompile(`\s+`)
)

// jsonLDArticle is a permissive view of a NewsArticle JSON-LD block.
type jsonLDArticle struct {
	Type        any    `json:"@type"`
	Headline    string `json:"headline"`
	Description string `json:"description"`
	DatePublished string `json:"datePublished"`
	Author      json.RawMessage `json:"author"`
	URL         string `json:"url"`
}

func parseArticle(html string) *Article {
	a := &Article{}

	// JSON-LD path: NYT articles publish a NewsArticle block with title, byline
	// and date. We use it for canonical fields, then fall back to HTML for the
	// body and bolded mentions.
	for _, m := range jsonLDRe.FindAllStringSubmatch(html, -1) {
		raw := strings.TrimSpace(m[1])
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			continue
		}
		for _, b := range flattenJSONLD(v) {
			if !typeMatches(b["@type"], "NewsArticle", "Article", "ReportageNewsArticle") {
				continue
			}
			raw2, _ := json.Marshal(b)
			var ja jsonLDArticle
			if err := json.Unmarshal(raw2, &ja); err != nil {
				continue
			}
			if a.Title == "" {
				a.Title = ja.Headline
			}
			if a.Subtitle == "" {
				a.Subtitle = stripAndCollapse(ja.Description)
			}
			if a.Date == "" {
				a.Date = ja.DatePublished
			}
			if a.Author == "" {
				a.Author = decodeAuthor(ja.Author)
			}
		}
	}

	// HTML fallbacks for any field still empty.
	if a.Title == "" {
		if m := h1Re.FindStringSubmatch(html); len(m) == 2 {
			a.Title = stripAndCollapse(m[1])
		} else if m := titleTagRe.FindStringSubmatch(html); len(m) == 2 {
			a.Title = stripAndCollapse(m[1])
		}
	}
	if a.Subtitle == "" {
		if m := h2Re.FindStringSubmatch(html); len(m) == 2 {
			a.Subtitle = stripAndCollapse(m[1])
		}
	}
	if a.Author == "" {
		if m := bylineRe.FindStringSubmatch(html); len(m) == 2 {
			a.Author = stripAndCollapse(m[1])
			a.Author = strings.TrimPrefix(a.Author, "By ")
			a.Author = strings.TrimPrefix(a.Author, "by ")
		}
	}
	if a.Date == "" {
		if m := timeRe.FindStringSubmatch(html); len(m) == 3 {
			if m[1] != "" {
				a.Date = m[1]
			} else {
				a.Date = stripAndCollapse(m[2])
			}
		}
	}

	a.Mentions = extractMentions(html)
	a.Body = extractBody(html)
	return a
}

// extractMentions walks every <strong>Name</strong> followed by description
// text up to the next <strong> or paragraph break. NYT 36 Hours pieces use
// bolded place names as the section anchors.
func extractMentions(html string) []Mention {
	out := []Mention{}
	seen := map[string]bool{}
	for _, m := range strongRe.FindAllStringSubmatch(html, -1) {
		name := stripAndCollapse(m[1])
		if name == "" || len(name) > 120 {
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		desc := stripAndCollapse(m[2])
		// Trim a leading separator (em-dash / colon / period) the writer used to
		// glue the label to the description.
		desc = strings.TrimLeft(desc, " —–-:.,")
		desc = strings.TrimSpace(desc)
		out = append(out, Mention{Name: name, Description: desc})
	}
	return out
}

// extractBody concatenates every <p> in the document, separated by blank lines.
// We deliberately keep this loose; NYT's actual article body lives inside a
// <section name="articleBody"> in newer templates and a div.story-body in older
// ones, so a global <p> sweep is the most stable thing across both.
func extractBody(html string) string {
	var sb strings.Builder
	for _, m := range paragraphRe.FindAllStringSubmatch(html, -1) {
		text := stripAndCollapse(m[1])
		if text == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(text)
	}
	return sb.String()
}

func decodeAuthor(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try a single object: {"@type":"Person","name":"..."}.
	var single struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &single); err == nil && single.Name != "" {
		return single.Name
	}
	// Try an array of objects.
	var multi []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &multi); err == nil {
		names := []string{}
		for _, p := range multi {
			if p.Name != "" {
				names = append(names, p.Name)
			}
		}
		if len(names) > 0 {
			return strings.Join(names, ", ")
		}
	}
	// Try a bare string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

func flattenJSONLD(v any) []map[string]any {
	switch tv := v.(type) {
	case map[string]any:
		if g, ok := tv["@graph"].([]any); ok {
			out := make([]map[string]any, 0, len(g))
			for _, sub := range g {
				out = append(out, flattenJSONLD(sub)...)
			}
			return out
		}
		return []map[string]any{tv}
	case []any:
		var out []map[string]any
		for _, sub := range tv {
			out = append(out, flattenJSONLD(sub)...)
		}
		return out
	}
	return nil
}

func typeMatches(got any, wanted ...string) bool {
	is := func(s string) bool {
		for _, w := range wanted {
			if strings.EqualFold(s, w) {
				return true
			}
		}
		return false
	}
	switch tv := got.(type) {
	case string:
		return is(tv)
	case []any:
		for _, x := range tv {
			if s, ok := x.(string); ok && is(s) {
				return true
			}
		}
	}
	return false
}

func stripAndCollapse(s string) string {
	s = tagStripRe.ReplaceAllString(s, " ")
	s = whitespaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
