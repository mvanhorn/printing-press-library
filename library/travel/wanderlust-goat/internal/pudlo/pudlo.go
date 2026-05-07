// Package pudlo is a v1 stub for Pudlo (French restaurant editorial).
// v1 ships sitemap discovery only; body extraction deferred to v2.
package pudlo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var ErrV1Stub = errors.New("pudlo: v1 ships sitemap discovery only; body extraction deferred to v2")

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

func (c *Client) SearchURL(query string) string {
	return "https://www.gaultmillau.fr/recherche?q=" + url.QueryEscape(query)
}

func (c *Client) Search(ctx context.Context, query string) ([]Restaurant, error) {
	return nil, fmt.Errorf("%w (search query was %q)", ErrV1Stub, query)
}

type Restaurant struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}
