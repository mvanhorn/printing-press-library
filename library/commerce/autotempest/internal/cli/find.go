// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/autotempest/internal/autotempest"
	"github.com/mvanhorn/printing-press-library/library/commerce/autotempest/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/autotempest/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/autotempest/internal/store"

	"github.com/spf13/cobra"
)

// findOpts is the resolved filter set for one search. Shared by `find` and
// `watch run` so both drive the identical search path. Zero values use the
// AutoTempest "any"/-1 defaults; see buildSourceParams.
type findOpts struct {
	Make         string
	Model        string
	Keywords     string // free-text; passed to the st source
	Zip          string
	Radius       int
	MinPrice     int
	MaxPrice     int
	MinYear      int
	MaxYear      int
	MinMiles     int
	MaxMiles     int
	Body         string
	Drive        string
	Fuel         string
	Transmission string
	Color        string
	Title        string // clean/salvage/any
	Seller       string // dealer/private/any
	Sort         string
	Sites        []string
	RPP          int
	MaxPages     int
	Limit        int
	SaveName     string // when set, persist as a saved search
}

// fetchFailure records a per-source failure so the envelope can surface it
// without zeroing the other sources' results.
type fetchFailure struct {
	Source string `json:"source"`
	Error  string `json:"error"`
}

// comparisonLink is a deep-link to an external comparison source (fbm/st) that
// AutoTempest only links to — it does not return inline parsed listings for
// these because the underlying sites block scraping / require login.
type comparisonLink struct {
	Source string `json:"source"`
	Name   string `json:"name"`
	URL    string `json:"url"`
}

// findEnvelope is the structured result of a search.
type findEnvelope struct {
	Meta            findMeta              `json:"meta"`
	Results         []autotempest.Listing `json:"results"`
	ComparisonLinks []comparisonLink      `json:"comparison_links"`
}

type findMeta struct {
	SourcesQueried []string       `json:"sources_queried"`
	Listings       int            `json:"listings"`
	FetchFailures  []fetchFailure `json:"fetch_failures"`
}

// queueResponse is the /queue-results (and fbm/st) response envelope. fbm/st
// carry their comparison URL on a different field (url / stUrl) and never
// return an inline listings array.
type queueResponse struct {
	Status      int               `json:"status"`
	Results     []json.RawMessage `json:"results"`
	SearchAfter json.RawMessage   `json:"searchAfter"`
	Errors      []string          `json:"errors"`
}

// linkOnlyResponse decodes just the comparison URL for fbm/st. fbm puts it on
// "url"; st puts it on "stUrl".
type linkOnlyResponse struct {
	URL   string `json:"url"`
	StURL string `json:"stUrl"`
}

// inlineSources are the 7 partnered sources that return inline parsed listings.
var inlineSources = []string{"te", "hem", "cs", "cv", "cm", "eb", "ot"}

// isLinkOnlySource reports whether a source is comparison-link-only (fbm/st):
// AutoTempest deep-links to these rather than returning inline listings.
func isLinkOnlySource(site string) bool {
	return site == "fbm" || site == "st"
}

const (
	atResourceListings = "at_listings"
	maxConcurrentSites = 5
	interCallDelay     = 250 * time.Millisecond
	queuePollAttempts  = 8
	queuePollInterval  = 1200 * time.Millisecond
)

// defaultSites returns the sources a bare `find` queries: the 7 partnered
// inline-listing sources only. fbm/st are comparison-link-only and would always
// land in fetch_failures (or produce no listings), so they are excluded from
// the default and only included when the user explicitly requests them.
func defaultSites() []string {
	out := make([]string, len(inlineSources))
	copy(out, inlineSources)
	return out
}

// dogfoodSites is the curtailed source subset used under the dogfood matrix so
// the flat 30s timeout is not blown by polling nine sources.
var dogfoodSites = []string{"te", "cs", "eb"}

func newFindCmd(flags *rootFlags) *cobra.Command {
	var (
		mk, model                              string
		zip                                    string
		radius                                 int
		minPrice, maxPrice, minYear, maxYear   int
		minMiles, maxMiles                     int
		body, drive, fuel, transmission, color string
		title, seller, sortBy                  string
		sitesCSV                               string
		rpp, maxPages, limit                   int
		saveName, dbPath                       string
	)

	cmd := &cobra.Command{
		Use:   "find [keywords]",
		Short: "Live multi-source car search across every AutoTempest source",
		Long: `Search every AutoTempest source live, persist results to the local store,
snapshot prices, and emit a JSON envelope.

The positional argument is free text: when --make/--model are not set, the first
word becomes the make and the rest the model (e.g. "honda civic"). The full text
is also passed as keywords to the SearchTempest source.`,
		Example: strings.Trim(`
  autotempest-pp-cli find "honda civic" --zip 33701 --radius 200 --json
  autotempest-pp-cli find "ford f-150" --zip 33701 --max-price 25000 --title clean --json
  autotempest-pp-cli find --make toyota --model tacoma --zip 33701 --drive 4wd --json`, "\n"),
		// Positional is free-text keywords: any string is valid input (a search
		// that matches nothing is exit 0, not an error), so the invalid-arg
		// probe does not apply.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would search %q across sources %s\n",
					strings.Join(args, " "), sitesCSV)
				return nil
			}

			opts := findOpts{
				Make: mk, Model: model, Zip: zip, Radius: radius,
				MinPrice: minPrice, MaxPrice: maxPrice,
				MinYear: minYear, MaxYear: maxYear,
				MinMiles: minMiles, MaxMiles: maxMiles,
				Body: body, Drive: drive, Fuel: fuel, Transmission: transmission,
				Color: color, Title: title, Seller: seller, Sort: sortBy,
				Sites: splitCSV(sitesCSV), RPP: rpp, MaxPages: maxPages,
				Limit: limit, SaveName: saveName,
			}
			applyPositional(&opts, args)

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			env, err := runFind(ctx, c, flags, opts, dbPath)
			if err != nil {
				return err
			}
			return emitFindEnvelope(cmd, flags, env)
		},
	}

	// Register flags via cmd.Flags() directly (not an `f :=` alias) so the
	// library's verify_skill.py static tracer can see each declaration.
	cmd.Flags().StringVar(&mk, "make", "", "Make slug (overrides positional)")
	cmd.Flags().StringVar(&model, "model", "", "Model slug")
	cmd.Flags().StringVar(&zip, "zip", "", "ZIP code to center the search")
	cmd.Flags().IntVar(&radius, "radius", -1, "Search radius in miles (-1 = national)")
	cmd.Flags().IntVar(&minPrice, "min-price", -1, "Minimum price (-1 = any)")
	cmd.Flags().IntVar(&maxPrice, "max-price", -1, "Maximum price (-1 = any)")
	cmd.Flags().IntVar(&minYear, "min-year", -1, "Minimum year (-1 = any)")
	cmd.Flags().IntVar(&maxYear, "max-year", -1, "Maximum year (-1 = any)")
	cmd.Flags().IntVar(&minMiles, "min-miles", -1, "Minimum mileage (-1 = any)")
	cmd.Flags().IntVar(&maxMiles, "max-miles", -1, "Maximum mileage (-1 = any)")
	cmd.Flags().StringVar(&body, "body", "", "Body style filter")
	cmd.Flags().StringVar(&drive, "drive", "", "Drivetrain filter")
	cmd.Flags().StringVar(&fuel, "fuel", "", "Fuel type filter")
	cmd.Flags().StringVar(&transmission, "transmission", "", "Transmission filter")
	cmd.Flags().StringVar(&color, "color", "", "Exterior color filter")
	cmd.Flags().StringVar(&title, "title", "", "Title status filter (clean/salvage/any)")
	cmd.Flags().StringVar(&seller, "seller", "", "Seller type filter (dealer/private/any)")
	cmd.Flags().StringVar(&sortBy, "sort", "best_match", "Sort order")
	cmd.Flags().StringVar(&sitesCSV, "sites", strings.Join(defaultSites(), ","), "Comma-separated source codes")
	cmd.Flags().IntVar(&rpp, "rpp", 50, "Results per page per source")
	cmd.Flags().IntVar(&maxPages, "max-pages", 1, "Max pages to fetch per source")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max total listings to emit")
	cmd.Flags().StringVar(&saveName, "save", "", "Persist this search under a name")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: per-user data dir)")

	return cmd
}

// applyPositional fills make/model/keywords from free-text args when --make is
// not set explicitly. The full positional always becomes Keywords (for the st
// source); --make wins over the positional split.
func applyPositional(opts *findOpts, args []string) {
	joined := strings.TrimSpace(strings.Join(args, " "))
	if joined != "" && opts.Keywords == "" {
		opts.Keywords = joined
	}
	if opts.Make != "" {
		return
	}
	fields := strings.Fields(joined)
	if len(fields) == 0 {
		return
	}
	opts.Make = strings.ToLower(fields[0])
	if len(fields) > 1 {
		opts.Model = strings.ToLower(strings.Join(fields[1:], " "))
	}
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// runFind executes the multi-source search, persists listings + price
// snapshots, optionally saves the search, and returns the envelope. Shared by
// the `find` command and `watch run`. dbPath is the explicit --db value or ""
// for the default location.
func runFind(ctx context.Context, c *client.Client, flags *rootFlags, opts findOpts, dbPath string) (findEnvelope, error) {
	sites := opts.Sites
	if len(sites) == 0 {
		sites = defaultSites()
	}
	maxPages := opts.MaxPages
	if maxPages < 1 {
		maxPages = 1
	}
	if cliutil.IsDogfoodEnv() {
		maxPages = 1
		sites = intersectSites(sites, dogfoodSites)
		if len(sites) == 0 {
			sites = dogfoodSites
		}
	}

	type sourceResult struct {
		listings []autotempest.Listing
		link     *comparisonLink
		failure  *fetchFailure
	}

	var (
		mu      sync.Mutex
		results = make(map[string]sourceResult, len(sites))
		wg      sync.WaitGroup
		sem     = make(chan struct{}, maxConcurrentSites)
	)

	for _, site := range sites {
		site := site
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if isLinkOnlySource(site) {
				// Link-only: fetch the comparison URL; never a fetch_failure
				// for a transport-level success that simply has no inline
				// listings. A real transport error still surfaces.
				link, ferr := fetchComparisonLink(ctx, c, opts, site)
				mu.Lock()
				defer mu.Unlock()
				res := sourceResult{}
				if ferr != nil {
					res.failure = &fetchFailure{Source: site, Error: ferr.Error()}
				} else if link != nil {
					res.link = link
				}
				results[site] = res
				return
			}
			ls, ferr := fetchSource(ctx, c, opts, site, maxPages)
			mu.Lock()
			defer mu.Unlock()
			res := sourceResult{listings: ls}
			if ferr != nil {
				res.failure = &fetchFailure{Source: site, Error: ferr.Error()}
			}
			results[site] = res
		}()
	}
	wg.Wait()

	// Assemble in stable site order.
	all := make([]autotempest.Listing, 0)
	failures := make([]fetchFailure, 0)
	links := make([]comparisonLink, 0)
	for _, site := range sites {
		r := results[site]
		all = append(all, r.listings...)
		if r.failure != nil {
			failures = append(failures, *r.failure)
		}
		if r.link != nil {
			links = append(links, *r.link)
		}
	}

	// Persist (best-effort dedupe by listing id, keep first occurrence).
	if err := persistListings(ctx, dbPath, opts.SaveName, all); err != nil {
		return findEnvelope{}, err
	}
	if opts.SaveName != "" {
		if err := persistSavedSearch(ctx, dbPath, opts); err != nil {
			return findEnvelope{}, err
		}
	}

	// Cap emitted listings.
	emitted := dedupeListings(all)
	if opts.Limit > 0 && len(emitted) > opts.Limit {
		emitted = emitted[:opts.Limit]
	}

	return findEnvelope{
		Meta: findMeta{
			SourcesQueried: sites,
			Listings:       len(emitted),
			FetchFailures:  failures,
		},
		Results:         emitted,
		ComparisonLinks: links,
	}, nil
}

func intersectSites(want, allow []string) []string {
	allowed := map[string]bool{}
	for _, a := range allow {
		allowed[a] = true
	}
	var out []string
	for _, w := range want {
		if allowed[w] {
			out = append(out, w)
		}
	}
	return out
}

// dedupeListings drops duplicate listing IDs, preserving first-seen order.
func dedupeListings(in []autotempest.Listing) []autotempest.Listing {
	seen := map[string]bool{}
	out := make([]autotempest.Listing, 0, len(in))
	for _, l := range in {
		if seen[l.ID] {
			continue
		}
		seen[l.ID] = true
		out = append(out, l)
	}
	return out
}

// fetchSource queries one inline-listing source, polling /queue-results until
// results populate (status:1 with empty results means the backend is still
// fetching) and paginating up to maxPages. Only called for the 7 partnered
// sources; link-only sources (fbm/st) go through fetchComparisonLink instead.
func fetchSource(ctx context.Context, c *client.Client, opts findOpts, site string, maxPages int) ([]autotempest.Listing, error) {
	const path = "/queue-results"

	var out []autotempest.Listing
	var searchAfter string
	for page := 0; page < maxPages; page++ {
		params := buildSourceParams(opts, site, searchAfter)
		resp, err := pollQueue(ctx, c, path, params, site)
		if err != nil {
			return out, err
		}
		if resp.Status == -2 {
			return out, fmt.Errorf("invalid token (status -2)")
		}
		pageListings := make([]autotempest.Listing, 0, len(resp.Results))
		for _, raw := range resp.Results {
			if l, ok := autotempest.ParseListing(raw, site); ok {
				pageListings = append(pageListings, l)
			}
		}
		out = append(out, pageListings...)

		// Decide whether to continue paginating.
		nextSA := normalizeSearchAfter(resp.SearchAfter)
		if len(pageListings) == 0 || nextSA == "" || nextSA == searchAfter {
			break
		}
		searchAfter = nextSA
		if page+1 < maxPages {
			_ = sleepCtx(ctx, interCallDelay)
		}
	}
	return out, nil
}

// fetchComparisonLink fetches a link-only source (fbm/st) and extracts its
// comparison URL. These sources never return inline listings — AutoTempest only
// deep-links to them — so a transport success with no listings is NOT a failure.
// Returns (nil, nil) when the response is missing a URL; (nil, err) only on a
// real transport/decode error.
func fetchComparisonLink(ctx context.Context, c *client.Client, opts findOpts, site string) (*comparisonLink, error) {
	path := "/api/facebookMarketplace"
	if site == "st" {
		path = "/api/searchtempest/direct"
	}
	fullPath := path + "?" + autotempest.SignedQuery(buildSourceParams(opts, site, ""))
	data, err := c.GetWithHeaders(ctx, fullPath, nil, nil)
	if err != nil {
		return nil, err
	}
	var resp linkOnlyResponse
	if uerr := json.Unmarshal(data, &resp); uerr != nil {
		return nil, fmt.Errorf("decoding %s comparison response: %w", site, uerr)
	}
	url := resp.URL
	if site == "st" {
		url = resp.StURL
	}
	if strings.TrimSpace(url) == "" {
		return nil, nil
	}
	return &comparisonLink{
		Source: site,
		Name:   autotempest.SourceName(site),
		URL:    url,
	}, nil
}

// pollQueue issues the signed GET against /queue-results and retries while the
// backend reports status:1 with no results yet (still fetching). Only called
// for the 7 inline-listing sources, all of which return the flat-array envelope.
func pollQueue(ctx context.Context, c *client.Client, path string, params []autotempest.KV, site string) (queueResponse, error) {
	var last queueResponse
	for attempt := 0; attempt < queuePollAttempts; attempt++ {
		fullPath := path + "?" + autotempest.SignedQuery(params)
		data, err := c.GetWithHeaders(ctx, fullPath, nil, nil)
		if err != nil {
			return queueResponse{}, err
		}
		var resp queueResponse
		if uerr := json.Unmarshal(data, &resp); uerr != nil {
			return queueResponse{}, fmt.Errorf("decoding %s queue response: %w", site, uerr)
		}
		last = resp
		if resp.Status == -2 {
			return resp, nil
		}
		if len(resp.Results) > 0 {
			return resp, nil
		}
		// status 0 with no results = genuinely empty source; stop polling.
		if resp.Status == 0 {
			return resp, nil
		}
		// status 1 + empty = still fetching; poll again.
		if attempt+1 < queuePollAttempts {
			if err := sleepCtx(ctx, queuePollInterval); err != nil {
				return last, err
			}
		}
	}
	return last, nil
}

// normalizeSearchAfter renders the searchAfter JSON array as a compact string
// for the next page's param, or "" when it is empty/absent.
func normalizeSearchAfter(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || trimmed == "[]" {
		return ""
	}
	return trimmed
}

// buildSourceParams assembles the ordered, signed-query param list for one
// source. Empty/default values are omitted to keep the query lean, but the
// order is stable. sites is the single source being queried.
func buildSourceParams(opts findOpts, site, searchAfter string) []autotempest.KV {
	params := make([]autotempest.KV, 0, 32)
	add := func(k, v string) {
		if v != "" {
			params = append(params, autotempest.KV{K: k, V: v})
		}
	}
	addInt := func(k string, v int) {
		// -1 means "any"/default; omit.
		if v >= 0 {
			add(k, strconv.Itoa(v))
		}
	}

	// AutoTempest model/make slugs strip hyphens and spaces (F-150 -> "f150",
	// CR-V -> "crv", "alfa romeo" -> "alfaromeo"). Normalize before sending so
	// the raw user text doesn't miss the catalog and return zero results. The
	// keywords param (st source) keeps the original text — set elsewhere.
	add("make", autotempest.NormalizeSlug(opts.Make))
	add("model", autotempest.NormalizeSlug(opts.Model))
	add("zip", opts.Zip)
	radius := opts.Radius
	if radius >= 0 {
		add("radius", strconv.Itoa(radius))
		add("originalradius", strconv.Itoa(radius))
	}
	addInt("minprice", opts.MinPrice)
	addInt("maxprice", opts.MaxPrice)
	addInt("minyear", opts.MinYear)
	addInt("maxyear", opts.MaxYear)
	addInt("minmiles", opts.MinMiles)
	addInt("maxmiles", opts.MaxMiles)
	add("bodystyle", nonAny(opts.Body))
	add("drive", nonAny(opts.Drive))
	add("fuel", nonAny(opts.Fuel))
	add("transmission", nonAny(opts.Transmission))
	add("exterior_color", nonAny(opts.Color))
	add("title", nonAny(opts.Title))
	add("saleby", nonAny(opts.Seller))
	if site == "st" && opts.Keywords != "" {
		add("keywords", opts.Keywords)
	}
	sortBy := opts.Sort
	if sortBy == "" {
		sortBy = "best_match"
	}
	add("sort", sortBy)
	add("sites", site)
	add("deduplicationSites", autotempest.DeduplicationSites)
	add("rpp", strconv.Itoa(rppOrDefault(opts.RPP)))
	if site == "st" {
		add("make_moved", "1")
		add("clBundleDuplicates", "1")
	}
	if searchAfter != "" {
		add("searchAfter", searchAfter)
	}
	return params
}

func rppOrDefault(rpp int) int {
	if rpp <= 0 {
		return 50
	}
	return rpp
}

// nonAny treats the literal "any" (and empty) as "no filter".
func nonAny(s string) string {
	s = strings.TrimSpace(s)
	if strings.EqualFold(s, "any") {
		return ""
	}
	return s
}

// persistListings upserts each listing and records a price snapshot when the
// price differs from the listing's last snapshot. When searchName is non-empty
// (a saved search via `watch run <name>` or `find --save <name>`), each listing
// is also registered in at_search_members so `drops <name>` can scope to it.
// Membership is many-to-many: a listing belongs to every search that found it,
// and a later run of a DIFFERENT search never removes it from earlier ones.
func persistListings(ctx context.Context, dbPath, searchName string, listings []autotempest.Listing) error {
	if len(listings) == 0 {
		return nil
	}
	db, err := openAutoTempestStore(ctx, dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	now := time.Now().Unix()
	sqlDB := db.DB()
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, l := range dedupeListings(listings) {
		raw := ""
		if len(l.Raw) > 0 {
			raw = string(l.Raw)
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO at_listings (
				listing_id, vin, title, make, model, year, trim, price_cents, mileage,
				location, zip, country, distance, dealer_name, seller_type, source,
				sitecode, vehicle_title, listing_type, current_bid_cents, bids, url, img,
				search_name, first_seen, last_seen, raw
			) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(listing_id) DO UPDATE SET
				vin=excluded.vin, title=excluded.title, make=excluded.make,
				model=excluded.model, year=excluded.year, trim=excluded.trim,
				price_cents=excluded.price_cents, mileage=excluded.mileage,
				location=excluded.location, zip=excluded.zip, country=excluded.country,
				distance=excluded.distance, dealer_name=excluded.dealer_name,
				seller_type=excluded.seller_type, source=excluded.source,
				sitecode=excluded.sitecode, vehicle_title=excluded.vehicle_title,
				listing_type=excluded.listing_type, current_bid_cents=excluded.current_bid_cents,
				bids=excluded.bids, url=excluded.url, img=excluded.img,
				search_name=COALESCE(NULLIF(excluded.search_name,''), at_listings.search_name),
				last_seen=excluded.last_seen, raw=excluded.raw`,
			l.ID, l.VIN, l.Title, l.Make, l.Model, l.Year, l.Trim, l.PriceCents, l.Mileage,
			l.Location, l.Zip, l.Country, l.Distance, l.DealerName, l.SellerType, l.Source,
			l.Sitecode, l.VehicleTitle, l.ListingType, l.CurrentBid, l.Bids, l.URL, l.Img,
			searchName, now, now, raw,
		)
		if err != nil {
			return fmt.Errorf("upserting listing %s: %w", l.ID, err)
		}

		// Record a snapshot only when the price differs from the listing's MOST
		// RECENT snapshot (not any historical value). This captures every price
		// CHANGE — including a recovery back to a prior value — while leaving
		// unchanged re-syncs as no-ops. The table's UNIQUE(listing_id, ts) makes
		// two writes in the same second a harmless no-op (INSERT OR IGNORE).
		if l.PriceCents >= 0 {
			var lastPrice sql.NullInt64
			row := tx.QueryRowContext(ctx, `
				SELECT price_cents FROM at_price_snapshots
				WHERE listing_id = ? ORDER BY ts DESC, id DESC LIMIT 1`, l.ID)
			_ = row.Scan(&lastPrice)
			if !lastPrice.Valid || lastPrice.Int64 != l.PriceCents {
				if _, err := tx.ExecContext(ctx, `
					INSERT OR IGNORE INTO at_price_snapshots (listing_id, ts, price_cents, mileage)
					VALUES (?,?,?,?)`, l.ID, now, l.PriceCents, l.Mileage); err != nil {
					return fmt.Errorf("snapshotting listing %s: %w", l.ID, err)
				}
			}
		}

		// Register the listing's membership in this saved search (if named).
		// INSERT OR IGNORE keeps first_added stable across re-runs and never
		// removes the listing from any OTHER search it already belongs to —
		// memberships only accumulate. Unnamed/ad-hoc finds add no rows.
		if searchName != "" {
			if _, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO at_search_members (search_name, listing_id, first_added)
				VALUES (?,?,?)`, searchName, l.ID, now); err != nil {
				return fmt.Errorf("recording search membership for %s: %w", l.ID, err)
			}
		}
	}
	return tx.Commit()
}

// persistSavedSearch writes (or updates) a saved search row from the opts.
func persistSavedSearch(ctx context.Context, dbPath string, opts findOpts) error {
	db, err := openAutoTempestStore(ctx, dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return saveSearchRow(ctx, db.DB(), opts.SaveName, opts, true)
}

// savedSearchParams is the JSON-serialized filter set stored in
// at_saved_searches.params; watch run / find replay reconstruct findOpts from it.
type savedSearchParams struct {
	Make         string   `json:"make"`
	Model        string   `json:"model"`
	Keywords     string   `json:"keywords"`
	Zip          string   `json:"zip"`
	Radius       int      `json:"radius"`
	MinPrice     int      `json:"min_price"`
	MaxPrice     int      `json:"max_price"`
	MinYear      int      `json:"min_year"`
	MaxYear      int      `json:"max_year"`
	MinMiles     int      `json:"min_miles"`
	MaxMiles     int      `json:"max_miles"`
	Body         string   `json:"body"`
	Drive        string   `json:"drive"`
	Fuel         string   `json:"fuel"`
	Transmission string   `json:"transmission"`
	Color        string   `json:"color"`
	Title        string   `json:"title"`
	Seller       string   `json:"seller"`
	Sort         string   `json:"sort"`
	Sites        []string `json:"sites"`
	RPP          int      `json:"rpp"`
	MaxPages     int      `json:"max_pages"`
	Limit        int      `json:"limit"`
}

func optsToParams(opts findOpts) savedSearchParams {
	return savedSearchParams{
		Make: opts.Make, Model: opts.Model, Keywords: opts.Keywords, Zip: opts.Zip,
		Radius: opts.Radius, MinPrice: opts.MinPrice, MaxPrice: opts.MaxPrice,
		MinYear: opts.MinYear, MaxYear: opts.MaxYear, MinMiles: opts.MinMiles, MaxMiles: opts.MaxMiles,
		Body: opts.Body, Drive: opts.Drive, Fuel: opts.Fuel, Transmission: opts.Transmission,
		Color: opts.Color, Title: opts.Title, Seller: opts.Seller, Sort: opts.Sort,
		Sites: opts.Sites, RPP: opts.RPP, MaxPages: opts.MaxPages, Limit: opts.Limit,
	}
}

func paramsToOpts(p savedSearchParams, saveName string) findOpts {
	return findOpts{
		Make: p.Make, Model: p.Model, Keywords: p.Keywords, Zip: p.Zip,
		Radius: p.Radius, MinPrice: p.MinPrice, MaxPrice: p.MaxPrice,
		MinYear: p.MinYear, MaxYear: p.MaxYear, MinMiles: p.MinMiles, MaxMiles: p.MaxMiles,
		Body: p.Body, Drive: p.Drive, Fuel: p.Fuel, Transmission: p.Transmission,
		Color: p.Color, Title: p.Title, Seller: p.Seller, Sort: p.Sort,
		Sites: p.Sites, RPP: p.RPP, MaxPages: p.MaxPages, Limit: p.Limit,
		SaveName: saveName,
	}
}

// saveSearchRow upserts a saved search. name is the key; query is a
// human-readable summary; params is the JSON filter set. When markRun is true,
// last_run is updated; created is set only on first insert (COALESCE).
func saveSearchRow(ctx context.Context, sqlDB *sql.DB, name string, opts findOpts, markRun bool) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("saved search name is required")
	}
	paramsJSON, err := json.Marshal(optsToParams(opts))
	if err != nil {
		return err
	}
	query := strings.TrimSpace(strings.Join([]string{opts.Make, opts.Model}, " "))
	if query == "" {
		query = opts.Keywords
	}
	now := time.Now().Unix()
	var lastRun any
	if markRun {
		lastRun = now
	}
	_, err = sqlDB.ExecContext(ctx, `
		INSERT INTO at_saved_searches (name, query, params, created, last_run)
		VALUES (?,?,?,?,?)
		ON CONFLICT(name) DO UPDATE SET
			query=excluded.query, params=excluded.params,
			last_run=COALESCE(excluded.last_run, at_saved_searches.last_run)`,
		name, query, string(paramsJSON), now, lastRun)
	return err
}

// openAutoTempestStore opens the store at dbPath (or default) and ensures the
// AutoTempest tables exist.
func openAutoTempestStore(ctx context.Context, dbPath string) (*store.Store, error) {
	path := dbPath
	if path == "" {
		path = defaultDBPath("autotempest-pp-cli")
	}
	db, err := store.OpenWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("opening local store: %w", err)
	}
	if err := store.EnsureAutoTempestTables(db.DB()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// sleepCtx sleeps for d unless ctx is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// emitFindEnvelope renders the envelope through the standard output pipeline.
// Default (terminal, no --json) prints a compact table of the results.
func emitFindEnvelope(cmd *cobra.Command, flags *rootFlags, env findEnvelope) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		rows := make([]map[string]any, 0, len(env.Results))
		for _, l := range env.Results {
			rows = append(rows, listingRow(l))
		}
		if len(rows) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "no results")
		} else if err := printAutoTable(cmd.OutOrStdout(), rows); err != nil {
			return err
		}
		for _, link := range env.ComparisonLinks {
			fmt.Fprintf(cmd.OutOrStdout(), "compare on %s: %s\n", link.Name, link.URL)
		}
		if len(env.Meta.FetchFailures) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "\n%d source(s) failed: %s\n",
				len(env.Meta.FetchFailures), failuresSummary(env.Meta.FetchFailures))
		}
		return nil
	}
	return printJSONFiltered(cmd.OutOrStdout(), env, flags)
}

func failuresSummary(fs []fetchFailure) string {
	parts := make([]string, 0, len(fs))
	for _, f := range fs {
		parts = append(parts, f.Source)
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

// listingRow projects a listing to a compact display map for table output.
func listingRow(l autotempest.Listing) map[string]any {
	return map[string]any{
		"title":   l.Title,
		"price":   centsDisplay(l.PriceCents),
		"mileage": milesDisplay(l.Mileage),
		"year":    l.Year,
		"source":  l.Source,
		"url":     l.URL,
	}
}

// centsDisplay renders integer cents as "$30,497" (with thousands separators)
// or "" for an unknown/negative value. Used across find/drops/deal/spread/
// auctions output. Delegates to autotempest.FormatCents so all price formatting
// shares one implementation.
func centsDisplay(cents int64) string {
	return autotempest.FormatCents(cents)
}

func milesDisplay(m int64) string {
	if m < 0 {
		return ""
	}
	return strconv.FormatInt(m, 10)
}
