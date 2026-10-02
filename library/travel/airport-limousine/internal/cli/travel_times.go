// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/airport-limousine/internal/limousine"
	"github.com/spf13/cobra"
)

func newNovelTravelTimesCmd(flags *rootFlags) *cobra.Command {
	var query, airport, direction string
	var limit int
	cmd := &cobra.Command{Use: "travel-times", Short: "Compare current and standard durations with explicit source states", Example: "  airport-limousine-pp-cli travel-times --airport haneda --query Shinjuku --agent", Annotations: limousineAnnotations("--airport=haneda"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "travel-times")
		}
		if e := limousineLive(flags); e != nil {
			return e
		}
		if len(args) != 0 {
			return usageErr(fmt.Errorf("travel-times takes --query, not positional arguments"))
		}
		if e := limousineAirport(airport); e != nil {
			return e
		}
		if e := limousineDirection(direction, true); e != nil {
			return e
		}
		if e := limousineLimit(limit); e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		p := limousine.New(flags.timeout, flags.rateLimit)
		data, e := p.Data(ctx, "/en/guide/realtime/__data.json")
		if e != nil {
			return limousineError(e)
		}
		rows, scanned, e := limousine.Durations(data, limousineQuery(query), airport, direction)
		if e != nil {
			return limousineError(e)
		}
		total := len(rows)
		if total > limit {
			rows = rows[:limit]
		}
		return flags.printJSON(cmd, limousine.Envelope{Meta: p.Meta([]string{limousine.PageURL("/en/guide/realtime", nil)}, scanned, total, len(rows), limousineNote(total, len(rows), "The source publishes a JST clock without a date. Numeric values are estimates, not arrival guarantees. Adjusting/retrieving/unknown values remain null; route labels do not identify an airport terminal.")), Results: rows})
	}}
	cmd.Flags().StringVar(&airport, "airport", "", "Select the provider's haneda or narita table")
	cmd.Flags().StringVar(&direction, "direction", "", "Keep from-airport or to-airport rows within that table")
	cmd.Flags().StringVar(&query, "query", "", "Case-insensitive substring of the exact source route label")
	cmd.Flags().IntVar(&limit, "limit", 40, "Maximum source estimate rows returned, from 1 to 200")
	return cmd
}
