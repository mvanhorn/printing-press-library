// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/airport-limousine/internal/limousine"
	"github.com/spf13/cobra"
)

func newNovelFareCmd(flags *rootFlags) *cobra.Command {
	var route, date, direction, from, to, trip string
	var adults, children int
	cmd := &cobra.Command{Use: "fare [route-id]", Short: "Calculate a one-way party total from a served pair's published fare", Example: "  airport-limousine-pp-cli fare Haneda-Narita --from-stop HanedaAirportTerminal3 --to-stop NaritaAirportTerminal1 --adults 2 --children 1 --agent", Annotations: limousineAnnotations("route-id=Haneda-Narita"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "fare")
		}
		if e := limousineLive(flags); e != nil {
			return e
		}
		id, e := limousineRoute(args, route)
		if e != nil {
			return e
		}
		if e = limousineDirection(direction, false); e != nil {
			return e
		}
		day, e := limousineDate(date)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		p := limousine.New(flags.timeout, flags.rateLimit)
		schedule, e := p.Timetable(ctx, id, day, direction)
		if e != nil {
			return limousineError(e)
		}
		quote, e := limousine.Quote(schedule, from, to, adults, children, trip)
		if e != nil {
			return usageErr(e)
		}
		return flags.printJSON(cmd, limousine.Envelope{Meta: p.Meta([]string{schedule.SourceURL}, schedule.ScannedTrains, 1, 1, "Planning arithmetic uses exact published units and does not check inventory or apply unverified discounts."), Results: []limousine.FareQuote{quote}})
	}}
	cmd.Flags().StringVar(&route, "route", "Haneda-Narita", "Provider area ID from routes, or one positional argument")
	cmd.Flags().StringVar(&date, "date", "", "JST service date YYYY-MM-DD; defaults to today in Tokyo")
	cmd.Flags().StringVar(&direction, "direction", "from-airport", "Route direction: from-airport or to-airport")
	cmd.Flags().StringVar(&from, "from-stop", "HanedaAirportTerminal3", "Exact boarding stop ID; default is Haneda Terminal 3")
	cmd.Flags().StringVar(&to, "to-stop", "NaritaAirportTerminal1", "Exact alighting stop ID; default is Narita Terminal 1")
	cmd.Flags().StringVar(&trip, "trip-id", "", "Select one served trip ID when fares differ across trips")
	cmd.Flags().IntVar(&adults, "adults", 1, "Passengers charged the adult unit, from zero to 100")
	cmd.Flags().IntVar(&children, "children", 0, "Passengers eligible for the child unit; infants occupying seats generally need child fare")
	return cmd
}
