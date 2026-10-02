// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/internal/haneda"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(rootCmd *cobra.Command, flags *rootFlags) {
		if group, _, err := rootCmd.Find([]string{"flights"}); err == nil && group.Name() == "flights" {
			group.Short = "Search first-party domestic/international flight status and codeshare groups"
		}
		rootCmd.AddCommand(newHanedaCatalogCmd(flags))
		rootCmd.AddCommand(newHanedaScheduleCmd(flags))
		if source, _, err := rootCmd.Find([]string{"source"}); err == nil && source.Name() == "source" {
			source.Hidden = true
			var hide func(*cobra.Command)
			hide = func(c *cobra.Command) {
				if c.Annotations == nil {
					c.Annotations = map[string]string{}
				}
				c.Annotations["mcp:hidden"] = "true"
				for _, child := range c.Commands() {
					hide(child)
				}
			}
			hide(source)
		}
	})
}

func hanedaAnnotations(source string) map[string]string {
	return map[string]string{"mcp:read-only": "true", "pp:data-source": source, "pp:happy-args": "--limit=5"}
}
func hanedaQueryFlags(cmd *cobra.Command, q *haneda.Query, filters bool) {
	cmd.Flags().StringVar(&q.Kind, "kind", "international", "Flight source: domestic, international, or all")
	cmd.Flags().StringVar(&q.Direction, "direction", "departure", "Haneda direction: departure, arrival, or both")
	cmd.Flags().StringVar(&q.Date, "date", "", "Exact service date YYYY-MM-DD; flight boards default to today JST")
	cmd.Flags().IntVar(&q.Limit, "limit", 20, "Maximum matching service groups returned (1–200)")
	cmd.Flags().IntVar(&q.Offset, "offset", 0, "Local result offset; each live call refreshes the source")
	cmd.Flags().IntVar(&q.MaxScan, "max-scan-records", 5000, "Maximum source records examined after bounded download (1–20000)")
	if filters {
		cmd.Flags().StringVar(&q.Flight, "flight", "", "Full listed flight number or stable hnd: group ID")
		cmd.Flags().StringVar(&q.Airline, "airline", "", "Airline provider code, flight prefix, or English/Japanese name")
		cmd.Flags().StringVar(&q.Destination, "destination", "", "Other airport code, provider city value, or English/Japanese name")
	}
}
func prepareHanedaQuery(q *haneda.Query, defaultDate bool) {
	q.Kind = strings.ToLower(strings.TrimSpace(q.Kind))
	q.Direction = strings.ToLower(strings.TrimSpace(q.Direction))
	if q.Kind == "int" {
		q.Kind = "international"
	}
	if q.Kind == "dms" {
		q.Kind = "domestic"
	}
	q.Direction = strings.TrimSuffix(q.Direction, "s")
	q.Terminal = strings.ToUpper(strings.TrimSpace(q.Terminal))
	q.Status = strings.ToLower(strings.TrimSpace(q.Status))
	if defaultDate && q.Date == "" {
		q.Date = time.Now().In(haneda.JST).Format("2006-01-02")
	}
}
func newHanedaClient(flags *rootFlags) (*haneda.Client, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	c, err := haneda.NewClient(cfg.BaseURL, flags.timeout, flags.rateLimit)
	if err != nil {
		return nil, configErr(err)
	}
	return c, nil
}
func hanedaContext(cmd *cobra.Command, flags *rootFlags) (context.Context, context.CancelFunc) {
	parent, cancel := boundCtx(cmd.Context(), flags)
	ctx, stop := context.WithTimeout(parent, 30*time.Second)
	return ctx, func() { stop(); cancel() }
}
func hanedaAPIError(cmd *cobra.Command, flags *rootFlags, err error) error {
	classified := classifyAPIErrorOnly(err)
	var throttled *cliutil.RateLimitError
	if errors.As(err, &throttled) {
		classified = rateLimitErr(err)
	}
	writeAPIErrorEnvelope(cmd.OutOrStdout(), flags, classified, ExitCode(classified))
	return classified
}
func checkHanedaLive(flags *rootFlags) error {
	if flags.dataSource == "local" {
		return usageErr(fmt.Errorf("this command reads live source data; use snapshot search for saved observations"))
	}
	return nil
}

func newHanedaFlightsSearchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "search"}
	q := haneda.Query{}
	hanedaQueryFlags(cmd, &q, true)
	return configureHanedaBoardCmd(cmd, flags, "search", &q)
}

func newHanedaFlightsDisruptionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "disruptions"}
	q := haneda.Query{}
	hanedaQueryFlags(cmd, &q, true)
	return configureHanedaBoardCmd(cmd, flags, "disruptions", &q)
}

func configureHanedaBoardCmd(cmd *cobra.Command, flags *rootFlags, mode string, q *haneda.Query) *cobra.Command {
	short := map[string]string{"search": "Search dated flight boards while preserving codeshare service groups", "disruptions": "Inspect provider-reported delayed, canceled, or diverted service groups", "rollover": "Inspect adjacent service days and explicit midnight time changes"}[mode]
	cmd.Short = short
	cmd.Long = short + ". Source-reported times and missing fields are preserved; actual/operating identity is not inferred. Requests stop within 30 seconds, with no automatic retries."
	cmd.Annotations = hanedaAnnotations("live")
	cmd.Example = "  haneda-airport-pp-cli flights " + mode + " --kind international --direction both --limit 5 --agent"
	cmd.Flags().BoolVar(&q.IncludeFacilities, "include-facilities", false, "Include full provider facility map/detail links alongside distinct gate/counter/exit fields")
	cmd.Flags().StringVar(&q.Status, "status", "", "Exact source category/text or comma-separated values; unknown selects blank status")
	cmd.Flags().StringVar(&q.Terminal, "terminal", "", "Published terminal filter: T1, T2, or T3")
	cmd.Flags().BoolVar(&q.ServiceDayOnly, "service-day-only", false, "Exclude adjacent service days explicitly included by the source")
	cmd.Annotations["pp:no-error-path-probe"] = "true"
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "flights "+mode)
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("flights %s takes filters as flags", mode))
		}
		if err := checkHanedaLive(flags); err != nil {
			return err
		}
		prepareHanedaQuery(q, true)
		if mode == "disruptions" {
			q.Status = "delayed,canceled,diverted"
		}
		if mode == "rollover" {
			q.RolloverOnly = true
		}
		if err := haneda.ValidateQuery(*q, time.Now(), true); err != nil {
			return usageErr(err)
		}
		ctx, cancel := hanedaContext(cmd, flags)
		defer cancel()
		c, err := newHanedaClient(flags)
		if err != nil {
			return err
		}
		r, err := c.FetchBoard(ctx, *q, time.Now(), "")
		if err != nil {
			return hanedaAPIError(cmd, flags, err)
		}
		if mode == "disruptions" {
			s, err := c.FetchSummary(ctx)
			if err != nil {
				return hanedaAPIError(cmd, flags, err)
			}
			r.Summary = s
			r.Budget = c.Budget()
			r.Notes = append(r.Notes, "Airport summary is an independent provider snapshot and can differ from this requested date/direction/filter scope; zero alerts do not prove punctuality.")
		}
		r = haneda.FilterBoard(r, *q, time.Now())
		if r.ScanCapHit {
			fmt.Fprintln(cmd.ErrOrStderr(), "warning: source scan cap reached; returned totals cover examined records only")
		}
		return printJSONFiltered(cmd.OutOrStdout(), r, flags)
	}
	return cmd
}

func newHanedaDetailCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "detail [flight-or-id]"}
	q := haneda.Query{}
	hanedaQueryFlags(cmd, &q, true)
	return configureHanedaDetailCmd(cmd, flags, false, &q)
}

func configureHanedaDetailCmd(cmd *cobra.Command, flags *rootFlags, plan bool, q *haneda.Query) *cobra.Command {
	path := "flights detail"
	short := "Resolve a flight number or stable ID to source detail and codeshare group"
	if plan {
		path = "plan"
		short = "Locate a flight's published terminal, gates, counters and source map handoffs"
	}
	cmd.Short = short
	cmd.Annotations = hanedaAnnotations("live")
	cmd.Example = "  haneda-airport-pp-cli " + path + " NH849 --kind international --agent"
	q.Direction = "both"
	cmd.Flags().Lookup("direction").DefValue = "both"
	cmd.Annotations["pp:happy-args"] = "flight=NH849;--kind=international;--limit=5"
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, path)
		}
		if len(args) == 0 && q.Flight == "" && !flags.asJSON && !hasChangedLocalFlags(cmd) {
			return cmd.Help()
		}
		if len(args) > 1 || (len(args) == 1 && q.Flight != "") {
			return usageErr(fmt.Errorf("provide one flight positional OR --flight"))
		}
		if len(args) == 1 {
			q.Flight = args[0]
		}
		if q.Flight == "" {
			return usageErr(fmt.Errorf("provide a flight number such as NH849, or a stable hnd: group ID"))
		}
		if err := checkHanedaLive(flags); err != nil {
			return err
		}
		if strings.HasPrefix(q.Flight, "hnd:") {
			kind, direction, date, number, err := haneda.ParseIdentity(q.Flight)
			if err != nil {
				return usageErr(err)
			}
			if cmd.Flags().Changed("kind") && q.Kind != kind {
				return usageErr(fmt.Errorf("ID kind conflicts with --kind"))
			}
			if cmd.Flags().Changed("date") && q.Date != date {
				return usageErr(fmt.Errorf("ID date conflicts with --date"))
			}
			if cmd.Flags().Changed("direction") && q.Direction != direction {
				return usageErr(fmt.Errorf("ID direction conflicts with --direction"))
			}
			q.Kind, q.Direction, q.Date = kind, direction, date
			_ = number
		}
		prepareHanedaQuery(q, true)
		q.IncludeFacilities = true
		if err := haneda.ValidateQuery(*q, time.Now(), true); err != nil {
			return usageErr(err)
		}
		number := q.Flight
		if strings.HasPrefix(number, "hnd:") {
			_, _, _, n, _ := haneda.ParseIdentity(number)
			number = n
		} else {
			n, err := haneda.NormalizeNumber(number)
			if err != nil {
				return usageErr(err)
			}
			number = n
		}
		ctx, cancel := hanedaContext(cmd, flags)
		defer cancel()
		c, err := newHanedaClient(flags)
		if err != nil {
			return err
		}
		r, err := c.FetchBoard(ctx, *q, time.Now(), number)
		if err != nil {
			return hanedaAPIError(cmd, flags, err)
		}
		r = haneda.FilterBoard(r, *q, time.Now())
		if r.TotalMatches == 0 {
			// The provider's exactMatch surface accepts source-primary codes only.
			// A bounded full-board lookup resolves marketing aliases and zero padding.
			fallback, fetchErr := c.FetchBoard(ctx, *q, time.Now(), "")
			if fetchErr != nil {
				return hanedaAPIError(cmd, flags, fetchErr)
			}
			fallback.Coverage.QueryMode = "board_with_local_flight_lookup"
			fallback.Coverage.FlightLookup = q.Flight
			fallback.Notes = append(fallback.Notes, "The provider exact endpoint did not resolve this identifier; bounded source boards were searched for its listed code or zero-padding equivalent.")
			r = haneda.FilterBoard(fallback, *q, time.Now())
		}
		if r.TotalMatches == 0 {
			_ = printJSONFiltered(cmd.OutOrStdout(), r, flags)
			return notFoundErr(fmt.Errorf("flight not reported in this source date/kind/direction scope; inspect the canonical airport page"))
		}
		if !plan {
			return printJSONFiltered(cmd.OutOrStdout(), r, flags)
		}
		floors := map[string]string{"T1": haneda.Origin + "/en/floor/floor-guide.html?area=7&tab=terminal1", "T2": haneda.Origin + "/en/floor/floor-guide.html?area=8&tab=terminal2", "T3": haneda.Origin + "/en/floor/floor-guide.html?area=9&tab=terminal3"}
		links := map[string]string{}
		for _, f := range r.Flights {
			if f.Terminal != nil && floors[*f.Terminal] != "" {
				links[*f.Terminal] = floors[*f.Terminal]
			}
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"flight_board": r, "terminal_floor_urls": links, "terminal_transfer_url": haneda.Origin + "/en/access/travel_between_terminals/index.html", "connection_guide_url": haneda.Origin + "/en/flight/transit/index.html", "notes": []string{"Facilities may change. Arrival exit gates are not aircraft boarding gates. Follow source/airline handoffs for current operational guidance; seat inventory and guaranteed connections are not established."}}, flags)
	}
	return cmd
}

func newHanedaCatalogCmd(flags *rootFlags) *cobra.Command {
	parent := &cobra.Command{Use: "catalog", Example: "  haneda-airport-pp-cli catalog airports --kind domestic --limit 5 --agent", Short: "Resolve bilingual airport/city and airline provider identifiers", RunE: parentNoSubcommandRunE(flags)}
	parent.AddCommand(newHanedaAirportsCmd(flags))
	parent.AddCommand(newHanedaAirlinesCmd(flags))
	return parent
}
func newHanedaAirportsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "airports"}
	return configureHanedaCatalogCmd(cmd, flags, "airports")
}
func newHanedaAirlinesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "airlines"}
	return configureHanedaCatalogCmd(cmd, flags, "airlines")
}
func configureHanedaCatalogCmd(cmd *cobra.Command, flags *rootFlags, entity string) *cobra.Command {
	q := haneda.Query{Direction: "departure", MaxScan: 5000}
	var query string
	cmd.Short = "Search the source English/Japanese " + entity + " catalog"
	cmd.Annotations = hanedaAnnotations("live")
	cmd.Example = "  haneda-airport-pp-cli catalog " + entity + " --kind domestic --limit 5 --agent"
	cmd.Flags().StringVar(&q.Kind, "kind", "international", "Catalog source: domestic, international, or all")
	cmd.Flags().StringVar(&query, "query", "", "Code, prefix, city value, or English/Japanese name text")
	cmd.Flags().IntVar(&q.Limit, "limit", 20, "Maximum catalog matches returned (1–200)")
	cmd.Flags().IntVar(&q.Offset, "offset", 0, "Local result offset within filtered catalog")
	cmd.Annotations["pp:no-error-path-probe"] = "true"
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "catalog "+entity)
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("use --query for catalog text"))
		}
		prepareHanedaQuery(&q, false)
		if err := haneda.ValidateQuery(q, time.Now(), false); err != nil {
			return usageErr(err)
		}
		if len(query) > 120 {
			return usageErr(fmt.Errorf("--query is limited to 120 characters"))
		}
		if err := checkHanedaLive(flags); err != nil {
			return err
		}
		ctx, cancel := hanedaContext(cmd, flags)
		defer cancel()
		c, err := newHanedaClient(flags)
		if err != nil {
			return err
		}
		rows := []any{}
		for _, kind := range []string{"domestic", "international"} {
			if q.Kind != "all" && q.Kind != kind {
				continue
			}
			if entity == "airports" {
				items, err := c.FetchAirports(ctx, kind)
				if err != nil {
					return hanedaAPIError(cmd, flags, err)
				}
				for _, v := range items {
					if query == "" || catalogContains(v.Code+" "+v.SearchValue+" "+v.Name+" "+v.NameJA, query) {
						rows = append(rows, v)
					}
				}
			} else {
				items, err := c.FetchAirlines(ctx, kind)
				if err != nil {
					return hanedaAPIError(cmd, flags, err)
				}
				for _, v := range items {
					if query == "" || catalogContains(v.Code+" "+v.Prefix+" "+v.Name+" "+v.NameJA, query) {
						rows = append(rows, v)
					}
				}
			}
		}
		total := len(rows)
		start := q.Offset
		if start > total {
			start = total
		}
		end := start + q.Limit
		if end > total {
			end = total
		}
		var next *int
		if end < total {
			v := end
			next = &v
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{entity: append([]any{}, rows[start:end]...), "observed_at": time.Now().In(haneda.JST).Format(time.RFC3339), "source_updated_at": nil, "source_url": haneda.Origin + "/en/flight/" + map[string]string{"airports": "city_list.html", "airlines": "company_list.html"}[entity], "total_matches": total, "limit": q.Limit, "offset": q.Offset, "next_offset": next, "budget": c.Budget()}, flags)
	}
	return cmd
}

func catalogContains(hay, needle string) bool {
	return strings.Contains(strings.ToLower(hay), strings.ToLower(needle))
}

func newHanedaScheduleCmd(flags *rootFlags) *cobra.Command {
	parent := &cobra.Command{Use: "schedule", Example: "  haneda-airport-pp-cli schedule search --kind international --flight NH849 --limit 5 --agent", Short: "Inspect published monthly periods and operating weekdays", RunE: parentNoSubcommandRunE(flags)}
	parent.AddCommand(newHanedaScheduleSearchCmd(flags))
	return parent
}

func newHanedaScheduleSearchCmd(flags *rootFlags) *cobra.Command {
	q := haneda.Query{}
	cmd := &cobra.Command{Use: "search", Short: "Filter published monthly schedules; these are not live flight statuses or seats", Annotations: hanedaAnnotations("live"), Example: "  haneda-airport-pp-cli schedule search --kind international --flight NH849 --limit 5 --agent"}
	hanedaQueryFlags(cmd, &q, true)
	cmd.Annotations["pp:no-error-path-probe"] = "true"
	cmd.Flags().Lookup("flight").Usage = "Full listed flight number such as NH849; board IDs belong to flights detail"
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "schedule search")
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("schedule search takes flags, including optional --date"))
		}
		if err := checkHanedaLive(flags); err != nil {
			return err
		}
		prepareHanedaQuery(&q, false)
		if strings.HasPrefix(q.Flight, "hnd:") {
			return usageErr(fmt.Errorf("schedule --flight accepts a listed number such as NH849; use flights detail for a board hnd: ID"))
		}
		if err := haneda.ValidateQuery(q, time.Now(), false); err != nil {
			return usageErr(err)
		}
		ctx, cancel := hanedaContext(cmd, flags)
		defer cancel()
		c, err := newHanedaClient(flags)
		if err != nil {
			return err
		}
		r, err := c.FetchSchedules(ctx, q, time.Now())
		if err != nil {
			return hanedaAPIError(cmd, flags, err)
		}
		if r.ScanCapHit {
			fmt.Fprintln(cmd.ErrOrStderr(), "warning: schedule scan cap reached; returned totals cover examined records only")
		}
		return printJSONFiltered(cmd.OutOrStdout(), r, flags)
	}
	return cmd
}
