// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/immo"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/store"
)

type yieldEstimate struct {
	Band             string   `json:"band"`
	SalePrice        *float64 `json:"sale_price,omitempty"`
	SaleComparables  int      `json:"sale_comparables,omitempty"`
	RentComparables  int      `json:"rent_comparables"`
	MedianRent       *float64 `json:"median_monthly_rent,omitempty"`
	RentP25          *float64 `json:"rent_p25,omitempty"`
	RentP75          *float64 `json:"rent_p75,omitempty"`
	GrossYieldPct    *float64 `json:"gross_yield_pct,omitempty"`
	GrossYieldLowPct *float64 `json:"gross_yield_low_pct,omitempty"`
	GrossYieldHiPct  *float64 `json:"gross_yield_high_pct,omitempty"`
	Insufficient     bool     `json:"insufficient_comparables"`
	Note             string   `json:"note,omitempty"`
}

type yieldView struct {
	Mode        string          `json:"mode"` // listing | commune
	ListingID   int64           `json:"listing_id,omitempty"`
	URL         string          `json:"url,omitempty"`
	Commune     string          `json:"commune,omitempty"`
	Type        string          `json:"type"`
	PostalCodes []string        `json:"postal_codes"`
	Estimates   []yieldEstimate `json:"estimates"`
	Caveat      string          `json:"caveat"`
	Warnings    []string        `json:"warnings,omitempty"`
	// RoomLetsExcluded counts student rooms and per-room lets left out.
	RoomLetsExcluded int `json:"room_lets_excluded,omitempty"`
}

func newNovelYieldCmd(flags *rootFlags) *cobra.Command {
	var flagCommune, flagType, dbPath, maxAge string
	var noPull, includeRooms bool
	var maxPages int

	cmd := &cobra.Command{
		Use:   "yield [listing id or URL]",
		Short: "Estimates gross rental yield for a sale listing or a whole commune from comparable rents",
		Long: `Estimate gross rental yield (12 × monthly rent ÷ price) from rentals and sales
Immoweb never shows together. For a sale listing: median rent of rentals with the
same postal code, type and bedroom count (±1 when too few). For a commune: median
sale price vs median rent per bedroom band. Refuses to estimate below 5 comparables.
Missing rent or sale data is pulled automatically (use --no-pull to stay offline).

Use this command to estimate the gross rental yield of a sale listing or a commune.
Do NOT use this command to judge whether a price is fair; use 'deal' instead.
Do NOT use it for general commune statistics; use 'market' instead.`,
		Example: strings.Trim(`
  immoweb-pp-cli yield --commune liege --type apartment --agent
  immoweb-pp-cli yield 21828249`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true", // reads Immoweb only; results land in the CLI's own cache
			"pp:data-source":      "auto",
			"pp:happy-args":       "--commune=ixelles;--type=apartment",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "yield")
			}
			useHarvestRate(cmd, flags)
			if len(args) == 0 && flagCommune == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give a sale listing ID/URL or --commune"))
			}
			age, err := parseMaxAge(maxAge)
			if err != nil {
				return usageErr(err)
			}
			offline := noPull || flags.dataSource == "local"
			if flags.dataSource == "local" || (noPull && len(args) == 0) {
				flags.agentSource = "local" // envelope reports local reads; listing mode still fetches the listing
			}
			ctx, cancel := harvestCtx(cmd, flags)
			defer cancel()
			db, err := openImmoStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			var c *client.Client
			if !offline || len(args) > 0 && flags.dataSource != "local" {
				if c, err = flags.newClient(); err != nil {
					return err
				}
			}
			var warnMu sync.Mutex
			warnings := []string{}
			warn := func(msg string) {
				warnMu.Lock()
				warnings = append(warnings, msg)
				warnMu.Unlock()
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: "+msg)
			}
			pull := func(crit immo.Criteria) []string {
				if offline || c == nil {
					if len(crit.Communes) > 0 {
						pcs, err := localPostcodesFor(ctx, db, crit.Communes[0])
						if err != nil {
							warn("local lookup failed: " + err.Error())
						}
						return pcs
					}
					return crit.PostalCodes
				}
				rep, _, err := ensureArea(ctx, cmd, c, db, crit, age, dogfoodPages(maxPages))
				if err != nil {
					warn(strings.ToLower(crit.Deal) + " pull failed: " + err.Error())
				}
				if len(rep.Communes) > 0 {
					return rep.Communes[0].PostalCodes
				}
				return crit.PostalCodes
			}
			view := yieldView{Estimates: make([]yieldEstimate, 0), Caveat: "gross yield on asking prices and asking rents; excludes charges, vacancy, taxes, registration duties and works"}
			if len(args) > 0 {
				id, err := immo.ParseListingID(args[0])
				if err != nil {
					return usageErr(err)
				}
				var target store.StoredListing
				if c != nil && flags.dataSource != "local" {
					d, raw, err := fetchDetail(ctx, c, id)
					if err != nil {
						return err
					}
					if err := db.SaveDetail(ctx, d, raw, time.Now()); err != nil {
						return err
					}
				}
				rows, err := db.QueryListings(ctx, store.ListingFilter{IDs: []int64{id}, IncludeGone: true})
				if err != nil {
					return err
				}
				if len(rows) == 0 {
					return notFoundErr(fmt.Errorf("listing %d is not stored; run without --no-pull", id))
				}
				target = rows[0]
				if target.Deal != "FOR_SALE" {
					return usageErr(fmt.Errorf("listing %d is a rental; yield needs a sale listing", id))
				}
				view.Mode, view.ListingID, view.URL, view.Type = "listing", id, target.URL, target.Type
				view.PostalCodes = []string{"BE-" + target.PostalCode}
				pcs := pull(immo.Criteria{Types: []string{target.Type}, Deal: "FOR_RENT", PostalCodes: view.PostalCodes})
				if len(pcs) == 0 {
					pcs = view.PostalCodes
				}
				rents, err := db.QueryListings(ctx, store.ListingFilter{Deal: "FOR_RENT", Types: []string{target.Type}, PostalCodes: pcs})
				if err != nil {
					return err
				}
				if !includeRooms {
					if immo.IsRoomLet(target.Listing) {
						return usageErr(fmt.Errorf("listing %d is a student room or per-room let; pass --include-rooms to estimate it against room rents", id))
					}
					var n int
					rents, n = dropRoomLets(rents)
					view.RoomLetsExcluded += n
				}
				est := yieldEstimate{Band: immo.BedroomBand(target.Bedrooms), SalePrice: target.Price}
				comps := rentsForBedrooms(rents, target.Bedrooms, 0)
				if len(comps) < minComparables {
					comps = rentsForBedrooms(rents, target.Bedrooms, 1)
					est.Note = "bedrooms widened to ±1"
				}
				fillYield(&est, target.Price, comps)
				view.Estimates = append(view.Estimates, est)
			} else {
				if flagType == "" {
					flagType = "apartment"
				}
				types, err := immo.NormalizeTypes(flagType)
				if err != nil {
					return usageErr(err)
				}
				view.Mode, view.Commune, view.Type = "commune", flagCommune, strings.Join(types, ",")
				crit := immo.Criteria{Types: types}
				if pc := immo.NormalizePostalCode(flagCommune); pc != "" {
					crit.PostalCodes = []string{pc}
				} else {
					crit.Communes = []string{flagCommune}
				}
				sc, rc := crit, crit
				sc.Deal, rc.Deal = "FOR_SALE", "FOR_RENT"
				var pcs, more []string
				parallel(2, 2, func(i int) {
					if i == 0 {
						pcs = pull(sc)
					} else {
						more = pull(rc)
					}
				})
				if len(pcs) == 0 {
					pcs = more
				}
				view.PostalCodes = pcs
				if len(pcs) == 0 {
					return notFoundErr(fmt.Errorf("no stored listings for %q; run without --no-pull", flagCommune))
				}
				sales, err := db.QueryListings(ctx, store.ListingFilter{Deal: "FOR_SALE", Types: types, PostalCodes: pcs})
				if err != nil {
					return err
				}
				rents, err := db.QueryListings(ctx, store.ListingFilter{Deal: "FOR_RENT", Types: types, PostalCodes: pcs})
				if err != nil {
					return err
				}
				if !includeRooms {
					var n, m int
					rents, n = dropRoomLets(rents)
					sales, m = dropRoomLets(sales)
					view.RoomLetsExcluded += n + m
				}
				for _, band := range immo.BedroomBands[:len(immo.BedroomBands)-1] { // "unknown" has no yield
					bs := filterBand(sales, band)
					br := filterBand(rents, band)
					if len(bs) == 0 && len(br) == 0 {
						continue
					}
					est := yieldEstimate{Band: band, SaleComparables: len(bs)}
					medSale := medianPtr(priceValues(bs))
					est.SalePrice = medSale
					if len(bs) < minComparables {
						est.RentComparables = len(br)
						est.Insufficient = true
						est.Note = fmt.Sprintf("only %d sale listings in this band", len(bs))
						view.Estimates = append(view.Estimates, est)
						continue
					}
					fillYield(&est, medSale, br)
					view.Estimates = append(view.Estimates, est)
				}
			}
			if len(warnings) > 0 {
				view.Warnings = warnings
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			if view.Mode == "listing" {
				fmt.Fprintf(w, "%s\n", view.URL)
			} else {
				fmt.Fprintf(w, "%s — %s (%s)\n", bold(view.Commune), strings.ToLower(view.Type), strings.Join(view.PostalCodes, ","))
			}
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "BEDROOMS\tSALE PRICE\tMEDIAN RENT\tRENTS\tGROSS YIELD\tRANGE")
			for _, e := range view.Estimates {
				y, rng := "-", ""
				if e.GrossYieldPct != nil {
					y = fmt.Sprintf("%.2f%%", *e.GrossYieldPct)
				}
				if e.GrossYieldLowPct != nil && e.GrossYieldHiPct != nil {
					rng = fmt.Sprintf("%.2f–%.2f%%", *e.GrossYieldLowPct, *e.GrossYieldHiPct)
				}
				if e.Insufficient {
					y = "n/a (" + e.Note + ")"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%s\n", e.Band, fmtPtrEUR(e.SalePrice), fmtPtrEUR(e.MedianRent), e.RentComparables, y, rng)
			}
			_ = tw.Flush()
			fmt.Fprintf(w, "\n%s\n", view.Caveat)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagCommune, "commune", "", "Commune name or postal code (commune mode)")
	cmd.Flags().StringVar(&flagType, "type", "", "Property type for commune mode (default apartment)")
	cmd.Flags().StringVar(&maxAge, "refresh-after", "72h", "Pull an area again when its last pull is older than this")
	cmd.Flags().IntVar(&maxPages, "max-pages", 20, "Search pages (30 each) to walk when pulling")
	cmd.Flags().BoolVar(&noPull, "no-pull", false, "Do not pull comparables (listing mode still fetches the listing live; --data-source local stays fully offline)")
	cmd.Flags().BoolVar(&includeRooms, "include-rooms", false, "Keep student rooms (kots) and per-room lets in the rent comparables")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.Flags().MarkHidden("db") // not an MCP tool argument: agents must not point the store at other files
	return cmd
}

func rentsForBedrooms(rents []store.StoredListing, beds *int, tolerance int) []store.StoredListing {
	out := []store.StoredListing{}
	for _, r := range rents {
		if r.Price == nil {
			continue
		}
		if beds == nil || r.Bedrooms == nil {
			if beds == nil {
				out = append(out, r)
			}
			continue
		}
		if absInt(*r.Bedrooms-*beds) <= tolerance {
			out = append(out, r)
		}
	}
	return out
}

func filterBand(ls []store.StoredListing, band string) []store.StoredListing {
	out := []store.StoredListing{}
	for _, l := range ls {
		if l.Price != nil && immo.BedroomBand(l.Bedrooms) == band {
			out = append(out, l)
		}
	}
	return out
}

func fillYield(est *yieldEstimate, price *float64, rents []store.StoredListing) {
	est.RentComparables = len(rents)
	if len(rents) < minComparables || price == nil {
		est.Insufficient = true
		msg := fmt.Sprintf("only %d comparable rents (need %d)", len(rents), minComparables)
		if est.Note == "" {
			est.Note = msg
		} else {
			est.Note += "; " + msg
		}
		return
	}
	vals := priceValues(rents)
	est.MedianRent = medianPtr(vals)
	est.RentP25 = quantilePtr(vals, 0.25)
	est.RentP75 = quantilePtr(vals, 0.75)
	if y, ok := immo.GrossYield(*est.MedianRent, *price); ok {
		est.GrossYieldPct = &y
	}
	if y, ok := immo.GrossYield(*est.RentP25, *price); ok {
		est.GrossYieldLowPct = &y
	}
	if y, ok := immo.GrossYield(*est.RentP75, *price); ok {
		est.GrossYieldHiPct = &y
	}
}
