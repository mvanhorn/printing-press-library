// Analyst and pipeline workflows — digest and feed. digest rolls a synced
// submarket into a single intelligence report; feed exports a synced
// submarket as a clean, run-stamped JSON/CSV file for an external pipeline.
package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/loopnet/internal/loopnet"
)

// --- digest -----------------------------------------------------------------

func newDigestCmd(flags *rootFlags) *cobra.Command {
	var market lnMarketFlags

	cmd := &cobra.Command{
		Use:   "digest <location>",
		Short: "One-command submarket roll-up: supply, price cuts, DOM, churn, distress",
		Long: `digest rolls a synced submarket into a single report: live supply count,
recent price-cut count, median days-on-market, new- and delisted-listing
counts, and the number of listings carrying motivated-seller signals. It
joins the local store's listings, price history, and observation snapshots
the way the individual tracking commands do — one call instead of six.

Needs at least two syncs for the new/delisted counts. Run 'sync' again
later to build the history.`,
		Example:     `  loopnet-pp-cli digest worcester-ma --type industrial --agent`,
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

			// Recent price cuts — same computation price-cuts and distress use.
			recentCut := map[string]bool{}
			cutoff := time.Now().Add(-90 * 24 * time.Hour)
			for _, c := range computePriceCuts(obs) {
				if t, err := time.Parse(time.RFC3339, c.CutDate); err == nil && t.After(cutoff) {
					recentCut[c.ListingID] = true
				}
			}

			// Distress hits — same scan the distress command runs.
			listings, _ := lnListings(st, location, market.propertyType, market.listingType)
			props, _ := lnProperties(st, location, market.propertyType, market.listingType)
			propByID := map[string]loopnet.Property{}
			for _, p := range props {
				propByID[p.ID] = p
			}
			distressHits := 0
			for _, l := range listings {
				text := l.Description
				auction := false
				if p, ok := propByID[l.ID]; ok {
					text += " " + p.Description + " " + p.SaleType
					auction = p.Auction
				}
				kw, _ := loopnet.HasDistressSignal(text)
				if auction || kw || recentCut[l.ID] {
					distressHits++
				}
			}

			result := map[string]any{
				"location":              location,
				"syncs_recorded":        len(times),
				"latest_sync":           times[0].Format(time.RFC3339),
				"supply":                len(latest),
				"recent_price_cuts":     len(recentCut),
				"median_days_on_market": median(doms),
				"distress_hits":         distressHits,
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
			result["new_listings"] = newCount
			result["delisted"] = delisted
			result["net_change"] = len(latest) - len(prior)
			return flags.printJSON(cmd, result)
		},
	}
	addMarketFlags(cmd, &market)
	return cmd
}

// --- feed -------------------------------------------------------------------

// feedRecord is one export row, with fields grouped by the six CRE
// market-intelligence data categories the feed is designed to populate.
type feedRecord struct {
	RunStamp  string `json:"run_stamp"`
	ListingID string `json:"listing_id"`
	URL       string `json:"url,omitempty"`
	// supply & inventory
	Location     string `json:"location"`
	PropertyType string `json:"property_type"`
	ListingType  string `json:"listing_type"`
	// pricing & deal sentiment
	Price        float64 `json:"price,omitempty"`
	PricePerSqft float64 `json:"price_per_sqft,omitempty"`
	// market velocity
	FirstSeen    string `json:"first_seen,omitempty"`
	DaysOnMarket int    `json:"days_on_market,omitempty"`
	// asset pricing & yield
	CapRate         float64 `json:"cap_rate,omitempty"`
	NOI             float64 `json:"noi,omitempty"`
	TotalAssessment float64 `json:"total_assessment,omitempty"`
	// distress & motivation
	DistressSignal bool `json:"distress_signal"`
	Auction        bool `json:"auction"`
	// asset detail
	Address       string  `json:"address,omitempty"`
	City          string  `json:"city,omitempty"`
	State         string  `json:"state,omitempty"`
	Zip           string  `json:"zip,omitempty"`
	SizeSqft      float64 `json:"size_sqft,omitempty"`
	YearBuilt     string  `json:"year_built,omitempty"`
	BuildingClass string  `json:"building_class,omitempty"`
	BrokerName    string  `json:"broker_name,omitempty"`
	BrokerCompany string  `json:"broker_company,omitempty"`
}

func newFeedCmd(flags *rootFlags) *cobra.Command {
	var market lnMarketFlags
	var format, outPath string

	cmd := &cobra.Command{
		Use:   "feed <location>",
		Short: "Export a synced submarket as a run-stamped JSON or CSV feed",
		Long: `feed exports the latest synced state of a submarket as a run-stamped feed,
with every record carrying fields grouped by the six CRE data categories
(supply, pricing, velocity, yield, distress, asset detail). Designed to drop
straight into an external market-intelligence pipeline's ingest folder.

By default the feed prints to stdout; pass --out to write a file instead.`,
		Example: `  loopnet-pp-cli feed worcester-ma --type industrial --format json
  loopnet-pp-cli feed worcester-ma --format csv --out ./loopnet-worcester.csv`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			location := loopnet.SlugLocation(args[0])
			if format != "json" && format != "csv" {
				return usageErr(fmt.Errorf("--format must be json or csv, got %q", format))
			}
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
			first := map[string]time.Time{}
			for _, o := range obs {
				if t, ok := first[o.ListingID]; !ok || o.ObservedAt.Before(t) {
					first[o.ListingID] = o.ObservedAt
				}
			}

			runStamp := time.Now().UTC().Format(time.RFC3339)
			records := make([]feedRecord, 0, len(listings))
			for _, l := range listings {
				r := feedRecord{
					RunStamp: runStamp, ListingID: l.ID, URL: l.URL,
					Location: l.Location, PropertyType: l.PropertyType, ListingType: l.ListingType,
					Price: l.Price, Address: l.Name, SizeSqft: l.SizeSqft,
					BrokerName: l.BrokerName, BrokerCompany: l.BrokerCompany,
				}
				if fs, ok := first[l.ID]; ok {
					r.FirstSeen = fs.Format(time.RFC3339)
					r.DaysOnMarket = lnDaysSince(fs)
				}
				text := l.Description
				if p, ok := propByID[l.ID]; ok {
					r.PricePerSqft = p.PricePerSqft
					r.CapRate = p.CapRate
					r.NOI = p.NOI
					r.TotalAssessment = p.TotalAssessment
					r.City, r.State, r.Zip = p.City, p.State, p.Zip
					r.YearBuilt, r.BuildingClass = p.YearBuilt, p.BuildingClass
					r.Auction = p.Auction
					text += " " + p.Description + " " + p.SaleType
				}
				if ok, _ := loopnet.HasDistressSignal(text); ok {
					r.DistressSignal = true
				}
				records = append(records, r)
			}

			if format == "csv" {
				return emitFeedCSV(cmd, records, outPath)
			}
			if outPath != "" {
				return writeFeedJSON(cmd, records, outPath)
			}
			return flags.printJSON(cmd, map[string]any{
				"location": location, "run_stamp": runStamp,
				"count": len(records), "records": records,
			})
		},
	}
	addMarketFlags(cmd, &market)
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json or csv")
	cmd.Flags().StringVar(&outPath, "out", "", "Write the feed to this file instead of stdout")
	return cmd
}

var feedCSVHeader = []string{
	"run_stamp", "listing_id", "location", "property_type", "listing_type",
	"price", "price_per_sqft", "first_seen", "days_on_market",
	"cap_rate", "noi", "total_assessment", "distress_signal", "auction",
	"address", "city", "state", "zip", "size_sqft", "year_built",
	"building_class", "broker_name", "broker_company", "url",
}

func (r feedRecord) csvRow() []string {
	f := func(v float64) string {
		if v == 0 {
			return ""
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	d := func(v int) string {
		if v == 0 {
			return ""
		}
		return strconv.Itoa(v)
	}
	return []string{
		r.RunStamp, r.ListingID, r.Location, r.PropertyType, r.ListingType,
		f(r.Price), f(r.PricePerSqft), r.FirstSeen, d(r.DaysOnMarket),
		f(r.CapRate), f(r.NOI), f(r.TotalAssessment),
		strconv.FormatBool(r.DistressSignal), strconv.FormatBool(r.Auction),
		r.Address, r.City, r.State, r.Zip, f(r.SizeSqft), r.YearBuilt,
		r.BuildingClass, r.BrokerName, r.BrokerCompany, r.URL,
	}
}

func emitFeedCSV(cmd *cobra.Command, records []feedRecord, outPath string) error {
	w := cmd.OutOrStdout()
	if outPath != "" {
		f, err := os.Create(outPath)
		if err != nil {
			return apiErr(fmt.Errorf("creating %s: %w", outPath, err))
		}
		defer f.Close()
		w = f
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(feedCSVHeader); err != nil {
		return err
	}
	for _, r := range records {
		if err := cw.Write(r.csvRow()); err != nil {
			return err
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}
	if outPath != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "wrote %d records to %s\n", len(records), outPath)
	}
	return nil
}

func writeFeedJSON(cmd *cobra.Command, records []feedRecord, outPath string) error {
	data, err := json.MarshalIndent(map[string]any{
		"run_stamp": time.Now().UTC().Format(time.RFC3339),
		"count":     len(records),
		"records":   records,
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return apiErr(fmt.Errorf("writing %s: %w", outPath, err))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "wrote %d records to %s\n", len(records), outPath)
	return nil
}
