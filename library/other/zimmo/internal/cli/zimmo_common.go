// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.

// Shared plumbing for the hand-written Zimmo commands: the typed client,
// criteria flags and commune resolution, page walking, store access and
// table rendering.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/store"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/zimmo"
)

const zimmoCLI = "zimmo-pp-cli"

// zimmoClient returns the hand-written typed client. ZIMMO_TOKEN, when
// set, replaces the anonymous token.
func zimmoClient(flags *rootFlags) *zimmo.Client {
	c := zimmo.NewClient(flags.timeout, strings.TrimSpace(os.Getenv("ZIMMO_TOKEN")))
	if flags.rateLimit > 0 {
		c.Limiter = cliutil.NewAdaptiveLimiter(flags.rateLimit)
	}
	return c
}

func zimmoDBPath(dbPath string) string {
	if dbPath == "" {
		return defaultDBPath(zimmoCLI)
	}
	return dbPath
}

func openZimmoStore(ctx context.Context, dbPath string) (*store.Store, error) {
	dbPath = zimmoDBPath(dbPath)
	if strings.ContainsAny(dbPath, "?#") {
		return nil, usageErr(fmt.Errorf("--db must be a plain file path"))
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local store %s: %w", dbPath, err)
	}
	if err := db.EnsureZimmoSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// localStoreExists reports whether the store file exists (offline commands).
func localStoreExists(dbPath string) (string, bool) {
	dbPath = zimmoDBPath(dbPath)
	_, err := os.Stat(dbPath)
	return dbPath, err == nil
}

// emptyStoreHint tells a human how to fill the listing store. Framework
// `sync` only covers places, so the hint names `find`, which stores every
// result it prints.
func emptyStoreHint(cmd *cobra.Command, dbPath string) {
	fmt.Fprintf(cmd.ErrOrStderr(), "hint: no stored listings in %s yet. Run '%s find --postcode 1050' (or 'watch run') to store listings first.\n", dbPath, zimmoCLI)
}

// batchFlags drops the whole-command deadline for batch commands unless the
// user set --timeout explicitly: --timeout still bounds every request
// through the client, but a 100-listing enrich or a --all watch run must not
// die at 60s halfway through its writes.
func batchFlags(flags *rootFlags) *rootFlags {
	if flags == nil || flags.timeoutExplicit {
		return flags
	}
	f := *flags
	f.timeout = 0
	return &f
}

// printZimmo is printJSONFiltered with spreadsheet-formula neutralisation of
// every string cell under --csv (upstream text such as an agency name can
// start with "=").
func printZimmo(w io.Writer, v any, flags *rootFlags) error {
	if flags == nil || !flags.csv {
		return printJSONFiltered(w, v, flags)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return err
	}
	return printJSONFiltered(w, csvSanitize(generic), flags)
}

func csvSanitize(v any) any {
	switch t := v.(type) {
	case string:
		return csvSafe(t)
	case []any:
		for i := range t {
			t[i] = csvSanitize(t[i])
		}
		return t
	case map[string]any:
		for k := range t {
			t[k] = csvSanitize(t[k])
		}
		return t
	}
	return v
}

func rejectDataSource(flags *rootFlags, strategy string) error {
	switch {
	case strategy == "live" && flags.dataSource == "local":
		return fmt.Errorf("this command has no local data source; drop --data-source local")
	case strategy == "local" && flags.dataSource == "live":
		return fmt.Errorf("this command has no live equivalent; drop --data-source live")
	}
	return nil
}

// harnessPages caps page walks under the verify/dogfood harness so the
// matrix stays inside its per-command timeout.
func harnessPages(n int) int {
	if cliutil.IsAnyHarness() && n > 1 {
		return 1
	}
	return n
}

// critFlags are the search flags shared by find, watch save and comps.
type critFlags struct {
	url        string
	communes   string
	postcodes  string
	placeIDs   string
	types      string
	status     string
	minPrice   int
	maxPrice   int
	minBeds    int
	maxBeds    int
	minSurface int
	maxSurface int
	minPlot    int
	minYear    int
	maxYear    int
	epc        string
	text       string
	newBuild   string
	sort       string
	cmd        *cobra.Command
}

func addCritFlags(cmd *cobra.Command, cf *critFlags) {
	cf.cmd = cmd
	f := cmd.Flags()
	f.StringVar(&cf.url, "url", "", "Start from a zimmo.be search URL (result page or advanced search with ?search=)")
	f.StringVar(&cf.communes, "commune", "", "Commune names, comma-separated (ixelles,uccle); resolved to Zimmo place ids")
	f.StringVar(&cf.postcodes, "postcode", "", "Postcodes, comma-separated (1050,1180)")
	f.StringVar(&cf.placeIDs, "place-id", "", "Zimmo place ids, comma-separated (see 'places')")
	f.StringVar(&cf.types, "type", "", "Property types: house, apartment, plot, garage, commercial, room (comma-separated)")
	f.StringVar(&cf.status, "status", "sale", "sale, rent, sold, rented or take-over")
	// --deal is the immoweb-pp-cli / immovlan-pp-cli spelling of --status.
	f.StringVar(&cf.status, "deal", "sale", "Alias of --status (immoweb/immovlan CLI spelling)")
	f.IntVar(&cf.minPrice, "min-price", 0, "Minimum price in EUR (monthly rent for --status rent)")
	f.IntVar(&cf.maxPrice, "max-price", 0, "Maximum price in EUR (monthly rent for --status rent)")
	f.IntVar(&cf.minBeds, "min-bedrooms", 0, "Minimum bedrooms")
	f.IntVar(&cf.maxBeds, "max-bedrooms", 0, "Maximum bedrooms")
	f.IntVar(&cf.minSurface, "min-surface", 0, "Minimum living surface in m²")
	f.IntVar(&cf.maxSurface, "max-surface", 0, "Maximum living surface in m²")
	f.IntVar(&cf.minPlot, "min-plot", 0, "Minimum plot surface in m²")
	f.IntVar(&cf.minYear, "min-year", 0, "Minimum construction year")
	f.IntVar(&cf.maxYear, "max-year", 0, "Maximum construction year")
	f.StringVar(&cf.epc, "epc", "", "EPC/PEB letters, comma-separated (F,G)")
	f.StringVar(&cf.text, "text", "", "Free-text query matched by Zimmo (terrasse, jardin, garage...)")
	f.StringVar(&cf.newBuild, "new-build", "", "true for new construction only, false to exclude it")
	f.StringVar(&cf.sort, "sort", "newest", "Order: newest, oldest, price-asc, price-desc, epc, relevance")
}

func (cf *critFlags) changed(name string) bool {
	return cf.cmd != nil && cf.cmd.Flags().Changed(name)
}

func intPtrIf(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// build validates flags into criteria; commune names stay unresolved in
// Criteria.Communes until resolveCriteria runs.
func (cf *critFlags) build() (zimmo.Criteria, error) {
	var c zimmo.Criteria
	if cf.url != "" {
		parsed, err := zimmo.ParseSearchURL(cf.url)
		if err != nil {
			return c, usageErr(err)
		}
		c = parsed
	}
	if cf.url == "" || cf.changed("status") || cf.changed("deal") {
		st, err := zimmo.ParseStatus(cf.status)
		if err != nil {
			return c, usageErr(err)
		}
		c.Statuses = []string{st}
	}
	for _, t := range splitCSV(cf.types) {
		cat, err := zimmo.ParseCategory(t)
		if err != nil {
			return c, usageErr(err)
		}
		c.Categories = append(c.Categories, cat)
	}
	for _, p := range splitCSV(cf.postcodes) {
		if _, err := strconv.Atoi(p); err != nil || len(p) != 4 {
			return c, usageErr(fmt.Errorf("--postcode %q is not a Belgian postcode (4 digits)", p))
		}
		c.Postcodes = append(c.Postcodes, p)
	}
	for _, p := range splitCSV(cf.placeIDs) {
		id, err := strconv.Atoi(p)
		if err != nil || id <= 0 {
			return c, usageErr(fmt.Errorf("--place-id %q must be a positive integer", p))
		}
		c.PlaceIDs = append(c.PlaceIDs, id)
	}
	for _, name := range splitCSV(cf.communes) {
		if _, err := strconv.Atoi(name); err == nil && len(name) == 4 {
			c.Postcodes = append(c.Postcodes, name)
			continue
		}
		c.Communes = append(c.Communes, name)
	}
	if cf.minPrice > 0 && cf.maxPrice > 0 && cf.minPrice > cf.maxPrice {
		return c, usageErr(fmt.Errorf("--min-price is above --max-price"))
	}
	if cf.changed("min-price") || cf.changed("max-price") {
		c.MinPrice, c.MaxPrice = intPtrIf(cf.minPrice), intPtrIf(cf.maxPrice)
	}
	if cf.changed("min-bedrooms") || cf.changed("max-bedrooms") {
		c.MinBeds, c.MaxBeds = intPtrIf(cf.minBeds), intPtrIf(cf.maxBeds)
	}
	if cf.changed("min-surface") || cf.changed("max-surface") {
		c.MinSurface, c.MaxSurface = intPtrIf(cf.minSurface), intPtrIf(cf.maxSurface)
	}
	c.MinPlot = intPtrIf(cf.minPlot)
	c.MinYear, c.MaxYear = intPtrIf(cf.minYear), intPtrIf(cf.maxYear)
	if cf.epc != "" {
		letters, err := zimmo.ParseEPC(cf.epc)
		if err != nil {
			return c, usageErr(err)
		}
		c.EPC = letters
	}
	if cf.text != "" {
		c.Text = cf.text
	}
	if cf.newBuild != "" {
		b, err := strconv.ParseBool(cf.newBuild)
		if err != nil {
			return c, usageErr(fmt.Errorf("--new-build must be true or false"))
		}
		c.NewBuild = &b
	}
	if cf.url == "" || cf.changed("sort") {
		c.Sort = cf.sort
	}
	if _, err := c.Sorting(); err != nil {
		return c, usageErr(err)
	}
	return c, nil
}

// resolveCriteria turns commune names into place ids (cached 30 days).
func resolveCriteria(ctx context.Context, zc *zimmo.Client, db *store.Store, c zimmo.Criteria) (zimmo.Criteria, error) {
	for _, name := range c.Communes {
		id, err := resolveCommune(ctx, zc, db, name)
		if err != nil {
			return c, err
		}
		c.PlaceIDs = append(c.PlaceIDs, id)
	}
	c.Communes = nil
	return c, nil
}

// resolveCommune picks the municipality-level (8) place whose name matches,
// falling back to a sub-municipality (9) match.
func resolveCommune(ctx context.Context, zc *zimmo.Client, db *store.Store, name string) (int, error) {
	places, err := lookupPlaces(ctx, zc, db, "", name)
	if err != nil {
		return 0, err
	}
	want := zimmo.Fold(strings.ReplaceAll(name, "-", " "))
	best, bestRank := 0, 99
	for _, p := range places {
		names := []string{p.Area.Name, p.Area.Slug}
		for _, v := range p.Translations {
			names = append(names, v)
		}
		match := false
		for _, n := range names {
			if zimmo.Fold(strings.ReplaceAll(n, "-", " ")) == want {
				match = true
			}
		}
		rank := 50
		switch p.Area.Level {
		case 8:
			rank = 1
		case 9:
			rank = 2
		case 10:
			rank = 3
		}
		if !match {
			rank += 20
		}
		if rank < bestRank {
			best, bestRank = p.ID, rank
		}
	}
	if best == 0 || bestRank > 20 {
		return 0, notFoundErr(fmt.Errorf("commune %q not found on Zimmo; try 'places --name %s' or --postcode", name, name))
	}
	return best, nil
}

// lookupPlaces queries geo-api with a 30-day local cache.
func lookupPlaces(ctx context.Context, zc *zimmo.Client, db *store.Store, postcode, name string) ([]zimmo.Place, error) {
	key := "pc=" + postcode + "|name=" + zimmo.Fold(name)
	if db != nil {
		if b, ok := db.CachedPlaces(ctx, key, 30*24*time.Hour); ok {
			var places []zimmo.Place
			if json.Unmarshal(b, &places) == nil {
				return places, nil
			}
		}
	}
	places, err := zc.Places(ctx, postcode, name)
	if err != nil {
		return nil, err
	}
	if db != nil && len(places) > 0 {
		if b, err := json.Marshal(places); err == nil {
			_ = db.PutPlaces(ctx, key, b, time.Now())
		}
	}
	return places, nil
}

// lookupPlacesCacheOnly reads a cached postcode lookup without the network.
func lookupPlacesCacheOnly(ctx context.Context, db *store.Store, postcode string) ([]zimmo.Place, error) {
	b, ok := db.CachedPlaces(ctx, "pc="+postcode+"|name=", 10*365*24*time.Hour)
	if !ok {
		return nil, fmt.Errorf("no cached place for postcode %s", postcode)
	}
	var places []zimmo.Place
	if err := json.Unmarshal(b, &places); err != nil {
		return nil, err
	}
	return places, nil
}

// communePlaceID returns the municipality (level 8) place id of a
// listing, used for €/m² lookups.
func communePlaceID(ctx context.Context, zc *zimmo.Client, db *store.Store, l zimmo.Listing) (int, error) {
	if l.PostalCode == "" {
		return 0, fmt.Errorf("listing %s has no postcode", l.Code)
	}
	places, err := lookupPlaces(ctx, zc, db, l.PostalCode, "")
	if err != nil {
		return 0, err
	}
	for _, p := range places {
		if p.Area.Level == 8 {
			return p.ID, nil
		}
	}
	for _, p := range places {
		if p.Area.Level == 9 {
			return p.ID, nil
		}
	}
	return 0, fmt.Errorf("no commune place for postcode %s", l.PostalCode)
}

// localityPrice fetches €/m² for a place with a 7-day local cache.
func localityPrice(ctx context.Context, zc *zimmo.Client, db *store.Store, placeID int, since string) (zimmo.LocalityPrice, error) {
	if db != nil && since == "" {
		if b, ok := db.CachedLocalityPrice(ctx, placeID, false, 7*24*time.Hour); ok {
			var lp zimmo.LocalityPrice
			if json.Unmarshal(b, &lp) == nil {
				return lp, nil
			}
		}
	}
	start := since
	if start == "" {
		start = time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
	}
	lp, err := zc.LocalityPriceFor(ctx, placeID, start, false)
	if err != nil {
		return lp, err
	}
	if db != nil && since == "" {
		if b, err := json.Marshal(lp); err == nil {
			_ = db.PutLocalityPrice(ctx, placeID, false, b, time.Now())
		}
	}
	return lp, nil
}

// walkResult is what a paged search walk returns.
type walkResult struct {
	Total    int
	Scanned  int // unique listings seen
	Pages    int
	Listings []zimmo.Listing
	// Truncated is set when a later page failed; Listings holds what the
	// earlier pages returned.
	Truncated error
}

// walkSearch pages through a search until limit listings or maxPages.
func walkSearch(ctx context.Context, zc *zimmo.Client, c zimmo.Criteria, maxPages, limit int) (walkResult, error) {
	var res walkResult
	size := zimmo.MaxPageSize
	if limit > 0 && limit < size {
		size = limit
	}
	now := time.Now()
	seen := map[string]bool{}
	for page := 0; page < maxPages; page++ {
		req, err := c.Request(page*size, size)
		if err != nil {
			return res, usageErr(err)
		}
		sr, err := zc.Search(ctx, req)
		if err != nil {
			if page > 0 {
				// Offset paging can hit the backend's result window; keep
				// what earlier pages returned.
				res.Truncated = err
				return res, nil
			}
			return res, err
		}
		res.Pages++
		res.Total = sr.Total
		for _, raw := range sr.Listings {
			l, err := zimmo.Parse(raw, "fr", now)
			if err != nil || l.Code == "" || seen[l.Code] {
				continue
			}
			// Offset pages can repeat a listing when the order shifts
			// between requests; count and keep each code once.
			seen[l.Code] = true
			res.Scanned++
			res.Listings = append(res.Listings, l)
			if limit > 0 && len(res.Listings) >= limit {
				return res, nil
			}
		}
		if len(sr.Listings) < size || (page+1)*size >= sr.Total {
			break
		}
	}
	return res, nil
}

func isNotFound(err error) bool { return errors.Is(err, zimmo.ErrNotFound) }

// parseCodeArg accepts a zimmo code (LAISZ), a listing UUID or a detail URL.
func parseCodeArg(s string) (string, error) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "zimmo.be/") {
		s = strings.TrimRight(strings.SplitN(s, "?", 2)[0], "/")
		s = s[strings.LastIndex(s, "/")+1:]
	}
	if s == "" {
		return "", fmt.Errorf("empty listing reference")
	}
	if len(s) == 36 && strings.Count(s, "-") == 4 {
		return strings.ToLower(s), nil
	}
	up := strings.ToUpper(s)
	for _, r := range up {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return "", fmt.Errorf("%q is not a Zimmo code (5 letters/digits like LAISZ), a listing UUID or a zimmo.be URL", s)
		}
	}
	return up, nil
}

func isUUID(s string) bool { return len(s) == 36 && strings.Count(s, "-") == 4 }

// refFilter selects a stored listing by zimmo code or by UUID.
func refFilter(ref string) store.ListingFilter {
	if isUUID(ref) {
		return store.ListingFilter{IDs: []string{strings.ToLower(ref)}, IncludeGone: true}
	}
	return store.ListingFilter{Codes: []string{strings.ToUpper(ref)}, IncludeGone: true}
}

// fetchListing gets one listing live and parses it.
func fetchListing(ctx context.Context, zc *zimmo.Client, ref string) (zimmo.Listing, error) {
	raw, err := zc.Listing(ctx, ref)
	if err != nil {
		if isNotFound(err) {
			return zimmo.Listing{}, notFoundErr(fmt.Errorf("listing %s not found on Zimmo (removed or wrong code)", ref))
		}
		return zimmo.Listing{}, err
	}
	return zimmo.Parse(raw, "fr", time.Now())
}

func fmtEUR(v float64) string {
	n := int64(v + 0.5)
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	out := b.String() + " €"
	if neg {
		out = "-" + out
	}
	return out
}

func fmtPtrEUR(p *float64) string {
	if p == nil {
		return "-"
	}
	return fmtEUR(*p)
}

func intStr(p *int) string {
	if p == nil {
		return "-"
	}
	return strconv.Itoa(*p)
}

func floatUnit(p *float64, unit string) string {
	if p == nil {
		return "-"
	}
	return strconv.FormatFloat(*p, 'f', -1, 64) + unit
}

func epcStr(l zimmo.Listing) string {
	if l.EPC == "" {
		return "-"
	}
	if l.EPCKWh != nil {
		return fmt.Sprintf("%s (%.0f)", l.EPC, *l.EPCKWh)
	}
	return l.EPC
}

// termSafe strips C0/C1 control characters and bidi overrides from
// upstream text before it reaches a terminal.
func termSafe(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f):
			return ' '
		case (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069):
			return -1
		}
		return r
	}, s)
}

func csvSafe(s string) string {
	t := strings.TrimLeft(s, " \t")
	if t != "" && strings.ContainsRune("=+-@\t\r", rune(t[0])) {
		return "'" + s
	}
	return s
}

func shortAddr(l zimmo.Listing) string {
	a := l.Address
	if len([]rune(a)) > 48 {
		a = string([]rune(a)[:47]) + "…"
	}
	return a
}

func printListingTable(w io.Writer, listings []zimmo.Listing) error {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "CODE\tTYPE\tPRICE\tM²\t€/M²\tBEDS\tEPC\tADDRESS")
	for _, l := range listings {
		pps := "-"
		if l.PricePerM2 != nil {
			pps = strconv.FormatFloat(*l.PricePerM2, 'f', 0, 64)
		}
		typ := strings.ToLower(l.Type)
		if l.IsProject {
			typ = "project"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", l.Code, typ, fmtPtrEUR(l.Price), floatUnit(l.Surface, ""), pps, intStr(l.Bedrooms), epcStr(l), termSafe(shortAddr(l)))
	}
	return tw.Flush()
}

func pct1(ratio float64) float64 {
	return float64(int64(ratio*1000+sign(ratio)*0.5)) / 10
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

// sortedByPPS orders listings by €/m² ascending, unknown last.
func sortedByPPS(ls []zimmo.Listing) {
	sort.SliceStable(ls, func(i, j int) bool {
		a, b := ls[i].PricePerM2, ls[j].PricePerM2
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		return *a < *b
	})
}
