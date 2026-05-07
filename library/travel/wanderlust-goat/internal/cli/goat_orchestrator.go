package cli

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/criteria"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/dispatch"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/goatstore"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/overpass"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/reddit"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sources"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/wikipedia"
)

// AnchorResolution describes a geocoded anchor location.
type AnchorResolution struct {
	Query   string  `json:"query"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Country string  `json:"country"`
	Display string  `json:"display"`
}

// resolveAnchor parses lat,lng or geocodes via Nominatim. The geocode path
// uses a stdlib HTTP call to Nominatim with a contact-bearing User-Agent
// (policy-required); if WANDERLUST_GOAT_UA is set, that overrides.
func resolveAnchor(ctx context.Context, anchor string) (AnchorResolution, error) {
	anchor = strings.TrimSpace(anchor)
	if anchor == "" {
		return AnchorResolution{}, fmt.Errorf("anchor cannot be empty")
	}
	// Lat,lng path
	if lat, lng, ok := parseLatLng(anchor); ok {
		return AnchorResolution{
			Query: anchor, Lat: lat, Lng: lng, Country: "*", Display: anchor,
		}, nil
	}
	// Geocode path. accept-language=en tells Nominatim to prefer English
	// names in display_name; without it the response can splice alternate-
	// language name fields (e.g. Cyrillic transliterations on Korean
	// addresses) into the display string.
	url := "https://nominatim.openstreetmap.org/search?q=" +
		strings.ReplaceAll(anchor, " ", "+") +
		"&format=json&limit=1&addressdetails=1&accept-language=en"
	body, err := httpGetJSON(ctx, url, userAgent(), 10*time.Second)
	if err != nil {
		return AnchorResolution{}, fmt.Errorf("geocode %q: %w", anchor, err)
	}
	var results []struct {
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		DisplayName string `json:"display_name"`
		Address     struct {
			CountryCode string `json:"country_code"`
		} `json:"address"`
	}
	if err := jsonUnmarshal(body, &results); err != nil {
		return AnchorResolution{}, fmt.Errorf("parse geocode response: %w", err)
	}
	if len(results) == 0 {
		return AnchorResolution{}, fmt.Errorf("geocode returned no results for %q", anchor)
	}
	r := results[0]
	lat, _ := strconv.ParseFloat(r.Lat, 64)
	lng, _ := strconv.ParseFloat(r.Lon, 64)
	cc := strings.ToUpper(r.Address.CountryCode)
	if cc == "" {
		cc = "*"
	}
	return AnchorResolution{
		Query: anchor, Lat: lat, Lng: lng, Country: cc, Display: r.DisplayName,
	}, nil
}

// parseLatLng accepts "lat,lng" or "lat lng" with optional whitespace.
func parseLatLng(s string) (lat, lng float64, ok bool) {
	for _, sep := range []string{",", " "} {
		if !strings.Contains(s, sep) {
			continue
		}
		parts := strings.SplitN(s, sep, 2)
		if len(parts) != 2 {
			continue
		}
		la, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		ln, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 == nil && err2 == nil && la >= -90 && la <= 90 && ln >= -180 && ln <= 180 {
			return la, ln, true
		}
	}
	return 0, 0, false
}

func userAgent() string {
	if ua := strings.TrimSpace(os.Getenv("WANDERLUST_GOAT_UA")); ua != "" {
		return ua
	}
	return "wanderlust-goat-pp-cli/0.1 (+https://github.com/mvanhorn/printing-press-library)"
}

// Pick is one ranked result returned by the fanout orchestrator.
type Pick struct {
	Name         string   `json:"name"`
	NameLocal    string   `json:"name_local,omitempty"`
	Sources      []string `json:"sources"`
	Lat          float64  `json:"lat"`
	Lng          float64  `json:"lng"`
	WalkingMin   float64  `json:"walking_min"`
	DistanceM    float64  `json:"distance_m"`
	WhySpecial   string   `json:"why_special"`
	Score        float64  `json:"score"`
	Trust        float64  `json:"trust"`
	CountryBoost float64  `json:"country_boost"`
	Intent       string   `json:"intent"`
	Address      string   `json:"address,omitempty"`
	URL          string   `json:"url,omitempty"`
}

// FanoutResult is the orchestrator return value.
type FanoutResult struct {
	Anchor   AnchorResolution `json:"anchor"`
	Criteria string           `json:"criteria,omitempty"`
	Identity string           `json:"identity,omitempty"`
	Minutes  int              `json:"minutes"`
	Plan     dispatch.Plan    `json:"plan"`
	Picks    []Pick           `json:"results"`
	Errors   []string         `json:"errors,omitempty"`
}

// Fanout is the shared engine used by `near`, `goat`, and (via the
// no-LLM path) `crossover`. It:
//
//  1. Resolves the anchor via Nominatim
//  2. Parses criteria into intent + OSM tags + reddit keywords
//  3. Builds a research plan (typed dispatch)
//  4. Executes a subset of the plan in parallel against universal sources
//     (Overpass, Wikipedia, Reddit) and any reachable regional source.
//     Sources whose v1 implementation is a stub return empty + a typed
//     error which is recorded in `Errors` but does not fail the run.
//  5. Scores each candidate place via sources.Source.Score and ranks.
//  6. Walking-minutes computed via crow-flies-to-walking-minute estimate
//     (4.5 km/h average, ×1.3 path tortuosity factor); OSRM precise time
//     is left to per-pick refinement on the user's request — keeps the
//     fanout cheap.
//  7. Returns top N (default 5).
func Fanout(ctx context.Context, anchor AnchorResolution, criteriaText, identity string, walkingMinutes int, store *goatstore.Store) FanoutResult {
	parsed := criteria.Parse(criteriaText)
	radiusMeters := walkingMinutesToMeters(walkingMinutes)
	plan := dispatch.Build(dispatch.AnchorRef{
		Query: anchor.Query, Lat: anchor.Lat, Lng: anchor.Lng,
		Country: anchor.Country, Display: anchor.Display,
	}, parsed.Intent, parsed.RedditKW, radiusMeters)

	out := FanoutResult{
		Anchor: anchor, Criteria: criteriaText, Identity: identity,
		Minutes: walkingMinutes, Plan: plan,
	}

	candidates := make(map[string]*Pick) // dedupe key: lower(name)+latlng
	addCandidate := func(p Pick) {
		key := dedupeKey(p.Name, p.Lat, p.Lng)
		if existing, ok := candidates[key]; ok {
			// Merge sources, keep highest-trust why_special.
			existing.Sources = appendUnique(existing.Sources, p.Sources...)
			if p.Trust > existing.Trust {
				existing.WhySpecial = p.WhySpecial
				existing.Trust = p.Trust
			}
			existing.Score += p.Score
			return
		}
		candidates[key] = &p
	}

	// --- Overpass: OSM tag-filtered POIs ---
	if len(parsed.OSMTags) > 0 {
		op := overpass.New(nil, userAgent())
		filters := make([]overpass.TagFilter, 0, len(parsed.OSMTags))
		for _, t := range parsed.OSMTags {
			filters = append(filters, overpass.TagFilter{Key: t.Key, Value: t.Value})
		}
		resp, err := op.NearbyByTags(ctx, anchor.Lat, anchor.Lng, radiusMeters, filters)
		if err != nil {
			out.Errors = append(out.Errors, fmt.Sprintf("overpass: %v", err))
		} else if resp != nil {
			osmSrc := *sources.BySlug("overpass")
			for _, el := range resp.Elements {
				lat, lng := el.Lat, el.Lon
				if el.Center != nil {
					lat = el.Center.Lat
					lng = el.Center.Lon
				}
				name := el.Tags["name"]
				nameLocal := ""
				switch anchor.Country {
				case "JP":
					nameLocal = el.Tags["name:ja"]
				case "KR":
					nameLocal = el.Tags["name:ko"]
				case "FR":
					nameLocal = el.Tags["name:fr"]
				}
				if name == "" {
					name = nameLocal
				}
				if name == "" {
					continue
				}
				dist := haversineMeters(anchor.Lat, anchor.Lng, lat, lng)
				if dist > float64(radiusMeters) {
					continue
				}
				walkMin := metersToWalkingMinutes(dist)
				score := osmSrc.Score(sources.Country(anchor.Country), walkMin, parsed.Intent)
				addCandidate(Pick{
					Name: name, NameLocal: nameLocal,
					Sources: []string{"overpass"},
					Lat:     lat, Lng: lng,
					WalkingMin: walkMin, DistanceM: dist,
					WhySpecial: buildWhyOSM(el.Tags),
					Score:      score, Trust: osmSrc.Trust,
					Intent:  string(parsed.Intent),
					Address: el.Tags["addr:full"],
				})
			}
		}
	}

	// --- Wikipedia GeoSearch ---
	wikiLocale := "en"
	switch anchor.Country {
	case "JP":
		wikiLocale = "ja"
	case "KR":
		wikiLocale = "ko"
	case "FR":
		wikiLocale = "fr"
	}
	wp := wikipedia.New(wikiLocale, nil, userAgent())
	if resp, err := wp.GeoSearch(ctx, anchor.Lat, anchor.Lng, radiusMeters, 8); err != nil {
		out.Errors = append(out.Errors, fmt.Sprintf("wikipedia(%s): %v", wikiLocale, err))
	} else if resp != nil {
		wpSrc := *sources.BySlug("wikipedia")
		for _, p := range resp.Pages {
			dist := haversineMeters(anchor.Lat, anchor.Lng, p.Lat, p.Lon)
			walkMin := metersToWalkingMinutes(dist)
			pageIntent := classifyWikiTitle(p.Title)
			// Skip Wikipedia pages whose classified intent doesn't match the
			// user-requested intent. Without this guard, every nearby
			// Wikipedia article (offices, transit, generic landmarks) gets
			// the trust=0.95 boost and dominates results regardless of what
			// the user asked for ("vintage clothing" returning a rugby
			// championship article was the canonical bug).
			if parsed.Intent != "" && pageIntent != parsed.Intent {
				continue
			}
			score := wpSrc.Score(sources.Country(anchor.Country), walkMin, pageIntent)
			addCandidate(Pick{
				Name:    p.Title,
				Sources: []string{"wikipedia." + wikiLocale},
				Lat:     p.Lat, Lng: p.Lon,
				WalkingMin: walkMin, DistanceM: dist,
				WhySpecial: "Wikipedia-notable (" + wikiLocale + ")",
				Score:      score, Trust: wpSrc.Trust,
				Intent: string(pageIntent),
				URL:    "https://" + wikiLocale + ".wikipedia.org/wiki/" + strings.ReplaceAll(p.Title, " ", "_"),
			})
		}
	}

	// --- Reddit search (criteria-keyword filtered) ---
	if len(parsed.RedditKW) > 0 {
		r := reddit.New(nil, userAgent())
		subs := strings.Split(redditSubsForCountry(anchor.Country), ",")
		for _, sub := range subs {
			sub = strings.TrimSpace(sub)
			if sub == "" {
				continue
			}
			query := strings.Join(parsed.RedditKW, " ")
			threads, err := r.Search(ctx, sub, query, reddit.SearchOpts{
				MinScore: 10, MinComments: 3, Limit: 10, KeywordFilter: parsed.RedditKW,
			})
			if err != nil {
				out.Errors = append(out.Errors, fmt.Sprintf("reddit(r/%s): %v", sub, err))
				continue
			}
			// Reddit threads aren't places; persist them so quiet-hour and
			// reddit-quotes can join later. They contribute a +0.1 trust
			// boost to any candidate place whose name appears in a thread
			// body, applied during ranking finalization.
			if store != nil {
				for _, th := range threads {
					_ = store.UpsertRedditThread(ctx, goatstore.RedditThread{
						ID: th.ID, Subreddit: sub,
						Title: th.Title, URL: th.URL, Permalink: th.Permalink,
						Score: th.Score, NumComments: th.NumComments,
						Body: th.Body,
					})
				}
			}
			// Apply reddit boost to existing candidates whose names appear
			// in any thread title/body.
			for _, th := range threads {
				blob := strings.ToLower(th.Title + " " + th.Body)
				for _, c := range candidates {
					if strings.Contains(blob, strings.ToLower(c.Name)) ||
						(c.NameLocal != "" && strings.Contains(blob, strings.ToLower(c.NameLocal))) {
						c.Sources = appendUnique(c.Sources, "reddit")
						c.Score *= 1.1
					}
				}
			}
		}
	}

	// --- Final ranking ---
	picks := make([]Pick, 0, len(candidates))
	for _, c := range candidates {
		picks = append(picks, *c)
	}
	sort.Slice(picks, func(i, j int) bool {
		if math.Abs(picks[i].Score-picks[j].Score) > 1e-6 {
			return picks[i].Score > picks[j].Score
		}
		return picks[i].WalkingMin < picks[j].WalkingMin
	})
	if len(picks) > 5 {
		picks = picks[:5]
	}
	out.Picks = picks
	return out
}

// walkingMinutesToMeters converts walking-minute budget to a meter radius
// using 4.5 km/h average pace (75 m/min). Brief spec used minute radius;
// this is the conversion to a metric for source queries.
func walkingMinutesToMeters(min int) int {
	return int(float64(min) * 75.0)
}

func metersToWalkingMinutes(m float64) float64 {
	// 75 m/min average + 1.3 tortuosity factor for path-vs-crow-flies.
	return (m * 1.3) / 75.0
}

func haversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371000.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

func redditSubsForCountry(c string) string {
	switch c {
	case "JP":
		return "japan,JapanTravel,Tokyo,osaka,Kyoto"
	case "KR":
		return "korea,seoul,KoreaTravel"
	case "FR":
		return "Paris,France,francetravel"
	case "GB", "UK":
		return "london,unitedkingdom"
	}
	return "travel"
}

func dedupeKey(name string, lat, lng float64) string {
	h := fnv.New64a()
	h.Write([]byte(strings.ToLower(strings.TrimSpace(name))))
	return fmt.Sprintf("%x:%.4f:%.4f", h.Sum64(), lat, lng)
}

func appendUnique(s []string, more ...string) []string {
	seen := map[string]bool{}
	for _, x := range s {
		seen[x] = true
	}
	for _, x := range more {
		if !seen[x] {
			s = append(s, x)
			seen[x] = true
		}
	}
	return s
}

func buildWhyOSM(tags map[string]string) string {
	parts := []string{}
	if heritage, ok := tags["heritage"]; ok && heritage != "" {
		parts = append(parts, "heritage:"+heritage)
	}
	if hist, ok := tags["historic"]; ok && hist != "" {
		parts = append(parts, "historic:"+hist)
	}
	if cuisine, ok := tags["cuisine"]; ok && cuisine != "" {
		parts = append(parts, "cuisine:"+cuisine)
	}
	if michelin, ok := tags["michelin"]; ok && michelin != "" {
		parts = append(parts, "michelin:"+michelin)
	}
	if start, ok := tags["start_date"]; ok && start != "" {
		parts = append(parts, "since "+start)
	}
	if len(parts) == 0 {
		return "OSM tag match"
	}
	return strings.Join(parts, " · ")
}

func classifyWikiTitle(title string) sources.Intent {
	low := strings.ToLower(title)
	switch {
	case strings.Contains(low, "park") || strings.Contains(low, "garden"):
		return sources.IntentNature
	case strings.Contains(low, "temple") || strings.Contains(low, "shrine") || strings.Contains(low, "castle"):
		return sources.IntentHistoric
	case strings.Contains(low, "museum") || strings.Contains(low, "gallery"):
		return sources.IntentCulture
	case strings.Contains(low, "market") || strings.Contains(low, "shop"):
		return sources.IntentShopping
	}
	return sources.IntentCulture
}
