// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/airport-limousine/internal/limousine"
	"github.com/spf13/cobra"
)

func newNovelTimetableCmd(flags *rootFlags) *cobra.Command {
	var route, date, direction, from, to, after string
	var limit int
	cmd := &cobra.Command{Use: "timetable [route-id]", Short: "Read a dated timetable with exact terminals and absolute JST times", Example: "  airport-limousine-pp-cli timetable Haneda-Narita --from-stop HanedaAirportTerminal3 --to-stop NaritaAirportTerminal1 --after 08:00 --agent", Annotations: limousineAnnotations("route-id=Haneda-Narita"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "timetable")
		}
		if e := limousineLive(flags); e != nil {
			return e
		}
		id, e := limousineRoute(args, route)
		if e != nil {
			return e
		}
		if e = limousineLimit(limit); e != nil {
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
		rows, e := limousine.FilterJourneys(schedule, from, to, after)
		if e != nil {
			return usageErr(e)
		}
		total := len(rows)
		if total > limit {
			rows = rows[:limit]
		}
		note := "Scheduled times and published station fares; seat inventory is intentionally omitted. Extended-hour and inferred rollover are identified in each time. Source created_at has no verified timezone. Confirm canonical source for operator changes."
		return flags.printJSON(cmd, limousine.Envelope{Meta: p.Meta([]string{schedule.SourceURL}, schedule.ScannedTrains, total, len(rows), limousineNote(total, len(rows), note)), Results: rows, Stations: schedule.Stations, Details: map[string]any{"route_id": schedule.RouteID, "route_name": schedule.RouteName, "direction": schedule.Direction, "source_direction": schedule.SourceDirection, "selected_date_jst": schedule.SelectedDate, "first_source_date_jst": schedule.FirstSourceDate, "last_source_date_jst": schedule.LastSourceDate, "suspended": schedule.Suspended, "source_alerts": schedule.Alerts}})
	}}
	cmd.Flags().StringVar(&route, "route", "Haneda-Narita", "Provider area ID from routes, or one positional argument")
	cmd.Flags().StringVar(&date, "date", "", "JST service date YYYY-MM-DD; defaults to today in Tokyo")
	cmd.Flags().StringVar(&direction, "direction", "from-airport", "Route direction: from-airport or to-airport")
	cmd.Flags().StringVar(&from, "from-stop", "", "Exact boarding stop ID; airport terminals are distinct")
	cmd.Flags().StringVar(&to, "to-stop", "", "Exact alighting stop ID; airport terminals are distinct")
	cmd.Flags().StringVar(&after, "after", "", "Earliest boarding time HH:MM in the service day; requires --from-stop")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum scheduled journeys returned, from 1 to 200")
	return cmd
}
