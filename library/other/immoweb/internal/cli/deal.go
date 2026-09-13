// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/immo"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/store"
)

type dealComparable struct {
	ID          int64    `json:"id"`
	URL         string   `json:"url"`
	Price       *float64 `json:"price,omitempty"`
	Surface     *float64 `json:"surface_m2,omitempty"`
	PricePerSqm *float64 `json:"price_per_m2,omitempty"`
	Bedrooms    *int     `json:"bedrooms,omitempty"`
	Gone        bool     `json:"gone"`
}

type dealView struct {
	Listing              immo.Listing     `json:"listing"`
	Verdict              string           `json:"verdict"`
	PricePerM2           *float64         `json:"price_per_m2,omitempty"`
	Comparables          int              `json:"comparables"`
	ComparableMedianM2   *float64         `json:"comparable_median_price_per_m2,omitempty"`
	PercentileVsComps    *float64         `json:"percentile_vs_comparables,omitempty"`
	DiffVsMedianPct      *float64         `json:"diff_vs_median_pct,omitempty"`
	FairPriceAtMedian    *float64         `json:"price_at_comparable_median,omitempty"`
	DaysListed           *int             `json:"days_listed,omitempty"`
	PriceCut             *cutInfo         `json:"price_cut,omitempty"`
	Views                *int             `json:"views,omitempty"`
	Bookmarks            *int             `json:"bookmarks,omitempty"`
	BookmarksPerDay      *float64         `json:"bookmarks_per_day,omitempty"`
	EPC                  string           `json:"epc,omitempty"`
	ComparableMedianEPC  string           `json:"comparable_median_epc,omitempty"`
	EPCGapLabels         *int             `json:"epc_gap_labels,omitempty"`
	RenovationObligation *bool            `json:"renovation_obligation,omitempty"`
	CheapestComparables  []dealComparable `json:"cheapest_comparables"`
	Reasons              []string         `json:"reasons"`
	Warnings             []string         `json:"warnings,omitempty"`
}

func newNovelDealCmd(flags *rootFlags) *cobra.Command {
	var dbPath, maxAge string
	var noPull bool
	var maxPages int
	cmd := &cobra.Command{
		Use:   "deal [id-or-url]",
		Short: "Tells you whether one listing is cheap or expensive: €/m² percentile against comparable listings, days on market, price cuts, demand",
		Long: `Judge one listing's asking price. Compares its €/m² with comparable listings
(same postal code, type and deal, bedrooms ±1, surface ±25%, including listings that
already disappeared), and adds days on market, price-cut history, demand
(views and saves per day) and the EPC gap versus comparables. The postal code is
pulled automatically when fewer than 5 comparables are stored.

Use this command to judge whether one listing's price is fair (€/m² vs comparables, days on market, price cuts, demand).
Do NOT use this command for commune-wide statistics; use 'market' instead.
Do NOT use it to estimate rental return; use 'yield' instead.`,
		Example: strings.Trim(`
  immoweb-pp-cli deal 21828249 --agent
  immoweb-pp-cli deal https://www.immoweb.be/fr/annonce/maison/a-vendre/dilbeek/1700/21828249`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true", // reads Immoweb only; results land in the CLI's own cache
			"pp:data-source":      "auto",
			"pp:happy-args":       "id=21828249",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "deal")
			}
			useHarvestRate(cmd, flags)
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a listing ID or URL is required"))
			}
			id, err := immo.ParseListingID(args[0])
			if err != nil {
				return usageErr(err)
			}
			age, err := parseMaxAge(maxAge)
			if err != nil {
				return usageErr(err)
			}
			offline := noPull || flags.dataSource == "local"
			if flags.dataSource == "local" {
				flags.agentSource = "local" // envelope reports local reads; --no-pull still fetches the listing
			}
			ctx, cancel := harvestCtx(cmd, flags)
			defer cancel()
			db, err := openImmoStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			view := dealView{CheapestComparables: make([]dealComparable, 0), Reasons: make([]string, 0)}
			var target store.StoredListing
			if flags.dataSource == "local" {
				rows, err := db.QueryListings(ctx, store.ListingFilter{IDs: []int64{id}, IncludeGone: true})
				if err != nil {
					return err
				}
				if len(rows) == 0 {
					return notFoundErr(fmt.Errorf("listing %d is not in the local store; run without --data-source local", id))
				}
				target = rows[0]
			} else {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				d, raw, err := fetchDetail(ctx, c, id)
				if err != nil {
					return err
				}
				if err := db.SaveDetail(ctx, d, raw, time.Now()); err != nil {
					return err
				}
				view.Views, view.Bookmarks, view.RenovationObligation = d.Views, d.Bookmarks, d.RenovationOblig
				rows, err := db.QueryListings(ctx, store.ListingFilter{IDs: []int64{id}, IncludeGone: true})
				if err != nil {
					return fmt.Errorf("reading stored listing %d: %w", id, err)
				}
				if len(rows) == 0 {
					return fmt.Errorf("listing %d was fetched but not stored locally", id)
				}
				target = rows[0]
				if !offline && target.PostalCode != "" {
					pool, err := db.QueryListings(ctx, store.ListingFilter{Deal: target.Deal, Types: []string{target.Type}, PostalCodes: []string{target.PostalCode}, IncludeGone: true})
					if err != nil {
						return err
					}
					if vals, _ := comparablePrices(target.Listing, pool); len(vals) < minComparables {
						crit := immo.Criteria{Types: []string{target.Type}, Deal: target.Deal, PostalCodes: []string{"BE-" + target.PostalCode}}
						if _, _, err := ensureArea(ctx, cmd, c, db, crit, age, dogfoodPages(maxPages)); err != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not pull comparables: %v\n", err)
							view.Warnings = append(view.Warnings, "comparables pull failed: "+err.Error())
						}
					}
				}
			}
			view.Listing = target.Listing
			view.PricePerM2 = target.PricePerSqm
			view.EPC = target.EPC
			if view.Views == nil {
				view.Views, view.Bookmarks = target.Views, target.Bookmarks
			}
			now := time.Now()
			view.DaysListed = daysOf(target, now)
			if view.Bookmarks != nil && view.DaysListed != nil && *view.DaysListed > 0 {
				b := immo.Round2(float64(*view.Bookmarks) / float64(*view.DaysListed))
				view.BookmarksPerDay = &b
			}
			hist, err := db.PriceHistory(ctx, id)
			if err != nil {
				return err
			}
			if ci, ok := priceCut(target, hist); ok {
				view.PriceCut = &ci
				view.Reasons = append(view.Reasons, fmt.Sprintf("price already cut by %.1f%% (%s)", ci.CutPct, ci.Source))
			} else if target.NewPrice {
				view.Reasons = append(view.Reasons, "Immoweb flags a recent price change (no previous price published)")
			}
			pool, err := db.QueryListings(ctx, store.ListingFilter{Deal: target.Deal, Types: []string{target.Type}, PostalCodes: []string{target.PostalCode}, IncludeGone: true})
			if err != nil {
				return err
			}
			vals, comps := comparablePrices(target.Listing, pool)
			view.Comparables = len(vals)
			view.Verdict = dealVerdict(target.PricePerSqm, vals, &view)
			sortListingsByPPS(comps)
			for i, cp := range comps {
				if i >= 5 {
					break
				}
				view.CheapestComparables = append(view.CheapestComparables, dealComparable{ID: cp.ID, URL: cp.URL, Price: cp.Price, Surface: cp.Surface, PricePerSqm: cp.PricePerSqm, Bedrooms: cp.Bedrooms, Gone: cp.GoneAt != ""})
			}
			ranks := []float64{}
			for _, cp := range comps {
				if r := immo.EPCRank(cp.EPC); r > 0 {
					ranks = append(ranks, float64(r))
				}
			}
			if m, ok := immo.Median(ranks); ok && immo.EPCRank(target.EPC) > 0 {
				labels := []string{"", "A", "B", "C", "D", "E", "F", "G"}
				mr := int(immo.Round0(m))
				view.ComparableMedianEPC = labels[mr]
				gap := immo.EPCRank(target.EPC) - mr
				view.EPCGapLabels = &gap
				if gap >= 2 {
					view.Reasons = append(view.Reasons, fmt.Sprintf("EPC %s is %d labels worse than comparables (%s): budget for energy works", target.EPC, gap, labels[mr]))
				}
			}
			if view.DaysListed != nil {
				switch {
				case *view.DaysListed >= 90:
					view.Reasons = append(view.Reasons, fmt.Sprintf("online for %d days: the seller may negotiate", *view.DaysListed))
				case *view.DaysListed <= 7:
					view.Reasons = append(view.Reasons, fmt.Sprintf("fresh listing (%d days)", *view.DaysListed))
				}
			}
			if target.UnderOption {
				view.Reasons = append(view.Reasons, "already under option")
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s  %s\n", bold(strings.ToUpper(view.Verdict)), target.URL)
			fmt.Fprintf(w, "  price %s · %s/m² · %s %s, %s\n", fmtPtrEUR(target.Price), fmtPtrEUR(target.PricePerSqm), intStr(target.Bedrooms)+" bed", floatUnit(target.Surface, " m²"), locLabel(target.Listing))
			if view.ComparableMedianM2 != nil {
				fmt.Fprintf(w, "  comparables: %d · median %s/m² · this listing is at the %.0fth percentile (%+.1f%% vs median)\n", view.Comparables, fmtEUR(*view.ComparableMedianM2), derefF(view.PercentileVsComps), derefF(view.DiffVsMedianPct))
				if view.FairPriceAtMedian != nil {
					fmt.Fprintf(w, "  price at the comparable median: %s\n", fmtEUR(*view.FairPriceAtMedian))
				}
			} else {
				fmt.Fprintf(w, "  only %d comparables with a surface: no price verdict\n", view.Comparables)
			}
			if view.DaysListed != nil {
				fmt.Fprintf(w, "  days listed: %d", *view.DaysListed)
				if view.BookmarksPerDay != nil {
					fmt.Fprintf(w, " · %.2f saves/day", *view.BookmarksPerDay)
				}
				fmt.Fprintln(w)
			}
			for _, r := range view.Reasons {
				fmt.Fprintf(w, "  - %s\n", r)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&maxAge, "refresh-after", "72h", "Pull comparables again when older than this")
	cmd.Flags().IntVar(&maxPages, "max-pages", 20, "Search pages (30 each) to walk when pulling comparables")
	cmd.Flags().BoolVar(&noPull, "no-pull", false, "Do not pull comparables (the listing itself is still fetched live; --data-source local stays fully offline)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.Flags().MarkHidden("db") // not an MCP tool argument: agents must not point the store at other files
	return cmd
}

func dealVerdict(pps *float64, comps []float64, v *dealView) string {
	if pps == nil {
		v.Reasons = append(v.Reasons, "no living surface published: cannot compute €/m²")
		return "unknown"
	}
	if len(comps) < minComparables {
		v.Reasons = append(v.Reasons, fmt.Sprintf("only %d comparables (need %d)", len(comps), minComparables))
		return "not enough comparables"
	}
	med, _ := immo.Median(comps)
	medR := immo.Round0(med)
	v.ComparableMedianM2 = &medR
	pct, _ := immo.PercentileRank(comps, *pps)
	v.PercentileVsComps = &pct
	diff := immo.Round1((*pps - med) / med * 100)
	v.DiffVsMedianPct = &diff
	if v.Listing.Surface != nil {
		fair := immo.Round0(med * *v.Listing.Surface)
		v.FairPriceAtMedian = &fair
	}
	v.Reasons = append(v.Reasons, fmt.Sprintf("€/m² is %+.1f%% vs the median of %d comparables (percentile %.0f)", diff, len(comps), pct))
	switch {
	case pct <= 25:
		return "cheap"
	case pct <= 60:
		return "fair"
	case pct <= 85:
		return "above market"
	default:
		return "expensive"
	}
}

func derefF(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
