// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: per-airport, review-aware recommendation view. Surfaces the
// top companies at an airport (rating-gated, best-available fully-insured
// price), a small+bigger fully-insured pick and cheap-with-cover picks, and a
// full company trust table.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/carsource"

	"github.com/spf13/cobra"
)

// offerFI is an offer with its computed fully-insured total.
type offerFI struct {
	offer      carsource.Offer
	canonical  string
	fiTotal    float64
	zeroExcess bool
	size       string // "small" | "bigger"
}

// companyRow is one line of the top-companies / trust table.
type companyRow struct {
	Company   string   `json:"company"`
	Rating    float64  `json:"rating"`
	Reviews   int      `json:"reviews"`
	Cheapest  float64  `json:"cheapest_fully_insured"`
	Currency  string   `json:"currency"`
	Source    string   `json:"price_source"` // "direct" | "aggregator"
	Caveats   []string `json:"caveats,omitempty"`
	belowGate bool
}

// pickRow is a recommended car.
type pickRow struct {
	Kind     string  `json:"kind"` // full-insurance-small | full-insurance-bigger | cover
	Company  string  `json:"company"`
	Car      string  `json:"car"`
	Total    float64 `json:"fully_insured_total"`
	Base     float64 `json:"base_total,omitempty"`
	Cover    float64 `json:"cover,omitempty"`
	Currency string  `json:"currency"`
	Rating   float64 `json:"rating,omitempty"`
}

func isDirectSource(s string) bool {
	switch s {
	case "delpaso", "centauro", "drivalia", "autoreisen", "clickrent", "cicar", "goldcar":
		return true
	}
	return false
}

// fiPrice computes an offer's fully-insured (zero-excess) total. Direct offers
// and zero-excess aggregator offers are already fully insured; other aggregator
// offers add a standalone excess-cover estimate.
func fiPrice(o carsource.Offer, days int, coverBase, coverPerDay float64) (total float64, zeroExcess bool) {
	if isDirectSource(o.Source) || (o.ExcessKnown && o.Excess == 0) {
		return o.Total, true
	}
	return o.Total + excessCoverEstimate(days, coverBase, coverPerDay), false
}

func newRecommendCmd(flags *rootFlags) *cobra.Command {
	var pickupTime, dropoffTime string
	var age, top, coverPicks int
	var minRating, coverPerDay, coverBase float64
	cmd := &cobra.Command{
		Use:   "recommend [location] <pickup> <dropoff>",
		Short: "Per-airport recommendation: top-rated companies, best full-insurance picks, and a trust table",
		Long: "The review-aware recommendation for an airport. Fetches every source, attaches customer\n" +
			"ratings (Rentalcars primary, DoYouSpain fallback), and shows:\n" +
			"  1. the top companies at this airport (rating-gated by --min-rating, ranked by trust),\n" +
			"     each priced best-available (our direct client where we have one, else the company's\n" +
			"     own cheapest fully-insured price from the aggregators);\n" +
			"  2. recommended cars — a small and a bigger fully-insured (zero-excess) pick, then the\n" +
			"     cheapest base-rate cars priced as base + standalone excess cover;\n" +
			"  3. a full trust table of every company at the airport, with caveats.\n\n" +
			"Location defaults to Málaga; pass an IATA code or airport name for elsewhere in Spain.",
		Example:     "  rentalcarspain-pp-cli recommend BIO 21/07/2026 28/07/2026\n  rentalcarspain-pp-cli recommend ALC 20/07/2026 27/07/2026 --min-rating 8 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			location, pickup, dropoff, ok := resolveSearchArgs(args)
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("recommend needs <pickup> <dropoff> (Málaga default) or <location> <pickup> <dropoff>"))
			}
			days := rentalDaysBetween(pickup, dropoff)
			loc := resolveLocationInput(location)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			now := time.Now()

			// Gather all offers concurrently: aggregators + direct clients.
			var mu sync.Mutex
			var all []carsource.Offer
			var aggErrs map[string]error
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				offers, errs := fetchOffers(ctx, flags, "all", location, pickup, dropoff, pickupTime, dropoffTime, age)
				mu.Lock()
				all = append(all, offers...)
				aggErrs = errs
				mu.Unlock()
			}()
			for _, co := range enabledDirectCompanies(flags) {
				wg.Add(1)
				go func(co directCompany) {
					defer wg.Done()
					offers, err := co.quote(ctx, flags, loc, pickup, dropoff, pickupTime, dropoffTime, age, now)
					if err != nil {
						return
					}
					mu.Lock()
					all = append(all, offers...)
					mu.Unlock()
				}(co)
			}
			wg.Wait()
			for src, e := range aggErrs {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: aggregator %s unavailable: %s\n", src, truncate(e.Error(), 90))
			}
			if len(all) == 0 {
				if err := sourceErrorsError(aggErrs); err != nil {
					return err
				}
				return apiErr(fmt.Errorf("no offers found for %s these dates", displayLocation(loc, location)))
			}

			reviewIdx := reviewIndexCached(ctx, flags, location, pickup, dropoff, pickupTime, dropoffTime, age, all)

			// Per-company aggregation + offer lists for the picks.
			type agg struct {
				display  string
				minFI    float64
				minSrc   string // "direct" | "aggregator"
				hasOffer bool
			}
			companies := map[string]*agg{}
			var zeroExcess, coverable []offerFI
			for _, o := range all {
				canon := carsource.CanonicalSupplier(o.Supplier)
				fi, zero := fiPrice(o, days, coverBase, coverPerDay)
				a := companies[canon]
				if a == nil {
					a = &agg{display: canon}
					companies[canon] = a
				}
				if !a.hasOffer || fi < a.minFI {
					a.minFI = fi
					a.hasOffer = true
					if isDirectSource(o.Source) {
						a.minSrc = "direct"
					} else {
						a.minSrc = "aggregator"
					}
				}
				ofi := offerFI{offer: o, canonical: canon, fiTotal: fi, zeroExcess: zero, size: carSize(o)}
				if zero {
					zeroExcess = append(zeroExcess, ofi)
				} else {
					coverable = append(coverable, ofi)
				}
			}

			// Build company rows (all companies present), rating from the index.
			var rows []companyRow
			for canon, a := range companies {
				ri := reviewIdx[canon]
				row := companyRow{
					Company: canon, Rating: ri.Score, Reviews: ri.Count,
					Cheapest: a.minFI, Currency: "EUR", Source: a.minSrc,
				}
				row.belowGate = ri.Score > 0 && ri.Score < minRating
				row.Caveats = companyCaveats(canon, ri.Score, minRating, a.minSrc)
				rows = append(rows, row)
			}
			// Sort by rating desc (unrated last), then by price.
			sort.SliceStable(rows, func(i, j int) bool {
				if (rows[i].Rating > 0) != (rows[j].Rating > 0) {
					return rows[i].Rating > 0
				}
				if rows[i].Rating != rows[j].Rating {
					return rows[i].Rating > rows[j].Rating
				}
				return rows[i].Cheapest < rows[j].Cheapest
			})

			gated := func(canon string) bool {
				ri := reviewIdx[canon]
				return ri.Score == 0 || ri.Score >= minRating // unknown rating passes (don't hide)
			}

			// Top companies: rating-gated, ranked, capped at --top.
			var topRows []companyRow
			for _, r := range rows {
				if r.Rating >= minRating {
					topRows = append(topRows, r)
				}
				if len(topRows) >= top {
					break
				}
			}

			// Picks. Fully-insured small + bigger from gated companies.
			sort.SliceStable(zeroExcess, func(i, j int) bool { return zeroExcess[i].fiTotal < zeroExcess[j].fiTotal })
			sort.SliceStable(coverable, func(i, j int) bool { return coverable[i].fiTotal < coverable[j].fiTotal })
			var picks []pickRow
			pickSize := func(size string) {
				for _, o := range zeroExcess {
					if o.size == size && gated(o.canonical) {
						picks = append(picks, pickRow{
							Kind: "full-insurance-" + size, Company: o.canonical, Car: o.offer.Car,
							Total: o.fiTotal, Currency: "EUR", Rating: reviewIdx[o.canonical].Score,
						})
						return
					}
				}
			}
			pickSize("small")
			pickSize("bigger")
			for _, o := range coverable {
				if len(picks) >= 2+coverPicks {
					break
				}
				if !gated(o.canonical) {
					continue
				}
				picks = append(picks, pickRow{
					Kind: "cover", Company: o.canonical, Car: o.offer.Car,
					Total: o.fiTotal, Base: o.offer.Total, Cover: excessCoverEstimate(days, coverBase, coverPerDay),
					Currency: "EUR", Rating: reviewIdx[o.canonical].Score,
				})
			}

			if wantsMachineOutput(flags) || flags.asJSON {
				b, _ := json.Marshal(map[string]any{
					"location": displayLocation(loc, location), "pickup": pickup, "dropoff": dropoff, "days": days,
					"min_rating": minRating, "top_companies": topRows, "picks": picks, "all_companies": rows,
				})
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			renderRecommend(cmd, displayLocation(loc, location), pickup, dropoff, days, age, minRating, topRows, picks, rows)
			return nil
		},
	}
	cmd.Flags().IntVar(&top, "top", 8, "Max top-rated companies to list")
	cmd.Flags().Float64Var(&minRating, "min-rating", 7.0, "Only recommend companies rated at or above this (0-10)")
	cmd.Flags().IntVar(&coverPicks, "cover-picks", 3, "How many cheap base+cover cars to suggest")
	cmd.Flags().Float64Var(&coverBase, "excess-cover-base", DefaultExcessCoverBase, "Standalone excess-insurance fixed policy base (EUR)")
	cmd.Flags().Float64Var(&coverPerDay, "excess-cover-per-day", DefaultExcessCoverPerDay, "Standalone excess-insurance per-day estimate (EUR)")
	cmd.Flags().StringVar(&pickupTime, "pickup-time", "10:00", "Pickup time HH:MM")
	cmd.Flags().StringVar(&dropoffTime, "dropoff-time", "10:00", "Dropoff time HH:MM")
	cmd.Flags().IntVar(&age, "driver-age", 35, "Driver age (used for eligibility/validation; under-25 surcharges are charged at the counter, not in the quote)")
	return cmd
}

func displayLocation(loc resolvedLocation, raw string) string {
	if loc.Name != "" {
		return loc.Name
	}
	if loc.IATA != "" {
		return loc.IATA
	}
	return raw
}

// companyCaveats generates the "things to be aware of" notes for a company.
func companyCaveats(canon string, rating, minRating float64, priceSrc string) []string {
	var cav []string
	if rating > 0 && rating < minRating {
		cav = append(cav, fmt.Sprintf("below-average rating (%.1f)", rating))
	}
	if rating > 0 && rating < 6.5 {
		cav = append(cav, "read fuel/deposit terms carefully")
	}
	if strings.EqualFold(canon, "Centauro") {
		cav = append(cav, "airport office 07:00–23:00")
	}
	if priceSrc == "aggregator" {
		cav = append(cav, "price incl. standalone excess cover")
	}
	return cav
}

func renderRecommend(cmd *cobra.Command, location, pickup, dropoff string, days, age int, minRating float64, top []companyRow, picks []pickRow, all []companyRow) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s — %s to %s (%d days). Prices are fully insured (zero excess).\n\n", location, pickup, dropoff, days)

	fmt.Fprintf(w, "TOP COMPANIES (rated ≥ %.1f, most trusted first)\n", minRating)
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "COMPANY\tRATING\tREVIEWS\tCHEAPEST\tSOURCE\tNOTES")
	for _, r := range top {
		fmt.Fprintf(tw, "%s\t%.1f/10\t%d\t%.2f %s\t%s\t%s\n", r.Company, r.Rating, r.Reviews, r.Cheapest, r.Currency, r.Source, strings.Join(r.Caveats, "; "))
	}
	tw.Flush()

	fmt.Fprintln(w, "\nRECOMMENDED CARS")
	tw2 := newTabWriter(w)
	fmt.Fprintln(tw2, "PICK\tCOMPANY\tCAR\tFULLY INSURED\tRATING")
	for _, p := range picks {
		label := map[string]string{"full-insurance-small": "small (full ins)", "full-insurance-bigger": "bigger (full ins)", "cover": "cheap + cover"}[p.Kind]
		rating := ""
		if p.Rating > 0 {
			rating = fmt.Sprintf("%.1f", p.Rating)
		}
		detail := ""
		if p.Kind == "cover" {
			detail = fmt.Sprintf("  (base %.0f + cover %.0f)", p.Base, p.Cover)
		}
		fmt.Fprintf(tw2, "%s\t%s\t%s\t%.2f %s%s\t%s\n", label, p.Company, truncate(p.Car, 22), p.Total, p.Currency, detail, rating)
	}
	tw2.Flush()

	fmt.Fprintln(w, "\nALL COMPANIES AT THIS AIRPORT (by rating)")
	tw3 := newTabWriter(w)
	fmt.Fprintln(tw3, "COMPANY\tRATING\tREVIEWS\tCHEAPEST\tNOTES")
	for _, r := range all {
		rating := "?"
		if r.Rating > 0 {
			rating = fmt.Sprintf("%.1f/10", r.Rating)
		}
		flag := strings.Join(r.Caveats, "; ")
		if r.belowGate && flag == "" {
			flag = "below rating gate"
		}
		fmt.Fprintf(tw3, "%s\t%s\t%d\t%.2f %s\t%s\n", r.Company, rating, r.Reviews, r.Cheapest, r.Currency, flag)
	}
	tw3.Flush()
	fmt.Fprintln(w, "\nCHEAPEST = cheapest fully-insured price (direct where we have it, else base + standalone cover).")
	if note := youngDriverNotice(age); note != "" {
		fmt.Fprintln(w, note)
	}
}
