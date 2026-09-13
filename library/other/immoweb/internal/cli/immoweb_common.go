// Shared helpers for the hand-authored Immoweb commands (find, pull, watch,
// market, deal, triage, drops, yield, ...). Kept in its own file so
// `generate --force` preserves it.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/immo"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/store"
)

const (
	immoPageSize   = 30
	immoMapSize    = 200
	immoResultsCap = 9969
)

// critFlags holds the friendly search flags shared by find, saved add, pull
// and market-style commands.
type critFlags struct {
	url         string
	types       string
	deal        string
	communes    string
	postcodes   string
	provinces   string
	districts   string
	minPrice    int
	maxPrice    int
	minBeds     int
	maxBeds     int
	minSurface  int
	maxSurface  int
	minLand     int
	minYear     int
	epc         string
	garden      bool
	terrace     bool
	pool        bool
	newBuild    bool
	lifeAnnuity bool
	publicSale  bool
	furnished   bool
	sort        string
	cmd         *cobra.Command // set by addCritFlags; tells explicit flags apart from defaults
}

func (cf *critFlags) changed(name string) bool {
	return cf.cmd != nil && cf.cmd.Flags().Changed(name)
}

func addCritFlags(cmd *cobra.Command, cf *critFlags, withSort bool) {
	cf.cmd = cmd
	f := cmd.Flags()
	f.StringVar(&cf.url, "url", "", "Start from a pasted Immoweb search URL (fr/nl/en); other flags override it")
	f.StringVar(&cf.types, "type", "", "Property type(s), comma-separated: house, apartment, land, office, garage, commercial, industry, other (FR/NL names work too)")
	f.StringVar(&cf.deal, "deal", "", "sale or rent")
	f.StringVar(&cf.communes, "commune", "", "Commune name(s), comma-separated (e.g. ixelles,saint-gilles); resolved to postal codes via Immoweb")
	f.StringVar(&cf.postcodes, "postcode", "", "Postal code(s), comma-separated (e.g. 1050,1060)")
	f.StringVar(&cf.provinces, "province", "", "Province(s), comma-separated, in English, French or Dutch (e.g. namur,walloon_brabant,liege)")
	f.StringVar(&cf.districts, "district", "", "District(s) / arrondissement(s), comma-separated (e.g. nivelles)")
	f.IntVar(&cf.minPrice, "min-price", 0, "Minimum price in EUR (monthly rent when --deal rent)")
	f.IntVar(&cf.maxPrice, "max-price", 0, "Maximum price in EUR (monthly rent when --deal rent)")
	f.IntVar(&cf.minBeds, "min-bedrooms", 0, "Minimum bedrooms")
	f.IntVar(&cf.maxBeds, "max-bedrooms", 0, "Maximum bedrooms")
	f.IntVar(&cf.minSurface, "min-surface", 0, "Minimum living surface in m²")
	f.IntVar(&cf.maxSurface, "max-surface", 0, "Maximum living surface in m²")
	f.IntVar(&cf.minLand, "min-land", 0, "Minimum land (plot) surface in m²")
	f.IntVar(&cf.minYear, "min-year", 0, "Minimum construction year")
	f.StringVar(&cf.epc, "epc", "", "EPC/PEB label(s), comma-separated (A,B,C,...)")
	f.BoolVar(&cf.garden, "garden", false, "Only listings with a garden")
	f.BoolVar(&cf.terrace, "terrace", false, "Only listings with a terrace")
	f.BoolVar(&cf.pool, "pool", false, "Only listings with a swimming pool")
	f.BoolVar(&cf.newBuild, "new-build", false, "Only new builds")
	f.BoolVar(&cf.lifeAnnuity, "life-annuity", false, "Only life-annuity (viager) sales")
	f.BoolVar(&cf.publicSale, "public-sale", false, "Only public (notary) sales")
	f.BoolVar(&cf.furnished, "furnished", false, "Only furnished rentals")
	if withSort {
		f.StringVar(&cf.sort, "sort", "relevance", "Sort: relevance, newest, cheapest, most-expensive, postal-code")
	}
}

// build turns flags into Criteria (communes still unresolved).
func (cf *critFlags) build() (immo.Criteria, error) {
	var c immo.Criteria
	if cf.url != "" {
		parsed, err := immo.ParseSearchURL(cf.url)
		if err != nil {
			return c, err
		}
		c = parsed
	}
	if cf.types != "" {
		ts, err := immo.NormalizeTypes(cf.types)
		if err != nil {
			return c, err
		}
		c.Types = ts
	}
	if cf.deal != "" {
		d, err := immo.NormalizeDeal(cf.deal)
		if err != nil {
			return c, err
		}
		c.Deal = d
	}
	if cf.url != "" && (cf.changed("postcode") || cf.changed("commune")) {
		// Explicit location flags replace the URL's location.
		c.PostalCodes, c.Communes = nil, nil
	}
	for _, name := range immo.SplitCSV(cf.communes) {
		if pc := immo.NormalizePostalCode(name); pc != "" {
			c.PostalCodes = appendUniq(c.PostalCodes, pc)
			continue
		}
		c.Communes = appendUniq(c.Communes, name)
	}
	for _, p := range immo.SplitCSV(cf.postcodes) {
		pc := immo.NormalizePostalCode(p)
		if pc == "" {
			return c, fmt.Errorf("--postcode %q is not a 4-digit Belgian postal code", p)
		}
		c.PostalCodes = appendUniq(c.PostalCodes, pc)
	}
	for _, p := range immo.SplitCSV(cf.provinces) {
		v, err := immo.NormalizeProvince(p)
		if err != nil {
			return c, err
		}
		c.Provinces = appendUniq(c.Provinces, v)
	}
	if cf.districts != "" {
		c.Districts = upperList(cf.districts)
	}
	setIf := func(dst *int, v int) {
		if v > 0 {
			*dst = v
		}
	}
	setIf(&c.MinPrice, cf.minPrice)
	setIf(&c.MaxPrice, cf.maxPrice)
	setIf(&c.MinBedrooms, cf.minBeds)
	setIf(&c.MaxBedrooms, cf.maxBeds)
	setIf(&c.MinSurface, cf.minSurface)
	setIf(&c.MaxSurface, cf.maxSurface)
	setIf(&c.MinLand, cf.minLand)
	setIf(&c.MinYear, cf.minYear)
	if cf.epc != "" {
		c.EPC = upperList(cf.epc)
	}
	c.Garden = c.Garden || cf.garden
	c.Terrace = c.Terrace || cf.terrace
	c.Pool = c.Pool || cf.pool
	c.NewBuild = c.NewBuild || cf.newBuild
	c.LifeAnnuity = c.LifeAnnuity || cf.lifeAnnuity
	c.PublicSale = c.PublicSale || cf.publicSale
	c.Furnished = c.Furnished || cf.furnished
	if cf.sort != "" && (cf.url == "" || cf.changed("sort")) {
		s, err := immo.NormalizeSort(cf.sort)
		if err != nil {
			return c, err
		}
		c.Sort = s
	}
	return c, nil
}

func upperList(csv string) []string {
	out := []string{}
	for _, v := range immo.SplitCSV(csv) {
		out = append(out, strings.ToUpper(strings.ReplaceAll(v, "-", "_")))
	}
	return out
}

func appendUniq(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

// resolvedCommune reports how a commune name was mapped.
type resolvedCommune struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	PostalCodes []string `json:"postal_codes"`
}

// lookupCommune resolves one commune name to Immoweb postal codes.
func lookupCommune(ctx context.Context, c *client.Client, name string) (resolvedCommune, error) {
	if pc := immo.NormalizePostalCode(name); pc != "" {
		return resolvedCommune{Name: name, Label: immo.BarePostalCode(pc), PostalCodes: []string{pc}}, nil
	}
	if codes := immo.BrusselsPostcodes(name); codes != nil {
		return resolvedCommune{Name: name, Label: immo.BrusselsLabel(name, codes), PostalCodes: codes}, nil
	}
	data, err := c.Get(ctx, "/en/search/autocomplete", map[string]string{"query": name})
	if err != nil {
		return resolvedCommune{}, fmt.Errorf("looking up commune %q: %w", name, err)
	}
	matches, err := immo.ParseAutocomplete(data)
	if err != nil {
		return resolvedCommune{}, nonJSONErr("commune lookup", err)
	}
	m, ok := immo.PickCommune(name, matches)
	if !ok {
		return resolvedCommune{}, usageErr(fmt.Errorf("unknown commune %q; look it up with 'immoweb-pp-cli locations --query %s' and pass its postal code with --postcode", name, name))
	}
	return resolvedCommune{Name: name, Label: m.Label, PostalCodes: m.PostalCodes()}, nil
}

// resolveCriteria resolves commune names into postal codes in place.
func resolveCriteria(ctx context.Context, c *client.Client, crit immo.Criteria) (immo.Criteria, []resolvedCommune, error) {
	resolved := make([]resolvedCommune, 0, len(crit.Communes))
	for _, name := range crit.Communes {
		rc, err := lookupCommune(ctx, c, name)
		if err != nil {
			return crit, resolved, err
		}
		for _, pc := range rc.PostalCodes {
			crit.PostalCodes = appendUniq(crit.PostalCodes, pc)
		}
		resolved = append(resolved, rc)
	}
	if err := checkDistricts(ctx, c, crit); err != nil {
		return crit, resolved, err
	}
	return crit, resolved, nil
}

// checkDistricts refuses district names Immoweb does not know. Immoweb
// ignores an unknown district and searches all of Belgium, which shows as a
// district count equal to the whole-country count.
func checkDistricts(ctx context.Context, c *client.Client, crit immo.Criteria) error {
	if len(crit.Districts) == 0 {
		return nil
	}
	base, err := fetchCount(ctx, c, immo.Criteria{Types: crit.Types, Deal: crit.Deal}.Params())
	if err != nil || base == 0 {
		return nil // cannot tell; let the search itself report
	}
	for _, d := range crit.Districts {
		n, err := fetchCount(ctx, c, immo.Criteria{Types: crit.Types, Deal: crit.Deal, Districts: []string{d}}.Params())
		if err == nil && n == base {
			return usageErr(fmt.Errorf("unknown district %q: Immoweb would search all of Belgium; district names are Immoweb's own (e.g. NIVELLES, LEUVEN) - check with 'immoweb-pp-cli locations --query %s'", d, strings.ToLower(d)))
		}
	}
	return nil
}

// nonJSONErr explains the two causes of a non-JSON Immoweb reply.
func nonJSONErr(what string, err error) error {
	return apiErr(fmt.Errorf("%s: Immoweb returned a non-JSON page (an invalid filter value, or a bot challenge): %v", what, err))
}

// searchPage is one decoded search-results / search-results-map page.
type searchPage struct {
	Raw   []json.RawMessage
	Items []immo.Listing
	Total int
}

// fetchResults fetches one page from a results endpoint. Soft throttling
// (HTTP 200 with an empty page before totalItems is reached) is returned as
// a typed rate-limit error instead of an empty result.
func fetchResults(ctx context.Context, c *client.Client, path string, params map[string]string, page, pageSize, knownTotal int) (searchPage, error) {
	// Immoweb soft-throttles with empty pages; back off and retry twice
	// before surfacing the typed rate-limit error.
	var sp searchPage
	var err error
	for attempt, wait := range []time.Duration{0, 2 * time.Second, 5 * time.Second} {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return sp, ctx.Err()
			case <-time.After(wait):
			}
		}
		sp, err = fetchResultsOnce(ctx, c, path, params, page, pageSize, knownTotal)
		var ce *cliError
		if err == nil || !errors.As(err, &ce) || ce.code != 7 {
			return sp, err
		}
	}
	return sp, err
}

func fetchResultsOnce(ctx context.Context, c *client.Client, path string, params map[string]string, page, pageSize, knownTotal int) (searchPage, error) {
	p := map[string]string{}
	for k, v := range params {
		p[k] = v
	}
	p["page"] = strconv.Itoa(page)
	data, err := c.GetNoCache(ctx, path, p)
	if err != nil {
		return searchPage{}, err
	}
	trimmed := strings.TrimSpace(string(data))
	var env struct {
		Results    []json.RawMessage `json:"results"`
		TotalItems int               `json:"totalItems"`
	}
	switch {
	case strings.HasPrefix(trimmed, "["):
		// Past the last page Immoweb returns a bare empty array.
		var arr []json.RawMessage
		if err := json.Unmarshal(data, &arr); err != nil {
			return searchPage{}, nonJSONErr("search", err)
		}
		env.Results = arr
	default:
		if err := json.Unmarshal(data, &env); err != nil {
			return searchPage{}, nonJSONErr("search", err)
		}
	}
	sp := searchPage{Total: env.TotalItems}
	if sp.Total == 0 {
		sp.Total = knownTotal
	}
	for _, r := range env.Results {
		l, err := immo.FromResult(r)
		if err != nil {
			continue
		}
		sp.Items = append(sp.Items, l)
		sp.Raw = append(sp.Raw, r)
	}
	expected := sp.Total
	if expected > immoResultsCap {
		expected = immoResultsCap
	}
	if len(sp.Items) == 0 && expected > (page-1)*pageSize {
		return sp, rateLimitErr(&cliutil.RateLimitError{
			URL:        path + " page " + strconv.Itoa(page),
			RetryAfter: time.Minute,
			Body:       fmt.Sprintf("Immoweb returned an empty page although %d results exist (soft throttling); wait a minute or fetch fewer pages", expected),
		})
	}
	return sp, nil
}

// fetchCount returns the exact number of listings for params.
func fetchCount(ctx context.Context, c *client.Client, params map[string]string) (int, error) {
	data, err := c.GetNoCache(ctx, "/en/search-results-count", params)
	if err != nil {
		return 0, err
	}
	var out struct {
		Count int `json:"classifiedsCount"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return 0, nonJSONErr("count", err)
	}
	return out.Count, nil
}

// fetchDetail fetches and flattens one classified.
func fetchDetail(ctx context.Context, c *client.Client, id int64) (immo.Detail, json.RawMessage, error) {
	data, err := c.GetNoCache(ctx, "/en/classified/get-result/"+strconv.FormatInt(id, 10), nil)
	if err != nil {
		var apiE *client.APIError
		if errors.As(err, &apiE) && (apiE.StatusCode == 404 || apiE.StatusCode == 400) {
			return immo.Detail{}, nil, notFoundErr(fmt.Errorf("listing %d not found on Immoweb (removed, sold or wrong ID)", id))
		}
		return immo.Detail{}, nil, err
	}
	d, err := immo.FromClassified(data)
	if err != nil {
		if errors.Is(err, immo.ErrNoID) {
			return immo.Detail{}, nil, notFoundErr(fmt.Errorf("listing %d not found on Immoweb", id))
		}
		return immo.Detail{}, nil, nonJSONErr("listing", err)
	}
	return d, data, nil
}

// harvestResult is the outcome of an area harvest.
type harvestResult struct {
	Listings []immo.Listing
	Total    int  // exact count on Immoweb
	Priced   int  // listings reachable through price bands (published price)
	Complete bool // every listing retrieved
	// PricedComplete: every listing with a published price retrieved, so
	// stored priced listings of the scope that are missing can be marked gone.
	PricedComplete bool
	Calls          int
}

// harvest collects every listing matching params and upserts it locally:
// exact count, then one map call when the area fits (<=200), otherwise
// price bands of <=200 listings each fetched through the map endpoint.
// Search-results paging (30 per page, maxSearchPages) is the fallback when
// banding cannot isolate the listings.
func harvest(ctx context.Context, c *client.Client, db *store.Store, params map[string]string, maxSearchPages int) (harvestResult, error) {
	var res harvestResult
	total, err := fetchCount(ctx, c, params)
	if err != nil {
		return res, err
	}
	res.Calls++
	res.Total = total
	if total == 0 {
		res.Complete, res.PricedComplete = true, true
		return res, nil
	}
	if total <= immoMapSize {
		sp, err := fetchResults(ctx, c, "/en/search-results-map", params, 1, immoMapSize, total)
		res.Calls++
		if err != nil {
			return res, err
		}
		if db != nil && len(sp.Items) > 0 {
			if err := db.UpsertImmoListings(ctx, sp.Items, sp.Raw, time.Now()); err != nil {
				return res, err
			}
		}
		res.Listings = sp.Items
		res.Priced = total
		res.Complete = len(sp.Items) >= total
		res.PricedComplete = res.Complete
		return res, nil
	}
	// Bands only pay off when the area fits in maxBands map calls; larger
	// areas (a whole province) go straight to search paging.
	var bands []priceBand
	covered := 0
	if total <= maxBands*bandCap {
		var calls int
		var err error
		bands, covered, calls, err = planPriceBands(ctx, c, params, total)
		res.Calls += calls
		if err != nil {
			return res, err
		}
	}
	oversized := len(bands) == 0
	for _, b := range bands {
		if b.Count > bandCap {
			oversized = true
		}
	}
	if !oversized && len(bands) <= maxBands {
		listings, n, err := harvestBands(ctx, c, db, params, bands)
		res.Calls += n
		res.Listings = listings
		res.Priced = covered
		res.PricedComplete = len(listings) >= covered
		res.Complete = len(listings) >= total
		return res, err
	}
	// Fallback: page through search results.
	limit := min(total, immoResultsCap)
	for page := 1; page <= maxSearchPages; page++ {
		sp, err := fetchResults(ctx, c, "/en/search-results", params, page, immoPageSize, total)
		res.Calls++
		if err != nil {
			return res, err
		}
		if db != nil && len(sp.Items) > 0 {
			if err := db.UpsertImmoListings(ctx, sp.Items, sp.Raw, time.Now()); err != nil {
				return res, err
			}
		}
		res.Listings = append(res.Listings, sp.Items...)
		if len(sp.Items) < immoPageSize || page*immoPageSize >= limit {
			break
		}
	}
	res.Priced = covered
	res.Complete = len(res.Listings) >= total
	res.PricedComplete = res.Complete
	return res, nil
}

// harvestCtx bounds a multi-request harvesting command. --timeout is a
// per-request budget for the HTTP client; unless the user set it explicitly,
// the whole command gets a generous overall deadline instead.
func harvestCtx(cmd *cobra.Command, flags *rootFlags) (context.Context, context.CancelFunc) {
	if flags.timeoutExplicit {
		return boundCtx(cmd.Context(), flags)
	}
	return context.WithTimeout(cmd.Context(), 5*time.Minute)
}

// openImmoStore opens (creating when needed) the local store with the
// Immoweb tables ready.
func openImmoStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath != "" && os.Getenv("IMMOWEB_LEARN_SURFACE") == "mcp" {
		return nil, usageErr(fmt.Errorf("--db is not available to MCP callers"))
	}
	if dbPath == "" {
		dbPath = defaultDBPath("immoweb-pp-cli")
	}
	if strings.ContainsAny(dbPath, "?#") {
		return nil, usageErr(fmt.Errorf("--db must be a plain file path (no '?' or '#')"))
	}
	if err := checkSQLiteFile(dbPath); err != nil {
		return nil, usageErr(err)
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local store %s: %w", dbPath, err)
	}
	if err := db.EnsureImmoSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// localStoreExists reports whether the default (or given) store file exists.
func localStoreExists(dbPath string) (string, bool) {
	if dbPath == "" {
		dbPath = defaultDBPath("immoweb-pp-cli")
	}
	_, err := os.Stat(dbPath)
	return dbPath, err == nil
}

// listingRow is the compact human/CSV row for a listing.
func listingRow(l immo.Listing) map[string]any {
	price := ""
	if l.Price != nil {
		price = fmtEUR(*l.Price)
		if l.Deal == "FOR_RENT" {
			price += "/mo"
		}
	}
	beds, surf, pps := "", "", ""
	if l.Bedrooms != nil {
		beds = strconv.Itoa(*l.Bedrooms)
	}
	if l.Surface != nil {
		surf = strconv.Itoa(int(*l.Surface))
	}
	if l.PricePerSqm != nil {
		pps = strconv.Itoa(int(*l.PricePerSqm))
	}
	flag := l.Flag
	if l.NewPrice && flag == "" {
		flag = "new_price"
	}
	return map[string]any{
		"id": l.ID, "type": strings.ToLower(l.Type), "price": price, "beds": beds, "m2": surf, "eur_m2": pps,
		"locality": locLabel(l), "flag": flag, "seller": sellerLabel(l),
	}
}

func sellerLabel(l immo.Listing) string {
	if l.Private {
		return "private"
	}
	return termSafe(truncate(l.Agency, 28))
}

func fmtEUR(v float64) string {
	n := int64(v + 0.5)
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String() + " €"
}

// printListingTable renders listings as a compact table in a stable column order.
func printListingTable(w io.Writer, listings []immo.Listing) error {
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "ID\tTYPE\tPRICE\tBEDS\tM²\t€/M²\tLOCALITY\tFLAG\tSELLER")
	for _, l := range listings {
		r := listingRow(l)
		fmt.Fprintf(tw, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n", r["id"], r["type"], r["price"], r["beds"], r["m2"], r["eur_m2"], r["locality"], r["flag"], r["seller"])
	}
	return tw.Flush()
}

// comparablePrices returns €/m² values of listings comparable to target:
// same deal and type, same postal code, bedrooms within ±1 and surface
// within ±25% when those are known. Gone listings are included (they are
// the closest thing to sold comparables).
func comparablePrices(target immo.Listing, pool []store.StoredListing) ([]float64, []store.StoredListing) {
	vals := []float64{}
	comps := []store.StoredListing{}
	for _, p := range pool {
		if p.ID == target.ID || p.PricePerSqm == nil || p.Deal != target.Deal || p.Type != target.Type {
			continue
		}
		// Rooms compare with rooms, whole homes with whole homes.
		if immo.IsRoomLet(p.Listing) != immo.IsRoomLet(target) {
			continue
		}
		if target.PostalCode != "" && p.PostalCode != target.PostalCode {
			continue
		}
		if target.Bedrooms != nil && p.Bedrooms != nil && absInt(*p.Bedrooms-*target.Bedrooms) > 1 {
			continue
		}
		if target.Surface != nil && p.Surface != nil {
			ratio := *p.Surface / *target.Surface
			if ratio < 0.75 || ratio > 1.25 {
				continue
			}
		}
		vals = append(vals, *p.PricePerSqm)
		comps = append(comps, p)
	}
	return vals, comps
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// sortListingsByPPS orders by €/m² ascending (unknown last).
func sortListingsByPPS(ls []store.StoredListing) {
	sort.SliceStable(ls, func(i, j int) bool {
		a, b := ls[i].PricePerSqm, ls[j].PricePerSqm
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		return *a < *b
	})
}

// rejectDataSource enforces a command's declared data-source strategy.
func rejectDataSource(flags *rootFlags, strategy string) error {
	return validateDataSourceStrategy(flags, strategy)
}

// dogfoodPages curtails page walks under the verifier, live-check probes and
// the live dogfood matrix so read-heavy commands fit their per-command timeout.
func dogfoodPages(n int) int {
	if cliutil.IsAnyHarness() && n > 1 {
		return 1
	}
	return n
}

// locLabel renders "postcode Locality" with the locality tidied for display.
func locLabel(l immo.Listing) string {
	return termSafe(strings.TrimSpace(l.PostalCode + " " + immo.CleanLocality(l.Locality)))
}

// dropRoomLets removes student rooms and per-room lets (see immo.IsRoomLet),
// which would otherwise be compared as whole homes. It returns the kept
// listings and how many were removed.
func dropRoomLets(ls []store.StoredListing) ([]store.StoredListing, int) {
	kept := make([]store.StoredListing, 0, len(ls))
	for _, l := range ls {
		if !immo.IsRoomLet(l.Listing) {
			kept = append(kept, l)
		}
	}
	return kept, len(ls) - len(kept)
}

// termSafe strips control characters (terminal escape sequences included)
// from advertiser-written text before it is printed to a terminal.
func termSafe(s string) string {
	if strings.IndexFunc(s, func(r rune) bool { return r != '\t' && unicode.IsControl(r) }) < 0 {
		return s
	}
	return strings.Map(func(r rune) rune {
		if r != '\t' && unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

// checkSQLiteFile refuses an existing non-empty file that is not an SQLite
// database, before the store opens (and chmods) it.
func checkSQLiteFile(path string) error {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil // missing: the store creates it
	}
	defer f.Close()
	head := make([]byte, 16)
	n, _ := io.ReadFull(f, head)
	if n == 0 || string(head[:n]) == "SQLite format 3\x00" {
		return nil
	}
	return fmt.Errorf("%s is not an SQLite database; refusing to use it as the local store", path)
}
