// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Intent kinds of the apps.apple.com server-rendered page models.
const (
	KindProductPage   = "ProductPageIntent"
	KindChartsPage    = "ChartsPageIntent"
	KindDeveloperPage = "DeveloperPageIntent"
	KindSeeAllPage    = "SeeAllPageIntent"
	KindArticlePage   = "RoutableArticlePageIntent"
	// Today pages use per-platform landing intents such as
	// "LandingPage_iphone/today_Intent"; match them with IsTodayKind.
	todayKindPrefix = "LandingPage_"
	todayKindSuffix = "today_Intent"
	// searchKindWant is the expectKind value for the (opt-in) web search
	// page, whose exact intent kind is not pinned: any kind that mentions
	// "search" matches (see isSearchKind).
	searchKindWant = "a search page intent"
)

// IsTodayKind reports whether an intent kind is a Today landing page.
func IsTodayKind(kind string) bool {
	return strings.HasPrefix(kind, todayKindPrefix) && strings.HasSuffix(kind, todayKindSuffix)
}

// isSearchKind reports whether an intent kind is a web search results page.
func isSearchKind(kind string) bool {
	return strings.Contains(strings.ToLower(kind), "search")
}

// PageModel is the server-side data model embedded in an apps.apple.com page
// (<script type="application/json" id="serialized-server-data">). Intent is
// the route intent ($kind, storefront, id, platform, genreId, chart ...);
// Data is the page body the parsers read.
type PageModel struct {
	URL       string         `json:"url"`
	FinalURL  string         `json:"finalUrl,omitempty"`
	Kind      string         `json:"kind"`
	Intent    map[string]any `json:"intent"`
	Data      map[string]any `json:"-"`
	FromCache bool           `json:"fromCache"`
	FetchedAt time.Time      `json:"fetchedAt"`
	// DataAsOf is FetchedAt minus the CDN copy's Age (see FetchMeta).
	DataAsOf time.Time `json:"dataAsOf"`
}

// IntentString returns a string field of the page intent ("storefront",
// "id", "platform", "genreId", "chart", "see-all", "language").
func (m *PageModel) IntentString(key string) string {
	if m == nil {
		return ""
	}
	return strOf(m.Intent[key])
}

// RawData re-encodes the page body (kept for parse-failure diagnostics).
func (m *PageModel) RawData() json.RawMessage {
	if m == nil || m.Data == nil {
		return nil
	}
	b, err := json.Marshal(m.Data)
	if err != nil {
		return nil
	}
	return b
}

// ErrNoPageModel reports HTML without the serialized-server-data script.
var ErrNoPageModel = errors.New("no serialized-server-data page model in the App Store HTML")

const ssdMarker = `id="serialized-server-data"`

// ExtractPageModel pulls the serialized-server-data JSON out of an
// apps.apple.com HTML page and decodes the first page entry. It accepts the
// current envelope {data:[{intent, data}], userTokenHash} and the older bare
// array [{intent, data}]. The userTokenHash is discarded.
func ExtractPageModel(html []byte) (*PageModel, error) {
	raw, err := extractSSD(html)
	if err != nil {
		return nil, err
	}
	return decodePageModel(raw)
}

func extractSSD(html []byte) ([]byte, error) {
	idx := bytes.Index(html, []byte(ssdMarker))
	if idx < 0 {
		// Accept a bare JSON model (fixtures and pre-extracted captures).
		trimmed := bytes.TrimSpace(html)
		if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
			return trimmed, nil
		}
		return nil, ErrNoPageModel
	}
	start := bytes.IndexByte(html[idx:], '>')
	if start < 0 {
		return nil, ErrNoPageModel
	}
	start += idx + 1
	end := bytes.Index(html[start:], []byte("</script>"))
	if end < 0 {
		return nil, ErrNoPageModel
	}
	return bytes.TrimSpace(html[start : start+end]), nil
}

func decodePageModel(raw []byte) (*PageModel, error) {
	var top any
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("decoding App Store page model: %w", err)
	}
	var entries []any
	switch v := top.(type) {
	case map[string]any:
		entries = asArray(v["data"])
	case []any:
		entries = v
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("app store page model has no page entries")
	}
	first := asObject(entries[0])
	if first == nil {
		return nil, fmt.Errorf("app store page model entry is not an object")
	}
	intent := asObject(first["intent"])
	data := asObject(first["data"])
	if data == nil {
		return nil, fmt.Errorf("app store page model has no data")
	}
	return &PageModel{
		Kind:   kindOf(intent),
		Intent: intent,
		Data:   data,
	}, nil
}

// expectKind returns a PageKindError unless the model's intent kind matches.
func expectKind(m *PageModel, want string) error {
	if m == nil {
		return fmt.Errorf("nil App Store page model")
	}
	if want == "" {
		return nil
	}
	switch want {
	case todayKindSuffix:
		if IsTodayKind(m.Kind) {
			return nil
		}
	case searchKindWant:
		if isSearchKind(m.Kind) {
			return nil
		}
	default:
		if m.Kind == want {
			return nil
		}
	}
	url := m.FinalURL
	if url == "" {
		url = m.URL
	}
	return &PageKindError{URL: url, Want: want, Got: m.Kind}
}
