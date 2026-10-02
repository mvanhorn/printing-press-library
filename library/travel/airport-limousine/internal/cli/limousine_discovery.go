// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/airport-limousine/internal/limousine"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newLimousineRoutesCmd(flags))
		addNovelCommandIfAbsent(root, newLimousineStopsCmd(flags))
		addNovelCommandIfAbsent(root, newLimousineHandoffCmd(flags))
	})
}
func newLimousineRoutesCmd(flags *rootFlags) *cobra.Command {
	var query, airport string
	var limit int
	var suspended bool
	cmd := &cobra.Command{Use: "routes", Short: "Find operator route IDs, airports and both timetable directions", Example: "  airport-limousine-pp-cli routes --query Shinjuku --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--query=Shinjuku"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "routes")
		}
		if e := limousineLive(flags); e != nil {
			return e
		}
		if len(args) != 0 {
			return usageErr(fmt.Errorf("use --query to filter routes"))
		}
		if e := limousineLimit(limit); e != nil {
			return e
		}
		if e := limousineAirport(airport); e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		p := limousine.New(flags.timeout, flags.rateLimit)
		data, e := p.Data(ctx, "/en/timetable/list/__data.json")
		if e != nil {
			return limousineError(e)
		}
		rows, scanned, e := limousine.Routes(data, limousineQuery(query), airport, suspended)
		if e != nil {
			return limousineError(e)
		}
		total := len(rows)
		if len(rows) > limit {
			rows = rows[:limit]
		}
		return flags.printJSON(cmd, limousine.Envelope{Meta: p.Meta([]string{limousine.PageURL("/en/timetable/list", nil)}, scanned, total, len(rows), limousineNote(total, len(rows), "Provider route-area listing; CMS placeholder records omitted. Suspended routes need --include-suspended.")), Results: rows})
	}}
	cmd.Flags().StringVar(&query, "query", "", "Case-insensitive substring of route ID or area name")
	cmd.Flags().StringVar(&airport, "airport", "", "Restrict the route airport to haneda or narita")
	cmd.Flags().IntVar(&limit, "limit", 30, "Maximum matching routes returned, from 1 to 200")
	cmd.Flags().BoolVar(&suspended, "include-suspended", false, "Include routes the source explicitly marks suspended")
	return cmd
}
func newLimousineStopsCmd(flags *rootFlags) *cobra.Command {
	parent := &cobra.Command{Use: "stops", Short: "Find exact stops and inspect boarding locations"}
	parent.AddCommand(newLimousineStopsFindCmd(flags), newLimousineStopGetCmd(flags))
	return parent
}
func newLimousineStopsFindCmd(flags *rootFlags) *cobra.Command {
	var query, airport string
	var limit int
	cmd := &cobra.Command{Use: "find", Short: "Search operator stops with exact English and Japanese IDs", Example: "  airport-limousine-pp-cli stops find --query Shinjuku --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--query=Shinjuku"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "stops find")
		}
		if e := limousineLive(flags); e != nil {
			return e
		}
		if len(args) != 0 {
			return usageErr(fmt.Errorf("stops find takes --query, not positional arguments"))
		}
		query = limousineQuery(query)
		if query == "" {
			if !hasChangedLocalFlags(cmd) && !flags.agent && !flags.asJSON {
				return cmd.Help()
			}
			return usageErr(fmt.Errorf("stops find requires --query"))
		}
		if len([]rune(query)) > 80 {
			return usageErr(fmt.Errorf("--query must have at most 80 characters"))
		}
		if e := limousineLimit(limit); e != nil {
			return e
		}
		if e := limousineAirport(airport); e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		p := limousine.New(flags.timeout, flags.rateLimit)
		data, e := p.StopSearch(ctx, query)
		if e != nil {
			return limousineError(e)
		}
		rows, scanned, e := limousine.Stops(data, query, airport)
		if e != nil {
			return limousineError(e)
		}
		total := len(rows)
		if total > limit {
			rows = rows[:limit]
		}
		return flags.printJSON(cmd, limousine.Envelope{Meta: p.Meta([]string{limousine.PageURL("/en/busstop", nil)}, scanned, total, len(rows), limousineNote(total, len(rows), "Public provider keyword search with an ephemeral private cookie jar; exact terminals remain separate. No further result pages are scanned.")), Results: rows})
	}}
	cmd.Flags().StringVar(&query, "query", "", "Provider stop-name keyword, from 1 to 80 characters")
	cmd.Flags().StringVar(&airport, "airport", "", "Keep stops connected to haneda or narita airport")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum returned stops, from 1 to 200")
	return cmd
}
func newLimousineStopGetCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "get [stop-id]", Short: "Inspect a stop's address, boarding maps and connected routes", Example: "  airport-limousine-pp-cli stops get HanedaAirportTerminal3 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "stop-id=HanedaAirportTerminal3"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "stops get")
		}
		if e := limousineLive(flags); e != nil {
			return e
		}
		if len(args) == 0 && !hasChangedLocalFlags(cmd) && !flags.agent && !flags.asJSON {
			return cmd.Help()
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("stops get requires one exact stop ID from stops find"))
		}
		if e := limousine.ValidateID(args[0]); e != nil {
			return usageErr(e)
		}
		if e := limousineLimit(limit); e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		p := limousine.New(flags.timeout, flags.rateLimit)
		data, e := p.Data(ctx, limousine.DataPath("/en/busstop/detail/"+args[0], nil))
		if e != nil {
			return limousineError(e)
		}
		detail, e := limousine.Detail(data, limit)
		if e != nil {
			return limousineError(e)
		}
		if detail.Stop.ID != args[0] {
			return apiErr(fmt.Errorf("provider resolved a different stop"))
		}
		return flags.printJSON(cmd, limousine.Envelope{Meta: p.Meta([]string{detail.Stop.SourceURL}, detail.TotalConnections, detail.TotalConnections, len(detail.Connections), "Location metadata has its own source update date; observations do not prove bus operation or seats."), Results: []limousine.StopDetail{detail}})
	}}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum connected route records returned, from 1 to 200")
	return cmd
}
func newLimousineHandoffCmd(flags *rootFlags) *cobra.Command {
	var date, direction, route string
	cmd := &cobra.Command{Use: "handoff [route-id]", Short: "Resolve a dated canonical timetable for user booking handoff", Example: "  airport-limousine-pp-cli handoff Haneda-Narita --agent", Annotations: limousineAnnotations("route-id=Haneda-Narita"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "handoff")
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
		rows := []map[string]any{{"route_id": id, "service_date_jst": day, "direction": direction, "timetable_url": schedule.SourceURL, "booking_handoff_url": schedule.SourceURL, "user_booking_required": true, "seat_availability_checked": false}}
		return flags.printJSON(cmd, limousine.Envelope{Meta: p.Meta([]string{schedule.SourceURL}, schedule.ScannedTrains, 1, 1, "Follow the canonical timetable's reservation links to book. This command performs no reservation or payment."), Results: rows})
	}}
	cmd.Flags().StringVar(&route, "route", "Haneda-Narita", "Provider area ID, also accepted as one positional argument")
	cmd.Flags().StringVar(&date, "date", "", "JST service date YYYY-MM-DD; defaults to today in Tokyo")
	cmd.Flags().StringVar(&direction, "direction", "from-airport", "Route direction: from-airport or to-airport")
	return cmd
}
