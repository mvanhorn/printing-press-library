// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Atlas Obscura source layer (hand-authored). This file holds the community-sourced
// HTTP contract discovered from atlasobscura.com (no official API), the local-store
// helpers for trips/visited, geocoding via Open-Meteo, and the interestingness score
// shared by near/route/gaps/surprise. Kept as a whole hand-authored file so it
// survives generator regen.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/atlas-obscura/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/atlas-obscura/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/atlas-obscura/internal/store"
)

// aoSourceNote is appended to JSON envelopes so consumers never mistake this for
// an official API contract.
const aoSourceNote = "community-sourced from atlasobscura.com; not an official API"

// AOPlace is the normalized place shape used across every command and the local store.
type AOPlace struct {
	ID                int      `json:"id"`
	Slug              string   `json:"slug"`
	Title             string   `json:"title"`
	Subtitle          string   `json:"subtitle,omitempty"`
	Location          string   `json:"location,omitempty"`
	City              string   `json:"city,omitempty"`
	Country           string   `json:"country,omitempty"`
	Lat               float64  `json:"lat"`
	Lng               float64  `json:"lng"`
	Description       string   `json:"description,omitempty"`
	Categories        []string `json:"categories,omitempty"`
	KnowBeforeYouGo   string   `json:"know_before_you_go,omitempty"`
	ImageURL          string   `json:"image_url,omitempty"`
	URL               string   `json:"url"`
	DistanceFromQuery string   `json:"distance_from_query,omitempty"`
	Score             int      `json:"score,omitempty"`
}

// --- Search/near JSON contract ---------------------------------------------

type aoSearchResp struct {
	Q     string `json:"q"`
	Total struct {
		Value int `json:"value"`
	} `json:"total"`
	PerPage     int             `json:"per_page"`
	CurrentPage int             `json:"current_page"`
	Results     []aoSearchEntry `json:"results"`
}

type aoSearchEntry struct {
	Title             string   `json:"title"`
	Subtitle          string   `json:"subtitle"`
	Location          string   `json:"location"`
	ThumbnailURL      string   `json:"thumbnail_url"`
	URL               string   `json:"url"`
	ID                int      `json:"id"`
	Coordinates       aoLatLng `json:"coordinates"`
	DistanceFromQuery string   `json:"distance_from_query"`
}

type aoLatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// aoJSONHeaders are the headers that flip atlasobscura.com's /search and /places/<id>
// routes from an HTML shell to a JSON response.
func aoJSONHeaders() map[string]string {
	return map[string]string{
		"Accept":           "application/json",
		"X-Requested-With": "XMLHttpRequest",
	}
}

func (e aoSearchEntry) toPlace() AOPlace {
	return AOPlace{
		ID:                e.ID,
		Slug:              slugFromPlaceURL(e.URL),
		Title:             cliutil.CleanText(e.Title),
		Subtitle:          cliutil.CleanText(e.Subtitle),
		Location:          cliutil.CleanText(e.Location),
		Lat:               e.Coordinates.Lat,
		Lng:               e.Coordinates.Lng,
		ImageURL:          e.ThumbnailURL,
		URL:               absoluteAOURL(e.URL),
		DistanceFromQuery: e.DistanceFromQuery,
	}
}

var placeSlugRe = regexp.MustCompile(`/places/([^/?#]+)`)

func slugFromPlaceURL(u string) string {
	if m := placeSlugRe.FindStringSubmatch(u); m != nil {
		return m[1]
	}
	return strings.TrimPrefix(u, "/places/")
}

func absoluteAOURL(u string) string {
	if u == "" {
		return ""
	}
	if strings.HasPrefix(u, "http") {
		return u
	}
	return "https://www.atlasobscura.com" + u
}

// aoSearch runs a keyword text search. kind defaults to "keyword" for relevance.
func aoSearch(ctx context.Context, c *client.Client, q, kind string, page int) (aoSearchResp, error) {
	params := map[string]string{"q": q}
	if kind == "" {
		kind = "keyword"
	}
	params["kind"] = kind
	if page > 1 {
		params["page"] = strconv.Itoa(page)
	}
	return aoFetchSearch(ctx, c, params)
}

// aoNear runs a geo search sorted by distance from a point (miles).
func aoNear(ctx context.Context, c *client.Client, lat, lng float64, page int) (aoSearchResp, error) {
	params := map[string]string{
		"lat": strconv.FormatFloat(lat, 'f', 6, 64),
		"lng": strconv.FormatFloat(lng, 'f', 6, 64),
	}
	if page > 1 {
		params["page"] = strconv.Itoa(page)
	}
	return aoFetchSearch(ctx, c, params)
}

func aoFetchSearch(ctx context.Context, c *client.Client, params map[string]string) (aoSearchResp, error) {
	var resp aoSearchResp
	data, err := c.GetWithHeaders(ctx, "/search", params, aoJSONHeaders())
	if err != nil {
		return resp, err
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, fmt.Errorf("parsing search response (Atlas Obscura may have changed its undocumented search): %w", err)
	}
	return resp, nil
}

// --- Place detail -----------------------------------------------------------

type aoPlaceShort struct {
	ID           int      `json:"id"`
	Title        string   `json:"title"`
	Subtitle     string   `json:"subtitle"`
	City         string   `json:"city"`
	Country      string   `json:"country"`
	Location     string   `json:"location"`
	URL          string   `json:"url"`
	ThumbnailURL string   `json:"thumbnail_url"`
	Coordinates  aoLatLng `json:"coordinates"`
}

// aoFetchPlaceShort fetches the compact JSON place object (1 request, cheap).
// Uses the no-read-cache variant so it never reads an HTML-cached body for the
// same slug; the local SQLite store is the real cache layer.
func aoFetchPlaceShort(ctx context.Context, c *client.Client, idOrSlug string) (AOPlace, error) {
	var s aoPlaceShort
	data, err := c.GetWithHeadersNoCache(ctx, "/places/"+url.PathEscape(idOrSlug), nil, aoJSONHeaders())
	if err != nil {
		return AOPlace{}, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return AOPlace{}, fmt.Errorf("parsing place JSON: %w", err)
	}
	return AOPlace{
		ID:       s.ID,
		Slug:     slugFromPlaceURL(s.URL),
		Title:    cliutil.CleanText(s.Title),
		Subtitle: cliutil.CleanText(s.Subtitle),
		Location: cliutil.CleanText(s.Location),
		City:     cliutil.CleanText(s.City),
		Country:  cliutil.CleanText(s.Country),
		Lat:      s.Coordinates.Lat,
		Lng:      s.Coordinates.Lng,
		ImageURL: s.ThumbnailURL,
		URL:      absoluteAOURL(s.URL),
	}, nil
}

// JSON-LD shapes embedded in place HTML pages.
type aoJSONLD struct {
	Type        json.RawMessage `json:"@type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	URL         string          `json:"url"`
	Geo         struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"geo"`
	Address struct {
		StreetAddress   string `json:"streetAddress"`
		AddressLocality string `json:"addressLocality"`
		AddressRegion   string `json:"addressRegion"`
		PostalCode      string `json:"postalCode"`
		AddressCountry  string `json:"addressCountry"`
	} `json:"address"`
	Image string `json:"image"`
}

var (
	ldJSONRe   = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
	categoryRe = regexp.MustCompile(`/categories/([a-z0-9-]+)`)
)

// aoFetchPlaceFull fetches the HTML place page and parses JSON-LD plus category
// tags and the "Know Before You Go" practical-info section.
func aoFetchPlaceFull(ctx context.Context, c *client.Client, idOrSlug string) (AOPlace, error) {
	// Force an HTML response (the client defaults Accept to application/json,
	// which would return the compact JSON instead of the parseable HTML page).
	data, err := c.GetWithHeadersNoCache(ctx, "/places/"+url.PathEscape(idOrSlug), nil, map[string]string{"Accept": "text/html"})
	if err != nil {
		return AOPlace{}, err
	}
	html := string(data)
	p := AOPlace{Slug: idOrSlug, URL: absoluteAOURL("/places/" + idOrSlug)}

	for _, m := range ldJSONRe.FindAllStringSubmatch(html, -1) {
		var ld aoJSONLD
		if json.Unmarshal([]byte(m[1]), &ld) != nil {
			continue
		}
		if ld.Geo.Latitude != 0 || ld.Geo.Longitude != 0 || strings.Contains(string(ld.Type), "Place") {
			p.Title = cliutil.CleanText(ld.Name)
			p.Description = cliutil.CleanText(ld.Description)
			p.Lat = ld.Geo.Latitude
			p.Lng = ld.Geo.Longitude
			if ld.URL != "" {
				p.URL = ld.URL
				p.Slug = slugFromPlaceURL(ld.URL)
			}
			loc := strings.TrimSpace(strings.Join(filterEmpty([]string{
				ld.Address.AddressLocality, ld.Address.AddressRegion, ld.Address.AddressCountry,
			}), ", "))
			if loc != "" {
				p.Location = loc
			}
			p.City = cliutil.CleanText(ld.Address.AddressLocality)
			p.Country = cliutil.CleanText(ld.Address.AddressCountry)
			if ld.Image != "" {
				p.ImageURL = ld.Image
			}
			break
		}
	}

	// Category tags from /categories/<slug> links.
	seen := map[string]bool{}
	for _, m := range categoryRe.FindAllStringSubmatch(html, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			p.Categories = append(p.Categories, m[1])
		}
	}
	p.KnowBeforeYouGo = extractKBYG(html)
	return p, nil
}

var kbygTrailers = []string{"Community Contributors", "Added By", "Make an Edit", "Been Here?",
	"Related Places", "Nearby Places", "In partnership", "Atlas Obscura", "Want to see"}

// extractKBYG pulls the free-text "Know Before You Go" practical-info section.
func extractKBYG(html string) string {
	i := strings.Index(html, "Know Before You Go")
	if i < 0 {
		return ""
	}
	seg := html[i+len("Know Before You Go"):]
	if len(seg) > 4000 {
		seg = seg[:4000]
	}
	text := cliutil.CleanText(stripTags(seg))
	for _, t := range kbygTrailers {
		if j := strings.Index(text, t); j > 0 {
			text = text[:j]
		}
	}
	text = strings.TrimSpace(text)
	// Truncate on a rune boundary, not a byte offset: Atlas Obscura carries
	// non-ASCII content, and slicing mid-rune would emit invalid UTF-8 that
	// json.Marshal silently replaces with U+FFFD.
	if r := []rune(text); len(r) > 1200 {
		text = strings.TrimSpace(string(r[:1200])) + "…"
	}
	return text
}

var tagRe = regexp.MustCompile(`(?s)<[^>]+>`)

func stripTags(s string) string {
	return strings.TrimSpace(tagRe.ReplaceAllString(s, " "))
}

func filterEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

// --- Geocoding (Open-Meteo, no auth) ----------------------------------------

var (
	geocodeCacheMu sync.RWMutex
	geocodeCache   = map[string]aoGeoHit{}
)

type aoGeoHit struct {
	Lat   float64
	Lng   float64
	Label string
	Found bool
}

var latLngRe = regexp.MustCompile(`^\s*(-?\d{1,3}(?:\.\d+)?)\s*,\s*(-?\d{1,3}(?:\.\d+)?)\s*$`)

// resolvePoint accepts "lat,lng" or a place name (geocoded via Open-Meteo).
// Returns the coordinates and a human label.
func resolvePoint(ctx context.Context, arg string) (lat, lng float64, label string, err error) {
	if m := latLngRe.FindStringSubmatch(arg); m != nil {
		lat, _ = strconv.ParseFloat(m[1], 64)
		lng, _ = strconv.ParseFloat(m[2], 64)
		return lat, lng, fmt.Sprintf("%.4f,%.4f", lat, lng), nil
	}
	hit, err := aoGeocode(ctx, arg)
	if err != nil {
		return 0, 0, "", err
	}
	if !hit.Found {
		return 0, 0, "", notFoundErr(fmt.Errorf("could not geocode %q; try a more specific name (e.g. \"Portland, Oregon\") or pass lat,lng", arg))
	}
	return hit.Lat, hit.Lng, hit.Label, nil
}

var geoHTTP = &http.Client{Timeout: 15 * time.Second}

func aoGeocode(ctx context.Context, name string) (aoGeoHit, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	geocodeCacheMu.RLock()
	h, ok := geocodeCache[key]
	geocodeCacheMu.RUnlock()
	if ok {
		return h, nil
	}
	// Open-Meteo matches a single place name and does not parse "City, State"
	// or "City, Country". Try the full string, then progressively simpler forms.
	candidates := geocodeCandidates(name)
	var hit aoGeoHit
	for _, cand := range candidates {
		h, err := aoGeocodeOne(ctx, cand)
		if err != nil {
			return aoGeoHit{}, err
		}
		if h.Found {
			hit = h
			break
		}
	}
	geocodeCacheMu.Lock()
	geocodeCache[key] = hit
	geocodeCacheMu.Unlock()
	return hit, nil
}

// geocodeCandidates expands "Portland, Oregon" -> ["Portland, Oregon", "Portland"].
func geocodeCandidates(name string) []string {
	name = strings.TrimSpace(name)
	out := []string{name}
	if i := strings.Index(name, ","); i > 0 {
		head := strings.TrimSpace(name[:i])
		if head != "" && head != name {
			out = append(out, head)
		}
	}
	return out
}

func aoGeocodeOne(ctx context.Context, name string) (aoGeoHit, error) {
	u := "https://geocoding-api.open-meteo.com/v1/search?count=1&language=en&format=json&name=" + url.QueryEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return aoGeoHit{}, err
	}
	resp, err := geoHTTP.Do(req)
	if err != nil {
		return aoGeoHit{}, fmt.Errorf("geocoding %q via Open-Meteo: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return aoGeoHit{}, fmt.Errorf("geocoding %q via Open-Meteo: unexpected status %d", name, resp.StatusCode)
	}
	var body struct {
		Results []struct {
			Name      string  `json:"name"`
			Country   string  `json:"country"`
			Admin1    string  `json:"admin1"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return aoGeoHit{}, fmt.Errorf("parsing Open-Meteo geocode response: %w", err)
	}
	hit := aoGeoHit{}
	if len(body.Results) > 0 {
		r := body.Results[0]
		label := r.Name
		if r.Admin1 != "" {
			label += ", " + r.Admin1
		}
		if r.Country != "" {
			label += ", " + r.Country
		}
		hit = aoGeoHit{Lat: r.Latitude, Lng: r.Longitude, Label: label, Found: true}
	}
	return hit, nil
}

// --- Interestingness score --------------------------------------------------

var mundaneRe = regexp.MustCompile(`(?i)\b(historical marker|highway marker|plaque|memorial marker|roadside marker|state marker|grave of|tombstone)\b`)

// aoScore is a lightweight 0-10 interestingness heuristic computed from the
// fields available on a search result (no extra fetch). It rewards a curiosity
// subtitle and a descriptive title, and penalizes generic markers/plaques.
// Documented as a heuristic, not an official Atlas Obscura ranking.
func aoScore(p AOPlace) int {
	score := 5
	sub := strings.TrimSpace(p.Subtitle)
	switch {
	case len(sub) >= 40:
		score += 3
	case len(sub) >= 15:
		score += 2
	case sub != "":
		score++
	}
	if len(strings.TrimSpace(p.Description)) >= 120 {
		score++
	}
	if n := len(p.Categories); n >= 3 {
		score++
	}
	if mundaneRe.MatchString(p.Title) || mundaneRe.MatchString(sub) {
		score -= 4
	}
	if score < 0 {
		score = 0
	}
	if score > 10 {
		score = 10
	}
	return score
}

// --- Geo math ---------------------------------------------------------------

func haversineMiles(lat1, lng1, lat2, lng2 float64) float64 {
	const r = 3958.7613 // earth radius, miles
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return r * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func parseDistanceMiles(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// --- Local store helpers ----------------------------------------------------

func aoDB(ctx context.Context) (*store.Store, error) {
	return store.OpenWithContext(ctx, defaultDBPath("atlas-obscura-pp-cli"))
}

// cachePlace upserts a place into the local mirror for offline search/sql.
func cachePlace(s *store.Store, p AOPlace) {
	if p.ID == 0 {
		return
	}
	data, err := json.Marshal(p)
	if err != nil {
		return
	}
	_ = s.Upsert("places", strconv.Itoa(p.ID), json.RawMessage(data))
}

// ensureAOTables lazily creates the hand-owned trip and visited tables.
func ensureAOTables(s *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS ao_trip_items (
			trip TEXT NOT NULL,
			place_id INTEGER NOT NULL,
			slug TEXT,
			title TEXT,
			location TEXT,
			lat REAL,
			lng REAL,
			added_at TEXT,
			PRIMARY KEY (trip, place_id)
		)`,
		`CREATE TABLE IF NOT EXISTS ao_visited (
			place_id INTEGER PRIMARY KEY,
			slug TEXT,
			title TEXT,
			visited_on TEXT,
			note TEXT
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.DB().Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func nowDate() string { return time.Now().UTC().Format("2006-01-02") }
