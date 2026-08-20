package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/roadside-america/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/roadside-america/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/roadside-america/internal/roadside"
	"github.com/mvanhorn/printing-press-library/library/travel/roadside-america/internal/store"
	"github.com/spf13/cobra"
)

// Shared scaffolding for the hand-authored roadside commands (near, state,
// show, search, sql, category, stats, random, trip, compare). All data is
// scraped/community-sourced from RoadsideAmerica.com; every record carries a
// source_url back to its page, and fetches go through the polite, rate-limited
// generated client.

const (
	politeUserAgent = "roadside-america-pp-cli/0.1.0 (user-initiated roadside-attraction lookup; polite, cached, rate-limited)"
	// detailTTL is the fresh-on-read window for cached attraction writeups.
	detailTTL = 720 * time.Hour // 30 days; attraction data changes slowly
	// htmlAccept signals the PHP/HTML surfaces we scrape, instead of the
	// generated client's default application/json Accept.
	htmlAccept = "text/html,application/xhtml+xml,*/*"
	// detailResourceType caches full writeups separately from list rows.
	detailResourceType = "detail"
)

// storeHandle aliases the SQLite store type so the hand-authored command files
// can reference it without each importing the store package.
type storeHandle = store.Store

func htmlHeaders() map[string]string { return map[string]string{"Accept": htmlAccept} }

func roadsideDBPath() string { return defaultDBPath("roadside-america-pp-cli") }

func openRoadsideStore(ctx context.Context) (*store.Store, error) {
	s, err := store.OpenWithContext(ctx, roadsideDBPath())
	if err != nil {
		return nil, fmt.Errorf("opening local cache: %w", err)
	}
	return s, nil
}

// openRoadsideStoreReadOnly opens the cache in driver-level read-only mode
// (mode=ro), so a malformed read-only query can never mutate the local cache
// even if the SQL guard misses an edge case. Friendly error when no cache yet.
func openRoadsideStoreReadOnly() (*store.Store, error) {
	p := roadsideDBPath()
	if _, err := os.Stat(p); err != nil {
		return nil, notFoundErr(fmt.Errorf("no local cache yet at %s; run 'sync', 'state', or 'near' first", p))
	}
	s, err := store.OpenReadOnly(p)
	if err != nil {
		return nil, fmt.Errorf("opening local cache (read-only): %w", err)
	}
	return s, nil
}

// --- fetch + parse (live) ---

func fetchStateAttractions(ctx context.Context, c *client.Client, state string) ([]roadside.Attraction, error) {
	raw, err := c.GetWithHeaders(ctx, "/map/attractionsByState.php", map[string]string{"state": state}, htmlHeaders())
	if err != nil {
		return nil, err
	}
	return roadside.ParseAttrList(string(raw)), nil
}

func fetchNearbyAttractions(ctx context.Context, c *client.Client, lat, lng, delta float64) ([]roadside.Attraction, error) {
	params := map[string]string{
		"long":  roadside.FormatFloat(lng),
		"lat":   roadside.FormatFloat(lat),
		"delta": roadside.FormatFloat(delta),
		"id":    "0",
	}
	raw, err := c.GetWithHeaders(ctx, "/map/nearbyAttractions.php", params, htmlHeaders())
	if err != nil {
		return nil, err
	}
	return roadside.ParseAttrList(string(raw)), nil
}

// fetchDetail fetches /tip/<id>, falling back to /story/<id> on a 404 or an
// empty parse (the two detail surfaces RoadsideAmerica.com uses).
func fetchDetail(ctx context.Context, c *client.Client, id string) (roadside.Detail, error) {
	var lastErr error
	for _, base := range []string{"/tip/", "/story/"} {
		path := base + id
		raw, err := c.GetWithHeaders(ctx, path, nil, htmlHeaders())
		if err != nil {
			var apiErr *client.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
				lastErr = err
				continue
			}
			return roadside.Detail{}, err
		}
		d := roadside.ParseDetail(id, path, string(raw))
		if d.Name != "" {
			return d, nil
		}
		lastErr = fmt.Errorf("attraction %s returned no parseable detail", id)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("attraction %s not found", id)
	}
	return roadside.Detail{}, notFoundErr(lastErr)
}

// --- cache (local SQLite) ---

func cacheAttractions(s *store.Store, atts []roadside.Attraction) {
	if s == nil || len(atts) == 0 {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	items := make([]json.RawMessage, 0, len(atts))
	for _, a := range atts {
		ca := a
		ca.CachedAt = now
		// Distance is query-specific; do not persist it on the cached row.
		ca.Distance = ""
		ca.DistanceMi = 0
		b, err := json.Marshal(ca)
		if err != nil {
			continue
		}
		items = append(items, b)
	}
	if _, _, err := s.UpsertBatch(roadside.ResourceType, items); err != nil {
		fmt.Fprintf(os.Stderr, "warning: caching attractions failed: %v\n", err)
	}
}

func cacheDetail(s *store.Store, d roadside.Detail) {
	if s == nil || d.ID == "" {
		return
	}
	d.CachedAt = time.Now().UTC().Format(time.RFC3339)
	if b, err := json.Marshal(d); err == nil {
		if err := s.Upsert(detailResourceType, d.ID, b); err != nil {
			fmt.Fprintf(os.Stderr, "warning: caching detail failed: %v\n", err)
		}
		// Also refresh the list-level row so search/category/stats see it.
		cacheAttractions(s, []roadside.Attraction{d.Attraction})
	}
}

func decodeAttractions(raws []json.RawMessage) []roadside.Attraction {
	out := make([]roadside.Attraction, 0, len(raws))
	for _, r := range raws {
		var a roadside.Attraction
		if json.Unmarshal(r, &a) == nil && a.ID != "" {
			out = append(out, a)
		}
	}
	return out
}

func loadCachedAttractions(s *store.Store, limit int) ([]roadside.Attraction, error) {
	if limit <= 0 {
		limit = 100000
	}
	raws, err := s.List(roadside.ResourceType, limit)
	if err != nil {
		return nil, err
	}
	return decodeAttractions(raws), nil
}

// getCachedDetail returns a cached writeup and whether it is fresh per detailTTL.
func getCachedDetail(s *store.Store, id string) (roadside.Detail, bool, bool) {
	raw, err := s.Get(detailResourceType, id)
	if err != nil {
		return roadside.Detail{}, false, false
	}
	var d roadside.Detail
	if json.Unmarshal(raw, &d) != nil || d.Name == "" {
		return roadside.Detail{}, false, false
	}
	fresh := false
	if t := cliutil.ParseStoredTime(d.CachedAt); !t.IsZero() {
		fresh = time.Since(t) < detailTTL
	}
	return d, true, fresh
}

// --- geocoding (place -> lat,lng), cache-first ---

func geocodeWithCache(ctx context.Context, s *store.Store, place string, timeout time.Duration) (roadside.GeoResult, error) {
	key := strings.ToLower(strings.TrimSpace(place))
	if s != nil {
		if raw, err := s.Get(roadside.GeocodeResourceType, key); err == nil {
			var gr roadside.GeoResult
			if json.Unmarshal(raw, &gr) == nil && (gr.Lat != 0 || gr.Lng != 0) {
				return gr, nil
			}
		}
	}
	g := roadside.NewGeocoder(politeUserAgent, 1.0, timeout)
	gr, err := g.Geocode(ctx, place)
	if err != nil {
		return roadside.GeoResult{}, err
	}
	gr.CachedAt = time.Now().UTC().Format(time.RFC3339)
	if s != nil {
		if b, err := json.Marshal(gr); err == nil {
			_ = s.Upsert(roadside.GeocodeResourceType, key, b)
		}
	}
	return gr, nil
}

var latLngRe = regexp.MustCompile(`^\s*(-?\d+(?:\.\d+)?)\s*,\s*(-?\d+(?:\.\d+)?)\s*$`)

// parseLatLng parses "lat,lng" input. Returns ok=false when the string is not
// a coordinate pair (so the caller falls back to geocoding a place name).
func parseLatLng(s string) (lat, lng float64, ok bool) {
	m := latLngRe.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, false
	}
	lat, _ = strconv.ParseFloat(m[1], 64)
	lng, _ = strconv.ParseFloat(m[2], 64)
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return 0, 0, false
	}
	return lat, lng, true
}

// resolveLocation turns a place-or-latlng string into coordinates, geocoding
// place names through the cache. label describes the resolved point.
func resolveLocation(ctx context.Context, s *store.Store, input string, timeout time.Duration) (lat, lng float64, label string, err error) {
	if lt, lg, ok := parseLatLng(input); ok {
		return lt, lg, fmt.Sprintf("%s,%s", roadside.FormatFloat(lt), roadside.FormatFloat(lg)), nil
	}
	gr, gerr := geocodeWithCache(ctx, s, input, timeout)
	if gerr != nil {
		return 0, 0, "", gerr
	}
	label = gr.Display
	if label == "" {
		label = input
	}
	return gr.Lat, gr.Lng, label, nil
}

// --- output ---

type attractionListView struct {
	Source      string                `json:"source"`
	Query       map[string]any        `json:"query,omitempty"`
	Count       int                   `json:"count"`
	Attractions []roadside.Attraction `json:"attractions"`
	Note        string                `json:"note,omitempty"`
}

// machineOutput reports whether output should be machine JSON/CSV rather than
// the human table.
func machineOutput(cmd *cobra.Command, flags *rootFlags) bool {
	w := cmd.OutOrStdout()
	return flags.asJSON || flags.csv || flags.quiet || flags.compact ||
		flags.selectFields != "" || (!isTerminal(w) && !humanFriendly)
}

func emitAttractions(cmd *cobra.Command, flags *rootFlags, view attractionListView) error {
	if view.Attractions == nil {
		view.Attractions = []roadside.Attraction{}
	}
	view.Count = len(view.Attractions)
	if view.Source == "" {
		view.Source = roadside.SourceLabel
	}
	if machineOutput(cmd, flags) {
		return flags.printJSON(cmd, view)
	}
	w := cmd.OutOrStdout()
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "NAME\tLOCATION\tDIST\tSOURCE")
	for _, a := range view.Attractions {
		loc := a.City
		if a.State != "" {
			if loc != "" {
				loc += ", " + a.State
			} else {
				loc = a.State
			}
		}
		dist := a.Distance
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", truncate(a.Name, 48), loc, dist, a.SourceURL)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if view.Note != "" {
		fmt.Fprintf(w, "\n%s\n", view.Note)
	}
	fmt.Fprintf(w, "\n%d result(s) — %s\n", view.Count, roadside.SourceLabel)
	return nil
}

func emitDetail(cmd *cobra.Command, flags *rootFlags, d roadside.Detail) error {
	if machineOutput(cmd, flags) {
		return flags.printJSON(cmd, d)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s\n", bold(d.Name))
	loc := d.City
	if d.State != "" {
		if loc != "" {
			loc += ", " + d.State
		} else {
			loc = d.State
		}
	}
	if loc != "" {
		fmt.Fprintf(w, "%s\n", loc)
	}
	if d.Street != "" {
		fmt.Fprintf(w, "Address: %s\n", d.Street)
	}
	if d.Categories != nil && len(d.Categories) > 0 {
		fmt.Fprintf(w, "Categories: %s\n", strings.Join(d.Categories, ", "))
	}
	if d.Writeup != "" {
		fmt.Fprintf(w, "\n%s\n", d.Writeup)
	} else if d.Summary != "" {
		fmt.Fprintf(w, "\n%s\n", d.Summary)
	}
	if d.Directions != "" {
		fmt.Fprintf(w, "\nDirections: %s\n", d.Directions)
	}
	fmt.Fprintf(w, "\nSource: %s\n", d.SourceURL)
	fmt.Fprintf(w, "(%s)\n", roadside.SourceLabel)
	return nil
}

// dogfoodScanCap lowers multi-request work under live dogfood so commands fit
// the matrix's per-command timeout.
func dogfoodScanCap(n int) int {
	if cliutil.IsDogfoodEnv() && n > 1 {
		return 1
	}
	return n
}
