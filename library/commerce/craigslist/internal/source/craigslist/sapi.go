package craigslist

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const defaultSiteSearchDistanceMi = 60

// QueryParams is the small query-parameter surface needed to apply Craigslist
// site scoping to both url.Values and generated command parameter maps.
type QueryParams interface {
	Get(string) string
	Set(string, string)
}

// SearchResults is the typed shape we extract from a sapi response. The wire format
// is positional arrays plus shared decode tables; we expand into these structs so
// commands and the local store work with named fields.
type SearchResults struct {
	APIVersion       int             `json:"apiVersion"`
	CategoryAbbr     string          `json:"categoryAbbr"`
	CanonicalURL     string          `json:"canonicalUrl"`
	TotalResultCount int             `json:"totalResultCount"`
	PageTitle        string          `json:"pageTitle"`
	HumanReadable    json.RawMessage `json:"humanReadableParams,omitempty"`
	Site             string          `json:"site"`     // primary area name (e.g. "sfbay")
	Hostname         string          `json:"hostname"` // same as Site here; kept for clarity
	Items            []Listing       `json:"items"`
}

// Listing is a single search-result item with the positional fields decoded.
type Listing struct {
	PostingID    int64    `json:"postingId"`
	UUID         string   `json:"uuid"` // rapi handle for listing detail
	CategoryID   int      `json:"categoryId"`
	Price        int      `json:"price"` // -1 means no price
	PriceDisplay string   `json:"priceDisplay,omitempty"`
	Title        string   `json:"title"`
	Slug         string   `json:"slug,omitempty"`
	PostedDelta  int64    `json:"postedDelta"`       // seconds before maxPostedDate; computed PostedAt below
	PostedAt     int64    `json:"postedAt"`          // unix seconds, derived
	Site         string   `json:"site,omitempty"`    // e.g. "sfbay"
	Subarea      string   `json:"subarea,omitempty"` // e.g. "sfc"
	Neighborhood string   `json:"neighborhood,omitempty"`
	Latitude     float64  `json:"lat,omitempty"`
	Longitude    float64  `json:"lng,omitempty"`
	ThumbnailID  string   `json:"thumbnailId,omitempty"`
	Images       []string `json:"images,omitempty"`
	CanonicalURL string   `json:"canonicalUrl"` // computed, not from sapi
}

// rawSAPI is the on-the-wire shape we unmarshal first.
type rawSAPI struct {
	APIVersion int         `json:"apiVersion"`
	Data       rawSAPIData `json:"data"`
	Errors     []any       `json:"errors"`
}

type rawSAPIData struct {
	APIVersion       int                `json:"apiVersion"`
	Areas            map[string]rawArea `json:"areas"`
	CategoryAbbr     string             `json:"categoryAbbr"`
	CanonicalURL     string             `json:"canonicalUrl"`
	Decode           rawDecode          `json:"decode"`
	HumanReadable    json.RawMessage    `json:"humanReadableParams,omitempty"`
	Items            []json.RawMessage  `json:"items"`
	PageTitle        string             `json:"pageTitle"`
	Params           json.RawMessage    `json:"params,omitempty"`
	TotalResultCount int                `json:"totalResultCount"`
}

type rawArea struct {
	Name string `json:"name"`
}

type rawDecode struct {
	Locations            []json.RawMessage `json:"locations"` // index -> [areaCode, sitename, subarea] OR scalar 0
	LocationDescriptions []json.RawMessage `json:"locationDescriptions"`
	Neighborhoods        []json.RawMessage `json:"neighborhoods"`
	MaxPostedDate        int64             `json:"maxPostedDate"`
	MinDate              int64             `json:"minDate"`
	MinPostedDate        int64             `json:"minPostedDate"`
	MinPostingID         int64             `json:"minPostingId"`
}

func (d *rawDecode) UnmarshalJSON(body []byte) error {
	if len(body) == 0 || string(body) == "null" || body[0] != '{' {
		*d = rawDecode{}
		return nil
	}
	type rawDecodeAlias rawDecode
	var alias rawDecodeAlias
	if err := json.Unmarshal(body, &alias); err != nil {
		return err
	}
	*d = rawDecode(alias)
	return nil
}

// Search hits sapi.craigslist.org/web/v8/postings/search/full and returns typed results.
// site is the area hostname or abbreviation (e.g. "sfbay", "portland", "nyc").
func (c *Client) Search(ctx context.Context, site string, q SearchQuery) (*SearchResults, error) {
	params := q.values()
	var err error
	site, err = c.ApplySiteScopeParams(ctx, site, params)
	if err != nil {
		return nil, err
	}
	body, err := c.RawGet(ctx, HostSAPI, "/postings/search/full", params)
	if err != nil {
		return nil, err
	}
	results, err := decodeSearchBody(body, site)
	if err != nil {
		return nil, err
	}
	if err := c.enrichSearchURLs(ctx, results); err != nil {
		return nil, err
	}
	return results, nil
}

// SearchQuery is the typed input. Cross-city callers re-use the same query against multiple sites.
type SearchQuery struct {
	Query          string
	SearchPath     string // category abbr; default sss
	CountryCode    string // default US
	Lang           string // default en
	Page           int    // 1-indexed; converts to batch=<page>-0-360-0-0
	MinPrice       int
	MaxPrice       int
	HasPic         bool
	Postal         string
	SearchDistance int
	Latitude       float64
	Longitude      float64
	TitleOnly      bool
	Sort           string // date|rel|priceasc|pricedsc; default rel
	BatchSize      int    // default 360
}

func (q SearchQuery) values() url.Values {
	v := url.Values{}
	if q.SearchPath == "" {
		q.SearchPath = "sss"
	}
	if q.CountryCode == "" {
		q.CountryCode = "US"
	}
	if q.Lang == "" {
		q.Lang = "en"
	}
	if q.BatchSize == 0 {
		q.BatchSize = 360
	}
	if q.Page < 1 {
		q.Page = 1
	}
	v.Set("searchPath", q.SearchPath)
	v.Set("cc", q.CountryCode)
	v.Set("lang", q.Lang)
	v.Set("batch", fmt.Sprintf("%d-0-%d-0-0", q.Page, q.BatchSize))
	if q.Query != "" {
		v.Set("query", q.Query)
	}
	if q.MinPrice > 0 {
		v.Set("min_price", strconv.Itoa(q.MinPrice))
	}
	if q.MaxPrice > 0 {
		v.Set("max_price", strconv.Itoa(q.MaxPrice))
	}
	if q.HasPic {
		v.Set("hasPic", "1")
	}
	if q.Postal != "" {
		v.Set("postal", q.Postal)
	}
	if q.Latitude != 0 && q.Longitude != 0 {
		v.Set("lat", strconv.FormatFloat(q.Latitude, 'f', -1, 64))
		v.Set("lon", strconv.FormatFloat(q.Longitude, 'f', -1, 64))
	}
	if q.SearchDistance > 0 {
		v.Set("search_distance", strconv.Itoa(q.SearchDistance))
	}
	if q.TitleOnly {
		v.Set("srchType", "T")
	}
	if q.Sort != "" {
		v.Set("sort", q.Sort)
	}
	return v
}

func (c *Client) ApplySiteScopeParams(ctx context.Context, site string, params QueryParams) (string, error) {
	site = strings.TrimSpace(site)
	if site == "" {
		return site, nil
	}
	area, ok, err := c.ResolveArea(ctx, site)
	if err != nil {
		return site, fmt.Errorf("resolve craigslist site %q: %w", site, err)
	}
	if !ok {
		return site, fmt.Errorf("unknown craigslist site %q", site)
	}
	if params.Get("postal") == "" && (params.Get("lat") == "" || params.Get("lon") == "") {
		params.Set("lat", strconv.FormatFloat(area.Latitude, 'f', -1, 64))
		params.Set("lon", strconv.FormatFloat(area.Longitude, 'f', -1, 64))
		if params.Get("search_distance") == "" {
			params.Set("search_distance", strconv.Itoa(defaultSiteSearchDistanceMi))
		}
	}
	return area.Hostname, nil
}

func decodeSearchBody(body []byte, site string) (*SearchResults, error) {
	var raw rawSAPI
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode sapi response: %w", err)
	}
	out := &SearchResults{
		APIVersion:       raw.Data.APIVersion,
		CategoryAbbr:     raw.Data.CategoryAbbr,
		CanonicalURL:     raw.Data.CanonicalURL,
		TotalResultCount: raw.Data.TotalResultCount,
		PageTitle:        raw.Data.PageTitle,
		HumanReadable:    raw.Data.HumanReadable,
		Site:             site,
		Hostname:         site,
	}
	dec := &raw0Decode{
		locations:     raw.Data.Decode.Locations,
		locationDescs: raw.Data.Decode.LocationDescriptions,
		neighborhoods: raw.Data.Decode.Neighborhoods,
		maxPostedDate: raw.Data.Decode.MaxPostedDate,
		minPostingID:  raw.Data.Decode.MinPostingID,
	}
	out.Items = make([]Listing, 0, len(raw.Data.Items))
	for _, item := range raw.Data.Items {
		l, err := decodeItem(item, dec)
		if err != nil {
			// Skip malformed items but keep going. Single-item drift shouldn't fail the page.
			continue
		}
		l.Site = site
		out.Items = append(out.Items, l)
	}
	return out, nil
}

type raw0Decode struct {
	locations     []json.RawMessage
	locationDescs []json.RawMessage
	neighborhoods []json.RawMessage
	maxPostedDate int64
	minPostingID  int64
}

// decodeItem expands one positional-array item into a typed Listing.
//
// Fixed prefix (positions 0–5):
//
//	[0] postingId int
//	[1] postedDelta int  (seconds before maxPostedDate)
//	[2] categoryId int
//	[3] price int  (-1 = no price)
//	[4] locationEnc str  ("<areaIdx>:<subAreaIdx>:<nbhIdx>~<lat>~<lng>")
//	[5] thumbnailId str
//
// Variable tail (positions 6..n-1) is a sequence of `[typeCode, value...]` pairs
// or sentinel scalars. Type codes seen: 4 = images list; 6 = slug; 10 = price display
// string; 13 = UUID. Last positional element is always the title string.
func decodeItem(raw json.RawMessage, decode *raw0Decode) (Listing, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return Listing{}, fmt.Errorf("item not array: %w", err)
	}
	if len(arr) < 7 {
		return Listing{}, fmt.Errorf("item has %d fields, need at least 7", len(arr))
	}
	var l Listing
	var postingOffset int64
	if err := json.Unmarshal(arr[0], &postingOffset); err != nil {
		return Listing{}, fmt.Errorf("postingId: %w", err)
	}
	l.PostingID = postingOffset
	if decode.minPostingID > 0 {
		l.PostingID = decode.minPostingID + postingOffset
	}
	_ = json.Unmarshal(arr[1], &l.PostedDelta)
	_ = json.Unmarshal(arr[2], &l.CategoryID)
	_ = json.Unmarshal(arr[3], &l.Price)
	if decode.maxPostedDate > 0 {
		l.PostedAt = decode.maxPostedDate - l.PostedDelta
	}
	var locEnc string
	_ = json.Unmarshal(arr[4], &locEnc)
	parseLocation(locEnc, &l, decode)
	_ = json.Unmarshal(arr[5], &l.ThumbnailID)

	// Last element is title (string).
	titleIdx := len(arr) - 1
	var title string
	if err := json.Unmarshal(arr[titleIdx], &title); err == nil {
		l.Title = title
	}
	// Tail: positions 6..titleIdx-1 are typed entries or sentinels.
	for i := 6; i < titleIdx; i++ {
		var entry []json.RawMessage
		if err := json.Unmarshal(arr[i], &entry); err != nil {
			// scalar sentinel; skip
			continue
		}
		if len(entry) < 2 {
			continue
		}
		var code int
		if err := json.Unmarshal(entry[0], &code); err != nil {
			continue
		}
		switch code {
		case 4:
			l.Images = make([]string, 0, len(entry)-1)
			for _, raw := range entry[1:] {
				var s string
				if err := json.Unmarshal(raw, &s); err == nil {
					l.Images = append(l.Images, s)
				}
			}
		case 6:
			var s string
			if err := json.Unmarshal(entry[1], &s); err == nil {
				l.Slug = s
			}
		case 10:
			var s string
			if err := json.Unmarshal(entry[1], &s); err == nil {
				l.PriceDisplay = s
			}
		case 13:
			var s string
			if err := json.Unmarshal(entry[1], &s); err == nil {
				l.UUID = s
			}
		}
	}
	return l, nil
}

func (c *Client) enrichSearchURLs(ctx context.Context, results *SearchResults) error {
	categories, err := c.cachedCategoryAbbrs(ctx)
	if err != nil {
		return fmt.Errorf("resolve craigslist categories: %w", err)
	}
	enrichSearchURLsWithCategories(results, categories)
	return nil
}

func enrichSearchURLsWithCategories(results *SearchResults, categories map[int]string) {
	if results == nil {
		return
	}
	for i := range results.Items {
		item := &results.Items[i]
		category := categories[item.CategoryID]
		item.CanonicalURL = listingURL(item.Site, item.Subarea, category, item.Slug, item.PostingID)
	}
}

func listingURL(site, subarea, category, slug string, postingID int64) string {
	if site == "" || category == "" || slug == "" || postingID <= 0 {
		return ""
	}
	if subarea != "" {
		return fmt.Sprintf("https://%s.craigslist.org/%s/%s/d/%s/%d.html", site, subarea, category, slug, postingID)
	}
	return fmt.Sprintf("https://%s.craigslist.org/%s/d/%s/%d.html", site, category, slug, postingID)
}

// parseLocation extracts subarea+neighborhood+lat+lng from "<areaIdx>:<subIdx>:<nbhIdx>~<lat>~<lng>".
// Tolerates short forms like "2:3~lat~lng" (subarea-only) and "0~lat~lng" (no indices).
func parseLocation(enc string, l *Listing, decode *raw0Decode) {
	if enc == "" {
		return
	}
	parts := strings.SplitN(enc, "~", 2)
	if len(parts) == 2 {
		coords := strings.SplitN(parts[1], "~", 2)
		if len(coords) == 2 {
			if v, err := strconv.ParseFloat(coords[0], 64); err == nil {
				l.Latitude = v
			}
			if v, err := strconv.ParseFloat(coords[1], 64); err == nil {
				l.Longitude = v
			}
		}
	}
	idxStr := parts[0]
	idxParts := strings.Split(idxStr, ":")
	if len(idxParts) >= 2 {
		// Subarea is in decode.locations. Index it.
		if subIdx, err := strconv.Atoi(idxParts[1]); err == nil && subIdx < len(decode.locations) {
			var loc []any
			if err := json.Unmarshal(decode.locations[subIdx], &loc); err == nil && len(loc) >= 3 {
				if s, ok := loc[2].(string); ok {
					l.Subarea = s
				}
			}
		}
	}
	if len(idxParts) >= 3 {
		if nbhIdx, err := strconv.Atoi(idxParts[2]); err == nil && nbhIdx < len(decode.neighborhoods) {
			var nbh string
			if err := json.Unmarshal(decode.neighborhoods[nbhIdx], &nbh); err == nil {
				l.Neighborhood = nbh
			}
		}
	}
}
