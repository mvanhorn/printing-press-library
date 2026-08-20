// Change-tracking commands — price-cuts, dom, velocity, delisted. Each is
// powered by the append-only ln_observations table: LoopNet shows only the
// current snapshot, so these are only possible because repeated syncs
// accumulate a local time series.
package cli

import (
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/loopnet/internal/loopnet"
)

// priceCutRecord is one detected asking-price drop on a listing.
type priceCutRecord struct {
	ListingID         string  `json:"listing_id"`
	Address           string  `json:"address,omitempty"`
	OldPrice          float64 `json:"old_price"`
	NewPrice          float64 `json:"new_price"`
	PctCut            float64 `json:"pct_cut"`
	CutDate           string  `json:"cut_date"`
	DaysOnMarketAtCut int     `json:"days_on_market_at_cut"`
	URL               string  `json:"url,omitempty"`
}

// computePriceCuts walks per-listing observation history (already ordered by
// listing then time by loadObservations) and returns the most recent
// asking-price drop for every listing that has one.
func computePriceCuts(obs []lnObservation) []priceCutRecord {
	byListing := map[string][]lnObservation{}
	var order []string
	for _, o := range obs {
		if _, ok := byListing[o.ListingID]; !ok {
			order = append(order, o.ListingID)
		}
		byListing[o.ListingID] = append(byListing[o.ListingID], o)
	}
	var cuts []priceCutRecord
	for _, id := range order {
		hist := byListing[id]
		if len(hist) == 0 {
			continue
		}
		firstSeen := hist[0].ObservedAt
		var prev float64
		havePrev := false
		var latest *priceCutRecord
		for _, o := range hist {
			if !o.HasPrice || o.Price <= 0 {
				continue
			}
			if havePrev && o.Price < prev {
				rec := priceCutRecord{
					ListingID:         id,
					OldPrice:          prev,
					NewPrice:          o.Price,
					PctCut:            round2((prev - o.Price) / prev * 100),
					CutDate:           o.ObservedAt.Format(time.RFC3339),
					DaysOnMarketAtCut: int(o.ObservedAt.Sub(firstSeen).Hours() / 24),
				}
				latest = &rec
			}
			prev = o.Price
			havePrev = true
		}
		if latest != nil {
			cuts = append(cuts, *latest)
		}
	}
	return cuts
}

// listingsAtSync returns the set of listing ids observed at sync time t.
func listingsAtSync(obs []lnObservation, t time.Time) map[string]bool {
	out := map[string]bool{}
	for _, o := range obs {
		if o.ObservedAt.Unix() == t.Unix() {
			out[o.ListingID] = true
		}
	}
	return out
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

// --- price-cuts -------------------------------------------------------------

func newPriceCutsCmd(flags *rootFlags) *cobra.Command {
	var market lnMarketFlags
	var since string

	cmd := &cobra.Command{
		Use:   "price-cuts <location>",
		Short: "List synced listings whose asking price dropped between syncs",
		Long: `price-cuts surfaces every listing in a synced submarket whose asking price
fell between two syncs — the strongest deal-sentiment signal LoopNet itself
never exposes. Needs at least two syncs of the same submarket over time.`,
		Example: `  loopnet-pp-cli price-cuts worcester-ma --type industrial
  loopnet-pp-cli price-cuts worcester-ma --since 30d --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			location := loopnet.SlugLocation(args[0])
			cutoff, err := parseSinceWindow(since)
			if err != nil {
				return usageErr(err)
			}
			st, err := lnOpenStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()

			obs, err := loadObservations(st.DB(), location, market.propertyType, market.listingType)
			if err != nil {
				return apiErr(err)
			}
			if len(obs) == 0 {
				return lnNoData(location)
			}
			cuts := computePriceCuts(obs)
			listings, _ := lnListings(st, location, market.propertyType, market.listingType)
			addr := map[string]loopnet.Listing{}
			for _, l := range listings {
				addr[l.ID] = l
			}
			out := make([]priceCutRecord, 0, len(cuts))
			for _, c := range cuts {
				ct, _ := time.Parse(time.RFC3339, c.CutDate)
				if !cutoff.IsZero() && ct.Before(cutoff) {
					continue
				}
				if l, ok := addr[c.ListingID]; ok {
					c.Address = l.Name
					c.URL = l.URL
				}
				out = append(out, c)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].PctCut > out[j].PctCut })

			return flags.printJSON(cmd, map[string]any{
				"location":   location,
				"count":      len(out),
				"price_cuts": out,
				"note":       priceCutsNote(len(obs), out),
			})
		},
	}
	addMarketFlags(cmd, &market)
	cmd.Flags().StringVar(&since, "since", "", "Only cuts within this window (e.g. 7d, 24h, 2w)")
	return cmd
}

func priceCutsNote(obsCount int, cuts []priceCutRecord) string {
	if len(cuts) == 0 {
		return "No price cuts found. price-cuts compares prices across syncs — run 'sync' again later to build history."
	}
	return ""
}

// --- dom (days on market) ---------------------------------------------------

func newDomCmd(flags *rootFlags) *cobra.Command {
	var market lnMarketFlags
	var minDays, limit int

	cmd := &cobra.Command{
		Use:   "dom <location>",
		Short: "Compute true days-on-market for currently-listed properties",
		Long: `dom computes how long each currently-listed property has been on the
market, measured from the first sync that saw it. LoopNet hides days-on-
market entirely; this is synthesized from the local observation history.`,
		Example: `  loopnet-pp-cli dom worcester-ma --type industrial
  loopnet-pp-cli dom worcester-ma --min-days 90 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			location := loopnet.SlugLocation(args[0])
			st, err := lnOpenStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()

			obs, err := loadObservations(st.DB(), location, market.propertyType, market.listingType)
			if err != nil {
				return apiErr(err)
			}
			if len(obs) == 0 {
				return lnNoData(location)
			}
			times, _ := lnSyncTimes(st.DB(), location, market.propertyType, market.listingType)
			if len(times) == 0 {
				return lnNoData(location)
			}
			present := listingsAtSync(obs, times[0])

			first := map[string]time.Time{}
			for _, o := range obs {
				if t, ok := first[o.ListingID]; !ok || o.ObservedAt.Before(t) {
					first[o.ListingID] = o.ObservedAt
				}
			}
			listings, _ := lnListings(st, location, market.propertyType, market.listingType)
			addr := map[string]loopnet.Listing{}
			for _, l := range listings {
				addr[l.ID] = l
			}

			type domRow struct {
				ListingID    string  `json:"listing_id"`
				Address      string  `json:"address,omitempty"`
				Price        float64 `json:"price,omitempty"`
				FirstSeen    string  `json:"first_seen"`
				DaysOnMarket int     `json:"days_on_market"`
				URL          string  `json:"url,omitempty"`
			}
			rows := []domRow{}
			for id := range present {
				fs := first[id]
				d := lnDaysSince(fs)
				if d < minDays {
					continue
				}
				r := domRow{ListingID: id, FirstSeen: fs.Format(time.RFC3339), DaysOnMarket: d}
				if l, ok := addr[id]; ok {
					r.Address, r.Price, r.URL = l.Name, l.Price, l.URL
				}
				rows = append(rows, r)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].DaysOnMarket > rows[j].DaysOnMarket })
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return flags.printJSON(cmd, map[string]any{
				"location": location,
				"count":    len(rows),
				"listings": rows,
				"note":     "Days-on-market is measured from the CLI's first sync of each listing; it reflects observed history, not LoopNet's true list date.",
			})
		},
	}
	addMarketFlags(cmd, &market)
	cmd.Flags().IntVar(&minDays, "min-days", 0, "Only show listings on market at least this many days")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	return cmd
}

// --- velocity ---------------------------------------------------------------

func newVelocityCmd(flags *rootFlags) *cobra.Command {
	var market lnMarketFlags

	cmd := &cobra.Command{
		Use:   "velocity <location>",
		Short: "Report submarket absorption: new listings, delistings, net supply change",
		Long: `velocity compares the two most recent syncs of a submarket and reports
absorption: new listings, delistings, net supply change, and the median
days-on-market of what is currently listed. Needs at least two syncs.`,
		Example:     `  loopnet-pp-cli velocity worcester-ma --type industrial --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			location := loopnet.SlugLocation(args[0])
			st, err := lnOpenStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()

			obs, err := loadObservations(st.DB(), location, market.propertyType, market.listingType)
			if err != nil {
				return apiErr(err)
			}
			if len(obs) == 0 {
				return lnNoData(location)
			}
			times, _ := lnSyncTimes(st.DB(), location, market.propertyType, market.listingType)
			if len(times) == 0 {
				return lnNoData(location)
			}

			latest := listingsAtSync(obs, times[0])
			first := map[string]time.Time{}
			for _, o := range obs {
				if t, ok := first[o.ListingID]; !ok || o.ObservedAt.Before(t) {
					first[o.ListingID] = o.ObservedAt
				}
			}
			var doms []float64
			for id := range latest {
				doms = append(doms, float64(lnDaysSince(first[id])))
			}

			result := map[string]any{
				"location":              location,
				"syncs_recorded":        len(times),
				"latest_sync":           times[0].Format(time.RFC3339),
				"listings_now":          len(latest),
				"median_days_on_market": median(doms),
			}
			if len(times) < 2 {
				result["note"] = "Only one sync recorded — new/delisted counts need at least two syncs. Run 'sync' again later."
				result["new_listings"] = 0
				result["delisted"] = 0
				result["net_change"] = 0
				return flags.printJSON(cmd, result)
			}
			prior := listingsAtSync(obs, times[1])
			newCount, delisted := 0, 0
			for id := range latest {
				if !prior[id] {
					newCount++
				}
			}
			for id := range prior {
				if !latest[id] {
					delisted++
				}
			}
			result["prior_sync"] = times[1].Format(time.RFC3339)
			result["listings_prior"] = len(prior)
			result["new_listings"] = newCount
			result["delisted"] = delisted
			result["net_change"] = len(latest) - len(prior)
			return flags.printJSON(cmd, result)
		},
	}
	addMarketFlags(cmd, &market)
	return cmd
}

// --- delisted ---------------------------------------------------------------

func newDelistedCmd(flags *rootFlags) *cobra.Command {
	var market lnMarketFlags
	var since string

	cmd := &cobra.Command{
		Use:   "delisted <location>",
		Short: "List listings present in a prior sync but absent from the latest",
		Long: `delisted reports listings that were present in an earlier sync of a
submarket but are gone from the most recent one — sold, withdrawn, or
expired. A proxy for transaction velocity. Needs at least two syncs.`,
		Example: `  loopnet-pp-cli delisted worcester-ma --type industrial
  loopnet-pp-cli delisted worcester-ma --since 30d --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			location := loopnet.SlugLocation(args[0])
			cutoff, err := parseSinceWindow(since)
			if err != nil {
				return usageErr(err)
			}
			st, err := lnOpenStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()

			obs, err := loadObservations(st.DB(), location, market.propertyType, market.listingType)
			if err != nil {
				return apiErr(err)
			}
			if len(obs) == 0 {
				return lnNoData(location)
			}
			times, _ := lnSyncTimes(st.DB(), location, market.propertyType, market.listingType)
			if len(times) < 2 {
				return flags.printJSON(cmd, map[string]any{
					"location": location, "count": 0, "delisted": []any{},
					"note": "Only one sync recorded — delisting detection needs at least two syncs. Run 'sync' again later.",
				})
			}
			latest := listingsAtSync(obs, times[0])

			first := map[string]time.Time{}
			last := map[string]time.Time{}
			lastPrice := map[string]float64{}
			for _, o := range obs {
				if t, ok := first[o.ListingID]; !ok || o.ObservedAt.Before(t) {
					first[o.ListingID] = o.ObservedAt
				}
				if t, ok := last[o.ListingID]; !ok || o.ObservedAt.After(t) {
					last[o.ListingID] = o.ObservedAt
				}
				if o.HasPrice {
					lastPrice[o.ListingID] = o.Price
				}
			}
			listings, _ := lnListings(st, location, market.propertyType, market.listingType)
			addr := map[string]loopnet.Listing{}
			for _, l := range listings {
				addr[l.ID] = l
			}

			type delistedRow struct {
				ListingID  string  `json:"listing_id"`
				Address    string  `json:"address,omitempty"`
				LastPrice  float64 `json:"last_price,omitempty"`
				FirstSeen  string  `json:"first_seen"`
				LastSeen   string  `json:"last_seen"`
				DaysListed int     `json:"days_listed"`
				URL        string  `json:"url,omitempty"`
			}
			rows := []delistedRow{}
			for id, ls := range last {
				if latest[id] {
					continue
				}
				if !cutoff.IsZero() && ls.Before(cutoff) {
					continue
				}
				r := delistedRow{
					ListingID:  id,
					LastPrice:  lastPrice[id],
					FirstSeen:  first[id].Format(time.RFC3339),
					LastSeen:   ls.Format(time.RFC3339),
					DaysListed: int(ls.Sub(first[id]).Hours() / 24),
				}
				if l, ok := addr[id]; ok {
					r.Address, r.URL = l.Name, l.URL
				}
				rows = append(rows, r)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].LastSeen > rows[j].LastSeen })
			return flags.printJSON(cmd, map[string]any{
				"location": location,
				"count":    len(rows),
				"delisted": rows,
			})
		},
	}
	addMarketFlags(cmd, &market)
	cmd.Flags().StringVar(&since, "since", "", "Only listings last seen within this window (e.g. 30d, 2w)")
	return cmd
}
