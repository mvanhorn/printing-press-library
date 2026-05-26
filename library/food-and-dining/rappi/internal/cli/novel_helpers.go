// Copyright 2026 bobeglz. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/rappi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/rappi/internal/source/rappi"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/rappi/internal/store"
)

var rappiPathSlugPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

type rappiHTMLFetcher interface {
	FetchHTML(context.Context, string) ([]byte, error)
}

type synchronizedRappiFetcher struct {
	mu     sync.Mutex
	client *rappi.Client
}

// PATCH: Share one configured Rappi client per command invocation.
func newRappiHTMLFetcher(flags *rootFlags) *synchronizedRappiFetcher {
	c := rappi.NewClient()
	if flags != nil {
		if flags.timeout > 0 {
			if c.HTTPClient == nil {
				c.HTTPClient = &http.Client{}
			}
			c.HTTPClient.Timeout = flags.timeout
		}
		c.Limiter = cliutil.NewAdaptiveLimiter(flags.rateLimit)
	}
	return &synchronizedRappiFetcher{client: c}
}

func (f *synchronizedRappiFetcher) FetchHTML(ctx context.Context, path string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.client.FetchHTML(ctx, path)
}

// fetchRestaurantListPage retrieves and parses a single restaurants list
// page for a (city[, category]) selector. Used by every novel command
// that ranks/filters/joins across multiple restaurants in a city.
func fetchRestaurantListPage(ctx context.Context, client rappiHTMLFetcher, city, category string) ([]rappi.RestaurantListItem, error) {
	// PATCH: Validate Rappi path slugs before building fetch URLs.
	if err := validateRappiPathSlug("city", city); err != nil {
		return nil, err
	}
	city = strings.TrimSpace(city)
	if category != "" {
		if err := validateRappiPathSlug("category", category); err != nil {
			return nil, err
		}
		category = strings.TrimSpace(category)
	}
	var path string
	if category == "" {
		path = "/" + city + "/restaurantes"
	} else {
		path = "/" + city + "/restaurantes/category/" + category
	}
	html, err := client.FetchHTML(ctx, path)
	if err != nil {
		return nil, err
	}
	return rappi.ParseRestaurantList(html, city, category), nil
}

// fetchRestaurantDetail retrieves a restaurant detail page and parses
// the Restaurant JSON-LD block.
func fetchRestaurantDetail(ctx context.Context, client rappiHTMLFetcher, idSlug, city, category string) (*rappi.Restaurant, error) {
	if err := validateRappiPathSlug("restaurant id slug", idSlug); err != nil {
		return nil, err
	}
	idSlug = strings.TrimSpace(idSlug)
	html, err := client.FetchHTML(ctx, "/restaurantes/"+idSlug)
	if err != nil {
		return nil, err
	}
	r := rappi.ParseRestaurant(html)
	if r == nil {
		return nil, fmt.Errorf("no Restaurant block found at /restaurantes/%s", idSlug)
	}
	r.City = city
	r.Category = category
	return r, nil
}

// fetchStoreListPage retrieves and parses a single store-by-type list page.
func fetchStoreListPage(ctx context.Context, client rappiHTMLFetcher, storeType string) ([]rappi.Store, error) {
	if err := validateRappiPathSlug("store type", storeType); err != nil {
		return nil, err
	}
	storeType = strings.TrimSpace(storeType)
	html, err := client.FetchHTML(ctx, "/tiendas/tipo/"+storeType)
	if err != nil {
		return nil, err
	}
	return rappi.ParseStoreList(html, storeType, ""), nil
}

// fetchStoreDetail retrieves a store detail page and parses its Store JSON-LD block.
func fetchStoreDetail(ctx context.Context, client rappiHTMLFetcher, idSlug, storeType, city string) (*rappi.Store, error) {
	// PATCH: Store adjacency uses detail-page geo instead of centroid placeholders.
	if err := validateRappiPathSlug("store id slug", idSlug); err != nil {
		return nil, err
	}
	idSlug = strings.TrimSpace(idSlug)
	html, err := client.FetchHTML(ctx, "/tiendas/"+idSlug)
	if err != nil {
		return nil, err
	}
	s := rappi.ParseStore(html)
	if s == nil {
		return nil, fmt.Errorf("no Store block found at /tiendas/%s", idSlug)
	}
	s.StoreType = storeType
	s.City = city
	return s, nil
}

func validateRappiPathSlug(name, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s cannot be empty", name)
	}
	if !rappiPathSlugPattern.MatchString(value) {
		return fmt.Errorf("%s must contain only lowercase letters, numbers, and hyphens", name)
	}
	return nil
}

func idSlugFromURL(url string) string {
	parts := strings.Split(strings.TrimRight(url, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// snapshotStore writes a list of restaurants under a snapshot key that
// includes the timestamp + selector — used by `restaurants diff` to
// reconstruct old snapshots when comparing.
func snapshotRestaurants(db *store.Store, city, category string, rows []rappi.RestaurantListItem) error {
	now := time.Now().UTC().Format(time.RFC3339)
	rt := "restaurant_snapshot"
	id := fmt.Sprintf("%s/%s/%s", city, category, now)
	payload := map[string]any{
		"city":     city,
		"category": category,
		"taken_at": now,
		"items":    rows,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return db.Upsert(rt, id, raw)
}

// snapshotStores writes a list of stores keyed by (city/store_type/timestamp).
func snapshotStores(db *store.Store, storeType, city string, rows []rappi.Store) error {
	now := time.Now().UTC().Format(time.RFC3339)
	rt := "store_snapshot"
	id := fmt.Sprintf("%s/%s/%s", storeType, city, now)
	payload := map[string]any{
		"store_type": storeType,
		"city":       city,
		"taken_at":   now,
		"items":      rows,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return db.Upsert(rt, id, raw)
}

// openLocalStore opens the default local SQLite database under the
// rappi-pp-cli config dir. Returns the store handle plus its on-disk
// path so callers can include it in diagnostics.
func openLocalStore(ctx context.Context) (*store.Store, string, error) {
	dbPath := defaultDBPath("rappi-pp-cli")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, "", err
	}
	return db, dbPath, nil
}

// haversineKm returns the great-circle distance in kilometers between
// two lat/lng points. Inputs are degrees.
func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const earthR = 6371.0
	rad := math.Pi / 180.0
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthR * c
}

// resolveCityLatLng returns the centroid lat/lng for a known city slug,
// or falls back to CDMX zócalo when the slug is unknown.
func resolveCityLatLng(slug string) (float64, float64) {
	if c := rappi.CityBySlug(slug); c != nil {
		return c.Latitude, c.Longitude
	}
	return 19.4326, -99.1332
}

// emitNoveJSON writes v as JSON via the shared --json/--select/--compact
// pipeline. Suppresses progress messages when --json/--quiet are set.
func emitNovelJSON(out interface {
	Write([]byte) (int, error)
}, v any, flags *rootFlags) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return printOutputWithFlags(out, json.RawMessage(raw), flags)
}

// listRestaurantSnapshots returns every restaurant_snapshot id stored
// under (city, category) in chronological order (oldest first).
func listRestaurantSnapshots(db *store.Store, city, category string) ([]string, error) {
	ids, err := db.ListIDs("restaurant_snapshot")
	if err != nil {
		return nil, err
	}
	prefix := city + "/" + category + "/"
	matches := []string{}
	for _, id := range ids {
		if strings.HasPrefix(id, prefix) {
			matches = append(matches, id)
		}
	}
	sort.Strings(matches)
	return matches, nil
}

// listStoreSnapshots returns every store_snapshot id under store_type.
func listStoreSnapshots(db *store.Store, storeType string) ([]string, error) {
	ids, err := db.ListIDs("store_snapshot")
	if err != nil {
		return nil, err
	}
	prefix := storeType + "/"
	matches := []string{}
	for _, id := range ids {
		if strings.HasPrefix(id, prefix) {
			matches = append(matches, id)
		}
	}
	sort.Strings(matches)
	return matches, nil
}

// loadRestaurantSnapshot reads a stored snapshot by id and returns the
// snapshot's items.
func loadRestaurantSnapshot(db *store.Store, id string) ([]rappi.RestaurantListItem, time.Time, error) {
	raw, err := db.Get("restaurant_snapshot", id)
	if err != nil {
		return nil, time.Time{}, err
	}
	var payload struct {
		TakenAt string                     `json:"taken_at"`
		Items   []rappi.RestaurantListItem `json:"items"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, time.Time{}, err
	}
	t, _ := time.Parse(time.RFC3339, payload.TakenAt)
	return payload.Items, t, nil
}

// loadStoreSnapshot reads a store snapshot by id.
func loadStoreSnapshot(db *store.Store, id string) ([]rappi.Store, time.Time, error) {
	raw, err := db.Get("store_snapshot", id)
	if err != nil {
		return nil, time.Time{}, err
	}
	var payload struct {
		TakenAt string        `json:"taken_at"`
		Items   []rappi.Store `json:"items"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, time.Time{}, err
	}
	t, _ := time.Parse(time.RFC3339, payload.TakenAt)
	return payload.Items, t, nil
}

// stderrf is a thin wrapper for progress messages that should never
// pollute --json/--quiet output (they only go to stderr).
func stderrf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
}
