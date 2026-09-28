package cli

// pp:data-source auto

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/tenki"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const tenkiTokyo = "https://tenki.jp/forecast/3/16/4410/13101/"
const tenkiFuji = "https://tenki.jp/mountain/famous100/5/25/150.html"
const tenkiOze = "https://tenki.jp/kouyou/3/13/30139.html"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		factory := func(config tenki.Config) tenkiSource { return tenki.NewClient(config) }
		for _, command := range newTenkiCommands(flags, factory) {
			addNovelCommandIfAbsent(root, command)
		}
		visible := map[string]bool{"places": true, "forecast": true, "seasonal": true, "mountain": true, "compare": true, "doctor": true, "version": true, "agent-context": true, "catalog": true, "help": true, "completion": true}
		for _, command := range root.Commands() {
			if visible[command.Name()] {
				continue
			}
			command.Hidden = true
			if command.Annotations == nil {
				command.Annotations = map[string]string{}
			}
			command.Annotations["mcp:hidden"] = "true"
		}
		visibleFlags := map[string]bool{"json": true, "compact": true, "csv": true, "plain": true, "quiet": true, "home": true, "timeout": true, "dry-run": true, "no-cache": true, "no-input": true, "no-color": true, "agent": true, "data-source": true, "select": true, "rate-limit": true}
		root.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
			if !visibleFlags[flag.Name] {
				flag.Hidden = true
			}
		})
		root.PersistentFlags().Lookup("data-source").Usage = "Page source: auto (fresh cache then live), live (refresh), local (page cache only)"
		root.PersistentFlags().Lookup("compact").Usage = "Compact product JSON; use --select to narrow fields"
		root.PersistentFlags().Lookup("rate-limit").Usage = "Lower request rate per command/client (at most 1 request/s; 0/auto uses 1; concurrent CLI/MCP invocations pace independently)"
		for _, command := range root.Commands() {
			if command.Name() == "catalog" && command.Annotations != nil {
				delete(command.Annotations, "pp:happy-args")
			}
		}
	})
}

func newTenkiCommands(flags *rootFlags, factory tenkiFactory) []*cobra.Command {
	places := tenkiGroup("places", "Find a source place and its forecast reference municipality")
	places.AddCommand(newTenkiSearch(flags, factory), newPlacesShowCmd(flags, factory))
	forecast := tenkiGroup("forecast", "Read actual daily or hourly source coverage in Japan Standard Time")
	forecast.AddCommand(newTenkiDaily(flags, factory), newTenkiHourly(flags, factory))
	mountain := tenkiGroup("mountain", "Inspect foothill identity and separate altitude model guidance")
	mountain.AddCommand(newMountainShowCmd(flags, factory))
	seasonal := tenkiGroup("seasonal", "Read sakura or foliage evidence with its source year and update status")
	seasonal.AddCommand(newTenkiSeasonalList(flags, factory), newSeasonalShowCmd(flags, factory))
	return []*cobra.Command{places, forecast, mountain, seasonal, newTenkiCompare(flags, factory)}
}

func tenkiGroup(name, description string) *cobra.Command {
	return &cobra.Command{Use: name, Short: description, RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() }, Annotations: map[string]string{"pp:parent-group": "true"}}
}

func newTenkiSearch(flags *rootFlags, factory tenkiFactory) *cobra.Command {
	var query, kind string
	var limit, pages int
	var read tenkiReadFlags
	cmd := &cobra.Command{Use: "search", Short: "Search bounded source candidates; select an explicit canonical URL", Long: "Municipality names/postal codes use the source search. Leisure names filter a bounded readable directory: default selected destinations, or --directory for a prefecture. A miss means no match in the scanned directory, not nationwide absence. Mountain names use the source mountain directory.", Example: strings.Trim(`
  tenki-pp-cli places search --query 千代田区 --kind municipality --limit 5
  tenki-pp-cli places search --query 金閣寺 --kind leisure --directory https://tenki.jp/leisure/6/29/ --limit 5
  tenki-pp-cli places search --query 富士山 --kind mountain --agent --select results.places.name,results.places.url
`, "\n"), Annotations: tenkiAnnotations("--query=千代田区;--kind=municipality;--limit=3")}
	cmd.Flags().StringVar(&query, "query", "", "Japanese name, postal code or canonical tenki.jp URL")
	cmd.Flags().StringVar(&kind, "kind", "municipality", "Candidate type: municipality, leisure or mountain")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum candidates (1–50)")
	cmd.Flags().IntVar(&pages, "max-scan-pages", 1, "Maximum source pages to inspect (1–2)")
	cmd.Flags().StringVar(&read.directory, "directory", "", "Readable leisure directory URL, e.g. https://tenki.jp/leisure/6/29/")
	read.attach(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if done, err := tenkiPrelude(cmd, args, flags); done {
			return err
		}
		if strings.TrimSpace(query) == "" {
			return usageErr(fmt.Errorf("--query is required; use a Japanese name, postal code or canonical URL"))
		}
		if kind != "municipality" && kind != "leisure" && kind != "mountain" {
			return usageErr(fmt.Errorf("--kind must be municipality, leisure or mountain"))
		}
		if read.directory != "" && (kind != "leisure" || !regexp.MustCompile(`^https://tenki\.jp/leisure/(?:[0-9]+/(?:[0-9]+/)?)?$`).MatchString(read.directory)) {
			return usageErr(fmt.Errorf("--directory requires --kind leisure and a canonical https://tenki.jp/leisure/ or region/prefecture directory URL"))
		}
		if err := tenkiBound("limit", limit, 50); err != nil {
			return err
		}
		if err := tenkiBound("max-scan-pages", pages, 2); err != nil {
			return err
		}
		client, err := read.source(flags, factory)
		if err != nil {
			return err
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		result, err := client.Search(ctx, query, kind, limit, pages)
		if err != nil {
			return tenkiProductError(err)
		}
		if result.Places == nil {
			result.Places = []tenki.Place{}
		}
		if len(result.Places) > limit {
			result.Places = result.Places[:limit]
			result.Truncated = true
		}
		tenkiWarnings(cmd, result.Warnings)
		return emitTenki(cmd, flags, client, result, result.Source)
	}
	return cmd
}

func newTenkiDaily(flags *rootFlags, factory tenkiFactory) *cobra.Command {
	var place, from string
	var days, limit int
	var detail bool
	var read tenkiReadFlags
	cmd := &cobra.Command{Use: "daily", Short: "Read 1–14 requested dates; report actual source horizon", Long: "Read daily outlook. Minimum temperature is the morning low; maximum is the daytime high. Today's period may be partial. --detail includes separate six-hour precipitation intervals and instantaneous temperature/wind values.", Example: strings.Trim(`
  tenki-pp-cli forecast daily --place https://tenki.jp/forecast/3/16/4410/13101/ --days 3
  tenki-pp-cli forecast daily --place https://tenki.jp/forecast/3/16/4410/13101/ --days 5 --agent --select results.periods.date,results.periods.weather,results.periods.precip_probability_pct,results.source
`, "\n"), Annotations: tenkiAnnotations("--place=" + tenkiTokyo + ";--days=2;--limit=2")}
	cmd.Flags().StringVar(&place, "place", "", "Canonical tenki.jp place URL (required)")
	cmd.Flags().StringVar(&from, "from", "", "First JST date YYYY-MM-DD (default: today)")
	cmd.Flags().IntVar(&days, "days", 14, "Number of requested dates (1–14)")
	cmd.Flags().IntVar(&limit, "limit", 14, "Maximum daily rows (1–14)")
	cmd.Flags().BoolVar(&detail, "detail", false, "Include separate six-hour intervals and instant source values")
	read.attach(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if done, err := tenkiPrelude(cmd, args, flags); done {
			return err
		}
		if err := validateTenkiPlace(place); err != nil {
			return err
		}
		if from == "" {
			from = tenkiToday()
		}
		start, err := tenkiDate("from", from)
		if err != nil {
			return err
		}
		if err := tenkiBound("days", days, 14); err != nil {
			return err
		}
		if err := tenkiBound("limit", limit, 14); err != nil {
			return err
		}
		client, err := read.source(flags, factory)
		if err != nil {
			return err
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		result, err := client.Daily(ctx, place)
		if err != nil {
			return tenkiProductError(err)
		}
		end := start.AddDate(0, 0, days).Format("2006-01-02")
		periods := []tenki.Period{}
		sourceDates := map[string]bool{}
		for _, period := range result.Periods {
			sourceDates[period.Date] = true
			if period.Date >= from && period.Date < end {
				periods = append(periods, period)
			}
		}
		unsupportedDates := []string{}
		for offset := 0; offset < days; offset++ {
			date := start.AddDate(0, 0, offset).Format("2006-01-02")
			if !sourceDates[date] {
				unsupportedDates = append(unsupportedDates, date)
			}
		}
		truncated := len(periods) > limit
		if truncated {
			periods = periods[:limit]
		}
		result.Periods = periods
		if !detail {
			result.Intervals = nil
			result.Instants = nil
		} else {
			result.Intervals = tenkiDatePeriods(result.Intervals, from, end)
			result.Instants = tenkiDatePeriods(result.Instants, from, end)
		}
		if result.Status == "ok" && len(unsupportedDates) > 0 {
			result.Status = "partial_horizon"
			if len(unsupportedDates) == days {
				result.Status = "out_of_horizon"
			}
		}
		tenkiWarnings(cmd, result.Warnings)
		view := map[string]any{"place": result.Place, "source": result.Source, "status": result.Status, "warnings": result.Warnings, "periods": result.Periods, "coverage_start": result.CoverageStart, "coverage_end": result.CoverageEnd, "requested_from": from, "requested_days": days, "unsupported_dates": unsupportedDates, "truncated": truncated, "units": tenkiUnits("daily")}
		if detail {
			view["intervals"] = result.Intervals
			view["instants"] = result.Instants
		}
		return emitTenki(cmd, flags, client, view, result.Source)
	}
	return cmd
}

func tenkiDatePeriods(periods []tenki.Period, from, end string) []tenki.Period {
	result := []tenki.Period{}
	for _, period := range periods {
		if period.Date >= from && period.Date < end {
			result = append(result, period)
		}
	}
	return result
}

func newTenkiHourly(flags *rootFlags, factory tenkiFactory) *cobra.Command {
	var place, date, hours string
	var limit int
	var read tenkiReadFlags
	cmd := &cobra.Command{Use: "hourly", Short: "Read source hours with preceding-hour rain and endpoint temperature/wind", Long: "Read actual hourly source rows. Precipitation belongs to start–end; temperature/wind belong to valid_at. For --hours 09:00-17:00, rows ending 09:00 through 17:00 provide instantaneous endpoints; the 08:00-09:00 rain interval lies outside the requested activity window. Past rows remain estimated_actual. Availability is today plus two calendar days, as actually returned.", Example: strings.Trim(`
  tenki-pp-cli forecast hourly --place https://tenki.jp/forecast/3/16/4410/13101/ --hours 09:00-17:00 --limit 12
  tenki-pp-cli forecast hourly --place https://tenki.jp/forecast/3/16/4410/13101/ --agent --select results.periods.valid_at,results.periods.kind,results.periods.precip_rate_mm_h,results.periods.temperature_c
`, "\n"), Annotations: tenkiAnnotations("--place=" + tenkiTokyo + ";--limit=4")}
	cmd.Flags().StringVar(&place, "place", "", "Canonical tenki.jp place URL (required)")
	cmd.Flags().StringVar(&date, "date", "", "Requested JST date YYYY-MM-DD (default: today)")
	cmd.Flags().StringVar(&hours, "hours", "00:00-24:00", "Endpoint hours HH:00-HH:00 on requested date")
	cmd.Flags().IntVar(&limit, "limit", 24, "Maximum source rows (1–72)")
	read.attach(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if done, err := tenkiPrelude(cmd, args, flags); done {
			return err
		}
		if err := validateTenkiPlace(place); err != nil {
			return err
		}
		if date == "" {
			date = tenkiToday()
		}
		day, err := tenkiDate("date", date)
		if err != nil {
			return err
		}
		window, err := parseTenkiHours(hours)
		if err != nil {
			return err
		}
		if err := tenkiBound("limit", limit, 72); err != nil {
			return err
		}
		client, err := read.source(flags, factory)
		if err != nil {
			return err
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		result, err := client.Hourly(ctx, place)
		if err != nil {
			return tenkiProductError(err)
		}
		start := day.Add(time.Duration(window.Start) * time.Hour)
		end := day.Add(time.Duration(window.End) * time.Hour)
		periods := []tenki.Period{}
		for _, period := range result.Periods {
			valid, err := time.Parse(time.RFC3339, period.ValidAt)
			if err == nil && !valid.Before(start) && !valid.After(end) && (period.Date == date || valid.Equal(start)) {
				periods = append(periods, period)
			}
		}
		truncated := len(periods) > limit
		if truncated {
			periods = periods[:limit]
		}
		result.Periods = periods
		result.Intervals = nil
		result.Instants = nil
		if len(periods) == 0 && result.Status == "ok" {
			result.Status = "out_of_horizon"
		}
		tenkiWarnings(cmd, result.Warnings)
		return emitTenki(cmd, flags, client, map[string]any{"place": result.Place, "source": result.Source, "status": result.Status, "warnings": result.Warnings, "periods": periods, "coverage_start": result.CoverageStart, "coverage_end": result.CoverageEnd, "requested_date": date, "requested_hours": hours, "truncated": truncated, "units": tenkiUnits("hourly")}, result.Source)
	}
	return cmd
}

func tenkiUnits(product string) map[string]string {
	rain := "mm per source daily period (when available)"
	if product == "hourly" {
		rain = "mm/h over preceding hour start–end"
	}
	return map[string]string{"temperature": "°C", "precip_probability": "% per source interval; not summed", "precipitation": rain, "wind_speed": "m/s", "timezone": "Asia/Tokyo"}
}

func validateTenkiSeason(kind string, year int) error {
	if kind != "sakura" && kind != "kouyou" {
		return usageErr(fmt.Errorf("--kind must be sakura or kouyou"))
	}
	if err := tenki.ValidateSeasonYear(year); err != nil {
		return usageErr(fmt.Errorf("--year: %w; unavailable source years remain explicit", err))
	}
	return nil
}

func newTenkiSeasonalList(flags *rootFlags, factory tenkiFactory) *cobra.Command {
	var kind, query string
	var year, limit, pages int
	var read tenkiReadFlags
	cmd := &cobra.Command{Use: "list", Short: "List bounded seasonal spots with actual source year and status", Long: "Foliage name search uses the source's spot search; --directory instead filters a bounded readable region/prefecture directory. Ended Sakura name search filters a selected retained directory, so use --directory to select it. A no_results status covers the reported scan, not nationwide absence.", Example: strings.Trim(`
  tenki-pp-cli seasonal list --kind kouyou --query 尾瀬 --limit 5
  tenki-pp-cli seasonal list --kind sakura --directory https://tenki.jp/sakura/3/16/ --query 上野 --year 2026 --limit 5
  tenki-pp-cli seasonal list --kind sakura --year 2026 --agent --select results.status,results.year,results.spots
`, "\n"), Annotations: tenkiAnnotations("--kind=kouyou;--query=尾瀬;--limit=3")}
	cmd.Flags().StringVar(&kind, "kind", "", "Seasonal product: sakura or kouyou (required)")
	cmd.Flags().StringVar(&query, "query", "", "Optional Japanese spot-name filter")
	cmd.Flags().IntVar(&year, "year", 0, "Requested season year (default: current JST year)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum spots (1–50)")
	cmd.Flags().IntVar(&pages, "max-scan-pages", 1, "Maximum source pages to inspect (1–2)")
	cmd.Flags().StringVar(&read.directory, "directory", "", "Readable directory matching --kind, e.g. https://tenki.jp/sakura/3/16/")
	read.attach(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if done, err := tenkiPrelude(cmd, args, flags); done {
			return err
		}
		if year == 0 && !cmd.Flags().Changed("year") {
			year = time.Now().In(time.FixedZone("JST", 9*60*60)).Year()
		}
		if err := validateTenkiSeason(kind, year); err != nil {
			return err
		}
		if read.directory != "" && !regexp.MustCompile(`^https://tenki\.jp/`+kind+`/(?:[0-9]+/(?:[0-9]+/)?)?$`).MatchString(read.directory) {
			return usageErr(fmt.Errorf("--directory must be a canonical %s directory matching --kind", kind))
		}
		if err := tenkiBound("limit", limit, 50); err != nil {
			return err
		}
		if err := tenkiBound("max-scan-pages", pages, 2); err != nil {
			return err
		}
		client, err := read.source(flags, factory)
		if err != nil {
			return err
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		result, err := client.SeasonalList(ctx, kind, query, year, limit, pages)
		if err != nil {
			return tenkiProductError(err)
		}
		if result.Spots == nil {
			result.Spots = []tenki.SeasonalSpot{}
		}
		if len(result.Spots) > limit {
			result.Spots = result.Spots[:limit]
			result.Truncated = true
		}
		tenkiWarnings(cmd, result.Warnings)
		return emitTenki(cmd, flags, client, result, result.Source)
	}
	return cmd
}
