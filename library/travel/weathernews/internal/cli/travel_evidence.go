package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/weathernews/internal/cliutil"
	wn "github.com/mvanhorn/printing-press-library/library/travel/weathernews/internal/evidence"
	"github.com/spf13/cobra"
	"path/filepath"
	"strings"
	"time"
)

// pp:data-source live
// Focused commands use public first-party read-through HTTP; local means fresh cache only.
func init() {
	registerNovelCommand(func(rootCmd *cobra.Command, flags *rootFlags) {
		rootCmd.AddCommand(newPlacesCmd(flags))
		rootCmd.AddCommand(newWeatherCmd(flags))
		rootCmd.AddCommand(newSeasonCmd(flags))
		for _, group := range rootCmd.Commands() {
			if group.Name() != "source" {
				continue
			}
			for _, leaf := range group.Commands() {
				if leaf.Annotations == nil {
					leaf.Annotations = map[string]string{}
				}
				leaf.Annotations["mcp:read-only"] = "true"
				originalRun := leaf.RunE
				leaf.RunE = func(cmd *cobra.Command, args []string) error {
					if dryRunOK(flags) {
						return writeDryRun(cmd.OutOrStdout(), flags, cmd.CommandPath())
					}
					return originalRun(cmd, args)
				}

				switch leaf.Name() {
				case "forecast":
					leaf.Example = "  weathernews-pp-cli source forecast --lat 35.01167 --lon 135.76806 --agent"
					leaf.Annotations["pp:happy-args"] = "--lat=35.01167;--lon=135.76806"
				case "locations":
					leaf.Example = "  weathernews-pp-cli source locations --query 京都 --lang ja --agent"
					leaf.Annotations["pp:happy-args"] = "--query=京都;--lang=ja"
				}
			}
		}

	})
}

type evidenceOptions struct {
	refresh, metrics bool
	cacheDir         string
}

func configureEvidenceGroup(c *cobra.Command, o *evidenceOptions) {
	c.PersistentFlags().BoolVar(&o.refresh, "refresh", false, "Explicitly refresh source inventory or evidence, bypassing cached responses")
	c.PersistentFlags().BoolVar(&o.metrics, "metrics", false, "Write request count, cache hits, download size and latency as JSON to stderr")
	c.PersistentFlags().StringVar(&o.cacheDir, "cache-dir", "", "Override the bounded HTTP cache directory for this invocation")
}
func sourceErr(e error) error {
	var limited *cliutil.RateLimitError
	if errors.As(e, &limited) {
		return rateLimitErr(e)
	}
	var x *wn.Error
	if errors.As(e, &x) {
		switch x.Code {
		case 2:
			return usageErr(e)
		case 3:
			return notFoundErr(e)
		case 4:
			return authErr(e)
		case 7:
			return rateLimitErr(e)
		}
	}
	return apiErr(e)
}
func withEvidence(cmd *cobra.Command, flags *rootFlags, o *evidenceOptions, run func(context.Context, *wn.Client) (map[string]any, error)) error {
	if dryRunOK(flags) {
		return writeDryRun(cmd.OutOrStdout(), flags, cmd.CommandPath())
	}
	if flags.timeout <= 0 {
		return usageErr(fmt.Errorf("--timeout must be positive"))
	}
	dir := o.cacheDir
	if dir == "" {
		p, e := cliutil.CacheDir()
		if e != nil {
			return configErr(e)
		}
		dir = filepath.Join(p, "public-http")
	}
	c := wn.NewClient(dir, flags.timeout)
	rate := 2.0
	if flags.rateLimit >= 0 {
		rate = flags.rateLimit
	}
	if rate > 5 {
		rate = 5
	}
	c.Limiter = cliutil.NewAdaptiveLimiter(rate)
	c.NoCache = flags.noCache
	c.Refresh = o.refresh
	c.Offline = flags.dataSource == "local"
	if c.Offline && (c.NoCache || c.Refresh) {
		return usageErr(fmt.Errorf("--data-source local is incompatible with --refresh or --no-cache"))
	}
	parentCtx, boundCancel := boundCtx(cmd.Context(), flags)
	defer boundCancel()
	ctx, cancel := context.WithTimeout(parentCtx, 90*time.Second)
	defer cancel()
	result, e := run(ctx, c)
	if o.metrics {
		b, _ := json.Marshal(c.Stats())
		fmt.Fprintln(cmd.ErrOrStderr(), string(b))
	}
	if e != nil {
		return sourceErr(e)
	}
	path := strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name()+" ")
	projected, e := projectEvidence(result, flags.selectFields, path)
	if e != nil {
		return usageErr(e)
	}
	flags.asJSON = true
	b, e := json.Marshal(projected)
	if e != nil {
		return e
	}
	fields := map[string]bool{}
	if flags.selectFields != "" {
		for _, path := range strings.Split(flags.selectFields, ",") {
			fields[strings.ToLower(strings.TrimSpace(path))] = true
		}
	} else {
		for k := range projected {
			fields[k] = true
		}
	}
	outputFlags := *flags
	outputFlags.selectFields = "" // Domain projection already validated and applied, including empty lists.
	outputFlags.compact = false   // Domain rows already contain only evidence fields; generic gravity pruning drops units and dates.
	var rendered bytes.Buffer
	transport := "live"
	if c.Metrics.Requests == 0 && c.Metrics.CacheHits > 0 {
		transport = "cache"
	} else if c.Metrics.CacheHits > 0 {
		transport = "mixed"
	}
	if e = printOutputWithFlagsMeta(&rendered, b, &outputFlags, map[string]any{"source": "live", "transport": transport, "timezone": "Asia/Tokyo"}, fields); e != nil {
		return usageErr(e)
	}
	if !flags.csv && !flags.plain && !flags.quiet {
		var compact bytes.Buffer
		if e = json.Compact(&compact, rendered.Bytes()); e != nil {
			return e
		}
		if _, e = fmt.Fprintln(cmd.OutOrStdout(), compact.String()); e != nil {
			return e
		}
	} else {
		if _, e = cmd.OutOrStdout().Write(rendered.Bytes()); e != nil {
			return e
		}
	}
	if result["partial"] == true {
		return apiErr(fmt.Errorf("comparison is partial; inspect per-candidate errors in JSON"))
	}
	return nil
}
func newPlacesCmd(flags *rootFlags) *cobra.Command {
	o := &evidenceOptions{}
	g := &cobra.Command{Use: "places", Short: "Resolve Japanese place identity and source coordinates", Annotations: map[string]string{"mcp:read-only": "true", "pp:parent-group": "true"}, RunE: parentNoSubcommandRunE(flags)}
	configureEvidenceGroup(g, o)
	g.AddCommand(newPlacesResolveCmd(flags, o))
	return g
}
func newPlacesResolveCmd(flags *rootFlags, o *evidenceOptions) *cobra.Command {
	var query string
	limit, offset := 10, 0
	c := &cobra.Command{Use: "resolve", Short: "Search public Weathernews places; preserve ambiguous matches", Example: strings.Trim(`
  weathernews-pp-cli places resolve --query 京都 --limit 5 --agent
  weathernews-pp-cli places resolve --query 高尾山 --select items.id,items.name_ja,items.latitude,items.longitude`, "\n"), Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:typed-exit-codes": "0,2,3,4,5,7", "pp:happy-args": "--query=京都;--limit=3"}, RunE: func(cmd *cobra.Command, args []string) error {
		return withEvidence(cmd, flags, o, func(ctx context.Context, c *wn.Client) (map[string]any, error) {
			if len(args) > 0 {
				return nil, &wn.Error{Code: 2, Message: "use --query for place resolution"}
			}
			return c.Places(ctx, query, limit, offset)
		})
	}}
	c.Flags().StringVar(&query, "query", "", "Place word in Japanese or provider-supported language")
	c.Flags().IntVar(&limit, "limit", 10, "Maximum matches returned, bounded to 1–50")
	c.Flags().IntVar(&offset, "offset", 0, "Source-result offset for bounded pagination")

	return c
}
func newWeatherCmd(flags *rootFlags) *cobra.Command {
	o := &evidenceOptions{}
	g := &cobra.Command{Use: "weather", Short: "Forecast evidence and caller-defined weather comparisons", Annotations: map[string]string{"mcp:read-only": "true", "pp:parent-group": "true"}, RunE: parentNoSubcommandRunE(flags)}
	configureEvidenceGroup(g, o)
	g.AddCommand(newWeatherForecastCmd(flags, o))
	g.AddCommand(newWeatherCompareCmd(flags, o))
	return g
}
func newWeatherForecastCmd(flags *rootFlags, o *evidenceOptions) *cobra.Command {
	opts := wn.ForecastOptions{Hours: 12, Days: 3}
	c := &cobra.Command{Use: "forecast", Short: "Read bounded hourly/daily forecasts with separate observation", Example: strings.Trim(`
  weathernews-pp-cli weather forecast --lat 35.01167 --lon 135.76806 --hours 6 --days 3 --agent
  weathernews-pp-cli weather forecast --lat 35.01167 --lon 135.76806 --date 2026-11-01 --agent`, "\n"), Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:typed-exit-codes": "0,2,3,4,5,7", "pp:happy-args": "--lat=35.01167;--lon=135.76806;--hours=3;--days=2"}, RunE: func(cmd *cobra.Command, args []string) error {
		return withEvidence(cmd, flags, o, func(ctx context.Context, c *wn.Client) (map[string]any, error) {
			if len(args) > 0 {
				return nil, &wn.Error{Code: 2, Message: "forecast accepts flags only"}
			}
			return c.Forecast(ctx, opts)
		})
	}}
	c.Flags().Float64Var(&opts.Lat, "lat", 0, "Latitude from places resolve; source verifies Japan coverage")
	c.Flags().Float64Var(&opts.Lon, "lon", 0, "Longitude from places resolve; source verifies Japan coverage")
	c.Flags().StringVar(&opts.Name, "name", "", "Optional caller label; source location name remains distinct")
	c.Flags().StringVar(&opts.Date, "date", "", "Optional YYYY-MM-DD in JST; unavailable dates are out_of_horizon")
	c.Flags().IntVar(&opts.Hours, "hours", 12, "Maximum hourly rows, 0–72; 0 omits hourly rows")
	c.Flags().IntVar(&opts.Days, "days", 3, "Maximum daily rows, 0–14; 0 omits daily rows")

	return c
}
func newWeatherCompareCmd(flags *rootFlags, o *evidenceOptions) *cobra.Command {
	var points []string
	var batchPoints string
	var date string
	var pop, maxTemp, minTemp float64
	cmp := &cobra.Command{Use: "compare", Short: "Compare 2–5 coordinates against explicit daily thresholds", Example: strings.Trim(`
  weathernews-pp-cli weather compare --points 'Kyoto=35.01167,135.76806|Tokyo=35.681,139.767' --date 2026-10-01 --max-pop 40 --max-temp 30 --agent`, "\n"), Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:typed-exit-codes": "0,2,3,4,5,7", "pp:happy-args": "--points=Kyoto=35.01167,135.76806|Tokyo=35.681,139.767;--date=2026-10-01;--max-pop=40"}, RunE: func(cmd *cobra.Command, args []string) error {
		return withEvidence(cmd, flags, o, func(ctx context.Context, c *wn.Client) (map[string]any, error) {
			if len(args) > 0 {
				return nil, &wn.Error{Code: 2, Message: "compare accepts --point flags only"}
			}
			candidates := points
			if batchPoints != "" {
				if len(points) > 0 {
					return nil, &wn.Error{Code: 2, Message: "use either --point or --points"}
				}
				candidates = strings.Split(batchPoints, "|")
			}
			if len(candidates) > 5 {
				return nil, &wn.Error{Code: 2, Message: "at most five --point candidates"}
			}
			p := []wn.Point{}
			seen := map[string]bool{}
			for _, s := range candidates {
				v, e := wn.ParsePoint(s)
				if e != nil {
					return nil, e
				}
				if seen[v.Name] {
					return nil, &wn.Error{Code: 2, Message: "--point names must be distinct"}
				}
				seen[v.Name] = true
				p = append(p, v)
			}
			criteria := wn.Criteria{}
			if cmd.Flags().Changed("max-pop") {
				criteria.MaxPOP = &pop
			}
			if cmd.Flags().Changed("max-temp") {
				criteria.MaxTemp = &maxTemp
			}
			if cmd.Flags().Changed("min-temp") {
				criteria.MinTemp = &minTemp
			}
			return c.CompareWeather(ctx, p, date, criteria)
		})
	}}
	cmp.Flags().StringVar(&batchPoints, "points", "", "Alternative batch name=latitude,longitude entries separated by |; 2–5 candidates")
	cmp.Flags().StringArrayVar(&points, "point", nil, "Repeat name=latitude,longitude for 2–5 candidates; preserves input order")
	cmp.Flags().StringVar(&date, "date", "", "Required real YYYY-MM-DD date in JST")
	cmp.Flags().Float64Var(&pop, "max-pop", 0, "Caller maximum daily precipitation probability in percent")
	cmp.Flags().Float64Var(&maxTemp, "max-temp", 0, "Caller maximum daily high temperature in Celsius")
	cmp.Flags().Float64Var(&minTemp, "min-temp", 0, "Caller minimum daily low temperature in Celsius")

	return cmp
}
func newSeasonCmd(flags *rootFlags) *cobra.Command {
	o := &evidenceOptions{}
	g := &cobra.Command{Use: "season", Short: "Sakura/foliage reports, predictions and historical normals", Annotations: map[string]string{"mcp:read-only": "true", "pp:parent-group": "true"}, RunE: parentNoSubcommandRunE(flags)}
	configureEvidenceGroup(g, o)
	g.AddCommand(newSeasonSearchCmd(flags, o))
	g.AddCommand(newSeasonShowCmd(flags, o))
	g.AddCommand(newSeasonCompareCmd(flags, o))
	return g
}
func newSeasonSearchCmd(flags *rootFlags, o *evidenceOptions) *cobra.Command {
	var p, area, query string
	limit, offset := 10, 0
	search := &cobra.Command{Use: "search", Short: "Search bounded seasonal summaries before lazy detail retrieval", Example: strings.Trim(`
  weathernews-pp-cli season search --product koyo --area kyoto --query 嵐山 --agent
  weathernews-pp-cli season search --product sakura --query 清水 --limit 5 --agent --select items.id,items.name_ja,season`, "\n"), Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:typed-exit-codes": "0,2,3,4,5,7", "pp:happy-args": "--product=koyo;--area=kyoto;--query=嵐山;--limit=3"}, RunE: func(cmd *cobra.Command, args []string) error {
		return withEvidence(cmd, flags, o, func(ctx context.Context, c *wn.Client) (map[string]any, error) {
			if len(args) > 0 {
				return nil, &wn.Error{Code: 2, Message: "season search accepts flags only"}
			}
			return c.SeasonSearch(ctx, p, area, query, limit, offset)
		})
	}}
	search.Flags().StringVar(&p, "product", "", "Season product: sakura or koyo")
	search.Flags().StringVar(&area, "area", "", "Weathernews prefecture/region slug, e.g. kyoto, hokkaido, kanto")
	search.Flags().StringVar(&query, "query", "", "All literal terms must match source name, kana, city, address or prefecture")
	search.Flags().IntVar(&limit, "limit", 10, "Maximum summaries returned, bounded to 1–50")
	search.Flags().IntVar(&offset, "offset", 0, "Offset into stable source-ID order within current inventory snapshot")

	return search
}
func newSeasonShowCmd(flags *rootFlags, o *evidenceOptions) *cobra.Command {
	var product, id string
	show := &cobra.Command{Use: "show", Short: "Fetch one source spot with observation/forecast/normal separation", Example: strings.Trim(`
  weathernews-pp-cli season show --product koyo --id 26102 --agent
  weathernews-pp-cli season show --product sakura --id 384 --agent`, "\n"), Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:typed-exit-codes": "0,2,3,4,5,7", "pp:happy-args": "--product=koyo;--id=26102"}, RunE: func(cmd *cobra.Command, args []string) error {
		return withEvidence(cmd, flags, o, func(ctx context.Context, c *wn.Client) (map[string]any, error) {
			if len(args) > 0 {
				return nil, &wn.Error{Code: 2, Message: "season show accepts --id only"}
			}
			return c.SeasonShow(ctx, product, id)
		})
	}}
	show.Flags().StringVar(&product, "product", "", "Season product: sakura or koyo")
	show.Flags().StringVar(&id, "id", "", "Numeric source spot ID obtained from season search")

	return show
}
func newSeasonCompareCmd(flags *rootFlags, o *evidenceOptions) *cobra.Command {
	var cp, ids, date string
	var tolerance int
	cmp := &cobra.Command{Use: "compare", Short: "Compare 2–5 spots by caller tolerance from published peak date", Example: strings.Trim(`
  weathernews-pp-cli season compare --product koyo --ids 26101,26111 --date 2026-11-26 --max-days-from-peak 3 --agent`, "\n"), Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:typed-exit-codes": "0,2,3,4,5,7", "pp:happy-args": "--product=koyo;--ids=26101,26111;--date=2026-11-26;--max-days-from-peak=3"}, RunE: func(cmd *cobra.Command, args []string) error {
		return withEvidence(cmd, flags, o, func(ctx context.Context, c *wn.Client) (map[string]any, error) {
			if len(args) > 0 || !cmd.Flags().Changed("max-days-from-peak") {
				return nil, &wn.Error{Code: 2, Message: "supply --ids, --date and explicit --max-days-from-peak (0–45)"}
			}
			return c.CompareSeason(ctx, cp, strings.Split(ids, ","), date, tolerance)
		})
	}}
	cmp.Flags().StringVar(&cp, "product", "", "Season product: sakura or koyo")
	cmp.Flags().StringVar(&ids, "ids", "", "Comma-separated distinct numeric source spot IDs, 2–5 maximum")
	cmp.Flags().StringVar(&date, "date", "", "Required real YYYY-MM-DD date in JST")
	cmp.Flags().IntVar(&tolerance, "max-days-from-peak", 0, "Explicit caller maximum absolute days from predicted peak, 0–45")

	return cmp
}
