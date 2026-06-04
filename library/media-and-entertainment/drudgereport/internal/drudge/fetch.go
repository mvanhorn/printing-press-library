package drudge

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// DefaultHTMLURL is the canonical Drudge Report HTML page URL.
const DefaultHTMLURL = "https://drudgereport.com/"

// DefaultRSSURL is the off-host Drudge Report RSS mirror URL.
const DefaultRSSURL = "https://feedpress.me/drudgereportfeed"

const userAgent = "Mozilla/5.0 (compatible; drudgereport-pp-cli/0.1; +https://github.com/mvanhorn/printing-press-library)"

// FetchHTML fetches the Drudge Report HTML page.
func FetchHTML(ctx context.Context, url string) ([]byte, error) {
	if verifyMode() {
		return []byte(`<! MAIN HEADLINE ><a href="https://example.com/story"><font color=red>Sample headline...</font></a><! MAIN HEADLINE END HERE><! TOP LEFT STARTS HERE ><! TOP LEFT HEADLINES END HERE><! LINKS FIRST COLUMN><! LINKS SECOND C OL U M N>`), nil
	}
	return fetch(ctx, url, 20*time.Second)
}

// FetchRSS fetches the Drudge Report RSS feed.
func FetchRSS(ctx context.Context, url string) ([]byte, error) {
	if verifyMode() {
		return []byte(`<?xml version="1.0"?><rss><channel><item><title>Sample headline</title><link>https://feedpress.me/drudgereportfeed/sample</link><guid>https://example.com/story</guid><pubDate>Thu, 21 May 2026 13:16:07 UTC</pubDate><description><![CDATA[(Main headline, 1st story, link)<br><span style="color:red">Sample headline</span>]]></description></item></channel></rss>`), nil
	}
	return fetch(ctx, url, 15*time.Second)
}

func fetch(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	return body, nil
}

func verifyMode() bool {
	return os.Getenv("PRINTING_PRESS_VERIFY") != ""
}
