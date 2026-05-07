// Package fourtravel is a v1 stub for 4travel.jp (Japanese travel blog
// platform). v1 ships sitemap discovery only — rich blog-body extraction
// is deferred to v2.
package fourtravel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var ErrV1Stub = errors.New("fourtravel: v1 ships sitemap discovery only; blog-body extraction deferred to v2")

const defaultUA = "wanderlust-goat-pp-cli/0.1 (+https://github.com/mvanhorn/printing-press-library)"

type Client struct {
	http *http.Client
	ua   string
}

func New(httpClient *http.Client, ua string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if ua == "" {
		ua = defaultUA
	}
	return &Client{http: httpClient, ua: ua}
}

// SearchURL returns the public URL for a 4travel keyword search.
func (c *Client) SearchURL(query string) string {
	return "https://4travel.jp/search?keyword=" + url.QueryEscape(query)
}

func (c *Client) Search(ctx context.Context, query string) ([]Post, error) {
	return nil, fmt.Errorf("%w (search query was %q)", ErrV1Stub, query)
}

type Post struct {
	Title  string `json:"title"`
	URL    string `json:"url"`
	Author string `json:"author,omitempty"`
}
