// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source auto
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/sunflower-ferry/internal/ferry"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(attachFerryCommands)
}

func attachFerryCommands(rootCmd *cobra.Command, flags *rootFlags) {
	// Own these command families whether generation emitted a scaffold or a
	// constructor. Literal registration also makes the source and runtime trees
	// agree for the Press's command-depth checks.
	owned := map[string]bool{"routes": true, "calendar": true, "quote": true, "sailings": true, "cabins": true, "ports": true, "conditions": true, "handoff": true}
	for _, existing := range rootCmd.Commands() {
		if owned[existing.Name()] {
			rootCmd.RemoveCommand(existing)
		}
	}
	rootCmd.AddCommand(ferryRoutesCmd(flags))
	rootCmd.AddCommand(newNovelCalendarCmd(flags))
	rootCmd.AddCommand(newNovelQuoteCmd(flags))
	rootCmd.AddCommand(newNovelSailingsCmd(flags))
	rootCmd.AddCommand(newNovelCabinsCmd(flags))
	rootCmd.AddCommand(newNovelPortsCmd(flags))
	rootCmd.AddCommand(newNovelConditionsCmd(flags))
	rootCmd.AddCommand(ferryHandoffCmd(flags))
}
func ferryAnnotations(strategy, happy string) map[string]string {
	return map[string]string{"mcp:read-only": "true", "pp:data-source": strategy, "pp:happy-args": happy}
}
func ferryWrite(cmd *cobra.Command, flags *rootFlags, data any, meta ferry.Metadata, warnings ...string) error {
	b, e := json.Marshal(ferry.Envelope{Data: data, Meta: meta, Warnings: warnings})
	if e != nil {
		return e
	}
	// Planning fields are already compact, bounded and consequential. Keep provenance
	// and uncertainty in the default agent projection as well as ordinary JSON.
	f := *flags
	f.compact = false
	if !f.csv && !f.plain && !f.quiet {
		f.asJSON = true
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), b, &f, map[string]any{"source": cmd.Annotations["pp:data-source"], "provider": "MOL Sunflower official public sources", "read_only": true})
}
func ferryCtx(cmd *cobra.Command, flags *rootFlags) (context.Context, context.CancelFunc) {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	ctx2, cancel2 := context.WithTimeout(ctx, 45*time.Second)
	return ctx2, func() { cancel2(); cancel() }
}
func ferryNoArgs(args []string) error {
	if len(args) > 0 {
		return usageErr(fmt.Errorf("this command accepts named flags; use --help"))
	}
	return nil
}
func ferryRouteFlags(cmd *cobra.Command, route, direction *string) {
	cmd.Flags().StringVar(route, "route", "osaka-beppu", "Official route slug or source direction code (see routes list)")
	cmd.Flags().StringVar(direction, "direction", "outbound", "Route direction: outbound or inbound; source direction codes already fix direction")
}
func ferryResolve(route, direction string) (ferry.Route, string, error) {
	r, line, e := ferry.ResolveRoute(route, direction)
	if e != nil {
		return r, line, usageErr(e)
	}
	return r, line, nil
}
func ferryLocalMeta() ferry.Metadata {
	return ferry.Metadata{ObservedAt: "2026-10-02", SourceURLs: []string{ferry.PublicBase + "/en/", ferry.BookingURL}, SourceLanguage: "en/ja; local reference registry verified on 2026-10-02", RequestCount: 0}
}
func ferryRoutesCmd(flags *rootFlags) *cobra.Command {
	parent := &cobra.Command{Use: "routes", Short: "Discover Kansai–Kyushu routes, source direction IDs and distinct Osaka terminals", Annotations: ferryAnnotations("local", "")}
	list := &cobra.Command{Use: "list", Short: "List official route and terminal identifiers from the local reference registry", Example: "  sunflower-ferry-pp-cli routes list --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "routes list")
		}
		if e := validateDataSourceStrategy(flags, "local"); e != nil {
			return usageErr(e)
		}
		if e := ferryNoArgs(args); e != nil {
			return e
		}
		return ferryWrite(cmd, flags, ferry.Registry, ferryLocalMeta(), "Reference registry; dynamic sailings, prices and availability require live lookups.")
	}}
	var query string
	search := &cobra.Command{Use: "search [query]", Short: "Search route, Japanese name, terminal or destination offline", Example: "  sunflower-ferry-pp-cli routes search Beppu --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "query=Beppu"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "routes search")
		}
		if e := validateDataSourceStrategy(flags, "local"); e != nil {
			return usageErr(e)
		}
		if len(args) > 1 || len(args) == 1 && query != "" {
			return usageErr(fmt.Errorf("provide one query either positionally or with --query"))
		}
		q := query
		if len(args) == 1 {
			q = args[0]
		}
		if len([]rune(q)) > 100 {
			return usageErr(fmt.Errorf("route query is limited to 100 characters"))
		}
		return ferryWrite(cmd, flags, ferry.SearchRoutes(q), ferryLocalMeta())
	}}
	search.Flags().StringVar(&query, "query", "", "Match a route, destination, terminal or Japanese name")
	var route, direction string
	show := &cobra.Command{Use: "show", Short: "Read the official normal weekday timetable and overnight-arrival rules", Example: "  sunflower-ferry-pp-cli routes show --route osaka-shibushi --direction inbound --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--route=osaka-shibushi;--direction=inbound"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "routes show")
		}
		if e := validateDataSourceStrategy(flags, "live"); e != nil {
			return usageErr(e)
		}
		if e := ferryNoArgs(args); e != nil {
			return e
		}
		r, line, e := ferryResolve(route, direction)
		if e != nil {
			return e
		}
		ctx, cancel := ferryCtx(cmd, flags)
		defer cancel()
		c := ferry.New(flags.rateLimit)
		data, e := c.Timetable(ctx, r, line)
		if e != nil {
			return classifyAPIError(cmd.OutOrStdout(), e, flags)
		}
		return ferryWrite(cmd, flags, data, c.Meta("en"), "Normal weekday patterns do not establish operation on a specific date; use sailings for source date lookup.")
	}}
	ferryRouteFlags(show, &route, &direction)
	parent.AddCommand(list, search, show)
	return parent
}

func ferryHandoffCmd(flags *rootFlags) *cobra.Command {
	var route, direction, date string
	cmd := &cobra.Command{Use: "handoff", Short: "Return the canonical official booking page and selected planning context", Example: "  sunflower-ferry-pp-cli handoff --route kobe-oita --direction inbound --agent", Annotations: ferryAnnotations("local", "--route=kobe-oita"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "handoff")
		}
		if e := validateDataSourceStrategy(flags, "local"); e != nil {
			return usageErr(e)
		}
		if e := ferryNoArgs(args); e != nil {
			return e
		}
		r, line, e := ferryResolve(route, direction)
		if e != nil {
			return e
		}
		if date != "" {
			if _, e := ferry.ParseDate(date); e != nil {
				return usageErr(e)
			}
		}
		return ferryWrite(cmd, flags, map[string]any{"booking_url": ferry.BookingURL, "route_id": r.ID, "source_line_code": line, "boarding_date": date, "prefilled": false, "reservation_created": false, "next_step": "Enter this route/date/party in the official portal and confirm current availability, final price and conditions before booking."}, ferryLocalMeta())
	}}
	ferryRouteFlags(cmd, &route, &direction)
	cmd.Flags().StringVar(&date, "date", "", "Optional exact Japan boarding date YYYY-MM-DD to include in the handoff context")
	return cmd
}
