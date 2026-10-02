package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/smartex/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/smartex/internal/smartex"
	"github.com/spf13/cobra"
)

type smartOptions struct {
	from, to, date, class, train, now, departure, product, query, topic, after, sourceID, special string
	adults, children, limit, pieces                                                               int
	length, width, height, weight                                                                 float64
	detail, check, offline, oversized                                                             bool
}

func configureSmartCommand(cmd *cobra.Command, flags *rootFlags, o *smartOptions) {
	kind := cmd.Use
	today := time.Now().In(smartex.JST).Format("2006-01-02")
	descriptions := map[string]string{
		"stations":  "Resolve Shinkansen English/Japanese station names and corridor order",
		"route":     "Plan corridor coverage and station order without claiming a through train",
		"fare":      "Compare fresh dated adult one-way public fares; exact seats require booking login",
		"products":  "Compare smartEX/Hayatoku calendar rules and unresolved eligibility conditions",
		"window":    "Calculate precise JST request, sale and cutoff rules with source uncertainty",
		"baggage":   "Check normal baggage limits and the need for reserved oversized area seats",
		"policy":    "Read sourced boarding, changes, refund and product planning guidance",
		"timetable": "Read current JR basic timetable links and curated regular service examples",
		"handoff":   "Prepare canonical smartEX booking links and a travel checklist",
		"sources":   "Inspect authoritative source IDs, snapshot dates and live page reachability",
	}
	examples := map[string]string{
		"stations":  "--query Osaka --limit 5",
		"route":     "--from Tokyo --to Hakata --detail",
		"fare":      "--from Tokyo --to Shin-Osaka --date " + today + " --class all",
		"products":  "--date " + time.Now().In(smartex.JST).AddDate(0, 0, 25).Format("2006-01-02"),
		"window":    "--date " + time.Now().In(smartex.JST).AddDate(0, 0, 25).Format("2006-01-02") + " --departure 09:00",
		"baggage":   "--length-cm 80 --width-cm 60 --height-cm 40 --weight-kg 20",
		"policy":    "--topic refund",
		"timetable": "--from Tokyo --to Shin-Osaka --limit 5",
		"handoff":   "--from Tokyo --to Shin-Osaka --date " + today,
		"sources":   "--check --source timetable --limit 1",
	}
	fixtures := map[string]string{
		"stations":  "--query=Osaka;--limit=5",
		"route":     "--from=Tokyo;--to=Hakata",
		"fare":      "--from=Tokyo;--to=Shin-Osaka;--date=" + today + ";--class=reserved",
		"products":  "--date=" + today,
		"window":    "--date=" + today + ";--departure=23:00",
		"baggage":   "--length-cm=80;--width-cm=60;--height-cm=40;--weight-kg=20",
		"policy":    "--topic=refund",
		"timetable": "--from=Tokyo;--to=Shin-Osaka;--limit=2",
		"handoff":   "--from=Tokyo;--to=Shin-Osaka;--date=" + today,
		"sources":   "--source=timetable",
	}
	source := "computed"
	if kind == "stations" || kind == "policy" || kind == "sources" {
		source = "local"
	}
	if kind == "fare" || kind == "timetable" {
		source = "live"
	}
	cmd.Short = descriptions[kind]
	cmd.Long = descriptions[kind] + ". Public planning only; dates/times are JST, prices are JPY. Unknown inventory stays null. Use --select to reduce output; command-specific flags are listed in help."
	cmd.Example = "  smartex-pp-cli " + kind + " " + examples[kind] + " --agent"
	cmd.Annotations = map[string]string{"mcp:read-only": "true", "pp:data-source": source, "pp:happy-args": fixtures[kind]}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, kind)
		}
		if len(args) != 0 {
			return usageErr(fmt.Errorf("%s accepts flags only; see %s --help", kind, kind))
		}
		if flags.dataSource == "local" {
			if kind == "fare" || (kind == "sources" && o.check) || (kind == "products" && o.detail) {
				return usageErr(fmt.Errorf("%s requested live HTTP work; --data-source local is incompatible", kind))
			}
			if kind == "timetable" {
				o.offline = true
			}
		}
		if flags.dataSource == "live" {
			if kind == "stations" || kind == "route" || kind == "window" || kind == "baggage" || kind == "policy" || kind == "handoff" {
				return usageErr(fmt.Errorf("%s uses embedded planning rules; live data source is unsupported; use the official reference commands", kind))
			}
			if kind == "sources" {
				o.check = true
			}
			if kind == "products" {
				o.detail = true
			}
			if kind == "timetable" && o.offline {
				return usageErr(fmt.Errorf("--offline conflicts with --data-source live"))
			}
		}
		actualSource := source
		if kind == "timetable" && o.offline {
			actualSource = "local"
		}
		if (kind == "sources" && o.check) || (kind == "products" && o.detail) {
			actualSource = "live"
		}
		cmd.Annotations["pp:data-source"] = actualSource
		flags.agentSource = actualSource
		if o.limit < 1 || o.limit > 50 {
			return usageErr(fmt.Errorf("--limit must be1–50"))
		}
		var result any
		now, e := smartex.ParseNow(o.now)
		if e != nil {
			return usageErr(e)
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		client := smartex.NewClient()
		client.SetRateLimit(flags.rateLimit)
		switch kind {
		case "stations":
			rows, total := smartex.Stations(o.query, o.limit)
			result = map[string]any{"stations": rows, "total_matches": total, "returned": len(rows), "source": smartex.Sources[4], "as_of": smartex.AsOf, "scope": "Tokaido/Sanyo/Kyushu Tokyo–Kagoshima-Chuo", "note": "Osaka/大阪 is not silently converted to Shin-Osaka; Sendai/川内 is in Kagoshima"}
		case "route":
			result, e = smartex.PlanRoute(o.from, o.to, o.detail)
			if e != nil {
				return usageErr(e)
			}
		case "fare":
			if o.date == "" {
				o.date = today
			}
			if _, e = smartex.PlanRoute(o.from, o.to, false); e != nil {
				return usageErr(e)
			}
			if _, e = smartex.ValidateFareDate(o.date, now); e != nil {
				return usageErr(e)
			}
			if e = smartex.ValidateParty(o.adults, o.children); e != nil {
				return usageErr(e)
			}
			if o.class != "all" && !smartex.ValidClass(o.class) {
				return usageErr(fmt.Errorf("--class must be reserved, unreserved, green or all"))
			}
			quote, err := client.Fares(ctx, o.from, o.to, o.date, o.class, o.adults, o.children, now)
			if len(quote.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d fare classes failed; %d successfully quoted classes returned\n", len(quote.FetchFailures), len(quote.Quotes))
			}
			if err != nil {
				if len(quote.FetchFailures) > 0 {
					if pe := flags.printJSON(cmd, quote); pe != nil {
						return pe
					}
				}
				return smartNetworkError(err)
			}
			result = quote
		case "products":
			views, err := smartex.CompareProducts(o.date, o.from, o.to, o.class, o.train, now, o.adults, o.children)
			if err != nil {
				return usageErr(err)
			}
			rows := []map[string]any{}
			checks := []smartex.SourceCheck{}
			failed := 0
			for _, v := range views {
				row := map[string]any{"id": v.ID, "name": v.Name, "assessment": v.Assessment, "minimum_party": v.MinParty, "facilities": v.Facilities, "train_rules": v.TrainRules, "price_jpy": nil, "source_url": v.SourceURL, "requires_confirmation": v.RequiresConfirmation, "exclusion_reasons": v.ExclusionReasons}
				if v.Window != nil {
					row["window"] = map[string]any{"opens_jst": v.Window.Opens, "closes_jst": v.Window.Closes, "status": v.Window.WindowStatus}
				}
				if o.detail {
					row["detail"] = v
					ch := client.Source(ctx, smartex.Source{ID: v.ID, URL: v.SourceURL, Kind: "official_product", AsOf: smartex.AsOf}, true)
					checks = append(checks, ch)
					if ch.Error != "" {
						failed++
					}
				}
				rows = append(rows, row)
			}
			if o.detail {
				cmd.Annotations["pp:data-source"] = "live"
			}
			if failed > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d product source checks failed; rule summaries remain the explicitly dated snapshot\n", failed, len(checks))
			}
			result = map[string]any{"as_of": smartex.AsOf, "products": rows, "adults": o.adults, "children": o.children, "now_jst": now.Format(time.RFC3339), "source_checks": checks, "upstream_requests": client.Requests(), "notes": []string{"Calendar/class checks are partial; requires_confirmation lists the remaining conditions", "Price is null until route/product fare is verified; public fare navigator covers basic adult fares only", "No round-trip smartEX discount for current travel; product ended March31,2026"}}
		case "window":
			result, e = smartex.BookingWindow(o.date, o.product, o.departure, now, o.adults, o.children, o.oversized, o.class)
			if e != nil {
				return usageErr(e)
			}
		case "baggage":
			result, e = smartex.CheckBaggage(smartex.BaggageInput{Length: o.length, Width: o.width, Height: o.height, Weight: o.weight, Pieces: o.pieces, Class: o.class, Special: o.special})
			if e != nil {
				return usageErr(e)
			}
		case "policy":
			result, e = smartex.Policies(o.topic)
			if e != nil {
				return usageErr(e)
			}
		case "timetable":
			if o.date != "" {
				if _, e = smartex.ParseDate(o.date); e != nil {
					return usageErr(e)
				}
			}
			if _, e = smartex.BasicMatches(o.from, o.to, o.train, o.after, o.limit, o.detail); e != nil {
				return usageErr(e)
			}
			if o.offline {
				cmd.Annotations["pp:data-source"] = "local"
			}
			result, e = client.Timetable(ctx, o.from, o.to, o.date, o.train, o.after, o.limit, o.detail, o.offline)
			if e != nil {
				return smartNetworkError(e)
			}
		case "handoff":
			result, e = smartex.Handoff(o.from, o.to, o.date, o.class, o.adults, o.children)
			if e != nil {
				return usageErr(e)
			}
		case "sources":
			rows := []smartex.Source{}
			for _, s := range smartex.Sources {
				if o.sourceID == "" || s.ID == o.sourceID {
					rows = append(rows, s)
				}
			}
			if len(rows) == 0 {
				return usageErr(fmt.Errorf("unknown --source ID; run sources to list valid IDs"))
			}
			if len(rows) > o.limit {
				rows = rows[:o.limit]
			}
			checks := []smartex.SourceCheck{}
			failed := 0
			if o.check {
				cmd.Annotations["pp:data-source"] = "live"
				for _, s := range rows {
					ch := client.Source(ctx, s, o.detail)
					checks = append(checks, ch)
					if ch.Error != "" {
						failed++
					}
				}
			}
			if failed > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d public source checks failed\n", failed, len(checks))
			}
			result = map[string]any{"sources": rows, "checks": checks, "upstream_requests": client.Requests(), "inventory": nil, "policy_snapshot_as_of": smartex.AsOf, "notes": []string{"HTTP reachability is not a re-verification of every policy fact", "Fare calculations are dated public adult quotes; published timetables are basic schedules; exact seats require booking login"}}
			if o.check && failed == len(checks) {
				if e = flags.printJSON(cmd, result); e != nil {
					return e
				}
				return smartNetworkError(smartex.AllSourceFailures(checks))
			}
		}
		return flags.printJSON(cmd, result)
	}
}

func smartNetworkError(err error) error {
	var rate *cliutil.RateLimitError
	if errors.As(err, &rate) || strings.Contains(err.Error(), "HTTP 429") {
		return rateLimitErr(err)
	}
	return apiErr(err)
}
