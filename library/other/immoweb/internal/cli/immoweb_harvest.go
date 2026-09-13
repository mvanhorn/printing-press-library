// Price-band harvesting: the map endpoint returns at most 200 listings and
// ignores paging, but it honours minPrice/maxPrice. Counting is nearly free
// (a 25-byte response), so an area is split into contiguous price bands of
// <=200 listings each and every band takes one map call. This retrieves the
// whole area in a handful of requests instead of 30-listing search pages.

package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/immo"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/store"
)

// priceBand is an inclusive [Min, Max] price range with its listing count.
// Max == 0 means unbounded.
type priceBand struct {
	Min, Max int
	Count    int
}

// bandCap keeps each band below the map endpoint's 200-result cap so a
// listing added between the count and the fetch cannot be truncated away.
const bandCap = 180

// maxBands caps map calls per harvest; maxCountCalls caps the count calls
// band planning may spend before falling back to search paging.
const (
	maxBands      = 80
	maxCountCalls = 160
)

var rentEdges = []int{0, 500, 650, 750, 850, 950, 1050, 1150, 1250, 1350, 1500, 1700, 1900, 2200, 2600, 3200, 4500}
var saleEdges = []int{0, 100000, 150000, 200000, 240000, 280000, 320000, 360000, 400000, 450000, 500000, 575000, 650000, 750000, 900000, 1100000, 1500000, 2500000}

func withBand(params map[string]string, b priceBand) map[string]string {
	out := make(map[string]string, len(params)+2)
	for k, v := range params {
		out[k] = v
	}
	delete(out, "minPrice")
	delete(out, "maxPrice")
	if b.Min > 0 {
		out["minPrice"] = strconv.Itoa(b.Min)
	}
	if b.Max > 0 {
		out["maxPrice"] = strconv.Itoa(b.Max)
	}
	return out
}

// planPriceBands counts initial bands in parallel, splits bands above the
// map cap, and merges adjacent small bands. It returns the bands and the
// number of listings they cover (listings without a published price are
// not reachable through price filters).
func planPriceBands(ctx context.Context, c *client.Client, params map[string]string, total int) ([]priceBand, int, int, error) {
	edges := saleEdges
	if params["transactionTypes"] == "FOR_RENT" {
		edges = rentEdges
	}
	// Start with roughly one band per 150 listings (evenly spread over the
	// static edge table) so small areas spend few count calls.
	if want := total/150 + 2; want < len(edges) {
		picked := make([]int, 0, want)
		for i := 0; i < want; i++ {
			picked = append(picked, edges[i*(len(edges)-1)/(want-1)])
		}
		edges = picked
	}
	userMin, _ := strconv.Atoi(params["minPrice"])
	userMax, _ := strconv.Atoi(params["maxPrice"])
	initial := []priceBand{}
	for i, lo := range edges {
		hi := 0
		if i+1 < len(edges) {
			hi = edges[i+1] - 1
		}
		if userMax > 0 && lo > userMax {
			break
		}
		if userMin > 0 && hi > 0 && hi < userMin {
			continue
		}
		b := priceBand{Min: max(lo, userMin), Max: hi}
		if userMax > 0 && (b.Max == 0 || b.Max > userMax) {
			b.Max = userMax
		}
		initial = append(initial, b)
	}
	calls := 0
	var mu sync.Mutex
	var firstErr error
	var count func(b *priceBand)
	count = func(b *priceBand) {
		n, err := fetchCount(ctx, c, withBand(params, *b))
		mu.Lock()
		calls++
		if err != nil && firstErr == nil {
			firstErr = err
		}
		mu.Unlock()
		b.Count = n
	}
	parallel(len(initial), 6, func(i int) { count(&initial[i]) })
	if firstErr != nil {
		return nil, 0, calls, firstErr
	}
	// Split oversized bands until each fits in one map call.
	final := []priceBand{}
	queue := initial
	for depth := 0; len(queue) > 0 && depth < 12 && calls < maxCountCalls; depth++ {
		next := []priceBand{}
		for _, b := range queue {
			if b.Count <= bandCap {
				final = append(final, b)
				continue
			}
			hi := b.Max
			if hi == 0 {
				hi = b.Min*2 + 1000
			}
			if hi-b.Min <= 1 {
				final = append(final, b) // cannot split further; map returns 200 of them
				continue
			}
			mid := b.Min + (hi-b.Min)/2
			next = append(next, priceBand{Min: b.Min, Max: mid}, priceBand{Min: mid + 1, Max: b.Max})
		}
		parallel(len(next), 6, func(i int) { count(&next[i]) })
		if firstErr != nil {
			return nil, 0, calls, firstErr
		}
		queue = next
	}
	final = append(final, queue...)
	sort.Slice(final, func(i, j int) bool { return final[i].Min < final[j].Min })
	// Merge adjacent bands while the merged count stays within one map call.
	merged := []priceBand{}
	for _, b := range final {
		if b.Count == 0 {
			continue
		}
		if n := len(merged); n > 0 && merged[n-1].Count+b.Count <= bandCap && merged[n-1].Max != 0 && merged[n-1].Max+1 == b.Min {
			merged[n-1].Max = b.Max
			merged[n-1].Count += b.Count
			continue
		}
		merged = append(merged, b)
	}
	covered := 0
	for _, b := range merged {
		covered += b.Count
	}
	return merged, covered, calls, nil
}

// harvestRate is the request ceiling used by area-harvesting commands when
// the user did not pass --rate-limit. Count calls return ~25 bytes and map
// calls ~25 KB gzipped, so the generic 2 req/s auto start would make a
// commune harvest take tens of seconds. HTTP 429s still back off through the
// adaptive limiter; soft throttling (empty pages) is retried in fetchResults.
const harvestRate = 8.0

// useHarvestRate raises the request ceiling for harvesting commands unless
// the user chose one with --rate-limit.
func useHarvestRate(cmd *cobra.Command, flags *rootFlags) {
	if f := cmd.Flag("rate-limit"); f == nil || !f.Changed {
		flags.rateLimit = harvestRate
	}
}

// parallel runs fn(0..n-1) with at most limit goroutines.
func parallel(n, limit int, fn func(i int)) {
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			fn(i)
		}(i)
	}
	wg.Wait()
}

// harvestBands fetches every band through the map endpoint (4 at a time),
// then writes sequentially so SQLite sees a single writer.
func harvestBands(ctx context.Context, c *client.Client, db *store.Store, params map[string]string, bands []priceBand) ([]immo.Listing, int, error) {
	type result struct {
		sp  searchPage
		err error
	}
	results := make([]result, len(bands))
	parallel(len(bands), 4, func(i int) {
		sp, err := fetchResults(ctx, c, "/en/search-results-map", withBand(params, bands[i]), 1, immoMapSize, bands[i].Count)
		results[i] = result{sp: sp, err: err}
	})
	seen := map[int64]bool{}
	out := []immo.Listing{}
	failures := 0
	var firstErr error
	for _, r := range results {
		if r.err != nil {
			failures++
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		if db != nil && len(r.sp.Items) > 0 {
			if err := db.UpsertImmoListings(ctx, r.sp.Items, r.sp.Raw, time.Now()); err != nil {
				return out, len(bands), err
			}
		}
		for _, l := range r.sp.Items {
			if !seen[l.ID] {
				seen[l.ID] = true
				out = append(out, l)
			}
		}
	}
	if failures > 0 && failures == len(bands) {
		return out, len(bands), firstErr
	}
	if failures > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d of %d price bands failed (%v); results cover the remaining %d listings\n", failures, len(bands), firstErr, len(out))
	}
	return out, len(bands), nil
}
