// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: direct full-insurance quotes from the supplier's own sites.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/carsource"

	"github.com/spf13/cobra"
)

// directCompany names a supplier we can quote directly (own booking site),
// always with zero-excess / full-insurance pricing.
type directCompany struct {
	name  string
	quote func(ctx context.Context, flags *rootFlags, loc resolvedLocation, pickup, dropoff, pickupTime, dropoffTime string, age int, now time.Time) ([]carsource.Offer, error)
}

// isMalaga reports whether a resolved location is Málaga (the default) or
// unspecified.
func isMalaga(loc resolvedLocation) bool {
	return loc.IATA == "" || strings.EqualFold(loc.IATA, "AGP")
}

func directCompanies() []directCompany {
	return []directCompany{
		{name: "Delpaso", quote: func(ctx context.Context, flags *rootFlags, loc resolvedLocation, pickup, dropoff, pt, dt string, age int, now time.Time) ([]carsource.Offer, error) {
			if !isMalaga(loc) {
				return nil, fmt.Errorf("Delpaso is Málaga-only")
			}
			return carsource.NewDelpaso(carHTTPClient(flags)).Quote(ctx, pickup, dropoff, pt, dt, age) // pp:client-call
		}},
		{name: "Centauro", quote: func(ctx context.Context, flags *rootFlags, loc resolvedLocation, pickup, dropoff, pt, dt string, age int, now time.Time) ([]carsource.Offer, error) {
			c := carsource.NewCentauro(carHTTPClient(flags))
			branch := ""
			if !isMalaga(loc) {
				b, err := c.BranchCodeForAirport(ctx, loc.Name) // pp:client-call
				if err != nil {
					return nil, err
				}
				branch = b
			}
			return c.Quote(ctx, branch, pickup, dropoff, pt, dt, age, now) // pp:client-call
		}},
		{name: "Drivalia", quote: func(ctx context.Context, flags *rootFlags, loc resolvedLocation, pickup, dropoff, pt, dt string, age int, now time.Time) ([]carsource.Offer, error) {
			d := carsource.NewDrivalia(carHTTPClient(flags))
			office := ""
			if !isMalaga(loc) {
				o, err := d.OfficeIDForAirport(ctx, loc.IATA) // pp:client-call
				if err != nil {
					return nil, err
				}
				office = o
			}
			return d.Quote(ctx, office, pickup, dropoff, pt, dt, age) // pp:client-call
		}},
		{name: "Autoreisen", quote: func(ctx context.Context, flags *rootFlags, loc resolvedLocation, pickup, dropoff, pt, dt string, age int, now time.Time) ([]carsource.Offer, error) {
			// Canary Islands only; base price is already zero-excess.
			return carsource.NewAutoreisen(carHTTPClient(flags)).Quote(ctx, loc.IATA, pickup, dropoff, pt, dt) // pp:client-call
		}},
		{name: "Clickrent", quote: func(ctx context.Context, flags *rootFlags, loc resolvedLocation, pickup, dropoff, pt, dt string, age int, now time.Time) ([]carsource.Offer, error) {
			return carsource.NewClickrent(carHTTPClient(flags)).Quote(ctx, clickrentIATA(loc), pickup, dropoff, pt, dt) // pp:client-call
		}},
		{name: "CICAR", quote: func(ctx context.Context, flags *rootFlags, loc resolvedLocation, pickup, dropoff, pt, dt string, age int, now time.Time) ([]carsource.Offer, error) {
			// Canary Islands only; base price is already all-inclusive zero-excess.
			return carsource.NewCicar(carHTTPClient(flags)).Quote(ctx, loc.IATA, pickup, dropoff, pt, dt) // pp:client-call
		}},
		{name: "Goldcar", quote: func(ctx context.Context, flags *rootFlags, loc resolvedLocation, pickup, dropoff, pt, dt string, age int, now time.Time) ([]carsource.Offer, error) {
			return carsource.NewGoldcar(carHTTPClient(flags)).Quote(ctx, clickrentIATA(loc), pickup, dropoff, pt, dt, age) // pp:client-call
		}},
	}
}

// enabledDirectCompanies returns the direct-supplier clients not turned off via
// --disable-supplier (matched case-insensitively by company name). Lets a user
// skip any supplier whose terms they'd rather not touch.
func enabledDirectCompanies(flags *rootFlags) []directCompany {
	disabled := disabledSupplierSet(flags)
	all := directCompanies()
	if len(disabled) == 0 {
		return all
	}
	out := make([]directCompany, 0, len(all))
	for _, co := range all {
		if !disabled[strings.ToLower(co.name)] {
			out = append(out, co)
		}
	}
	return out
}

// clickrentIATA returns the IATA for the resolved location, defaulting to
// Málaga when unspecified.
func clickrentIATA(loc resolvedLocation) string {
	if loc.IATA == "" {
		return "AGP"
	}
	return loc.IATA
}

type directResult struct {
	Company  string            `json:"company"`
	Cheapest *carsource.Offer  `json:"cheapest"`
	Options  []carsource.Offer `json:"options,omitempty"`
	Error    string            `json:"error,omitempty"`
}

func newDirectCmd(flags *rootFlags) *cobra.Command {
	var pickupTime, dropoffTime string
	var age, limit, options int
	var classFilter string
	cmd := &cobra.Command{
		Use:         "direct [location] <pickup> <dropoff>",
		Short:       "Full-insurance (zero-excess) quotes straight from the suppliers' own sites",
		Long:        "Query the rental companies' own booking sites directly and show their cheapest full-insurance (zero-excess) price for the dates — the true all-inclusive price, not an aggregator teaser. Companies queried: Delpaso (Málaga only), Centauro, Drivalia, Clickrent and Goldcar (mainland + islands), plus CICAR and Autoreisen (Canary Islands only). Each company that doesn't serve the chosen airport is simply marked unavailable. Location defaults to Málaga; pass an IATA code or airport name for elsewhere in Spain. Dates are dd/mm/yyyy or yyyy-mm-dd.\n\nNote: some offices have limited opening hours (e.g. Centauro Málaga Airport is 07:00–23:00), so a very early/late return time may be rejected — the per-company error will say so.",
		Example:     "  rentalcarspain-pp-cli direct 14/07/2026 22/07/2026 --pickup-time 16:30 --dropoff-time 10:00 --agent",
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
				return usageErr(fmt.Errorf("direct needs <pickup> <dropoff> (Málaga default) or <location> <pickup> <dropoff>"))
			}
			loc := resolveLocationInput(location)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			now := time.Now()

			companies := enabledDirectCompanies(flags)
			results := make([]directResult, len(companies))
			var wg sync.WaitGroup
			for i, co := range companies {
				wg.Add(1)
				go func(i int, co directCompany) {
					defer wg.Done()
					r := directResult{Company: co.name}
					offers, err := co.quote(ctx, flags, loc, pickup, dropoff, pickupTime, dropoffTime, age, now)
					if err != nil {
						r.Error = err.Error()
						results[i] = r
						return
					}
					if classFilter != "" {
						kept := offers[:0]
						for _, o := range offers {
							if classFilterMatch(o, classFilter) {
								kept = append(kept, o)
							}
						}
						offers = kept
					}
					sort.SliceStable(offers, func(a, b int) bool { return offers[a].Total < offers[b].Total })
					if len(offers) > 0 {
						best := offers[0]
						r.Cheapest = &best
					}
					cap := limit
					if options > cap {
						cap = options
					}
					if cap > 0 && len(offers) > cap {
						offers = offers[:cap]
					}
					r.Options = offers
					results[i] = r
				}(i, co)
			}
			wg.Wait()

			if wantsMachineOutput(flags) || flags.asJSON {
				b, _ := json.Marshal(map[string]any{"pickup": pickup, "dropoff": dropoff, "companies": results})
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			w := cmd.OutOrStdout()
			mf, mfErr := newMoneyFmt(ctx, flags)
			if mfErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", mfErr)
			}
			tw := newTabWriter(w)
			if options <= 1 {
				fmt.Fprintln(tw, "COMPANY\tCHEAPEST CAR\tTOTAL (zero-excess)")
			} else {
				fmt.Fprintln(tw, "COMPANY\tCAR\tCLASS\tTOTAL (zero-excess)")
			}
			for _, r := range results {
				if r.Error != "" {
					fmt.Fprintf(tw, "%s\t(unavailable)\t%s\n", r.Company, truncate(r.Error, 48))
					continue
				}
				if len(r.Options) == 0 {
					fmt.Fprintf(tw, "%s\t(no offers)\t-\n", r.Company)
					continue
				}
				if options <= 1 {
					fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Company, carCellWithAge(*r.Cheapest, age, 28), mf.format(r.Cheapest.Total))
					continue
				}
				// One row per car, company name only on the first row.
				n := options
				if n > len(r.Options) {
					n = len(r.Options)
				}
				for j := 0; j < n; j++ {
					o := r.Options[j]
					name := r.Company
					if j > 0 {
						name = ""
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", name, carCellWithAge(o, age, 28), truncate(o.CarClass, 10), mf.format(o.Total))
				}
			}
			tw.Flush()
			fmt.Fprintln(w, "\nAll prices are full-insurance / zero-excess, quoted from each supplier's own site.")
			if mf.active() {
				fmt.Fprintf(w, "%s amounts are indicative (ECB rate %s); you are billed in EUR.\n", mf.currency, mf.rates.Date)
			}
			if note := youngDriverNotice(age); note != "" {
				fmt.Fprintln(w, note)
			}
			fmt.Fprintln(w, "Book at:")
			for _, r := range results {
				if len(r.Options) > 0 && r.Options[0].URL != "" {
					fmt.Fprintf(w, "  %-9s %s\n", r.Company, r.Options[0].URL)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&pickupTime, "pickup-time", "10:00", "Pickup time HH:MM")
	cmd.Flags().StringVar(&dropoffTime, "dropoff-time", "10:00", "Dropoff time HH:MM (offices may have limited hours)")
	cmd.Flags().IntVar(&age, "driver-age", 35, "Driver age (used for eligibility/validation; under-25 surcharges are charged at the counter, not in the quote)")
	cmd.Flags().IntVar(&limit, "limit", 6, "Max options to include per company in JSON output")
	cmd.Flags().IntVar(&options, "options", 1, "Cars to show per company in the table (1 = cheapest only)")
	cmd.Flags().StringVar(&classFilter, "class", "", "Only show cars matching these brand/type/size substrings (comma-separated, e.g. bmw,cabrio,suv)")
	return cmd
}
