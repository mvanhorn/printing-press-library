// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/carsource"

	"github.com/spf13/cobra"
)

type searchView struct {
	Location  string            `json:"location"`
	Pickup    string            `json:"pickup"`
	Dropoff   string            `json:"dropoff"`
	DriverAge int               `json:"driver_age"`
	Basis     string            `json:"price_basis"`
	Count     int               `json:"count"`
	Offers    []carsource.Offer `json:"offers"`
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var f searchFilters
	var pickupTime, dropoffTime, source string
	cmd := &cobra.Command{
		Use:   "search [location-code] <pickup> <dropoff>",
		Short: "Search DoYouSpain for the cheapest full-insurance rental offers (Málaga by default)",
		Long: strings.Trim(`
Search DoYouSpain — the aggregator that surfaces the cheapest rental companies
in an area — for a pickup location and date range. Prices default to the Full
Insurance / Zero Excess tier; pass --base for the bare rate.

The location defaults to Málaga Airport (MAL02), so you can pass just the dates:
'search 20/08/2026 27/08/2026'. Pass an explicit code as the first argument to
search elsewhere (codes come from 'locations'). Dates are dd/mm/yyyy or
yyyy-mm-dd.`, "\n"),
		Example: strings.Trim(`
  rentalcarspain-pp-cli search 20/08/2026 27/08/2026 --sort cheapest
  rentalcarspain-pp-cli search 20/08/2026 27/08/2026 --supplier delpaso,recordgo,wiber --agent
  rentalcarspain-pp-cli search MAL02 20/08/2026 27/08/2026`, "\n"),
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
				return usageErr(fmt.Errorf("search needs <pickup> <dropoff> (Málaga default) or <location-code> <pickup> <dropoff>"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			offers, srcErrs := fetchOffers(ctx, flags, source, location, pickup, dropoff, pickupTime, dropoffTime, f.driverAge)
			if len(offers) == 0 {
				if err := sourceErrorsError(srcErrs); err != nil {
					return err
				}
				return apiErr(fmt.Errorf("no offers returned"))
			}
			recordSnapshotFor(ctx, flags, searchKey(location, "", pickup, dropoff, f.driverAge), offers)

			filtered := f.apply(offers)
			basis := "full-insurance"
			if f.base {
				basis = "base"
			}
			view := searchView{
				Location:  location,
				Pickup:    pickup,
				Dropoff:   dropoff,
				DriverAge: f.driverAge,
				Basis:     basis,
				Count:     len(filtered),
				Offers:    filtered,
			}
			if wantsMachineOutput(flags) || flags.asJSON {
				b, _ := json.Marshal(view)
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			if len(srcErrs) > 0 {
				for s, e := range srcErrs {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: source %s unavailable: %s\n", s, truncate(e.Error(), 80))
				}
			}
			return renderOffersTable(cmd, filtered, basis)
		},
	}
	// Register directly on the command (not via a helper) so static skill/flag
	// verification can see every flag on this command.
	cmd.Flags().StringVar(&f.suppliers, "supplier", "", "Only show these suppliers (comma-separated, e.g. delpaso,recordgo,wiber)")
	cmd.Flags().StringVar(&f.class, "class", "", "Only show these car classes (comma-separated substrings, e.g. small,suv)")
	cmd.Flags().Float64Var(&f.maxTotal, "max-total", 0, "Only show offers at or below this total price")
	cmd.Flags().Float64Var(&f.maxPerDay, "max-per-day", 0, "Only show offers at or below this per-day price")
	cmd.Flags().IntVar(&f.driverAge, "driver-age", 35, "Driver age (used for eligibility/validation; under-25 surcharges are charged at the counter, not in the quote)")
	cmd.Flags().StringVar(&f.transmission, "transmission", "", "Filter by transmission: manual or automatic")
	cmd.Flags().StringVar(&f.currency, "currency", "EUR", "Currency label for prices (EUR)")
	cmd.Flags().BoolVar(&f.base, "base", false, "Show the struck-through original rate instead of the current discounted total")
	cmd.Flags().StringVar(&f.sortBy, "sort", "cheapest", "Sort order: cheapest|per-day|total|supplier")
	cmd.Flags().IntVar(&f.limit, "limit", 30, "Maximum offers to return")
	cmd.Flags().StringVar(&pickupTime, "pickup-time", "10:00", "Pickup time HH:MM (DoYouSpain uses 30-minute slots)")
	cmd.Flags().StringVar(&dropoffTime, "dropoff-time", "10:00", "Dropoff time HH:MM (DoYouSpain uses 30-minute slots)")
	cmd.Flags().StringVar(&source, "source", "doyouspain", "Aggregator source: doyouspain|rentalcars|all")
	return cmd
}

func renderOffersTable(cmd *cobra.Command, offers []carsource.Offer, basis string) error {
	w := cmd.OutOrStdout()
	if len(offers) == 0 {
		fmt.Fprintln(w, "No offers found. Try widening filters or check 'rentalcarspain-pp-cli doctor'.")
		return nil
	}
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "SUPPLIER\tCAR\tTOTAL\tEXCESS\tSOURCE")
	for _, o := range offers {
		fmt.Fprintf(tw, "%s\t%s\t%.2f %s\t%s\t%s\n",
			o.Supplier, truncate(o.Car, 26), offerPrice(o, basis == "base"), o.Currency, excessLabel(o), o.Source)
	}
	tw.Flush()
	fmt.Fprintln(w, "\nTOTAL is the aggregator base rate (CDW + Theft Protection). EXCESS is the deductible/")
	fmt.Fprintln(w, "deposit you're liable for; 'none' = fully insured, '?' = not stated. For true zero-excess")
	fmt.Fprintln(w, "prices, use 'direct' (supplier sites) or 'compare'.")
	return nil
}

// excessLabel renders an offer's excess for the table: an amount when known,
// "none" for a zero-excess (fully insured) offer, or "?" when not stated.
func excessLabel(o carsource.Offer) string {
	if !o.ExcessKnown {
		return "?"
	}
	if o.Excess == 0 {
		return "none"
	}
	return fmt.Sprintf("%.0f %s", o.Excess, o.Currency)
}
