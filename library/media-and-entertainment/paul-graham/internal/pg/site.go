package pg

import (
	"context"
	"fmt"
	"html"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/paul-graham/internal/cliutil"
	xhtml "golang.org/x/net/html"
)

const (
	BaseURL       = "https://www.paulgraham.com"
	ArticlesURL   = BaseURL + "/articles.html"
	maxIndexBytes = 2 << 20
	maxEssayBytes = 4 << 20

	fullTextSearchConcurrency = 4
)

type Index struct {
	SourceURL string    `json:"source_url"`
	FetchedAt time.Time `json:"fetched_at"`
	Essays    []Essay   `json:"essays"`
}

type Essay struct {
	Title string `json:"title"`
	Slug  string `json:"slug"`
	URL   string `json:"url"`
	Order int    `json:"order"`
}

type EssayText struct {
	Essay
	FetchedAt  time.Time `json:"fetched_at"`
	WordCount  int       `json:"word_count"`
	Text       string    `json:"text"`
	Excerpt    string    `json:"excerpt,omitempty"`
	SourceHTML string    `json:"source_html,omitempty"`
}

type Link struct {
	Text string `json:"text,omitempty"`
	URL  string `json:"url"`
}

func FetchIndex(ctx context.Context, timeout time.Duration) (Index, error) {
	body, err := fetch(ctx, ArticlesURL, timeout, maxIndexBytes)
	if err != nil {
		return Index{}, err
	}
	doc, err := xhtml.Parse(strings.NewReader(string(body)))
	if err != nil {
		return Index{}, fmt.Errorf("parsing essay index: %w", err)
	}
	essays := parseIndex(doc)
	if len(essays) == 0 {
		return Index{}, fmt.Errorf("no essays found in %s; page layout may have changed", ArticlesURL)
	}
	return Index{SourceURL: ArticlesURL, FetchedAt: time.Now().UTC(), Essays: essays}, nil
}

func Filter(essays []Essay, query string, limit int) []Essay {
	query = strings.ToLower(strings.TrimSpace(query))
	var out []Essay
	for _, essay := range essays {
		if query != "" {
			haystack := strings.ToLower(essay.Title + " " + essay.Slug)
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		out = append(out, essay)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func Find(essays []Essay, needle string) (Essay, bool) {
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return Essay{}, false
	}
	lower := strings.ToLower(needle)
	for _, essay := range essays {
		switch {
		case strings.EqualFold(essay.Slug, strings.TrimSuffix(needle, ".html")):
			return essay, true
		case strings.EqualFold(essay.Title, needle):
			return essay, true
		case strings.EqualFold(essay.URL, needle):
			return essay, true
		case strings.Contains(strings.ToLower(essay.Title), lower):
			return essay, true
		}
	}
	return Essay{}, false
}

func Random(essays []Essay, seed int64) (Essay, bool) {
	if len(essays) == 0 {
		return Essay{}, false
	}
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	r := rand.New(rand.NewSource(seed))
	return essays[r.Intn(len(essays))], true
}

func Read(ctx context.Context, essay Essay, timeout time.Duration) (EssayText, error) {
	body, err := fetch(ctx, essay.URL, timeout, maxEssayBytes)
	if err != nil {
		return EssayText{}, err
	}
	doc, err := xhtml.Parse(strings.NewReader(string(body)))
	if err != nil {
		return EssayText{}, fmt.Errorf("parsing essay page: %w", err)
	}
	text := extractMainText(doc)
	if text == "" {
		return EssayText{}, fmt.Errorf("no essay body found in %s; page layout may have changed", essay.URL)
	}
	title := essay.Title
	if title == "" {
		title = pageTitle(doc)
	}
	return EssayText{
		Essay: Essay{
			Title: title,
			Slug:  essay.Slug,
			URL:   essay.URL,
			Order: essay.Order,
		},
		FetchedAt: time.Now().UTC(),
		WordCount: len(strings.Fields(text)),
		Text:      text,
		Excerpt:   excerpt(text, 320),
	}, nil
}

func Links(ctx context.Context, essay Essay, timeout time.Duration) ([]Link, error) {
	body, err := fetch(ctx, essay.URL, timeout, maxEssayBytes)
	if err != nil {
		return nil, err
	}
	doc, err := xhtml.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("parsing essay page: %w", err)
	}
	base, _ := url.Parse(essay.URL)
	var links []Link
	seen := map[string]bool{}
	walk(doc, func(n *xhtml.Node) {
		if n.Type != xhtml.ElementNode || n.Data != "a" {
			return
		}
		href := attr(n, "href")
		if href == "" {
			return
		}
		u, err := base.Parse(href)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return
		}
		resolved := u.String()
		if seen[resolved] {
			return
		}
		seen[resolved] = true
		links = append(links, Link{Text: cleanText(textContent(n)), URL: resolved})
	})
	return links, nil
}

func SearchFullText(ctx context.Context, essays []Essay, query string, timeout time.Duration, limit int) ([]EssayText, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil, nil
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var out []EssayText
	for start := 0; start < len(essays); start += fullTextSearchConcurrency {
		end := start + fullTextSearchConcurrency
		if end > len(essays) {
			end = len(essays)
		}
		batch := make([]indexedEssay, 0, end-start)
		for i := start; i < end; i++ {
			batch = append(batch, indexedEssay{Index: i, Essay: essays[i]})
		}

		results, errs := cliutil.FanoutRun(ctx, batch, func(item indexedEssay) string {
			return item.Essay.Slug
		}, func(ctx context.Context, item indexedEssay) (fullTextMatch, error) {
			txt, err := Read(ctx, item.Essay, timeout)
			if err != nil {
				return fullTextMatch{}, err
			}
			titleMatch := strings.Contains(strings.ToLower(item.Essay.Title+" "+item.Essay.Slug), query)
			bodyMatch := strings.Contains(strings.ToLower(txt.Text), query)
			return fullTextMatch{Index: item.Index, Text: txt, Match: titleMatch || bodyMatch}, nil
		}, cliutil.WithConcurrency(fullTextSearchConcurrency))
		if len(errs) > 0 {
			cliutil.FanoutReportErrors(os.Stderr, errs)
		}

		sort.SliceStable(results, func(i, j int) bool {
			return results[i].Value.Index < results[j].Value.Index
		})
		for _, result := range results {
			if !result.Value.Match {
				continue
			}
			out = append(out, result.Value.Text)
			if limit > 0 && len(out) >= limit {
				cancel()
				return out, nil
			}
		}
	}
	return out, nil
}

type indexedEssay struct {
	Index int
	Essay Essay
}

type fullTextMatch struct {
	Index int
	Text  EssayText
	Match bool
}

func parseIndex(doc *xhtml.Node) []Essay {
	base, _ := url.Parse(BaseURL + "/")
	var essays []Essay
	seen := map[string]bool{}
	walk(doc, func(n *xhtml.Node) {
		if n.Type != xhtml.ElementNode || n.Data != "a" || !isEssayListAnchor(n) {
			return
		}
		href := strings.TrimSpace(attr(n, "href"))
		if !strings.HasSuffix(strings.ToLower(href), ".html") || strings.Contains(href, "://") {
			return
		}
		u, err := base.Parse(href)
		if err != nil {
			return
		}
		slug := strings.TrimSuffix(path.Base(u.Path), ".html")
		if slug == "" || seen[slug] {
			return
		}
		seen[slug] = true
		essays = append(essays, Essay{
			Title: cleanText(textContent(n)),
			Slug:  slug,
			URL:   u.String(),
			Order: len(essays) + 1,
		})
	})
	return essays
}

func isEssayListAnchor(a *xhtml.Node) bool {
	for p := a.Parent; p != nil; p = p.Parent {
		if p.Type != xhtml.ElementNode || p.Data != "td" {
			continue
		}
		for c := p.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == xhtml.ElementNode && c.Data == "img" && strings.Contains(attr(c, "src"), "the-reddits") {
				return true
			}
		}
		return false
	}
	return false
}

func extractMainText(doc *xhtml.Node) string {
	var candidates []string
	walk(doc, func(n *xhtml.Node) {
		if n.Type != xhtml.ElementNode || n.Data != "td" {
			return
		}
		if attr(n, "width") != "435" {
			return
		}
		text := cleanText(textContent(n))
		if strings.Count(text, " ") >= 30 {
			candidates = append(candidates, text)
		}
	})
	sort.SliceStable(candidates, func(i, j int) bool {
		return len(candidates[i]) > len(candidates[j])
	})
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0]
}

func pageTitle(doc *xhtml.Node) string {
	var title string
	walk(doc, func(n *xhtml.Node) {
		if title != "" || n.Type != xhtml.ElementNode || n.Data != "title" {
			return
		}
		title = cleanText(textContent(n))
	})
	return title
}

func fetch(ctx context.Context, target string, timeout time.Duration, maxBytes int64) ([]byte, error) {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")
	req.Header.Set("User-Agent", "paul-graham-pp-cli/1.0 (+https://github.com/mvanhorn/printing-press-library)")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limited: %s returned HTTP 429", target)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s returned HTTP %d", target, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("%s response exceeds %d bytes", target, maxBytes)
	}
	return body, nil
}

func walk(n *xhtml.Node, fn func(*xhtml.Node)) {
	if n == nil {
		return
	}
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}

func attr(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return strings.TrimSpace(a.Val)
		}
	}
	return ""
}

func textContent(n *xhtml.Node) string {
	var b strings.Builder
	walk(n, func(cur *xhtml.Node) {
		if cur.Type == xhtml.TextNode {
			b.WriteString(cur.Data)
			b.WriteByte(' ')
		}
	})
	return html.UnescapeString(b.String())
}

func cleanText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func excerpt(value string, max int) string {
	value = cleanText(value)
	runes := []rune(value)
	if max <= 0 || len(runes) <= max {
		return value
	}
	cut := string(runes[:max])
	if idx := strings.LastIndex(cut, " "); idx > 80 {
		cut = cut[:idx]
	}
	return cut + "..."
}
