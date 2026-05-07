// Package kakaomap is a v1 stub for Kakao Map (한국 지도). The package
// shell exists so dispatch and coverage can reference the source by slug,
// but rich body extraction is deferred to v2 — Naver Map already covers
// the canonical KR mapping signal in v1. The light listing surface uses
// `https://map.kakao.com/?q=<urlencoded>` to discover place URLs without
// extracting place details.
//
// Promote-to-full requires a JSON endpoint walk plus session-cookie
// handling that's outside v1 scope.
package kakaomap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// ErrV1Stub is returned by methods that are intentionally not yet wired.
// Callers should treat this as a non-error: the source is registered for
// dispatch and coverage purposes but produces no rows in v1.
var ErrV1Stub = errors.New("kakaomap: v1 ships package shell only; rich extraction deferred to v2")

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

// SearchURL returns the public URL for a Kakao Map search. The v1 stub
// emits the URL so callers can hand it to the user; it does not parse.
func (c *Client) SearchURL(query string) string {
	return "https://map.kakao.com/?q=" + url.QueryEscape(query)
}

// Search returns ErrV1Stub. Present so the dispatch table compiles and
// the source slug resolves to a callable client.
func (c *Client) Search(ctx context.Context, query string) ([]Place, error) {
	return nil, fmt.Errorf("%w (search query was %q)", ErrV1Stub, query)
}

// Place is the v1 stub place type. v2 will populate Lat/Lng/Address.
type Place struct {
	Name    string  `json:"name"`
	URL     string  `json:"url"`
	Lat     float64 `json:"lat,omitempty"`
	Lng     float64 `json:"lng,omitempty"`
	Address string  `json:"address,omitempty"`
}
