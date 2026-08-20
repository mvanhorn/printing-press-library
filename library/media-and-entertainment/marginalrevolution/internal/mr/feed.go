package mr

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/marginalrevolution/internal/cliutil"
)

const (
	FeedURL      = "https://marginalrevolution.com/feed"
	maxFeedBytes = 10 << 20
)

type Feed struct {
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Description string    `json:"description"`
	LastBuild   time.Time `json:"last_build,omitempty"`
	Items       []Item    `json:"items"`
}

type Item struct {
	Title        string    `json:"title"`
	Link         string    `json:"link"`
	Author       string    `json:"author,omitempty"`
	Published    time.Time `json:"published,omitempty"`
	Categories   []string  `json:"categories,omitempty"`
	GUID         string    `json:"guid,omitempty"`
	CommentsURL  string    `json:"comments_url,omitempty"`
	CommentFeed  string    `json:"comment_feed,omitempty"`
	CommentCount int       `json:"comment_count,omitempty"`
	Summary      string    `json:"summary,omitempty"`
	ContentText  string    `json:"content_text,omitempty"`
	Links        []Link    `json:"links,omitempty"`
}

type Link struct {
	Text string `json:"text,omitempty"`
	URL  string `json:"url"`
}

type rawRSS struct {
	Channel rawChannel `xml:"channel"`
}

type rawChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	LastBuild   string    `xml:"lastBuildDate"`
	Items       []rawItem `xml:"item"`
}

type rawItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	Comments    []string `xml:"comments"`
	Creator     string   `xml:"http://purl.org/dc/elements/1.1/ creator"`
	PubDate     string   `xml:"pubDate"`
	Categories  []string `xml:"category"`
	GUID        string   `xml:"guid"`
	Description string   `xml:"description"`
	Content     string   `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	CommentFeed string   `xml:"http://wellformedweb.org/CommentAPI/ commentRss"`
}

func Fetch(ctx context.Context, timeout time.Duration) (Feed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, FeedURL, nil)
	if err != nil {
		return Feed{}, err
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml;q=0.9, */*;q=0.8")
	req.Header.Set("User-Agent", "marginalrevolution-pp-cli/1.0")

	client := &http.Client{Timeout: timeout}
	limiter := cliutil.NewAdaptiveLimiter(0)
	limiter.Wait()
	resp, err := client.Do(req)
	if err != nil {
		return Feed{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return Feed{}, fmt.Errorf("rate limited: feed returned HTTP 429")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Feed{}, fmt.Errorf("feed returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes+1))
	if err != nil {
		return Feed{}, err
	}
	if len(body) > maxFeedBytes {
		return Feed{}, fmt.Errorf("feed response exceeds %d bytes", maxFeedBytes)
	}

	var raw rawRSS
	if err := xml.Unmarshal(body, &raw); err != nil {
		return Feed{}, fmt.Errorf("parsing RSS: %w", err)
	}

	build, _ := parseRSSDate(raw.Channel.LastBuild)
	feed := Feed{
		Title:       strings.TrimSpace(raw.Channel.Title),
		Link:        strings.TrimSpace(raw.Channel.Link),
		Description: strings.TrimSpace(raw.Channel.Description),
		LastBuild:   build,
	}
	for _, item := range raw.Channel.Items {
		published, _ := parseRSSDate(item.PubDate)
		content := cleanHTML(item.Content)
		summary := cleanHTML(item.Description)
		commentsURL, commentCount := splitComments(item.Comments)
		feed.Items = append(feed.Items, Item{
			Title:        strings.TrimSpace(item.Title),
			Link:         stripTracking(strings.TrimSpace(item.Link)),
			Author:       strings.TrimSpace(item.Creator),
			Published:    published,
			Categories:   cleanStrings(item.Categories),
			GUID:         strings.TrimSpace(item.GUID),
			CommentsURL:  commentsURL,
			CommentFeed:  strings.TrimSpace(item.CommentFeed),
			CommentCount: commentCount,
			Summary:      summary,
			ContentText:  content,
			Links:        extractLinks(item.Content),
		})
	}
	return feed, nil
}

func splitComments(values []string) (string, int) {
	var commentsURL string
	var commentCount int
	for _, value := range values {
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
			commentsURL = value
			continue
		}
		if n, err := strconv.Atoi(value); err == nil {
			commentCount = n
		}
	}
	return commentsURL, commentCount
}

func Filter(items []Item, query, author, category string, limit int) []Item {
	query = strings.ToLower(strings.TrimSpace(query))
	author = strings.ToLower(strings.TrimSpace(author))
	category = strings.ToLower(strings.TrimSpace(category))
	var out []Item
	for _, item := range items {
		if query != "" {
			haystack := strings.ToLower(item.Title + "\n" + item.Summary + "\n" + item.ContentText + "\n" + strings.Join(item.Categories, " "))
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		if author != "" && !strings.Contains(strings.ToLower(item.Author), author) {
			continue
		}
		if category != "" && !hasCategory(item.Categories, category) {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func CategoryCounts(items []Item) map[string]int {
	counts := map[string]int{}
	for _, item := range items {
		for _, category := range item.Categories {
			counts[category]++
		}
	}
	return counts
}

func AuthorCounts(items []Item) map[string]int {
	counts := map[string]int{}
	for _, item := range items {
		if item.Author != "" {
			counts[item.Author]++
		}
	}
	return counts
}

func SortedCounts(counts map[string]int) []struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
} {
	out := make([]struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}, 0, len(counts))
	for name, count := range counts {
		out = append(out, struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}{Name: name, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Name < out[j].Name
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func Find(items []Item, needle string) (Item, bool) {
	needle = strings.TrimSpace(needle)
	for _, item := range items {
		if item.Link == needle || item.GUID == needle || strings.EqualFold(item.Title, needle) {
			return item, true
		}
		if strings.Contains(strings.ToLower(item.Link), strings.ToLower(needle)) {
			return item, true
		}
	}
	return Item{}, false
}

func parseRSSDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC1123Z, value); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC1123, value)
}

func hasCategory(categories []string, needle string) bool {
	for _, category := range categories {
		if strings.Contains(strings.ToLower(category), needle) {
			return true
		}
	}
	return false
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

var (
	linkRE  = regexp.MustCompile(`(?is)<a\s+[^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`)
	tagRE   = regexp.MustCompile(`(?is)<[^>]+>`)
	spaceRE = regexp.MustCompile(`\s+`)
)

func cleanHTML(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", " ")
	value = tagRE.ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	return strings.TrimSpace(spaceRE.ReplaceAllString(value, " "))
}

func extractLinks(value string) []Link {
	matches := linkRE.FindAllStringSubmatch(value, -1)
	links := make([]Link, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		url := stripTracking(strings.TrimSpace(html.UnescapeString(match[1])))
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		links = append(links, Link{Text: cleanHTML(match[2]), URL: url})
	}
	return links
}

func stripTracking(raw string) string {
	raw = strings.ReplaceAll(raw, "&#038;", "&")
	if idx := strings.Index(raw, "?utm_"); idx >= 0 {
		return raw[:idx]
	}
	if idx := strings.Index(raw, "&utm_"); idx >= 0 {
		return raw[:idx]
	}
	return raw
}
