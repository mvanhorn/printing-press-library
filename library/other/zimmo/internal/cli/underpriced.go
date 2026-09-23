// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/store"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/zimmo"
)

type underRow struct {
	Code          string   `json:"zimmo_code"`
	Address       string   `json:"address"`
	Type          string   `json:"type"`
	SubType       string   `json:"subtype"`
	Price         *float64 `json:"price"`
	Surface       *float64 `json:"surface_m2"`
	PricePerM2    float64  `json:"price_per_m2"`
	CommunePPM2   float64  `json:"commune_price_per_m2"`
	CommuneSample int      `json:"commune_sample"`
	DiscountPct   float64  `json:"discount_pct"`
	EPC           string   `json:"epc"`
	Condition     string   `json:"condition"`
	DaysOnMarket  *int     `json:"days_on_market"`
	SaleFlags     []string `json:"sale_flags,omitempty"`
	URL           string   `json:"url"`
}

type underView struct {
	Checked        int            `json:"checked"`
	SkippedSpecial int            `json:"skipped_special_sales"`
	Results        []underRow     `json:"results"`
	FetchFailures  []fetchFailure `json:"fetch_failures,omitempty"`
	Note           string         `json:"note,omitempty"`
}

func newNovelUnderpricedCmd(flags *rootFlags) *cobra.Command {
	var postcode, typ, dbPath string
	var below float64
	var limit int
	var includeSpecial bool
	cmd := &cobra.Command{
		Use:   "underpriced",
		Short: "Rank stored for-sale listings by discount to their commune's €/m² for the same property type.",
		Long: `Compares each stored for-sale house or apartment's asking €/m² with Zimmo's
current €/m² for its commune and property type (the price indicator,
cached for 7 days) and ranks by discount. Condition and EPC are shown
because a cheap €/m² often means works. Viager, bare-ownership, usufruct
and shared-ownership sales, whole buildings (APARTMENT_BUILDING,
MIXED_USE_HOUSE) and service flats are skipped, because their €/m² is not
comparable with the commune's apartment or house price, unless
--include-special.
Use this command to rank stored for-sale listings by discount to their
commune's €/m². Do NOT use this command for one property's sold
comparables; use 'comps' instead.`,
		Example: strings.Trim(`
  zimmo-pp-cli underpriced --postcode 1050 --type apartment --below 15
  zimmo-pp-cli underpriced --below 25 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--below=10;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rank stored listings by discount to commune €/m²")
			}
			var types []string
			if typ != "" {
				cat, err := zimmo.ParseCategory(typ)
				if err != nil {
					return usageErr(err)
				}
				if cat != "HOUSE" && cat != "APARTMENT" {
					return usageErr(fmt.Errorf("--type must be house or apartment: Zimmo publishes commune €/m² for those only"))
				}
				types = []string{cat}
			} else {
				types = []string{"HOUSE", "APARTMENT"}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			v := underView{Results: make([]underRow, 0)}
			path, ok := localStoreExists(dbPath)
			if !ok {
				emptyStoreHint(cmd, path)
				return printZimmo(cmd.OutOrStdout(), v, flags)
			}
			db, err := openZimmoStore(ctx, path)
			if err != nil {
				return err
			}
			defer db.Close()
			listings, err := db.QueryZimmoListings(ctx, store.ListingFilter{Postcodes: splitCSV(postcode), Statuses: []string{"FOR_SALE"}, Types: types})
			if err != nil {
				return err
			}
			if len(listings) == 0 {
				emptyStoreHint(cmd, path)
			}
			zc := zimmoClient(flags)
			if flags.dataSource == "local" {
				zc = nil
			}
			prices := map[string]*zimmo.LocalityPrice{}
			failed := map[string]bool{}
			for _, l := range listings {
				if l.PricePerM2 == nil || *l.PricePerM2 <= 0 {
					continue
				}
				if !includeSpecial && ((len(l.SaleFlags) > 0 && !onlyPublicSale(l.SaleFlags)) || notComparableSubtype[l.SubType]) {
					v.SkippedSpecial++
					continue
				}
				v.Checked++
				lp, ok := prices[l.PostalCode]
				if !ok && !failed[l.PostalCode] {
					got, err := communePrice(ctx, zc, db, l.Listing)
					if err != nil {
						failed[l.PostalCode] = true
						v.FetchFailures = append(v.FetchFailures, fetchFailure{Code: l.PostalCode, Error: err.Error()})
					} else {
						prices[l.PostalCode] = &got
						lp = &got
					}
				}
				if lp == nil {
					continue
				}
				ref, n, _ := lp.ForType(l.Type)
				if ref <= 0 {
					continue
				}
				disc := pct1((ref - *l.PricePerM2) / ref)
				if disc < below {
					continue
				}
				v.Results = append(v.Results, underRow{Code: l.Code, Address: l.Address, Type: l.Type, SubType: l.SubType, Price: l.Price, Surface: l.Surface,
					PricePerM2: *l.PricePerM2, CommunePPM2: math.Round(ref), CommuneSample: n, DiscountPct: disc, EPC: l.EPC,
					Condition: l.Condition, DaysOnMarket: l.DaysOnMarket, SaleFlags: l.SaleFlags, URL: l.URL})
			}
			sort.SliceStable(v.Results, func(i, j int) bool { return v.Results[i].DiscountPct > v.Results[j].DiscountPct })
			if limit > 0 && len(v.Results) > limit {
				v.Results = v.Results[:limit]
			}
			if len(v.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: commune price unavailable for %d postcodes; their listings were skipped\n", len(v.FetchFailures))
			}
			if len(v.Results) == 0 && v.Checked > 0 {
				v.Note = fmt.Sprintf("none of %d priced listings is %.0f%% or more below its commune's €/m²", v.Checked, below)
			}
			flags.agentSource = "local"
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printZimmo(cmd.OutOrStdout(), v, flags)
			}
			if len(v.Results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), firstNonEmptyStr(v.Note, "No stored listing to compare."))
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "CODE\tDISCOUNT\t€/M²\tCOMMUNE €/M²\tPRICE\tM²\tEPC\tCONDITION\tADDRESS")
			for _, r := range v.Results {
				fmt.Fprintf(tw, "%s\t-%.1f%%\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.Code, r.DiscountPct, fmtEUR(r.PricePerM2), fmtEUR(r.CommunePPM2), fmtPtrEUR(r.Price),
					floatUnit(r.Surface, ""), r.EPC, strings.ToLower(strings.ReplaceAll(r.Condition, "_", " ")), termSafe(r.Address))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&postcode, "postcode", "", "Only these postcodes (comma-separated)")
	cmd.Flags().StringVar(&typ, "type", "", "house or apartment (default both)")
	cmd.Flags().Float64Var(&below, "below", 15, "Minimum discount to the commune €/m², in percent")
	cmd.Flags().IntVar(&limit, "limit", 30, "Maximum rows")
	cmd.Flags().BoolVar(&includeSpecial, "include-special", false, "Keep viager/bare-ownership/shared sales, whole buildings and service flats")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's data.db)")
	return cmd
}

// communePrice returns the cached €/m² of a listing's commune; with a nil
// client (--data-source local) only the cache is used.
func communePrice(ctx context.Context, zc *zimmo.Client, db *store.Store, l zimmo.Listing) (zimmo.LocalityPrice, error) {
	if zc == nil {
		places, err := lookupPlacesCacheOnly(ctx, db, l.PostalCode)
		if err != nil {
			return zimmo.LocalityPrice{}, err
		}
		for _, p := range places {
			if p.Area.Level == 8 {
				if b, ok := db.CachedLocalityPrice(ctx, p.ID, false, 10*365*24*time.Hour); ok {
					var lp zimmo.LocalityPrice
					if err := json.Unmarshal(b, &lp); err == nil {
						return lp, nil
					}
				}
			}
		}
		return zimmo.LocalityPrice{}, fmt.Errorf("no cached commune price for %s (run without --data-source local once)", l.PostalCode)
	}
	id, err := communePlaceID(ctx, zc, db, l)
	if err != nil {
		return zimmo.LocalityPrice{}, err
	}
	return localityPrice(ctx, zc, db, id, "")
}

// onlyPublicSale: a notary public sale has a real (starting) price, so it
// is not skipped like viager or bare ownership.
func onlyPublicSale(flags []string) bool {
	for _, f := range flags {
		if f != "public_sale" {
			return false
		}
	}
	return true
}

// notComparableSubtype lists subtypes whose €/m² is not comparable with the
// commune's apartment or house indicator.
var notComparableSubtype = map[string]bool{
	"APARTMENT_BUILDING":            true,
	"COMMERCIAL_APARTMENT_BUILDING": true,
	"MIXED_USE_HOUSE":               true,
	"SERVICE_FLAT":                  true,
}
