// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command. The RunE body below is implemented; do not
// replace it with a scaffold.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/store"
	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/unitprice"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Shared Woolworths tile plumbing.
//
// swap, multibuy and basket all read the same product tile off the same two
// POST endpoints, so the wire types and the paging loop live here once.
// Nothing in this block is specific to swap.
// ---------------------------------------------------------------------------

const (
	// wowSearchPath is the product search endpoint. It is a POST that does not
	// mutate anything, so it goes through the client's PostQuery* family.
	wowSearchPath = "/apis/ui/Search/products"
	// wowCategoryPath is the category browse endpoint, used when a positional
	// looks like a category id rather than a search term.
	wowCategoryPath = "/apis/ui/browse/category"
	// wowMaxPageSize is a hard server limit. A PageSize above 36 is rejected
	// with HTTP 400, so every request built here is clamped to it.
	wowMaxPageSize = 36
	// wowDefaultMaxScanPages bounds the scan-and-filter loops in swap and
	// multibuy, which page through results and filter locally.
	wowDefaultMaxScanPages = 5
)

// wowMultibuyData is the multi-buy offer attached to a tile's centre tag:
// "buy Quantity for $Price", with CupTag as the retailer's own unit price for
// the offer.
type wowMultibuyData struct {
	Quantity int     `json:"Quantity"`
	Price    float64 `json:"Price"`
	CupTag   string  `json:"CupTag"`
}

type wowCentreTag struct {
	TagType      string           `json:"TagType"`
	MultibuyData *wowMultibuyData `json:"MultibuyData"`
}

// wowTile is the subset of a Woolworths product tile these commands need.
// Stockcode arrives as a JSON number; json.Number keeps it exact and keeps it
// printable without float formatting artefacts.
type wowTile struct {
	Stockcode     json.Number   `json:"Stockcode"`
	Name          string        `json:"Name"`
	DisplayName   string        `json:"DisplayName"`
	Brand         string        `json:"Brand"`
	Price         float64       `json:"Price"`
	WasPrice      float64       `json:"WasPrice"`
	SavingsAmount float64       `json:"SavingsAmount"`
	IsOnSpecial   bool          `json:"IsOnSpecial"`
	IsHalfPrice   bool          `json:"IsHalfPrice"`
	CupPrice      float64       `json:"CupPrice"`
	CupMeasure    string        `json:"CupMeasure"`
	CupString     string        `json:"CupString"`
	PackageSize   string        `json:"PackageSize"`
	IsAvailable   bool          `json:"IsAvailable"`
	IsInStock     bool          `json:"IsInStock"`
	CentreTag     *wowCentreTag `json:"CentreTag"`
}

func (t wowTile) stockcode() string { return strings.TrimSpace(t.Stockcode.String()) }

func (t wowTile) displayName() string {
	if strings.TrimSpace(t.DisplayName) != "" {
		return t.DisplayName
	}
	return t.Name
}

// available reports whether the tile can actually be bought. Both flags are
// checked: recommending something nobody can put in a trolley is worse than
// returning one fewer row.
func (t wowTile) available() bool { return t.IsAvailable && t.IsInStock }

// observation converts a tile into the shape the local price mirror records.
//
// IsAvailable is recorded as available(), i.e. the SAME two-flag rule swap,
// multibuy and basket filter on. Recording the raw IsAvailable flag here while
// filtering on IsAvailable && IsInStock elsewhere is what let the mirror and
// the live commands disagree about what is buyable.
func (t wowTile) observation(observedAt int64) store.PriceObservation {
	return store.PriceObservation{
		Stockcode:   t.stockcode(),
		ObservedAt:  observedAt,
		Name:        t.Name,
		Brand:       t.Brand,
		Price:       t.Price,
		WasPrice:    t.WasPrice,
		Savings:     t.SavingsAmount,
		IsOnSpecial: t.IsOnSpecial,
		IsHalfPrice: t.IsHalfPrice,
		CupPrice:    t.CupPrice,
		CupMeasure:  t.CupMeasure,
		CupString:   t.CupString,
		PackageSize: t.PackageSize,
		IsAvailable: t.available(),
	}
}

// multibuy returns the tile's multi-buy offer, or nil when it carries none.
// An offer with a non-positive quantity or price is treated as absent rather
// than as a free product.
func (t wowTile) multibuy() *wowMultibuyData {
	if t.CentreTag == nil || t.CentreTag.MultibuyData == nil {
		return nil
	}
	mb := t.CentreTag.MultibuyData
	if mb.Quantity <= 0 || mb.Price <= 0 {
		return nil
	}
	return mb
}

// wowSearchResponse mirrors the search payload. The product array is DOUBLE
// nested — the outer Products is a list of groups, each with its own Products
// list — and it is null, not empty, when the term matches nothing.
type wowSearchResponse struct {
	Products []struct {
		Products []wowTile `json:"Products"`
	} `json:"Products"`
	SearchResultsCount int `json:"SearchResultsCount"`
}

func (r wowSearchResponse) tiles() []wowTile {
	out := make([]wowTile, 0, len(r.Products)*wowMaxPageSize)
	for _, group := range r.Products {
		out = append(out, group.Products...)
	}
	return out
}

// wowCategoryResponse mirrors the category browse payload, which uses Bundles
// rather than the search endpoint's nested Products.
type wowCategoryResponse struct {
	Bundles []struct {
		Products []wowTile `json:"Products"`
	} `json:"Bundles"`
	TotalRecordCount int `json:"TotalRecordCount"`
}

func (r wowCategoryResponse) tiles() []wowTile {
	out := make([]wowTile, 0, len(r.Bundles)*wowMaxPageSize)
	for _, bundle := range r.Bundles {
		out = append(out, bundle.Products...)
	}
	return out
}

// wowFetchFailure records one fetch that did not come back. It travels with
// the result set instead of aborting the command, and whatever it represents
// is excluded from every count, total and average reported downstream.
type wowFetchFailure struct {
	Kind  string `json:"kind"`
	Query string `json:"query"`
	Page  int    `json:"page,omitempty"`
	Error string `json:"error"`
}

// wowScanResult is the outcome of a bounded scan-and-filter pass.
type wowScanResult struct {
	Tiles      []wowTile
	Pages      int
	TotalCount int
	Failures   []wowFetchFailure
}

// wowClampPageSize keeps every request under the server's hard cap. Sending
// PageSize > 36 returns HTTP 400, so this is the single choke point that all
// request builders in this package go through.
func wowClampPageSize(n int) int {
	if n <= 0 {
		return wowMaxPageSize
	}
	if n > wowMaxPageSize {
		return wowMaxPageSize
	}
	return n
}

// wowResolveMaxScanPages applies the --max-scan-pages flag, dropping to a
// single page under the dogfood matrix so a scan-and-filter command cannot
// spend the whole per-command timeout paginating.
func wowResolveMaxScanPages(requested int) int {
	if requested <= 0 {
		requested = wowDefaultMaxScanPages
	}
	if cliutil.IsDogfoodEnv() && requested > 1 {
		return 1
	}
	return requested
}

// wowSearchPage fetches one page of search results.
func wowSearchPage(ctx context.Context, c *client.Client, term string, page, pageSize int, sortType string) (wowSearchResponse, error) {
	body := map[string]any{
		"SearchTerm": term,
		"PageNumber": page,
		"PageSize":   wowClampPageSize(pageSize),
		"SortType":   sortType,
		"IsSpecial":  false,
	}
	var parsed wowSearchResponse
	data, _, err := c.PostQueryWithParams(ctx, wowSearchPath, map[string]string{}, body)
	if err != nil {
		return parsed, err
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return parsed, fmt.Errorf("decoding search response: %w", err)
	}
	return parsed, nil
}

// wowCategoryPage fetches one page of a category listing.
func wowCategoryPage(ctx context.Context, c *client.Client, categoryID string, page, pageSize int, sortType string) (wowCategoryResponse, error) {
	body := map[string]any{
		"categoryId":      categoryID,
		"pageNumber":      page,
		"pageSize":        wowClampPageSize(pageSize),
		"sortType":        sortType,
		"isSpecial":       false,
		"filters":         []any{},
		"categoryVersion": "v2",
	}
	var parsed wowCategoryResponse
	data, _, err := c.PostQueryWithParams(ctx, wowCategoryPath, map[string]string{}, body)
	if err != nil {
		return parsed, err
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return parsed, fmt.Errorf("decoding category response: %w", err)
	}
	return parsed, nil
}

// wowScanSearch pages through search results up to maxPages, deduplicating by
// stockcode. A page that fails is recorded in Failures and skipped; the scan
// keeps going so one flaky page does not lose the whole result set, and the
// caller can report the real denominator.
func wowScanSearch(ctx context.Context, c *client.Client, term string, maxPages, pageSize int, sortType string) wowScanResult {
	res := wowScanResult{Tiles: make([]wowTile, 0, wowMaxPageSize), Failures: make([]wowFetchFailure, 0)}
	seen := map[string]bool{}
	size := wowClampPageSize(pageSize)
	for page := 1; page <= maxPages; page++ {
		if err := ctx.Err(); err != nil {
			res.Failures = append(res.Failures, wowFetchFailure{Kind: "search", Query: term, Page: page, Error: err.Error()})
			break
		}
		resp, err := wowSearchPage(ctx, c, term, page, size, sortType)
		if err != nil {
			res.Failures = append(res.Failures, wowFetchFailure{Kind: "search", Query: term, Page: page, Error: err.Error()})
			continue
		}
		res.Pages++
		if resp.SearchResultsCount > res.TotalCount {
			res.TotalCount = resp.SearchResultsCount
		}
		batch := resp.tiles()
		for _, tile := range batch {
			code := tile.stockcode()
			if code == "" || seen[code] {
				continue
			}
			seen[code] = true
			res.Tiles = append(res.Tiles, tile)
		}
		// The endpoint returns a null product list rather than an empty one
		// when a term matches nothing; a short page means the tail.
		if len(batch) == 0 || len(batch) < size {
			break
		}
		if res.TotalCount > 0 && len(res.Tiles) >= res.TotalCount {
			break
		}
	}
	return res
}

// wowScanCategory is wowScanSearch for the category browse endpoint.
func wowScanCategory(ctx context.Context, c *client.Client, categoryID string, maxPages, pageSize int, sortType string) wowScanResult {
	res := wowScanResult{Tiles: make([]wowTile, 0, wowMaxPageSize), Failures: make([]wowFetchFailure, 0)}
	seen := map[string]bool{}
	size := wowClampPageSize(pageSize)
	for page := 1; page <= maxPages; page++ {
		if err := ctx.Err(); err != nil {
			res.Failures = append(res.Failures, wowFetchFailure{Kind: "category", Query: categoryID, Page: page, Error: err.Error()})
			break
		}
		resp, err := wowCategoryPage(ctx, c, categoryID, page, size, sortType)
		if err != nil {
			res.Failures = append(res.Failures, wowFetchFailure{Kind: "category", Query: categoryID, Page: page, Error: err.Error()})
			continue
		}
		res.Pages++
		if resp.TotalRecordCount > res.TotalCount {
			res.TotalCount = resp.TotalRecordCount
		}
		batch := resp.tiles()
		for _, tile := range batch {
			code := tile.stockcode()
			if code == "" || seen[code] {
				continue
			}
			seen[code] = true
			res.Tiles = append(res.Tiles, tile)
		}
		if len(batch) == 0 || len(batch) < size {
			break
		}
		if res.TotalCount > 0 && len(res.Tiles) >= res.TotalCount {
			break
		}
	}
	return res
}

// wowLooksNumeric reports whether a positional is all digits, which is how
// these commands tell a stockcode or category id from a search term.
func wowLooksNumeric(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// wowNormalizeTile restates a tile's unit price on its kind's canonical basis.
// It prefers the retailer's own cup price and falls back to shelf price over
// pack size when the cup fields are missing. It reports failure rather than
// inventing a zero, so an unrankable tile is excluded instead of leading.
func wowNormalizeTile(t wowTile) (unitprice.Normalized, bool) {
	if n, ok := unitprice.Normalize(t.CupPrice, t.CupMeasure); ok {
		return n, true
	}
	if n, ok := unitprice.Normalize(t.Price, t.PackageSize); ok {
		return n, true
	}
	return unitprice.Normalized{}, false
}

// wowOpenHistoryDB opens the local price-history database if it exists.
// A missing database is not an error: history is an enrichment and every
// command here works without it. The returned closer is always safe to call.
func wowOpenHistoryDB(ctx context.Context, dbPath string) (*sql.DB, func(), bool) {
	if strings.TrimSpace(dbPath) == "" {
		dbPath = defaultDBPath("woolworths-pp-cli")
	}
	if _, err := os.Stat(dbPath); err != nil {
		return nil, func() {}, false
	}
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, func() {}, false
	}
	if err := store.EnsureWoolworthsTables(ctx, s.DB()); err != nil {
		_ = s.Close()
		return nil, func() {}, false
	}
	return s.DB(), func() { _ = s.Close() }, true
}

// wowHistoryLow is what the local database knows about one product's own past
// prices. It is nil when nothing has been recorded for that stockcode.
type wowHistoryLow struct {
	RecordedLow      float64 `json:"recorded_low"`
	RecordedLowAt    int64   `json:"recorded_low_at"`
	Observations     int     `json:"observations"`
	AtRecordedLow    bool    `json:"at_recorded_low"`
	AboveLowByAmount float64 `json:"above_recorded_low_by"`
	AboveLowByPct    float64 `json:"above_recorded_low_by_pct"`
}

// wowRecordedLow compares a current price against the lowest price this
// product has ever been recorded at locally. A product with no observations
// returns a nil report, never a zero-valued one that would read as "$0.00 low".
func wowRecordedLow(ctx context.Context, db *sql.DB, stockcode string, current float64) (*wowHistoryLow, error) {
	if db == nil || stockcode == "" {
		return nil, nil
	}
	history, err := store.PriceHistory(ctx, db, stockcode)
	if err != nil {
		return nil, err
	}
	low := math.Inf(1)
	var lowAt int64
	counted := 0
	for _, obs := range history {
		if obs.Price <= 0 {
			continue
		}
		counted++
		if obs.Price < low {
			low = obs.Price
			lowAt = obs.ObservedAt
		}
	}
	if counted == 0 || math.IsInf(low, 1) {
		return nil, nil
	}
	out := &wowHistoryLow{
		RecordedLow:   roundMoney(low),
		RecordedLowAt: lowAt,
		Observations:  counted,
	}
	if current > 0 {
		out.AtRecordedLow = current <= low+0.005
		out.AboveLowByAmount = roundMoney(current - low)
		if low > 0 {
			out.AboveLowByPct = roundMoney((current - low) / low * 100)
		}
	}
	return out, nil
}

// roundMoney rounds to two decimal places. It is the single 2-dp rounder in
// this package and is used for percentages and per-basis figures too, all of
// which are reported to the cent.
func roundMoney(v float64) float64 { return math.Round(v*100) / 100 }

// wowWarnFailures prints the partial-failure warning to stderr, naming the
// failed count and the denominator the reported numbers are actually based on.
// Silent partial results are the thing this exists to prevent.
func wowWarnFailures(command string, failures []wowFetchFailure, succeeded int) {
	if len(failures) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: warning: %d of %d fetches failed; every figure reported is computed over the %d that succeeded\n",
		command, len(failures), len(failures)+succeeded, succeeded)
	for _, f := range failures {
		fmt.Fprintf(os.Stderr, "  %s %q page %d: %s\n", f.Kind, f.Query, f.Page, f.Error)
	}
}

// wowHistoryProbe accumulates the outcome of the per-product local history
// lookups a command makes.
//
// Dropping the error from each lookup makes a locked or corrupt database
// indistinguishable from "nothing has ever been recorded": every row silently
// loses its history block while the envelope still claims
// history_available: true. This records what actually happened so the command
// can say so once, on stderr, and stop advertising history it could not read.
type wowHistoryProbe struct {
	attempts int
	failures int
	firstErr error
}

// record notes one lookup outcome and reports whether it succeeded.
func (p *wowHistoryProbe) record(err error) bool {
	p.attempts++
	if err != nil {
		p.failures++
		if p.firstErr == nil {
			p.firstErr = err
		}
		return false
	}
	return true
}

// available reports whether history should still be advertised. It is false
// when lookups were attempted and every single one of them failed, because a
// database nothing can be read out of is not available history.
func (p *wowHistoryProbe) available(haveDB bool) bool {
	if !haveDB {
		return false
	}
	return p.attempts == 0 || p.failures < p.attempts
}

// warn prints ONE stderr line naming the first error when any lookup failed.
func (p *wowHistoryProbe) warn(command string) {
	if p.failures == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: warning: %d of %d local price-history lookup(s) failed, so the affected rows carry no history comparison; this is a database error, not an empty history. first error: %v\n",
		command, p.failures, p.attempts, p.firstErr)
}

// ---------------------------------------------------------------------------
// swap
// ---------------------------------------------------------------------------

// swapAlternative is one ranked substitute for the anchor product.
type swapAlternative struct {
	Stockcode         string         `json:"stockcode"`
	Name              string         `json:"name"`
	Brand             string         `json:"brand,omitempty"`
	Price             float64        `json:"price"`
	PackageSize       string         `json:"package_size,omitempty"`
	IsOnSpecial       bool           `json:"is_on_special"`
	IsHalfPrice       bool           `json:"is_half_price"`
	UnitPrice         float64        `json:"unit_price"`
	UnitBasis         string         `json:"unit_basis"`
	MeasureKind       string         `json:"measure_kind"`
	ShelfCupString    string         `json:"shelf_cup_string,omitempty"`
	SavingPct         float64        `json:"saving_pct_vs_anchor"`
	SavingPerBasis    float64        `json:"saving_per_basis_vs_anchor"`
	CheaperThanAnchor bool           `json:"cheaper_than_anchor"`
	History           *wowHistoryLow `json:"history,omitempty"`
}

// swapAnchor is the product every alternative is measured against.
type swapAnchor struct {
	Stockcode      string         `json:"stockcode"`
	Name           string         `json:"name"`
	Brand          string         `json:"brand,omitempty"`
	Price          float64        `json:"price"`
	PackageSize    string         `json:"package_size,omitempty"`
	IsOnSpecial    bool           `json:"is_on_special"`
	UnitPrice      float64        `json:"unit_price"`
	UnitBasis      string         `json:"unit_basis,omitempty"`
	MeasureKind    string         `json:"measure_kind"`
	ShelfCupString string         `json:"shelf_cup_string,omitempty"`
	History        *wowHistoryLow `json:"history,omitempty"`
}

type swapEnvelope struct {
	Query                string            `json:"query"`
	Anchor               *swapAnchor       `json:"anchor"`
	Alternatives         []swapAlternative `json:"alternatives"`
	CheaperCount         int               `json:"cheaper_count"`
	ScannedProducts      int               `json:"scanned_products"`
	ScannedPages         int               `json:"scanned_pages"`
	ExcludedIncomparable int               `json:"excluded_incomparable"`
	ExcludedUnavailable  int               `json:"excluded_unavailable"`
	ExcludedUnparseable  int               `json:"excluded_unparseable_measure"`
	HistoryAvailable     bool              `json:"history_available"`
	FetchFailures        []wowFetchFailure `json:"fetch_failures"`
	Note                 string            `json:"note,omitempty"`
}

func newNovelSwapCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLimit        int
		flagDB           string
		flagMaxScanPages int
	)

	cmd := &cobra.Command{
		Use:   "swap <term|stockcode>",
		Short: "Ranks in-stock alternatives by unit price normalised across different measure bases",
		Long: strings.Trim(`
Finds cheaper equivalents for a product by ranking alternatives on a unit price
that has been normalised to one basis per measure kind.

Woolworths quotes each tile's cup price against whatever measure it likes:
"100G" on one tile, "1KG" on the next, "1L", "100ML" or "1EA" on the ones after.
A raw CupPrice ranking is therefore wrong the moment two bases differ, because
$3.00/100G is $30.00 per kilo and loses to $25.00/1KG.

swap normalises every candidate to price per 1kg, per 1L, or per countable unit
before ranking, and it refuses to rank across kinds: an alternative whose
measure kind differs from the anchor is excluded and counted in
excluded_incomparable rather than quietly dropped. Products with an unparseable
measure are excluded too, because a zero unit price would sort them to the front
as the apparent bargain.

Where the local price-history database has seen an alternative before, each row
also reports how its current price compares with its own recorded low.
`, "\n"),
		Example: strings.Trim(`
  # Cheaper equivalents for the first olive oil result
  woolworths-pp-cli swap "olive oil" --limit 5

  # Anchor on a specific product by stockcode
  woolworths-pp-cli swap 698861 --limit 10 --json

  # Widen the local filter pass over more pages of results
  woolworths-pp-cli swap "greek yoghurt" --max-scan-pages 8 --agent
`, "\n"),
		Annotations: map[string]string{
			// Empty is a valid answer, not an error: a nonsense term is indistinguishable from a valid term with no comparable alternatives.
			// Inventing an "empty means not found" rule would break honest-empty output.
			"pp:no-error-path-probe": "true",
			"mcp:read-only":          "true",
			"pp:happy-args":          "term=milk",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "swap")
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<term|stockcode> is required"))
			}
			query := strings.TrimSpace(args[0])
			limit := flagLimit
			if limit <= 0 {
				limit = 10
			}
			maxPages := wowResolveMaxScanPages(flagMaxScanPages)

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Akamai drops unwarmed POST connections; one GET of /shop primes the jar.
			c.WarmSession(ctx)

			// Resolve the anchor. A numeric positional is a stockcode, which
			// the search endpoint resolves to exactly one tile; the ranking
			// pass then re-searches on that product's own name.
			anchorTile, searchTerm, err := swapResolveAnchor(ctx, c, query)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}

			env := swapEnvelope{
				Query:         query,
				Alternatives:  make([]swapAlternative, 0, limit),
				FetchFailures: make([]wowFetchFailure, 0),
			}

			if anchorTile == nil {
				if wowLooksNumeric(query) {
					env.Note = fmt.Sprintf("stockcode %s was not among the results the search endpoint returned for it, so there is no anchor; nothing is ranked rather than reporting savings against a substituted product you did not name - check the stockcode, or pass the product name instead", query)
				} else {
					env.Note = fmt.Sprintf("no product matched %q, so there is no anchor to compare against; check the spelling or try a broader term", query)
				}
				return swapEmit(cmd, flags, env)
			}

			anchorNorm, anchorOK := wowNormalizeTile(*anchorTile)
			env.Anchor = &swapAnchor{
				Stockcode:      anchorTile.stockcode(),
				Name:           anchorTile.displayName(),
				Brand:          anchorTile.Brand,
				Price:          roundMoney(anchorTile.Price),
				PackageSize:    anchorTile.PackageSize,
				IsOnSpecial:    anchorTile.IsOnSpecial,
				MeasureKind:    anchorNorm.Kind.String(),
				ShelfCupString: anchorTile.CupString,
			}
			if !anchorOK {
				env.Note = fmt.Sprintf("anchor %q (stockcode %s) has no parseable unit measure (CupMeasure=%q, PackageSize=%q), so no like-for-like ranking is possible; nothing is ranked rather than ranking on a guessed basis",
					anchorTile.displayName(), anchorTile.stockcode(), anchorTile.CupMeasure, anchorTile.PackageSize)
				return swapEmit(cmd, flags, env)
			}
			env.Anchor.UnitPrice = anchorNorm.Price
			env.Anchor.UnitBasis = anchorNorm.Basis

			scan := wowScanSearch(ctx, c, searchTerm, maxPages, wowMaxPageSize, "TraderRelevance")
			env.FetchFailures = append(env.FetchFailures, scan.Failures...)
			env.ScannedPages = scan.Pages
			env.ScannedProducts = len(scan.Tiles)

			db, closeDB, haveDB := wowOpenHistoryDB(ctx, flagDB)
			defer closeDB()
			var histProbe wowHistoryProbe
			if haveDB {
				hist, herr := wowRecordedLow(ctx, db, anchorTile.stockcode(), anchorTile.Price)
				if histProbe.record(herr) {
					env.Anchor.History = hist
				}
			}

			ranked := make([]swapAlternative, 0, len(scan.Tiles))
			for _, tile := range scan.Tiles {
				if tile.stockcode() == anchorTile.stockcode() {
					continue
				}
				if !tile.available() {
					env.ExcludedUnavailable++
					continue
				}
				norm, ok := wowNormalizeTile(tile)
				if !ok {
					env.ExcludedUnparseable++
					continue
				}
				if !unitprice.Comparable(anchorNorm, norm) {
					env.ExcludedIncomparable++
					continue
				}
				alt := swapAlternative{
					Stockcode:      tile.stockcode(),
					Name:           tile.displayName(),
					Brand:          tile.Brand,
					Price:          roundMoney(tile.Price),
					PackageSize:    tile.PackageSize,
					IsOnSpecial:    tile.IsOnSpecial,
					IsHalfPrice:    tile.IsHalfPrice,
					UnitPrice:      norm.Price,
					UnitBasis:      norm.Basis,
					MeasureKind:    norm.Kind.String(),
					ShelfCupString: tile.CupString,
				}
				if pct, ok := unitprice.PercentCheaper(anchorNorm, norm); ok {
					alt.SavingPct = pct
					alt.SavingPerBasis = roundMoney(anchorNorm.Price - norm.Price)
					alt.CheaperThanAnchor = norm.Price < anchorNorm.Price
				}
				ranked = append(ranked, alt)
			}

			// Ascending normalised unit price. The stockcode tie-break keeps
			// the ordering stable across runs.
			sort.SliceStable(ranked, func(i, j int) bool {
				if ranked[i].UnitPrice != ranked[j].UnitPrice {
					return ranked[i].UnitPrice < ranked[j].UnitPrice
				}
				return ranked[i].Stockcode < ranked[j].Stockcode
			})
			for _, alt := range ranked {
				if alt.CheaperThanAnchor {
					env.CheaperCount++
				}
			}
			if len(ranked) > limit {
				ranked = ranked[:limit]
			}
			if haveDB {
				for i := range ranked {
					hist, herr := wowRecordedLow(ctx, db, ranked[i].Stockcode, ranked[i].Price)
					if histProbe.record(herr) {
						ranked[i].History = hist
					}
				}
			}
			env.HistoryAvailable = histProbe.available(haveDB)
			histProbe.warn("swap")
			env.Alternatives = ranked

			if len(env.Alternatives) == 0 {
				env.Note = fmt.Sprintf("no comparable alternative found in %d scanned product(s) across %d page(s); widen the local filter pass with --max-scan-pages (currently %d) or use a broader term",
					env.ScannedProducts, env.ScannedPages, maxPages)
			}
			wowWarnFailures("swap", env.FetchFailures, env.ScannedPages)
			return swapEmit(cmd, flags, env)
		},
	}

	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum alternatives to return")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", wowDefaultMaxScanPages,
		"Maximum result pages to scan and filter locally; raise it to widen the search")
	return cmd
}

// swapResolveAnchor picks the product every alternative is ranked against, and
// returns the term the ranking pass should search on.
//
// A numeric positional is a stockcode, i.e. a request for ONE specific product.
// When page 1 carries no tile with that stockcode there is no anchor: falling
// through to tiles[0] would report savings against a product the caller never
// named, which is worse than returning nothing. This mirrors real-special,
// which drops every live result whose stockcode is not the one asked for.
func swapResolveAnchor(ctx context.Context, c *client.Client, query string) (*wowTile, string, error) {
	resp, err := wowSearchPage(ctx, c, query, 1, wowMaxPageSize, "TraderRelevance")
	if err != nil {
		return nil, query, err
	}
	tiles := resp.tiles()
	if len(tiles) == 0 {
		return nil, query, nil
	}
	if wowLooksNumeric(query) {
		for i := range tiles {
			if tiles[i].stockcode() == query {
				// Rank against products that look like this one, which means
				// searching on its own name rather than on its stockcode.
				return &tiles[i], swapTermForTile(tiles[i]), nil
			}
		}
		return nil, query, nil
	}
	return &tiles[0], query, nil
}

// swapTermForTile builds a search term from a resolved product. The brand is
// dropped because the point of a swap is to leave the brand behind; the pack
// size is not part of Name, which is what makes pack size free to vary.
func swapTermForTile(t wowTile) string {
	name := t.Name
	if strings.TrimSpace(name) == "" {
		name = t.DisplayName
	}
	if brand := strings.TrimSpace(t.Brand); brand != "" {
		if trimmed := strings.TrimSpace(strings.TrimPrefix(name, brand)); trimmed != "" {
			name = trimmed
		}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return t.stockcode()
	}
	return name
}

func swapEmit(cmd *cobra.Command, flags *rootFlags, env swapEnvelope) error {
	out := cmd.OutOrStdout()
	if !wantsHumanTable(out, flags) {
		return printJSONFiltered(out, env, flags)
	}
	if env.Anchor == nil {
		fmt.Fprintf(out, "%s\n", env.Note)
		return nil
	}
	fmt.Fprintf(out, "anchor: %s", env.Anchor.Name)
	if env.Anchor.Brand != "" {
		fmt.Fprintf(out, " (%s)", env.Anchor.Brand)
	}
	fmt.Fprintf(out, "  $%.2f", env.Anchor.Price)
	if env.Anchor.UnitBasis != "" {
		fmt.Fprintf(out, "  $%s/%s", formatUnitPrice(env.Anchor.UnitPrice), env.Anchor.UnitBasis)
	}
	fmt.Fprintln(out)
	if len(env.Alternatives) == 0 {
		fmt.Fprintf(out, "%s\n", env.Note)
		return nil
	}
	rows := make([]map[string]any, 0, len(env.Alternatives))
	for _, alt := range env.Alternatives {
		row := map[string]any{
			"name":       alt.Name,
			"brand":      alt.Brand,
			"price":      alt.Price,
			"pack":       alt.PackageSize,
			"unit_price": fmt.Sprintf("$%s/%s", formatUnitPrice(alt.UnitPrice), alt.UnitBasis),
			"saving_pct": alt.SavingPct,
			"special":    alt.IsOnSpecial,
		}
		if alt.History != nil {
			if alt.History.AtRecordedLow {
				row["vs_own_low"] = "at recorded low"
			} else {
				row["vs_own_low"] = fmt.Sprintf("+$%.2f (%.1f%%)", alt.History.AboveLowByAmount, alt.History.AboveLowByPct)
			}
		}
		rows = append(rows, row)
	}
	if err := printAutoTable(out, rows); err != nil {
		return err
	}
	fmt.Fprintf(out, "\nscanned %d product(s) over %d page(s); excluded %d incomparable, %d unavailable, %d with an unparseable measure\n",
		env.ScannedProducts, env.ScannedPages, env.ExcludedIncomparable, env.ExcludedUnavailable, env.ExcludedUnparseable)
	if !env.HistoryAvailable {
		fmt.Fprintln(out, "no local price history yet; run 'woolworths-pp-cli real-special <term>' over several weeks to record it (sync does not build price history on this API)")
	}
	return nil
}
