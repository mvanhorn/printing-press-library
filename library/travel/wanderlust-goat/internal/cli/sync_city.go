package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/atlasobscura"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/eater"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/goatstore"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/overpass"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/reddit"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sources"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/wikipedia"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/wikivoyage"
)

// city is a small known-cities table v1 ships with. Users with offline
// caches use the slug; users with live mode geocode the slug to a center
// at runtime. The table prevents the slug→country lookup from itself
// requiring a Nominatim call.
type cityRecord struct {
	Slug    string
	Name    string
	Country sources.Country
	Lat     float64
	Lng     float64
}

var knownCities = []cityRecord{
	{Slug: "tokyo", Name: "Tokyo", Country: sources.CountryJapan, Lat: 35.6895, Lng: 139.6917},
	{Slug: "kyoto", Name: "Kyoto", Country: sources.CountryJapan, Lat: 35.0116, Lng: 135.7681},
	{Slug: "osaka", Name: "Osaka", Country: sources.CountryJapan, Lat: 34.6937, Lng: 135.5023},
	{Slug: "seoul", Name: "Seoul", Country: sources.CountryKorea, Lat: 37.5665, Lng: 126.9780},
	{Slug: "busan", Name: "Busan", Country: sources.CountryKorea, Lat: 35.1796, Lng: 129.0756},
	{Slug: "paris", Name: "Paris", Country: sources.CountryFrance, Lat: 48.8566, Lng: 2.3522},
	{Slug: "lyon", Name: "Lyon", Country: sources.CountryFrance, Lat: 45.7640, Lng: 4.8357},
	{Slug: "marseille", Name: "Marseille", Country: sources.CountryFrance, Lat: 43.2965, Lng: 5.3698},
}

func lookupCity(slug string) *cityRecord {
	slug = strings.ToLower(strings.TrimSpace(slug))
	for i := range knownCities {
		if knownCities[i].Slug == slug {
			return &knownCities[i]
		}
	}
	return nil
}

func newSyncCityCmd(flags *rootFlags) *cobra.Command {
	var (
		layers      string
		concurrency int
		sinceText   string
	)
	cmd := &cobra.Command{
		Use:   "sync-city <slug>",
		Short: "Pre-cache editorial best-of, Reddit threads, Wikipedia, Wikivoyage, OSM POIs, Atlas Obscura, and regional-language sources for offline use.",
		Long: `Pull every layer for a city into the local SQLite store. Run this two
weeks before a trip; afterwards you can use --data-source local to query
without internet. Layers covered: editorial (Eater best-of), Atlas Obscura
city listings, OSM Overpass POIs (heritage, michelin, viewpoint, historic),
Wikipedia GeoSearch (locale-aware), Wikivoyage, /r/<city> top threads.

Known city slugs (v1): tokyo, kyoto, osaka, seoul, busan, paris, lyon,
marseille. Other slugs fall through to a Nominatim geocode of the slug.`,
		Example: strings.Trim(`
  # Heavy pre-trip sync of Tokyo
  wanderlust-goat-pp-cli sync-city tokyo --layers all --concurrency 2

  # Refresh only Reddit threads from the last 7 days
  wanderlust-goat-pp-cli sync-city paris --layers reddit --since 7d

  # Custom slug — geocode the term and treat as the city center
  wanderlust-goat-pp-cli sync-city "Mexico City"`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		Args:        cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			slug := strings.Join(args, " ")
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			city, err := resolveCity(ctx, slug)
			if err != nil {
				return err
			}
			store, err := openGoatStore(cmd, flags)
			if err != nil {
				return err
			}
			defer store.Close()
			summary := runSync(ctx, store, city, layers)
			return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
		},
	}
	cmd.Flags().StringVar(&layers, "layers", "all", "Comma-separated layers to sync: all,osm,wikipedia,wikivoyage,reddit,atlasobscura,eater (default \"all\").")
	cmd.Flags().IntVar(&concurrency, "concurrency", 2, "Concurrent fetches; keep low for polite rate limiting (default 2).")
	cmd.Flags().StringVar(&sinceText, "since", "", "Skip rows synced more recently than this (e.g. 7d, 24h, 1w). Empty means full re-sync.")
	return cmd
}

type syncSummary struct {
	City    string                  `json:"city"`
	Country string                  `json:"country"`
	Lat     float64                 `json:"lat"`
	Lng     float64                 `json:"lng"`
	Layers  []string                `json:"layers"`
	Stored  map[string]int          `json:"stored_per_source"`
	Errors  map[string]string       `json:"errors,omitempty"`
	Started time.Time               `json:"started_at"`
	Took    string                  `json:"took"`
}

func resolveCity(ctx context.Context, slug string) (cityRecord, error) {
	if c := lookupCity(slug); c != nil {
		return *c, nil
	}
	// Fall through to Nominatim
	res, err := resolveAnchor(ctx, slug)
	if err != nil {
		return cityRecord{}, fmt.Errorf("unknown city slug %q and geocode failed: %w", slug, err)
	}
	return cityRecord{
		Slug: strings.ToLower(strings.ReplaceAll(slug, " ", "-")),
		Name: slug, Country: sources.Country(res.Country),
		Lat: res.Lat, Lng: res.Lng,
	}, nil
}

func runSync(ctx context.Context, store *goatstore.Store, city cityRecord, layersCSV string) syncSummary {
	wantLayer := func(name string) bool {
		if layersCSV == "" || strings.EqualFold(layersCSV, "all") {
			return true
		}
		for _, p := range strings.Split(layersCSV, ",") {
			if strings.EqualFold(strings.TrimSpace(p), name) {
				return true
			}
		}
		return false
	}
	out := syncSummary{
		City: city.Name, Country: string(city.Country), Lat: city.Lat, Lng: city.Lng,
		Stored: map[string]int{}, Errors: map[string]string{}, Started: time.Now(),
	}
	radiusMeters := 8000 // 8km city-center radius for sync

	// OSM Overpass — heritage, michelin, historic, viewpoint, cafe.
	if wantLayer("osm") {
		op := overpass.New(nil, userAgent())
		filters := []overpass.TagFilter{
			{Key: "heritage", Value: ""},
			{Key: "michelin", Value: ""},
			{Key: "historic", Value: ""},
			{Key: "tourism", Value: "viewpoint"},
			{Key: "amenity", Value: "cafe"},
		}
		resp, err := op.NearbyByTags(ctx, city.Lat, city.Lng, radiusMeters, filters)
		if err != nil {
			out.Errors["overpass"] = err.Error()
		} else if resp != nil {
			n := 0
			for _, el := range resp.Elements {
				lat, lng := el.Lat, el.Lon
				if el.Center != nil {
					lat = el.Center.Lat
					lng = el.Center.Lon
				}
				name := el.Tags["name"]
				if name == "" {
					continue
				}
				_ = store.UpsertPlace(ctx, goatstore.Place{
					ID: fmt.Sprintf("overpass:%d", el.ID),
					Source: "overpass", Intent: classifyOverpassIntent(el.Tags),
					Name: name, NameLocal: el.Tags["name:"+localeForCountry(city.Country)],
					Lat: lat, Lng: lng,
					Country: string(city.Country), CitySlug: city.Slug,
					Trust: 0.90, WhySpecial: buildWhyOSM(el.Tags),
					Address: el.Tags["addr:full"],
				})
				n++
			}
			out.Stored["overpass"] = n
			_ = store.SaveSync(ctx, "overpass", city.Slug, "", n)
			out.Layers = append(out.Layers, "osm")
		}
	}

	// Wikipedia GeoSearch (multi-locale).
	if wantLayer("wikipedia") {
		for _, loc := range localesForCountry(city.Country) {
			wp := wikipedia.New(loc, nil, userAgent())
			resp, err := wp.GeoSearch(ctx, city.Lat, city.Lng, radiusMeters, 30)
			if err != nil {
				out.Errors["wikipedia."+loc] = err.Error()
				continue
			}
			n := 0
			for _, p := range resp.Pages {
				_ = store.UpsertPlace(ctx, goatstore.Place{
					ID: fmt.Sprintf("wikipedia.%s:%d", loc, p.PageID),
					Source: "wikipedia." + loc, Intent: string(classifyWikiTitle(p.Title)),
					Name: p.Title, Lat: p.Lat, Lng: p.Lon,
					Country: string(city.Country), CitySlug: city.Slug,
					Trust: 0.95, WhySpecial: "Wikipedia-notable (" + loc + ")",
				})
				n++
			}
			out.Stored["wikipedia."+loc] = n
			_ = store.SaveSync(ctx, "wikipedia."+loc, city.Slug, "", n)
			out.Layers = append(out.Layers, "wikipedia."+loc)
		}
	}

	// Wikivoyage prose (en + locale).
	if wantLayer("wikivoyage") {
		for _, loc := range []string{"en", localeForCountry(city.Country)} {
			if loc == "" {
				continue
			}
			wv := wikivoyage.New(loc, nil, userAgent())
			summary, err := wv.PageSummary(ctx, city.Name)
			if err != nil {
				out.Errors["wikivoyage."+loc] = err.Error()
				continue
			}
			if summary != nil && summary.Title != "" {
				out.Stored["wikivoyage."+loc] = 1
				_ = store.SaveSync(ctx, "wikivoyage."+loc, city.Slug, "", 1)
				out.Layers = append(out.Layers, "wikivoyage."+loc)
			}
		}
	}

	// Reddit (city subs + travel sub).
	if wantLayer("reddit") {
		r := reddit.New(nil, userAgent())
		subs := strings.Split(redditSubsForCountry(string(city.Country)), ",")
		for _, sub := range subs {
			sub = strings.TrimSpace(sub)
			if sub == "" {
				continue
			}
			threads, err := r.Search(ctx, sub, "best "+city.Name, reddit.SearchOpts{
				MinScore: 10, MinComments: 3, Limit: 25,
			})
			if err != nil {
				out.Errors["reddit:r/"+sub] = err.Error()
				continue
			}
			for _, th := range threads {
				_ = store.UpsertRedditThread(ctx, goatstore.RedditThread{
					ID: th.ID, Subreddit: sub,
					Title: th.Title, URL: th.URL, Permalink: th.Permalink,
					Score: th.Score, NumComments: th.NumComments, Body: th.Body,
					CitySlug: city.Slug,
				})
			}
			out.Stored["reddit:r/"+sub] = len(threads)
			_ = store.SaveSync(ctx, "reddit:r/"+sub, city.Slug, "", len(threads))
			out.Layers = append(out.Layers, "reddit:r/"+sub)
		}
	}

	// Atlas Obscura — by city slug.
	if wantLayer("atlasobscura") {
		ao := atlasobscura.New(nil, userAgent())
		entries, err := ao.City(ctx, city.Slug)
		if err != nil {
			out.Errors["atlasobscura"] = err.Error()
		} else {
			for i, e := range entries {
				_ = store.UpsertPlace(ctx, goatstore.Place{
					ID: fmt.Sprintf("atlasobscura:%s:%d", city.Slug, i),
					Source: "atlasobscura", Intent: string(sources.IntentCulture),
					Name: e.Title, Lat: e.Lat, Lng: e.Lng,
					Country: string(city.Country), CitySlug: city.Slug,
					Trust: 0.80, WhySpecial: e.Description,
				})
			}
			out.Stored["atlasobscura"] = len(entries)
			_ = store.SaveSync(ctx, "atlasobscura", city.Slug, "", len(entries))
			out.Layers = append(out.Layers, "atlasobscura")
		}
	}

	// Eater best-of — only if the user passes a URL via env hint.
	if wantLayer("eater") {
		if eaterURL := strings.TrimSpace(envOr("WANDERLUST_GOAT_EATER_URL_"+strings.ToUpper(city.Slug), "")); eaterURL != "" {
			et := eater.New(nil, userAgent())
			items, err := et.BestOf(ctx, eaterURL)
			if err != nil {
				out.Errors["eater"] = err.Error()
			} else {
				for i, e := range items {
					_ = store.UpsertPlace(ctx, goatstore.Place{
						ID: fmt.Sprintf("eater:%s:%d", city.Slug, i),
						Source: "eater", Intent: string(sources.IntentFood),
						Name: e.Name, Lat: e.Lat, Lng: e.Lng,
						Country: string(city.Country), CitySlug: city.Slug,
						Trust: 0.90, WhySpecial: e.Description,
						Address: e.Address,
					})
				}
				out.Stored["eater"] = len(items)
				_ = store.SaveSync(ctx, "eater", city.Slug, "", len(items))
				out.Layers = append(out.Layers, "eater")
			}
		}
	}

	out.Took = time.Since(out.Started).Round(time.Millisecond).String()
	return out
}

func classifyOverpassIntent(tags map[string]string) string {
	if v := tags["amenity"]; v == "cafe" {
		return string(sources.IntentCoffee)
	}
	if v := tags["amenity"]; v == "restaurant" {
		return string(sources.IntentFood)
	}
	if v := tags["amenity"]; v == "bar" {
		return string(sources.IntentDrinks)
	}
	if v := tags["tourism"]; v == "viewpoint" {
		return string(sources.IntentViewpoint)
	}
	if _, ok := tags["historic"]; ok {
		return string(sources.IntentHistoric)
	}
	return string(sources.IntentCulture)
}

func localeForCountry(c sources.Country) string {
	switch c {
	case sources.CountryJapan:
		return "ja"
	case sources.CountryKorea:
		return "ko"
	case sources.CountryFrance:
		return "fr"
	}
	return "en"
}

func localesForCountry(c sources.Country) []string {
	if c == sources.CountryUniversal || c == "" {
		return []string{"en"}
	}
	return []string{"en", localeForCountry(c)}
}

func envOr(key, fallback string) string {
	v := osLookupEnv(key)
	if v == "" {
		return fallback
	}
	return v
}
