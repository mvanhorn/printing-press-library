// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/yamato/internal/yamato"
	"github.com/spf13/cobra"
	"time"
)

// pp:data-source live
func newNovelQuoteCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "quote"}
	var in yamato.QuoteInput
	var detail bool
	cmd = luggageCmd(cmd, "Fetch current source rate and calendar result for a distinct luggage product.", "--origin 1000005 --destination 6008216 --date "+time.Now().In(time.FixedZone("JST", 32400)).AddDate(0, 0, 7).Format("2006-01-02")+" --size 160 --agent", "live", f)
	cmd.Long = "Query Yamato's public read-only calculator. Use date-kind ship for shipping date, delivery for standard requested arrival, boarding for airport flight date, and use for lodging roundtrip check-in. Source dates are estimated and local dispatch cutoff times remain unknown. Rates are tax-inclusive list tariffs per parcel; conditional discounts and packaging are not automatically applied. Use parcel to determine the chargeable size from both measured dimensions and weight."
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "quote")
		}
		if e := validateDataSourceStrategy(f, cmd.Annotations["pp:data-source"]); e != nil {
			return usageErr(e)
		}
		if cmd.Annotations["pp:data-source"] == "computed" && f.dataSource == "live" {
			return usageErr(fmt.Errorf("parcel uses verified local rules; choose --data-source auto or local"))
		}
		if e := noLuggageArgs(args); e != nil {
			return e
		}
		if e := in.Validate(time.Now()); e != nil {
			return usageErr(e)
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		c := luggageClient(f)
		q, e := c.Quote(ctx, in, detail)
		if e != nil {
			return luggageError(e)
		}
		return luggageOutput(cmd, f, c, q)
	}
	cmd.Flags().StringVar(&in.Service, "service", "takkyubin", "Luggage product: takkyubin, airport, roundtrip or airport-roundtrip")
	cmd.Flags().StringVar(&in.Origin, "origin", "", "Origin seven-digit Japanese postal code, optional hyphen")
	cmd.Flags().StringVar(&in.Destination, "destination", "", "Destination Japanese postal code for standard or lodging roundtrip")
	cmd.Flags().StringVar(&in.Airport, "airport", "", "Exact live airport terminal ID from the airports command")
	cmd.Flags().StringVar(&in.Date, "date", "", "Shipping, arrival, boarding or use date YYYY-MM-DD in JST")
	cmd.Flags().StringVar(&in.DateKind, "date-kind", "ship", "Date meaning: ship, delivery, boarding or use; must match service")
	cmd.Flags().IntVar(&in.Size, "size", 0, "Chargeable category60..200 from parcel; weight must also meet category")
	cmd.Flags().BoolVar(&detail, "detail", false, "Include the full source cash/cashless size-rate table")
	return cmd
}
