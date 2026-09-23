// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/store"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/zimmo"
)

type yieldRow struct {
	Code          string   `json:"zimmo_code"`
	Address       string   `json:"address"`
	Type          string   `json:"type"`
	Price         float64  `json:"price"`
	Surface       float64  `json:"surface_m2"`
	Bedrooms      *int     `json:"bedrooms"`
	RentMonthly   float64  `json:"rent_monthly"`
	RentSource    string   `json:"rent_source"` // current_lease | comparables
	RentComps     int      `json:"rent_comparables"`
	RentBasis     string   `json:"rent_basis,omitempty"` // similar_surface | same_bedrooms
	RentPerM2     *float64 `json:"rent_per_m2_month,omitempty"`
	GrossYieldPct float64  `json:"gross_yield_pct"`
	PriceCheck    string   `json:"price_check,omitempty"` // far_below_local_median: the asking €/m² may not be a market price
	EPC           string   `json:"epc"`
	URL           string   `json:"url"`
}

type yieldView struct {
	Checked        int            `json:"checked"`
	Skipped        int            `json:"skipped_no_comparables"`
	SkippedSpecial int            `json:"skipped_special_sales"`
	Results        []yieldRow     `json:"results"`
	FetchFailures  []fetchFailure `json:"fetch_failures,omitempty"`
	Note           string         `json:"note,omitempty"`
}

// rentPool is the €/m²/month of rental listings for one postcode+type.
type rentPool struct {
	comps    []rentComp
	fetchErr error
}

type rentComp struct {
	perM2    float64
	surface  float64
	bedrooms *int
}

// sample picks the rent €/m² of the comparables closest to a sale listing:
// surface within ±30% when at least five exist, else the same bedroom
// count, else nothing (a whole-postcode median misprices big properties).
func (p *rentPool) sample(surface float64, beds *int) ([]float64, string) {
	var bySurface, byBeds []float64
	for _, c := range p.comps {
		if c.surface >= surface*0.7 && c.surface <= surface*1.3 {
			bySurface = append(bySurface, c.perM2)
		}
		if beds != nil && c.bedrooms != nil && *c.bedrooms == *beds {
			byBeds = append(byBeds, c.perM2)
		}
	}
	switch {
	case len(bySurface) >= 5:
		return bySurface, "similar_surface"
	case len(byBeds) >= 5:
		return byBeds, "same_bedrooms"
	}
	return nil, ""
}

func newNovelYieldCmd(flags *rootFlags) *cobra.Command {
	var postcode, typ, dbPath string
	var all bool
	var minYield float64
	var limit, rentPages int
	cmd := &cobra.Command{
		Use:   "yield [zimmo-code]",
		Short: "Estimate gross rental yield of sale listings from the published rent, or rentals of similar surface in the same postcode and type",
		Long: `Gross yield = 12 × monthly rent / asking price. When a listing is currently
let, Zimmo publishes the rent and that is used (rent_source current_lease).
Otherwise the rent is estimated as the median €/m²/month of Zimmo's rental
listings (to rent and rented) in the same postcode and property type with a
living surface within ±30% (or else the same bedroom count), at least five,
times the living surface. Listings without enough such comparables, and
viager or bare-ownership sales, are skipped.
Use this command to rank sale listings by estimated gross rental yield. Do
NOT use this command to list the rental comparables themselves; use
'comps --status rented' instead.`,
		Example: strings.Trim(`
  zimmo-pp-cli yield LRWGW
  zimmo-pp-cli yield --all --postcode 1060 --min 5
  zimmo-pp-cli yield --all --type apartment --agent --select results.zimmo_code,results.gross_yield_pct`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "code=LRWGW"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "estimate gross rental yields")
			}
			if len(args) == 0 && !all {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give a Zimmo code or --all"))
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("yield needs live rental comparables; drop --data-source local"))
			}
			var types []string
			if typ != "" {
				cat, err := zimmo.ParseCategory(typ)
				if err != nil {
					return usageErr(err)
				}
				types = []string{cat}
			}
			if cliutil.IsAnyHarness() && rentPages > 1 {
				rentPages = 1
			}
			ctx, cancel := boundCtx(cmd.Context(), batchFlags(flags))
			defer cancel()
			zc := zimmoClient(flags)
			var targets []zimmo.Listing
			v := yieldView{Results: make([]yieldRow, 0)}
			if len(args) == 1 {
				ref, err := parseCodeArg(args[0])
				if err != nil {
					return usageErr(err)
				}
				l, err := fetchListing(ctx, zc, ref)
				if err != nil {
					return err
				}
				if l.Status != "FOR_SALE" && l.Status != "TAKE_OVER" {
					return usageErr(fmt.Errorf("listing %s is %s: yield needs a listing for sale (its price is a sale price)", l.Code, strings.ToLower(l.Status)))
				}
				targets = []zimmo.Listing{l}
				flags.agentSource = "live"
			} else {
				path, ok := localStoreExists(dbPath)
				if !ok {
					emptyStoreHint(cmd, path)
					return printZimmo(cmd.OutOrStdout(), v, flags)
				}
				db, err := openZimmoStore(ctx, path)
				if err != nil {
					return err
				}
				rows, err := db.QueryZimmoListings(ctx, store.ListingFilter{Postcodes: splitCSV(postcode), Statuses: []string{"FOR_SALE"}, Types: types})
				_ = db.Close()
				if err != nil {
					return err
				}
				if len(rows) == 0 {
					emptyStoreHint(cmd, path)
				}
				for _, r := range rows {
					targets = append(targets, r.Listing)
				}
				flags.agentSource = "local"
			}
			localMedian := saleMedians(targets)
			pools := map[string]*rentPool{}
			for _, l := range targets {
				if l.Price == nil || *l.Price <= 0 {
					continue
				}
				if len(l.SaleFlags) > 0 && !onlyPublicSale(l.SaleFlags) {
					v.SkippedSpecial++
					continue
				}
				v.Checked++
				row := yieldRow{Code: l.Code, Address: l.Address, Type: l.Type, Price: *l.Price, Bedrooms: l.Bedrooms, EPC: l.EPC, URL: l.URL}
				if l.Surface != nil {
					row.Surface = *l.Surface
				}
				switch {
				case l.Rented && l.RentPerYear != nil:
					row.RentMonthly = math.Round(*l.RentPerYear / 12)
					row.RentSource = "current_lease"
				case l.Surface != nil && *l.Surface > 0 && l.PostalCode != "":
					key := l.PostalCode + "|" + l.Type
					pool, ok := pools[key]
					if !ok {
						pool = fetchRentPool(ctx, zc, l.PostalCode, l.Type, rentPages)
						pools[key] = pool
						if pool.fetchErr != nil {
							v.FetchFailures = append(v.FetchFailures, fetchFailure{Code: key, Error: pool.fetchErr.Error()})
						}
					}
					sample, basis := pool.sample(*l.Surface, l.Bedrooms)
					if len(sample) == 0 {
						v.Skipped++
						continue
					}
					m, _ := zimmo.Median(sample)
					row.RentPerM2 = ptrF(math.Round(m*100) / 100)
					row.RentMonthly = math.Round(m * *l.Surface)
					row.RentSource, row.RentComps, row.RentBasis = "comparables", len(sample), basis
				default:
					continue
				}
				row.GrossYieldPct = math.Round(row.RentMonthly*12/row.Price*1000) / 10
				if med, ok := localMedian[l.PostalCode+"|"+l.Type]; ok && l.PricePerM2 != nil && *l.PricePerM2 < 0.6*med {
					row.PriceCheck = "far_below_local_median"
				}
				if row.GrossYieldPct < minYield {
					continue
				}
				v.Results = append(v.Results, row)
			}
			sort.SliceStable(v.Results, func(i, j int) bool { return v.Results[i].GrossYieldPct > v.Results[j].GrossYieldPct })
			if limit > 0 && len(v.Results) > limit {
				v.Results = v.Results[:limit]
			}
			if len(v.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: rental comparables unavailable for %d postcode/type groups; their listings were skipped\n", len(v.FetchFailures))
			}
			if len(v.Results) == 0 && v.Checked > 0 {
				v.Note = "no listing reached --min, or fewer than 5 rental comparables of similar surface or bedrooms in its postcode"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printZimmo(cmd.OutOrStdout(), v, flags)
			}
			if len(v.Results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), firstNonEmptyStr(v.Note, "No listing to evaluate."))
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "CODE\tYIELD\tRENT/MONTH\tSOURCE\tPRICE\tM²\tEPC\tADDRESS")
			for _, r := range v.Results {
				src := "lease"
				if r.RentSource == "comparables" {
					src = fmt.Sprintf("%d comps", r.RentComps)
				}
				yield := fmt.Sprintf("%.1f%%", r.GrossYieldPct)
				if r.PriceCheck != "" {
					yield += " !"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%.0f\t%s\t%s\n", r.Code, yield, fmtEUR(r.RentMonthly), src, fmtEUR(r.Price), r.Surface, r.EPC, termSafe(r.Address))
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			for _, r := range v.Results {
				if r.PriceCheck != "" {
					fmt.Fprintln(cmd.OutOrStdout(), "! asking €/m² under 60% of the local median: check it is a market price (lots, special sale) before trusting the yield")
					break
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Rank every stored for-sale listing")
	cmd.Flags().StringVar(&postcode, "postcode", "", "With --all: only these postcodes (comma-separated)")
	cmd.Flags().StringVar(&typ, "type", "", "With --all: only this property type")
	cmd.Flags().Float64Var(&minYield, "min", 0, "Minimum gross yield in percent")
	cmd.Flags().IntVar(&limit, "limit", 30, "Maximum rows")
	cmd.Flags().IntVar(&rentPages, "rent-pages", 2, "Rental result pages (100 each) per postcode and type")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's data.db)")
	return cmd
}

// fetchRentPool collects €/m²/month of rental listings for one postcode and
// property type (both currently to rent and already rented).
func fetchRentPool(ctx context.Context, zc *zimmo.Client, postcode, category string, pages int) *rentPool {
	pool := &rentPool{}
	crit := zimmo.Criteria{Statuses: []string{"TO_RENT", "RENTED"}, Postcodes: []string{postcode}, Categories: []string{category}, Sort: "newest"}
	res, err := walkSearch(ctx, zc, crit, pages, 0)
	if err != nil {
		pool.fetchErr = err
		return pool
	}
	for _, r := range res.Listings {
		if r.Price == nil || r.Surface == nil || *r.Surface < 10 || *r.Price <= 0 || *r.Price > 20000 {
			continue
		}
		pool.comps = append(pool.comps, rentComp{perM2: *r.Price / *r.Surface, surface: *r.Surface, bedrooms: r.Bedrooms})
	}
	return pool
}

// saleMedians is the median asking €/m² per postcode and type among the
// listings being evaluated; it flags prices that are not market prices.
func saleMedians(ls []zimmo.Listing) map[string]float64 {
	pools := map[string][]float64{}
	for _, l := range ls {
		if l.PricePerM2 != nil && *l.PricePerM2 > 0 {
			k := l.PostalCode + "|" + l.Type
			pools[k] = append(pools[k], *l.PricePerM2)
		}
	}
	out := map[string]float64{}
	for k, v := range pools {
		if len(v) >= 10 {
			out[k], _ = zimmo.Median(v)
		}
	}
	return out
}
