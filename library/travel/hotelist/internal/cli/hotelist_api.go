// Hand-authored core for the Hotelist CLI. Not generated — survives regen as a
// whole hand-authored unit. Implements the reverse-engineered hotelist.com /api
// contract (jQuery nested-array filter grammar), shared hotel types, output
// rendering with honest "scraped / AI-rated" labeling, and the polite client.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/hotelist/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/hotelist/internal/config"
)

// hotelistDisclaimer is appended to every machine and human output so callers
// never mistake this for an official, real-time hotel API.
const hotelistDisclaimer = "Data scraped from Hotelist.com (by @levelsio); community/AI-rated, not an official API. Ratings are AI-normalized 0-10; prices are AI-estimated nightly figures, not date-specific quotes."

const hotelistSource = "hotelist.com (scraped, AI-rated)"

// apiFilter mirrors one element of the /api `filters[]` array. Value is either a
// scalar (string/number) or, for sort-by, a nested key/order object.
type apiFilter struct {
	Target string
	Value  string
	Type   string
	// SortKey/SortOrder are used only when Target == "sort-by".
	SortKey   string
	SortOrder string
	// Bbox is used only when Target == "bbox": [latMin, latMax, lngMin, lngMax].
	Bbox []float64
}

func filterBbox(b [4]float64) apiFilter {
	return apiFilter{Target: "bbox", Bbox: []float64{b[0], b[1], b[2], b[3]}}
}

func filterGeohash(prefix string) apiFilter {
	return apiFilter{Target: "geohash", Value: prefix, Type: "starts_with"}
}
func filterCountry(country string) apiFilter {
	return apiFilter{Target: "country", Value: country, Type: "exact-match"}
}
func filterChain(code string) apiFilter {
	return apiFilter{Target: "parent_chain_code", Value: code, Type: "exact-match"}
}
func filterSubChain(code string) apiFilter {
	return apiFilter{Target: "chain_code", Value: code, Type: "exact-match"}
}
func filterAmenity(label string) apiFilter {
	return apiFilter{Target: "amenities", Value: label, Type: "contains"}
}
func filterMinRating(v float64) apiFilter {
	return apiFilter{Target: "hotellist_rating", Value: strconv.FormatFloat(v, 'f', -1, 64), Type: "greater-than"}
}
func filterMaxPrice(v float64) apiFilter {
	return apiFilter{Target: "price", Value: strconv.FormatFloat(v, 'f', -1, 64), Type: "less-than"}
}
func filterMinPrice(v float64) apiFilter {
	return apiFilter{Target: "price", Value: strconv.FormatFloat(v, 'f', -1, 64), Type: "greater-than"}
}
func filterBuiltAfter(year int) apiFilter {
	return apiFilter{Target: "year_built", Value: strconv.Itoa(year), Type: "greater-than"}
}
func filterSort(key, order string) apiFilter {
	return apiFilter{Target: "sort-by", SortKey: key, SortOrder: order}
}

// buildAPIParams flattens filters + search into the jQuery nested-array query
// keys the PHP backend expects (filters[0][target]=...). q.Encode() percent-
// encodes the brackets, which the server URL-decodes back into $_GET['filters'].
func buildAPIParams(filters []apiFilter, search string) map[string]string {
	params := map[string]string{}
	for i, f := range filters {
		base := "filters[" + strconv.Itoa(i) + "]"
		params[base+"[target]"] = f.Target
		params[base+"[type]"] = f.Type
		if f.Target == "sort-by" {
			params[base+"[value][key]"] = f.SortKey
			params[base+"[value][order]"] = f.SortOrder
			// sort-by carries no top-level [type]; harmless but drop it.
			delete(params, base+"[type]")
			continue
		}
		if f.Target == "bbox" && len(f.Bbox) == 4 {
			params[base+"[value][lat_min]"] = strconv.FormatFloat(f.Bbox[0], 'f', -1, 64)
			params[base+"[value][lat_max]"] = strconv.FormatFloat(f.Bbox[1], 'f', -1, 64)
			params[base+"[value][lng_min]"] = strconv.FormatFloat(f.Bbox[2], 'f', -1, 64)
			params[base+"[value][lng_max]"] = strconv.FormatFloat(f.Bbox[3], 'f', -1, 64)
			delete(params, base+"[type]")
			continue
		}
		params[base+"[value]"] = f.Value
	}
	if s := strings.TrimSpace(search); s != "" {
		params["search"] = s
	}
	return params
}

// hlHotel is the raw shape of one element of the /api `hotels` array.
type hlHotel struct {
	HotelID         string  `json:"hotel_id"`
	Name            string  `json:"name"`
	Rating          float64 `json:"hotellist_rating"`
	Price           float64 `json:"price"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	Photo           string  `json:"photo"`
	Pros            string  `json:"pros"`
	Cons            string  `json:"cons"`
	YearBuilt       *int    `json:"year_built"`
	ParentChainCode *string `json:"parent_chain_code"`
	YoutubeID       *string `json:"youtube_id"`
}

// apiResponse is the envelope returned by /api.
type apiResponse struct {
	Hotels             []hlHotel       `json:"hotels"`
	PriceHistogram     json.RawMessage `json:"price_histogram"`
	RatingHistogram    json.RawMessage `json:"rating_histogram"`
	YearBuiltHistogram json.RawMessage `json:"year_built_histogram"`
	RuntimeMs          json.RawMessage `json:"runtime_ms"`
}

// fetchHotels issues one /api call and returns the parsed hotels. It uses the
// generated client (rate limiting, retries, required headers, response cache).
func fetchHotels(ctx context.Context, c *client.Client, filters []apiFilter, search string) ([]hlHotel, error) {
	params := buildAPIParams(filters, search)
	data, err := c.Get(ctx, "/api", params)
	if err != nil {
		return nil, err
	}
	var resp apiResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing /api response: %w", err)
	}
	return resp.Hotels, nil
}

// adaptiveFetch is fetchHotels with geohash-precision fallback. Hotelist's city
// <option> geohashes vary in precision: a 4-char prefix is right for a dense
// city (A Coruna 'ezdn' -> 22 hotels) but too narrow for a sparse one (Tulum
// 'd59f' -> 0, while 'd59' -> 20), because individual hotel geohashes don't
// align with the city centroid at fine precision. When a geohash query returns
// too few results, widen the prefix one character at a time down to 3 chars.
// Country/bbox queries have no geohash filter and fetch exactly once.
func adaptiveFetch(ctx context.Context, c *client.Client, filters []apiFilter, search string) ([]hlHotel, error) {
	const minResults = 5
	const minPrefix = 3

	ghIdx := -1
	for i, f := range filters {
		if f.Target == "geohash" {
			ghIdx = i
			break
		}
	}
	hotels, err := fetchHotels(ctx, c, filters, search)
	if err != nil {
		return nil, err
	}
	if ghIdx < 0 {
		return hotels, nil
	}
	gh := filters[ghIdx].Value
	for len(hotels) < minResults && len(gh) > minPrefix {
		gh = gh[:len(gh)-1]
		widened := append([]apiFilter{}, filters...)
		widened[ghIdx] = filterGeohash(gh)
		h2, err := fetchHotels(ctx, c, widened, search)
		if err != nil {
			break
		}
		// Only accept the wider search when it actually improves coverage — a
		// wider geohash can occasionally return fewer rows, and we must never
		// hand back fewer hotels than the initial fetch produced.
		if len(h2) > len(hotels) {
			hotels = h2
		}
	}
	return hotels, nil
}

// politeClient builds the client with a polite default request rate when the
// user has not set --rate-limit. Hotelist is a free community site with no API;
// we never hammer it. Multi-location commands additionally fetch sequentially.
func (f *rootFlags) politeClient() (*client.Client, error) {
	cfg, err := config.Load(f.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	rate := f.rateLimit
	if rate <= 0 {
		rate = 2.0 // ~2 req/s default; polite for a no-API community site
	}
	c := client.New(cfg, f.timeout, rate)
	c.DryRun = f.dryRun
	c.NoCache = f.noCache
	return c, nil
}

// ---- output model ----

type hotelOut struct {
	HotelID        string   `json:"hotel_id"`
	Name           string   `json:"name"`
	Rating         float64  `json:"hotelist_rating"`
	Price          float64  `json:"price"`
	ValuePer100USD float64  `json:"value_rating_per_100usd,omitempty"`
	YearBuilt      *int     `json:"year_built,omitempty"`
	Chain          string   `json:"chain,omitempty"`
	Exceptional    bool     `json:"exceptional"`
	Pros           []string `json:"pros,omitempty"`
	Cons           []string `json:"cons,omitempty"`
	Latitude       float64  `json:"latitude,omitempty"`
	Longitude      float64  `json:"longitude,omitempty"`
	Photo          string   `json:"photo,omitempty"`
	YoutubeURL     string   `json:"youtube_url,omitempty"`
	URL            string   `json:"url"`
}

type hotelListView struct {
	Source     string     `json:"source"`
	Disclaimer string     `json:"disclaimer"`
	Location   string     `json:"location,omitempty"`
	Count      int        `json:"count"`
	Hotels     []hotelOut `json:"hotels"`
	Note       string     `json:"note,omitempty"`
}

// chainNameByCode is the reverse of the chain-code map for display. Populated
// from chainCodeByName in hotelist_locations.go.
func toHotelOut(h hlHotel) hotelOut {
	out := hotelOut{
		HotelID:   h.HotelID,
		Name:      cleanTitle(h.Name),
		Rating:    round1(h.Rating),
		Price:     round2(h.Price),
		YearBuilt: h.YearBuilt,
		Latitude:  h.Latitude,
		Longitude: h.Longitude,
		Photo:     h.Photo,
		Pros:      splitBullets(h.Pros),
		Cons:      splitBullets(h.Cons),
		URL:       "https://hotelist.com/hotel/" + h.HotelID,
	}
	if h.Price > 0 {
		out.ValuePer100USD = round2(h.Rating / h.Price * 100)
	}
	if h.ParentChainCode != nil && *h.ParentChainCode != "" {
		out.Chain = chainDisplay(*h.ParentChainCode)
	}
	if h.YoutubeID != nil && *h.YoutubeID != "" {
		out.YoutubeURL = "https://www.youtube.com/watch?v=" + *h.YoutubeID
	}
	out.Exceptional = isExceptional(h)
	return out
}

// isExceptional approximates Hotelist's "Exceptional" badge. The list /api does
// not return the consensus signal, so we apply the two checks we can: score 8+
// and built within the last ~10 years (or unknown build year). Honest about the
// missing consensus dimension in the README/help.
func isExceptional(h hlHotel) bool {
	if h.Rating < 8.0 {
		return false
	}
	if h.YearBuilt == nil {
		return true
	}
	return *h.YearBuilt >= hotelistRecentYear
}

const hotelistRecentYear = 2016 // ~"built in the last 10 years" relative to 2026

func splitBullets(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func cleanTitle(s string) string { return strings.TrimSpace(s) }

func round1(f float64) float64 { return math.Round(f*10) / 10 }
func round2(f float64) float64 { return math.Round(f*100) / 100 }

// dropUnpriced removes hotels with no usable price. price <= 0 means Hotelist
// has no AI price estimate; such hotels cannot honestly satisfy a price filter
// or be ranked by rating-per-dollar, so value/price-filtered commands exclude
// them rather than treating "unknown" as "free".
func dropUnpriced(hs []hlHotel) []hlHotel {
	out := hs[:0:0]
	for _, h := range hs {
		if h.Price > 0 {
			out = append(out, h)
		}
	}
	return out
}

// sortHotelsByValue sorts descending by rating/price (best value first). Hotels
// with no price sink to the bottom.
func sortHotelsByValue(hs []hlHotel) {
	sort.SliceStable(hs, func(i, j int) bool {
		return valueScore(hs[i]) > valueScore(hs[j])
	})
}
func sortHotelsByRating(hs []hlHotel) {
	sort.SliceStable(hs, func(i, j int) bool { return hs[i].Rating > hs[j].Rating })
}
func valueScore(h hlHotel) float64 {
	if h.Price <= 0 {
		return -1
	}
	return h.Rating / h.Price
}

// dedupeHotels removes duplicate hotel_id entries, keeping first occurrence.
func dedupeHotels(hs []hlHotel) []hlHotel {
	seen := make(map[string]bool, len(hs))
	out := make([]hlHotel, 0, len(hs))
	for _, h := range hs {
		if h.HotelID == "" || seen[h.HotelID] {
			continue
		}
		seen[h.HotelID] = true
		out = append(out, h)
	}
	return out
}
