// Package mangoplate is a v1 stub for MangoPlate. The service has been
// winding down public coverage; v1 ships a read-only listing surface
// (search-page URL emitter) and defers body extraction to v2 if/when the
// service stabilizes.
package mangoplate

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var ErrV1Stub = errors.New("mangoplate: v1 ships listing-URL surface only; body extraction deferred (service is winding down public coverage)")

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

// SearchURL returns the public URL for a MangoPlate search.
func (c *Client) SearchURL(query string) string {
	return "https://www.mangoplate.com/en/search/" + url.PathEscape(query)
}

// Search returns ErrV1Stub.
func (c *Client) Search(ctx context.Context, query string) ([]Restaurant, error) {
	return nil, fmt.Errorf("%w (search query was %q)", ErrV1Stub, query)
}

type Restaurant struct {
	Name    string  `json:"name"`
	URL     string  `json:"url"`
	Lat     float64 `json:"lat,omitempty"`
	Lng     float64 `json:"lng,omitempty"`
	Address string  `json:"address,omitempty"`
}
