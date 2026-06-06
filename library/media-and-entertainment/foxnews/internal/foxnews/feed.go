package foxnews

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	userAgent    = "Mozilla/5.0 (compatible; foxnews-pp-cli/1.0; +https://github.com/mvanhorn/printing-press-library)"
	maxFeedBytes = 10 << 20
)

// Headline is one RSS item from a Fox News Google Publisher feed.
type Headline struct {
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	GUID        string    `json:"guid,omitempty"`
	Published   time.Time `json:"published,omitempty"`
	Section     string    `json:"section"`
	Categories  []string  `json:"categories,omitempty"`
	Description string    `json:"description,omitempty"`
}

// Feed is a parsed RSS channel with items.
type Feed struct {
	Title       string     `json:"title"`
	Link        string     `json:"link"`
	Description string     `json:"description,omitempty"`
	Section     string     `json:"section"`
	FeedURL     string     `json:"feed_url"`
	Items       []Headline `json:"items"`
}

type rawRSS struct {
	Channel rawChannel `xml:"channel"`
}

type rawChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rawItem `xml:"item"`
}

type rawItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	GUID        string   `xml:"guid"`
	Description string   `xml:"description"`
	PubDate     string   `xml:"pubDate"`
	Categories  []string `xml:"category"`
}

var tagRE = regexp.MustCompile(`(?is)<[^>]+>`)
var spaceRE = regexp.MustCompile(`\s+`)

var (
	sharedHTTPClient     *http.Client
	sharedHTTPClientOnce sync.Once
)

func rssHTTPClient() *http.Client {
	sharedHTTPClientOnce.Do(func() {
		sharedHTTPClient = &http.Client{
			Transport: &http.Transport{
				Proxy:           http.ProxyFromEnvironment,
				MaxIdleConns:    10,
				IdleConnTimeout: 90 * time.Second,
			},
		}
	})
	return sharedHTTPClient
}

// Fetch loads and parses the RSS feed for section at baseURL (empty uses DefaultFeedBase).
func Fetch(ctx context.Context, section Section, baseURL string, timeout time.Duration) (Feed, error) {
	if verifyMode() {
		return parseFeedBody([]byte(verifyFixtureXML), section, FeedURL(section, baseURL))
	}

	if env := strings.TrimSpace(os.Getenv("FOX_NEWS_FEED_BASE")); env != "" {
		baseURL = env
	}
	feedURL := FeedURL(section, baseURL)

	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, feedURL, nil)
	if err != nil {
		return Feed{}, err
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml;q=0.9, */*;q=0.8")
	req.Header.Set("User-Agent", userAgent)

	resp, err := rssHTTPClient().Do(req)
	if err != nil {
		return Feed{}, fmt.Errorf("fetch %s: %w", feedURL, err)
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
	return parseFeedBody(body, section, feedURL)
}

func parseFeedBody(body []byte, section Section, feedURL string) (Feed, error) {
	var raw rawRSS
	if err := xml.Unmarshal(body, &raw); err != nil {
		return Feed{}, fmt.Errorf("decode RSS: %w", err)
	}

	feed := Feed{
		Title:       strings.TrimSpace(raw.Channel.Title),
		Link:        strings.TrimSpace(raw.Channel.Link),
		Description: cleanHTML(raw.Channel.Description),
		Section:     section.ID,
		FeedURL:     feedURL,
	}
	for _, item := range raw.Channel.Items {
		title := strings.TrimSpace(item.Title)
		link := strings.TrimSpace(item.Link)
		if link == "" {
			link = strings.TrimSpace(item.GUID)
		}
		if title == "" || link == "" {
			continue
		}
		published, _ := parseRSSDate(item.PubDate)
		feed.Items = append(feed.Items, Headline{
			Title:       title,
			Link:        link,
			GUID:        strings.TrimSpace(item.GUID),
			Published:   published,
			Section:     section.ID,
			Categories:  cleanStrings(item.Categories),
			Description: cleanHTML(item.Description),
		})
	}
	return feed, nil
}

// Limit returns at most n items (n <= 0 means no limit).
func Limit(items []Headline, n int) []Headline {
	if n <= 0 || len(items) <= n {
		return items
	}
	return items[:n]
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

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func cleanHTML(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", " ")
	value = tagRE.ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	return strings.TrimSpace(spaceRE.ReplaceAllString(value, " "))
}

func verifyMode() bool {
	return os.Getenv("PRINTING_PRESS_VERIFY") != ""
}

const verifyFixtureXML = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
  <title>Fox News</title>
  <link>https://www.foxnews.com/</link>
  <item>
    <title>Sample Fox headline for verify</title>
    <link>https://www.foxnews.com/example-story</link>
    <guid>https://www.foxnews.com/example-story</guid>
    <pubDate>Wed, 03 Jun 2026 10:00:00 -0400</pubDate>
    <category>fox-news/us</category>
  </item>
</channel>
</rss>`
