// Package notecom is a v1 stub for Note.com (Japanese long-form blog
// platform). v1 ships search-only URL emitter; body extraction deferred to v2.
package notecom

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var ErrV1Stub = errors.New("notecom: v1 ships search-URL surface only; body extraction deferred to v2")

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
	return "https://note.com/search?q=" + url.QueryEscape(query)
}

func (c *Client) Search(ctx context.Context, query string) ([]Post, error) {
	return nil, fmt.Errorf("%w (search query was %q)", ErrV1Stub, query)
}

type Post struct {
	Title  string `json:"title"`
	URL    string `json:"url"`
	Author string `json:"author,omitempty"`
}
