// Pricing, yield, and distress intelligence — caprate and distress. These
// read detail-grain property records (and the observation history) from the
// local store; LoopNet exposes per-listing facts but never the submarket
// distribution or cross-listing screen.
package cli

import (
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/loopnet/internal/loopnet"
)

// distribution summarizes a slice of numbers for a JSON report.
type distribution struct {
	Count  int     `json:"count"`
	Min    float64 `json:"min,omitempty"`
	P25    float64 `json:"p25,omitempty"`
	Median float64 `json:"median,omitempty"`
	P75    float64 `json:"p75,omitempty"`
	Max    float64 `json:"max,omitempty"`
}

func describe(v []float64) distribution {
	d := distribution{Count: len(v)}
	if len(v) == 0 {
		return d
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	d.Min = s[0]
	d.Max = s[len(s)-1]
	d.Median = round2(median(s))
	d.P25 = round2(quantile(s, 0.25))
	d.P75 = round2(quantile(s, 0.75))
	return d
}

// --- caprate ----------------------------------------------------------------

func newCapRateCmd(flags *rootFlags) *cobra.Command {
	var market lnMarketFlags

	cmd := &cobra.Command{
		Use:   "caprate <location>",
		Short: "Cap-rate, NOI, and price-per-SF distribution for a synced submarket",
		Long: `caprate reports the cap-rate, NOI, and price-per-square-foot distribution
across the detail-grain property records synced for a submarket. LoopNet
shows a single listing's cap rate but never the submarket spread. Each
listing is flagged as a cap-rate outlier when its cap rate falls below the
submarket's first quartile or above its third quartile.

Cap rate and NOI come from listing detail pages — run 'sync' without
--no-details so the property records exist.`,
		Example:     `  loopnet-pp-cli caprate worcester-ma --type industrial --agent`,
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

			props, err := lnProperties(st, location, market.propertyType, market.listingType)
			if err != nil {
				return apiErr(err)
			}
			if len(props) == 0 {
				return lnNoData(location)
			}
			var caps, nois, ppsf []float64
			for _, p := range props {
				if p.CapRate > 0 {
					caps = append(caps, p.CapRate)
				}
				if p.NOI > 0 {
					nois = append(nois, p.NOI)
				}
				if p.PricePerSqft > 0 {
					ppsf = append(ppsf, p.PricePerSqft)
				}
			}
			capDist := describe(caps)
			// Per-listing rows, each flagged when its cap rate falls outside
			// the submarket interquartile range (below Q1 or above Q3).
			type capRow struct {
				ListingID string  `json:"listing_id"`
				Address   string  `json:"address,omitempty"`
				CapRate   float64 `json:"cap_rate_pct"`
				Outlier   bool    `json:"outlier"`
				URL       string  `json:"url,omitempty"`
			}
			rows := []capRow{}
			for _, p := range props {
				if p.CapRate <= 0 {
					continue
				}
				outlier := capDist.Count > 0 &&
					(p.CapRate < capDist.P25 || p.CapRate > capDist.P75)
				rows = append(rows, capRow{
					ListingID: p.ID,
					Address:   p.Address,
					CapRate:   p.CapRate,
					Outlier:   outlier,
					URL:       p.URL,
				})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].CapRate > rows[j].CapRate })
			result := map[string]any{
				"location":       location,
				"properties":     len(props),
				"cap_rate_pct":   capDist,
				"noi":            describe(nois),
				"price_per_sqft": describe(ppsf),
				"listings":       rows,
			}
			if len(caps) == 0 {
				result["note"] = "No cap rates found — the synced listings are mostly owner-user (non-investment) sales, or detail pages were not fetched. Re-run 'sync' without --no-details."
			}
			return flags.printJSON(cmd, result)
		},
	}
	addMarketFlags(cmd, &market)
	return cmd
}

// --- distress ---------------------------------------------------------------

func newDistressCmd(flags *rootFlags) *cobra.Command {
	var market lnMarketFlags

	cmd := &cobra.Command{
		Use:   "distress <location>",
		Short: "Flag synced listings carrying motivated-seller signals",
		Long: `distress sweeps a synced submarket for motivated-seller signals: price-
reduced / must-sell / motivated keyword hits in the listing description,
Ten-X auction listings, and recent price cuts. Keyword matching is
deterministic string search, not classification.`,
		Example:     `  loopnet-pp-cli distress worcester-ma --type industrial --agent`,
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

			listings, err := lnListings(st, location, market.propertyType, market.listingType)
			if err != nil {
				return apiErr(err)
			}
			if len(listings) == 0 {
				return lnNoData(location)
			}
			props, _ := lnProperties(st, location, market.propertyType, market.listingType)
			propByID := map[string]loopnet.Property{}
			for _, p := range props {
				propByID[p.ID] = p
			}
			obs, _ := loadObservations(st.DB(), location, market.propertyType, market.listingType)
			recentCut := map[string]bool{}
			cutoff := time.Now().Add(-90 * 24 * time.Hour)
			for _, c := range computePriceCuts(obs) {
				if t, err := time.Parse(time.RFC3339, c.CutDate); err == nil && t.After(cutoff) {
					recentCut[c.ListingID] = true
				}
			}

			type distressRow struct {
				ListingID string   `json:"listing_id"`
				Address   string   `json:"address,omitempty"`
				Price     float64  `json:"price,omitempty"`
				Signals   []string `json:"signals"`
				URL       string   `json:"url,omitempty"`
			}
			rows := []distressRow{}
			for _, l := range listings {
				var signals []string
				text := l.Description
				if p, ok := propByID[l.ID]; ok {
					text += " " + p.Description + " " + p.SaleType
					if p.Auction {
						signals = append(signals, "auction")
					}
				}
				if ok, kw := loopnet.HasDistressSignal(text); ok {
					signals = append(signals, "keyword: "+kw)
				}
				if recentCut[l.ID] {
					signals = append(signals, "recent price cut")
				}
				if len(signals) == 0 {
					continue
				}
				rows = append(rows, distressRow{
					ListingID: l.ID, Address: l.Name, Price: l.Price, Signals: signals, URL: l.URL,
				})
			}
			sort.Slice(rows, func(i, j int) bool { return len(rows[i].Signals) > len(rows[j].Signals) })
			return flags.printJSON(cmd, map[string]any{
				"location":   location,
				"scanned":    len(listings),
				"count":      len(rows),
				"distressed": rows,
			})
		},
	}
	addMarketFlags(cmd, &market)
	return cmd
}
