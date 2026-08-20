// LoopNet data layer and shared command plumbing. Hand-written (not
// generated): the generic generated store keeps only a latest-snapshot
// resources table, so the time-series intelligence commands (price-cuts,
// dom, velocity, delisted) need an append-only observations table. This
// file owns that table plus the fetch/parse/sync helpers every LoopNet
// command shares.
package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/loopnet/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/loopnet/internal/loopnet"
	"github.com/mvanhorn/printing-press-library/library/commerce/loopnet/internal/store"
)

const (
	lnResourceListing  = "loopnet_listing"
	lnResourceProperty = "loopnet_property"
)

// lnObservation is one (listing, sync-time) price snapshot — the append-only
// row that turns repeated syncs into a time series.
type lnObservation struct {
	ListingID    string
	Location     string
	PropertyType string
	ListingType  string
	Price        float64
	HasPrice     bool
	ObservedAt   time.Time
}

// ensureLNSchema creates the append-only observations table. Kept out of the
// generated store package (which carries a DO NOT EDIT header) so a future
// regen cannot clobber it; idempotent so every command can call it on open.
func ensureLNSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS ln_observations (
			listing_id    TEXT NOT NULL,
			location      TEXT NOT NULL,
			property_type TEXT NOT NULL,
			listing_type  TEXT NOT NULL,
			price         REAL,
			observed_at   DATETIME NOT NULL,
			PRIMARY KEY (listing_id, observed_at)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ln_obs_market ON ln_observations(location, property_type, listing_type)`,
		`CREATE INDEX IF NOT EXISTS idx_ln_obs_listing ON ln_observations(listing_id, observed_at)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("creating ln_observations: %w", err)
		}
	}
	return nil
}

// lnOpenStore opens the local SQLite store and ensures the LoopNet schema.
func lnOpenStore(flags *rootFlags) (*store.Store, error) {
	st, err := store.Open(defaultDBPath("loopnet-pp-cli"))
	if err != nil {
		return nil, configErr(fmt.Errorf("opening local store: %w", err))
	}
	if err := ensureLNSchema(st.DB()); err != nil {
		st.Close()
		return nil, configErr(err)
	}
	return st, nil
}

// lnOpenStoreRO opens the local store read-only, for query-only commands.
// mode=ro enforces read-only at the SQLite driver level — a guarantee no
// query-text screen can make. The store file is created by 'sync'; if it
// does not exist yet, callers get a clear "run sync first" error.
func lnOpenStoreRO() (*store.Store, error) {
	path := defaultDBPath("loopnet-pp-cli")
	if _, err := os.Stat(path); err != nil {
		return nil, notFoundErr(fmt.Errorf(
			"no local store yet — run 'loopnet-pp-cli sync <location>' first"))
	}
	st, err := store.OpenReadOnly(path)
	if err != nil {
		return nil, configErr(fmt.Errorf("opening local store: %w", err))
	}
	return st, nil
}

// recordObservations appends one observation row per listing for this sync.
func recordObservations(db *sql.DB, obs []lnObservation) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, o := range obs {
		var price any
		if o.HasPrice && o.Price > 0 {
			price = o.Price
		}
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO ln_observations
			 (listing_id, location, property_type, listing_type, price, observed_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			o.ListingID, o.Location, o.PropertyType, o.ListingType, price, o.ObservedAt,
		); err != nil {
			return fmt.Errorf("recording observation %s: %w", o.ListingID, err)
		}
	}
	return tx.Commit()
}

// loadObservations returns every observation for a location, optionally
// narrowed to a property type and/or listing type ("" = any). Rows are
// ordered by listing then observation time so callers can walk per-listing
// history in one pass.
func loadObservations(db *sql.DB, location, propertyType, listingType string) ([]lnObservation, error) {
	q := `SELECT listing_id, location, property_type, listing_type, price, observed_at
	      FROM ln_observations WHERE location = ?`
	args := []any{loopnet.SlugLocation(location)}
	if propertyType != "" {
		q += ` AND property_type = ?`
		args = append(args, loopnet.NormalizeType(propertyType))
	}
	if listingType != "" {
		q += ` AND listing_type = ?`
		args = append(args, loopnet.NormalizeListingType(listingType))
	}
	q += ` ORDER BY listing_id, observed_at`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []lnObservation
	for rows.Next() {
		var o lnObservation
		var price sql.NullFloat64
		if err := rows.Scan(&o.ListingID, &o.Location, &o.PropertyType, &o.ListingType, &price, &o.ObservedAt); err != nil {
			continue
		}
		o.Price = price.Float64
		o.HasPrice = price.Valid
		out = append(out, o)
	}
	return out, rows.Err()
}

// lnSyncTimes returns the distinct sync timestamps recorded for a market,
// most recent first.
func lnSyncTimes(db *sql.DB, location, propertyType, listingType string) ([]time.Time, error) {
	obs, err := loadObservations(db, location, propertyType, listingType)
	if err != nil {
		return nil, err
	}
	seen := map[int64]time.Time{}
	for _, o := range obs {
		// Bucket by second so a single sync's rows collapse to one stamp.
		key := o.ObservedAt.Unix()
		seen[key] = o.ObservedAt
	}
	var times []time.Time
	for _, t := range seen {
		times = append(times, t)
	}
	sort.Slice(times, func(i, j int) bool { return times[i].After(times[j]) })
	return times, nil
}

// lnSaveListing upserts a search-grain listing into the generic store.
func lnSaveListing(st *store.Store, l loopnet.Listing) error {
	if l.ID == "" {
		return nil
	}
	data, err := json.Marshal(l)
	if err != nil {
		return err
	}
	return st.Upsert(lnResourceListing, l.ID, data)
}

// lnSaveProperty upserts a detail-grain property into the generic store.
func lnSaveProperty(st *store.Store, p loopnet.Property) error {
	if p.ID == "" {
		return nil
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return st.Upsert(lnResourceProperty, p.ID, data)
}

// lnListings loads stored search-grain listings for a market filter.
func lnListings(st *store.Store, location, propertyType, listingType string) ([]loopnet.Listing, error) {
	rows, err := lnQueryByMarket(st, lnResourceListing, location, propertyType, listingType)
	if err != nil {
		return nil, err
	}
	var out []loopnet.Listing
	for _, raw := range rows {
		var l loopnet.Listing
		if json.Unmarshal(raw, &l) == nil {
			out = append(out, l)
		}
	}
	return out, nil
}

// lnProperties loads stored detail-grain properties for a market filter.
func lnProperties(st *store.Store, location, propertyType, listingType string) ([]loopnet.Property, error) {
	rows, err := lnQueryByMarket(st, lnResourceProperty, location, propertyType, listingType)
	if err != nil {
		return nil, err
	}
	var out []loopnet.Property
	for _, raw := range rows {
		var p loopnet.Property
		if json.Unmarshal(raw, &p) == nil {
			out = append(out, p)
		}
	}
	return out, nil
}

// lnQueryByMarket pulls resources rows whose stored JSON matches a market.
// location is required; propertyType and listingType are optional ("" = any).
func lnQueryByMarket(st *store.Store, resourceType, location, propertyType, listingType string) ([]json.RawMessage, error) {
	q := `SELECT data FROM resources WHERE resource_type = ?
	      AND json_extract(data, '$.location') = ?`
	args := []any{resourceType, loopnet.SlugLocation(location)}
	if propertyType != "" {
		q += ` AND json_extract(data, '$.property_type') = ?`
		args = append(args, loopnet.NormalizeType(propertyType))
	}
	if listingType != "" {
		q += ` AND json_extract(data, '$.listing_type') = ?`
		args = append(args, loopnet.NormalizeListingType(listingType))
	}
	rows, err := st.DB().Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			continue
		}
		out = append(out, json.RawMessage(data))
	}
	return out, rows.Err()
}

// lnGetProperty fetches one stored property by listing id.
func lnGetProperty(st *store.Store, id string) (loopnet.Property, bool) {
	raw, err := st.Get(lnResourceProperty, id)
	if err != nil {
		return loopnet.Property{}, false
	}
	var p loopnet.Property
	if json.Unmarshal(raw, &p) != nil {
		return loopnet.Property{}, false
	}
	return p, true
}

// errLNChallenge is returned when LoopNet serves an Akamai bot-challenge
// sensor page instead of real content — the clearance cookies are missing
// or expired.
var errLNChallenge = errors.New("loopnet returned an Akamai bot-challenge page")

// lnConfigDir is the directory holding the CLI's config and cookie store.
func lnConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "loopnet-pp-cli")
}

// lnCookiePath is the on-disk location of the stored clearance cookies.
func lnCookiePath() string {
	return filepath.Join(lnConfigDir(), "cookies.json")
}

type lnCookieStore struct {
	Cookie  string `json:"cookie"`
	SavedAt string `json:"saved_at"`
}

// lnLoadCookies returns the stored clearance-cookie header and when it was
// saved. An empty cookie means none has been set.
func lnLoadCookies() (string, time.Time) {
	data, err := os.ReadFile(lnCookiePath())
	if err != nil {
		return "", time.Time{}
	}
	var cs lnCookieStore
	if json.Unmarshal(data, &cs) != nil {
		return "", time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, cs.SavedAt)
	return cs.Cookie, t
}

// lnSaveCookies writes the clearance-cookie header to the cookie store
// (0600 — it is session state, not a shared secret, but still private).
func lnSaveCookies(cookie string) error {
	if err := os.MkdirAll(lnConfigDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(lnCookieStore{
		Cookie: cookie, SavedAt: time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(lnCookiePath(), data, 0o600)
}

// lnBrowserHeaders returns the headers a real Chrome top-level navigation
// sends, plus the stored clearance Cookie when one has been set.
func lnBrowserHeaders() map[string]string {
	h := map[string]string{
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		"Accept-Language":           "en-US,en;q=0.9",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
		"Upgrade-Insecure-Requests": "1",
	}
	if cookie, _ := lnLoadCookies(); cookie != "" {
		h["Cookie"] = cookie
	}
	return h
}

var (
	lnFetchMu   sync.Mutex
	lnLastFetch time.Time
)

// lnMinFetchGap is the minimum spacing between LoopNet HTTP fetches. Akamai
// re-challenges a non-browser client that requests faster than roughly one
// page per ten seconds, even with valid clearance cookies, so fetches are
// deliberately paced (this gap plus jitter).
const lnMinFetchGap = 9 * time.Second

// lnPace blocks until at least lnMinFetchGap (plus jitter) has elapsed since
// the previous LoopNet fetch in this process. The first fetch never waits.
func lnPace() {
	lnFetchMu.Lock()
	defer lnFetchMu.Unlock()
	if !lnLastFetch.IsZero() {
		jitter := time.Duration(rand.Intn(3500)) * time.Millisecond
		if wait := lnMinFetchGap + jitter - time.Since(lnLastFetch); wait > 0 {
			time.Sleep(wait)
		}
	}
	lnLastFetch = time.Now()
}

// lnFetchHTML fetches a LoopNet page over the Surf (Chrome-fingerprint)
// transport with the stored clearance cookies, and returns the raw HTML.
//
// A fresh client is built per fetch: reusing one client across requests
// lets the surf session accumulate LoopNet's rotated Set-Cookie values,
// which downgrades the clearance after the first request and trips an
// Akamai re-challenge. A fresh client per fetch keeps every request on the
// pristine stored cookies. Fetches are paced (lnPace) to stay under
// Akamai's velocity threshold. Caching is disabled so a stale bot-challenge
// response can never mask a later success. Returns errLNChallenge when
// LoopNet serves a sensor page.
func lnFetchHTML(flags *rootFlags, path string, params map[string]string) ([]byte, error) {
	lnPace()
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	c.NoCache = true
	raw, err := c.GetWithHeaders(path, params, lnBrowserHeaders())
	if err != nil {
		return nil, err
	}
	html := []byte(raw)
	if loopnet.IsChallengePage(html) {
		return nil, errLNChallenge
	}
	return html, nil
}

// lnClassifyFetchError maps a fetch error to a typed CLI error: an Akamai
// bot challenge becomes an auth error with a fix hint; anything else is
// classified by HTTP status.
func lnClassifyFetchError(err error, flags *rootFlags) error {
	if errors.Is(err, errLNChallenge) {
		return authErr(fmt.Errorf(
			"LoopNet returned HTTP 403 — an Akamai bot-challenge page. Authentication required: " +
				"clearance cookies are missing or expired.\n" +
				"Run 'loopnet-pp-cli auth refresh' to mint fresh cookies, or 'loopnet-pp-cli auth set --cookies \"<header>\"'."))
	}
	return classifyAPIError(err, flags)
}

// lnFetchSearch fetches and parses one LoopNet search-results page.
func lnFetchSearch(flags *rootFlags, propertyType, location, listingType string, page int, f loopnet.SearchFilters) (*loopnet.SearchResult, error) {
	path := loopnet.SearchPath(propertyType, location, listingType, page)
	html, err := lnFetchHTML(flags, path, f.FilterParams())
	if err != nil {
		return nil, err
	}
	return loopnet.ParseSearchHTML(html)
}

// lnFetchDetail fetches and parses one LoopNet listing detail page.
func lnFetchDetail(flags *rootFlags, id string) (*loopnet.Property, error) {
	html, err := lnFetchHTML(flags, loopnet.DetailPath(id), nil)
	if err != nil {
		return nil, err
	}
	return loopnet.ParseDetailHTML(html)
}

// --- shared market-flag plumbing -------------------------------------------

// lnMarketFlags holds the --type / --listing narrowing flags shared by every
// store-backed LoopNet command.
type lnMarketFlags struct {
	propertyType string
	listingType  string
}

// addMarketFlags registers the shared --type / --listing flags on a command.
func addMarketFlags(cmd *cobra.Command, m *lnMarketFlags) {
	cmd.Flags().StringVar(&m.propertyType, "type", "", "Narrow to a property type (office, industrial, retail, multifamily, land, ...)")
	cmd.Flags().StringVar(&m.listingType, "listing", "", "Narrow to a listing type (for-sale or for-lease)")
}

// lnDaysSince returns whole days between t and now (0 for the zero time).
func lnDaysSince(t time.Time) int {
	if t.IsZero() {
		return 0
	}
	d := time.Since(t).Hours() / 24
	if d < 0 {
		return 0
	}
	return int(d)
}

// parseSinceWindow turns a "30d" / "12h" / "2w" window into a cutoff time.
// An empty string returns the zero time (no cutoff).
func parseSinceWindow(s string) (time.Time, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return time.Time{}, nil
	}
	unit := s[len(s)-1]
	num := s[:len(s)-1]
	var n int
	if _, err := fmt.Sscanf(num, "%d", &n); err != nil || n < 0 {
		return time.Time{}, fmt.Errorf("invalid --since window %q (use forms like 7d, 24h, 2w)", s)
	}
	var dur time.Duration
	switch unit {
	case 'h':
		dur = time.Duration(n) * time.Hour
	case 'd':
		dur = time.Duration(n) * 24 * time.Hour
	case 'w':
		dur = time.Duration(n) * 7 * 24 * time.Hour
	default:
		return time.Time{}, fmt.Errorf("invalid --since unit in %q (use h, d, or w)", s)
	}
	return time.Now().Add(-dur), nil
}

// lnNoData builds a typed not-found error with an actionable hint for the
// store-backed commands when a market has not been synced yet.
func lnNoData(location string) error {
	return notFoundErr(fmt.Errorf(
		"no synced data for %q — run 'loopnet-pp-cli sync %s' first (then again later to build history)",
		location, location))
}

// median returns the median of a sorted-or-unsorted float slice (0 if empty).
func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// quantile returns the p-quantile (0..1) of a float slice (0 if empty).
func quantile(v []float64, p float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	idx := int(p * float64(len(s)-1))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(s) {
		idx = len(s) - 1
	}
	return s[idx]
}

// dogfoodCurtail reports whether the run is inside `dogfood --live`, where
// long network commands must curtail work to fit the per-command timeout.
func dogfoodCurtail() bool {
	return cliutil.IsDogfoodEnv()
}
