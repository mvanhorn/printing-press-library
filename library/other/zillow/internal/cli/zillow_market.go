// Copyright 2026 Hunter Veltri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:client-call
// pp:data-source auto

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/zillow/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/zillow/internal/zillowdata"
	"github.com/spf13/cobra"
)

const zillowAttribution = "Data provided by Zillow Group"

type marketEvidence struct {
	Metric       string    `json:"metric"`
	SourceURL    string    `json:"source_url"`
	DataSource   string    `json:"data_source"`
	FetchedAt    time.Time `json:"fetched_at"`
	ETag         string    `json:"etag,omitempty"`
	LastModified string    `json:"last_modified,omitempty"`
	Attribution  string    `json:"attribution"`
}

type metricValue struct {
	Metric   string         `json:"metric"`
	RegionID int64          `json:"region_id"`
	Region   string         `json:"region"`
	Date     string         `json:"date"`
	Value    float64        `json:"value"`
	Unit     string         `json:"unit"`
	MoM      *float64       `json:"mom_percent,omitempty"`
	YoY      *float64       `json:"yoy_percent,omitempty"`
	Evidence marketEvidence `json:"evidence"`
}

func init() {
	registerNovelCommand(addZillowMarketCommands)
}

func addZillowMarketCommands(root *cobra.Command, flags *rootFlags) {
	commands := []*cobra.Command{
		newAuthInfoCmd(flags),
		newDatasetsCmd(flags),
		newRegionCmd(flags),
		newLatestCmd(flags),
		newTrendsCmd(flags),
		newCompareMarketsCmd(flags),
		newSummaryCmd(flags),
		newAffordabilityCmd(flags),
		newYieldProxyCmd(flags),
		newSupplyRatioCmd(flags),
		newTurningPointsCmd(flags),
		newShortlistCmd(flags),
		newQualityCmd(flags),
		newBreadthCmd(flags),
		newBuyVsRentCmd(flags),
		newNegotiationCmd(flags),
		newTierSpreadCmd(flags),
		newDemandPressureCmd(flags),
		newBuildGapCmd(flags),
		newClientBriefCmd(flags),
		newExplainCmd(flags),
		newMortgageCmd(flags),
		newOpenCmd(flags),
	}
	for _, cmd := range commands {
		makeMarketDryRunSafe(cmd, flags)
	}
	root.AddCommand(commands...)
}

func makeMarketDryRunSafe(cmd *cobra.Command, flags *rootFlags) {
	if run := cmd.RunE; run != nil {
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return emitMarket(cmd, flags, map[string]any{
					"status":  "dry_run",
					"command": cmd.CommandPath(),
					"args":    args,
				}, nil)
			}
			return run(cmd, args)
		}
	}
	for _, child := range cmd.Commands() {
		makeMarketDryRunSafe(child, flags)
	}
}

func newAuthInfoCmd(flags *rootFlags) *cobra.Command {
	group := &cobra.Command{Use: "auth", Short: "Show authentication requirements"}
	run := func(cmd *cobra.Command, args []string) error {
		return emitMarket(cmd, flags, map[string]any{
			"official_research": "no authentication required",
			"bridge":            "optional; separately approved BRIDGE_ACCESS_TOKEN required for permissioned datasets",
		}, nil)
	}
	group.AddCommand(
		&cobra.Command{Use: "status", Short: "Show auth status", RunE: run},
		&cobra.Command{Use: "setup", Short: "Explain auth setup", RunE: run},
	)
	return group
}

func marketLoader(flags *rootFlags) (zillowdata.Loader, error) {
	cacheDir, err := cliutil.CacheDir()
	if err != nil {
		return zillowdata.Loader{}, err
	}
	timeout := flags.timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return zillowdata.Loader{
		BaseURL:    zillowdata.DefaultBaseURL,
		CacheDir:   cacheDir,
		HTTPClient: &http.Client{Timeout: timeout},
		MaxAge:     flags.maxAge,
		Limiter:    cliutil.NewAdaptiveLimiter(flags.rateLimit),
	}, nil
}

func loadRegion(ctx context.Context, flags *rootFlags, metric, region string) (*zillowdata.Table, zillowdata.Row, error) {
	loader, err := marketLoader(flags)
	if err != nil {
		return nil, zillowdata.Row{}, err
	}
	table, err := loader.Load(ctx, metric, flags.dataSource)
	if err != nil {
		return nil, zillowdata.Row{}, err
	}
	row, err := table.ResolveRegion(region)
	if err != nil {
		return nil, zillowdata.Row{}, err
	}
	return table, row, nil
}

func evidence(table *zillowdata.Table) marketEvidence {
	return marketEvidence{
		Metric: table.Dataset.Key, SourceURL: table.SourceURL, DataSource: table.Source,
		FetchedAt: table.FetchedAt, ETag: table.ETag, LastModified: table.LastModified,
		Attribution: zillowAttribution,
	}
}

func emitMarket(cmd *cobra.Command, flags *rootFlags, value any, meta map[string]any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if meta == nil {
		meta = map[string]any{}
	}
	meta["attribution"] = zillowAttribution
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, meta)
}

func latestMetric(table *zillowdata.Table, row zillowdata.Row) (metricValue, error) {
	date, value, ok := row.Latest()
	if !ok {
		return metricValue{}, fmt.Errorf("%s has no observations for %s", table.Dataset.Key, row.DisplayName())
	}
	result := metricValue{
		Metric: table.Dataset.Key, RegionID: row.RegionID, Region: row.DisplayName(),
		Date: date.Format("2006-01-02"), Value: value, Unit: table.Dataset.Unit, Evidence: evidence(table),
	}
	if change, _, _, ok := row.ChangeMonths(1); ok {
		result.MoM = floatPtr(change)
	}
	if change, _, _, ok := row.ChangeMonths(12); ok {
		result.YoY = floatPtr(change)
	}
	return result, nil
}

func floatPtr(value float64) *float64 { return &value }

func commonLatest(rows ...zillowdata.Row) (time.Time, []float64, bool) {
	if len(rows) == 0 {
		return time.Time{}, nil, false
	}
	dates := rows[0].SortedDates()
	for i := len(dates) - 1; i >= 0; i-- {
		values := make([]float64, len(rows))
		ok := true
		for j, row := range rows {
			value, found := row.ValueAt(dates[i])
			if !found {
				ok = false
				break
			}
			values[j] = value
		}
		if ok {
			return dates[i], values, true
		}
	}
	return time.Time{}, nil, false
}

func changeAt(row zillowdata.Row, end time.Time, months int) (float64, bool) {
	endValue, ok := row.ValueAt(end)
	if !ok {
		return 0, false
	}
	target := end.AddDate(0, -months, 0)
	var best time.Time
	for date := range row.Values {
		if date.After(end) {
			continue
		}
		delta := date.Sub(target)
		if delta < 0 {
			delta = -delta
		}
		bestDelta := best.Sub(target)
		if bestDelta < 0 {
			bestDelta = -bestDelta
		}
		if best.IsZero() || delta < bestDelta {
			best = date
		}
	}
	if best.IsZero() || math.Abs(best.Sub(target).Hours()) > 45*24 {
		return 0, false
	}
	startValue := row.Values[best]
	if startValue == 0 {
		return 0, false
	}
	return (endValue/startValue - 1) * 100, true
}

func newDatasetsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use: "datasets", Short: "List supported official Zillow Research datasets",
		Example:     `  zillow-pp-cli datasets --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			type row struct {
				zillowdata.Dataset
				SourceURL string `json:"source_url"`
			}
			out := make([]row, 0, len(zillowdata.Datasets()))
			for _, dataset := range zillowdata.Datasets() {
				out = append(out, row{Dataset: dataset, SourceURL: zillowdata.DefaultBaseURL + dataset.Path})
			}
			return emitMarket(cmd, flags, out, map[string]any{"count": len(out)})
		},
	}
}

func newRegionCmd(flags *rootFlags) *cobra.Command {
	group := &cobra.Command{Use: "region", Short: "Resolve Zillow Research region names and IDs"}
	var metric string
	resolve := &cobra.Command{
		Use: "resolve <name-or-id>", Args: cobra.ExactArgs(1), Short: "Resolve one region against a dataset",
		Example:     `  zillow-pp-cli region resolve "Austin, TX" --metric zhvi --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			table, row, err := loadRegion(cmd.Context(), flags, metric, args[0])
			if err != nil {
				return err
			}
			return emitMarket(cmd, flags, map[string]any{
				"region_id": row.RegionID, "region": row.DisplayName(), "region_type": row.RegionType,
				"state": row.StateName, "size_rank": row.SizeRank, "evidence": evidence(table),
			}, nil)
		},
	}
	resolve.Flags().StringVar(&metric, "metric", "zhvi", "Dataset used for region coverage")
	group.AddCommand(resolve)
	return group
}

func newLatestCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use: "latest <metric> <region>", Args: cobra.ExactArgs(2), Short: "Show latest value with month-over-month and year-over-year change",
		Example:     `  zillow-pp-cli latest zhvi "Austin, TX" --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<metric>=zhvi;<region>=Austin, TX"},
		RunE: func(cmd *cobra.Command, args []string) error {
			table, row, err := loadRegion(cmd.Context(), flags, args[0], args[1])
			if err != nil {
				return err
			}
			result, err := latestMetric(table, row)
			if err != nil {
				return err
			}
			return emitMarket(cmd, flags, result, nil)
		},
	}
}

func newTrendsCmd(flags *rootFlags) *cobra.Command {
	var months int
	cmd := &cobra.Command{
		Use: "trends <metric> <region>", Args: cobra.ExactArgs(2), Short: "Show one regional time series",
		Example:     `  zillow-pp-cli trends zhvi "Austin, TX" --months 24 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<metric>=zhvi;<region>=Austin, TX"},
		RunE: func(cmd *cobra.Command, args []string) error {
			table, row, err := loadRegion(cmd.Context(), flags, args[0], args[1])
			if err != nil {
				return err
			}
			dates := row.SortedDates()
			if months > 0 && len(dates) > months {
				dates = dates[len(dates)-months:]
			}
			out := make([]map[string]any, 0, len(dates))
			for _, date := range dates {
				out = append(out, map[string]any{
					"metric": table.Dataset.Key, "region_id": row.RegionID, "region": row.DisplayName(),
					"date": date.Format("2006-01-02"), "value": row.Values[date], "unit": table.Dataset.Unit,
				})
			}
			return emitMarket(cmd, flags, map[string]any{"data": out, "evidence": evidence(table)}, nil)
		},
	}
	cmd.Flags().IntVar(&months, "months", 24, "Number of most recent observations; 0 means all")
	return cmd
}

func newCompareMarketsCmd(flags *rootFlags) *cobra.Command {
	var regions string
	cmd := &cobra.Command{
		Use: "compare <metric>", Args: cobra.ExactArgs(1), Short: "Compare one metric across regions on latest common data",
		Example:     `  zillow-pp-cli compare zhvi --regions "394355,394530" --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<metric>=zhvi;--regions=394355,394530"},
		RunE: func(cmd *cobra.Command, args []string) error {
			names := splitList(regions)
			if len(names) < 2 {
				return usageErr(fmt.Errorf("--regions requires at least two comma-separated regions"))
			}
			loader, err := marketLoader(flags)
			if err != nil {
				return err
			}
			table, err := loader.Load(cmd.Context(), args[0], flags.dataSource)
			if err != nil {
				return err
			}
			out := make([]metricValue, 0, len(names))
			for _, name := range names {
				row, resolveErr := table.ResolveRegion(name)
				if resolveErr != nil {
					return resolveErr
				}
				value, valueErr := latestMetric(table, row)
				if valueErr != nil {
					return valueErr
				}
				out = append(out, value)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Value > out[j].Value })
			return emitMarket(cmd, flags, out, nil)
		},
	}
	cmd.Flags().StringVar(&regions, "regions", "", "Comma-separated region names or IDs")
	_ = cmd.MarkFlagRequired("regions")
	return cmd
}

func newSummaryCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use: "summary <region>", Args: cobra.ExactArgs(1), Short: "Build a one-shot regional market snapshot",
		Example:     `  zillow-pp-cli summary "Austin, TX" --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<region>=Austin, TX"},
		RunE: func(cmd *cobra.Command, args []string) error {
			metrics := []string{"zhvi", "zhvf", "zori", "inventory", "sales", "days_pending", "market_temperature", "homeowner_income"}
			out := make(map[string]any, len(metrics)+2)
			var evidenceRows []marketEvidence
			available := 0
			for _, metric := range metrics {
				table, row, err := loadRegion(cmd.Context(), flags, metric, args[0])
				if err != nil {
					out[metric] = map[string]any{"unavailable": err.Error()}
					continue
				}
				value, err := latestMetric(table, row)
				if err != nil {
					out[metric] = map[string]any{"unavailable": err.Error()}
					continue
				}
				out[metric] = value
				available++
				evidenceRows = append(evidenceRows, evidence(table))
			}
			if available == 0 {
				return fmt.Errorf("region %q was unavailable in every summary dataset", args[0])
			}
			out["region"] = args[0]
			out["evidence"] = evidenceRows
			return emitMarket(cmd, flags, out, map[string]any{"partial_results_allowed": true})
		},
	}
}

func newAffordabilityCmd(flags *rootFlags) *cobra.Command {
	group := &cobra.Command{Use: "affordability", Short: "Affordability analysis from official Zillow estimates"}
	var income float64
	gap := &cobra.Command{
		Use: "gap <region>", Args: cobra.ExactArgs(1), Short: "Compare household income with Zillow income needed",
		Example:     `  zillow-pp-cli affordability gap "Austin, TX" --income 120000 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<region>=Austin, TX;--income=120000"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if income <= 0 {
				return usageErr(fmt.Errorf("--income must be greater than zero"))
			}
			table, row, err := loadRegion(cmd.Context(), flags, "homeowner_income", args[0])
			if err != nil {
				return err
			}
			value, err := latestMetric(table, row)
			if err != nil {
				return err
			}
			gapUSD := income - value.Value
			return emitMarket(cmd, flags, map[string]any{
				"region": value.Region, "date": value.Date, "household_income": income,
				"income_needed": value.Value, "gap_usd": gapUSD,
				"gap_percent_of_needed":          gapUSD / value.Value * 100,
				"affordable_at_zillow_threshold": gapUSD >= 0, "evidence": value.Evidence,
			}, nil)
		},
	}
	gap.Flags().Float64Var(&income, "income", 0, "Annual household income in USD")
	_ = gap.MarkFlagRequired("income")
	group.AddCommand(gap)
	return group
}

func newYieldProxyCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use: "yield-proxy <region>", Args: cobra.ExactArgs(1), Short: "Compute annualized rent-to-value proxy from ZORI and ZHVI",
		Example:     `  zillow-pp-cli yield-proxy "Austin, TX" --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<region>=Austin, TX"},
		RunE: func(cmd *cobra.Command, args []string) error {
			zhviTable, zhvi, err := loadRegion(cmd.Context(), flags, "zhvi", args[0])
			if err != nil {
				return err
			}
			zoriTable, zori, err := loadRegion(cmd.Context(), flags, "zori", args[0])
			if err != nil {
				return err
			}
			date, values, ok := commonLatest(zhvi, zori)
			if !ok || values[0] == 0 {
				return fmt.Errorf("no common ZHVI/ZORI observation for %s", args[0])
			}
			zhviYoY, _ := changeAt(zhvi, date, 12)
			zoriYoY, _ := changeAt(zori, date, 12)
			return emitMarket(cmd, flags, map[string]any{
				"region": zhvi.DisplayName(), "date": date.Format("2006-01-02"),
				"home_value": values[0], "monthly_rent": values[1],
				"annual_rent_to_value_percent": values[1] * 12 / values[0] * 100,
				"rent_growth_yoy_percent":      zoriYoY, "value_growth_yoy_percent": zhviYoY,
				"growth_spread_percentage_points": zoriYoY - zhviYoY,
				"caveat":                          "Gross market proxy; excludes vacancy, taxes, insurance, maintenance, financing, and property-specific differences.",
				"evidence":                        []marketEvidence{evidence(zhviTable), evidence(zoriTable)},
			}, nil)
		},
	}
}

func newSupplyRatioCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use: "supply-ratio <region>", Args: cobra.ExactArgs(1), Short: "Compute inventory divided by monthly sales flow",
		Example:     `  zillow-pp-cli supply-ratio "Austin, TX" --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<region>=Austin, TX"},
		RunE: func(cmd *cobra.Command, args []string) error {
			inventoryTable, inventory, err := loadRegion(cmd.Context(), flags, "inventory", args[0])
			if err != nil {
				return err
			}
			salesTable, sales, err := loadRegion(cmd.Context(), flags, "sales", args[0])
			if err != nil {
				return err
			}
			date, values, ok := commonLatest(inventory, sales)
			if !ok || values[1] == 0 {
				return fmt.Errorf("no common non-zero inventory/sales observation for %s", args[0])
			}
			return emitMarket(cmd, flags, map[string]any{
				"region": inventory.DisplayName(), "date": date.Format("2006-01-02"),
				"inventory": values[0], "monthly_sales_nowcast": values[1],
				"supply_absorption_ratio": values[0] / values[1],
				"interpretation":          "Approximate months of inventory at current monthly sales flow; not Zillow's months-of-supply metric.",
				"evidence":                []marketEvidence{evidence(inventoryTable), evidence(salesTable)},
			}, nil)
		},
	}
}

func splitList(raw string) []string {
	var out []string
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func clamp(value, low, high float64) float64 {
	return math.Max(low, math.Min(high, value))
}

func newTurningPointsCmd(flags *rootFlags) *cobra.Command {
	var months int
	cmd := &cobra.Command{
		Use: "turning-points <region>", Args: cobra.ExactArgs(1), Short: "Find dated slope reversals across core market series",
		Example:     `  zillow-pp-cli turning-points "Austin, TX" --months 24 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<region>=Austin, TX"},
		RunE: func(cmd *cobra.Command, args []string) error {
			metrics := []string{"market_temperature", "inventory", "days_pending", "zhvi"}
			var points []map[string]any
			var evidenceRows []marketEvidence
			for _, metric := range metrics {
				table, row, err := loadRegion(cmd.Context(), flags, metric, args[0])
				if err != nil {
					return err
				}
				evidenceRows = append(evidenceRows, evidence(table))
				dates := row.SortedDates()
				if months > 0 && len(dates) > months+2 {
					dates = dates[len(dates)-months-2:]
				}
				for i := 2; i < len(dates); i++ {
					prev := row.Values[dates[i-1]] - row.Values[dates[i-2]]
					current := row.Values[dates[i]] - row.Values[dates[i-1]]
					if prev == 0 || current == 0 || math.Signbit(prev) == math.Signbit(current) {
						continue
					}
					direction := "downturn"
					if current > 0 {
						direction = "upturn"
					}
					points = append(points, map[string]any{
						"metric": metric, "date": dates[i].Format("2006-01-02"), "direction": direction,
						"previous_change": prev, "current_change": current, "value": row.Values[dates[i]],
					})
				}
			}
			sort.Slice(points, func(i, j int) bool {
				return fmt.Sprint(points[i]["date"]) > fmt.Sprint(points[j]["date"])
			})
			if len(points) > 20 {
				points = points[:20]
			}
			return emitMarket(cmd, flags, map[string]any{
				"region": args[0], "turning_points": points,
				"method":   "Mechanical month-to-month slope sign reversals; not a market forecast.",
				"evidence": evidenceRows,
			}, nil)
		},
	}
	cmd.Flags().IntVar(&months, "months", 24, "Observation window")
	return cmd
}

type weightedMetric struct {
	metric string
	weight float64
}

func parseWeights(raw []string) ([]weightedMetric, error) {
	if len(raw) == 0 {
		raw = []string{"zhvi=0.25", "zori=0.20", "inventory=0.15", "market_temperature=0.20", "homeowner_income=-0.20"}
	}
	out := make([]weightedMetric, 0, len(raw))
	for _, item := range raw {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			return nil, fmt.Errorf("invalid weight %q; expected metric=weight", item)
		}
		if _, found := zillowdata.DatasetByKey(key); !found {
			return nil, fmt.Errorf("unknown weighted metric %q", key)
		}
		weight, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || weight == 0 {
			return nil, fmt.Errorf("invalid non-zero weight %q", value)
		}
		out = append(out, weightedMetric{metric: zillowdata.NormalizeMetric(key), weight: weight})
	}
	return out, nil
}

func newShortlistCmd(flags *rootFlags) *cobra.Command {
	var regions string
	var rawWeights []string
	cmd := &cobra.Command{
		Use: "shortlist", Short: "Rank regions with explicit weighted Zillow metrics",
		Example:     `  zillow-pp-cli shortlist --regions "394355,394530" --weight zhvi=-1 --weight inventory=1 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--regions=394355,394530"},
		RunE: func(cmd *cobra.Command, args []string) error {
			names := splitList(regions)
			if len(names) < 2 {
				return usageErr(fmt.Errorf("--regions requires at least two comma-separated regions"))
			}
			weights, err := parseWeights(rawWeights)
			if err != nil {
				return usageErr(err)
			}
			type candidate struct {
				Name       string             `json:"region"`
				RegionID   int64              `json:"region_id"`
				Score      float64            `json:"score"`
				Raw        map[string]float64 `json:"raw_values"`
				Components map[string]float64 `json:"components"`
			}
			candidates := make([]candidate, len(names))
			for i, name := range names {
				candidates[i] = candidate{Name: name, Raw: map[string]float64{}, Components: map[string]float64{}}
			}
			var evidenceRows []marketEvidence
			for _, weighted := range weights {
				loader, loadErr := marketLoader(flags)
				if loadErr != nil {
					return loadErr
				}
				table, loadErr := loader.Load(cmd.Context(), weighted.metric, flags.dataSource)
				if loadErr != nil {
					return loadErr
				}
				evidenceRows = append(evidenceRows, evidence(table))
				minValue, maxValue := math.Inf(1), math.Inf(-1)
				for i, name := range names {
					row, resolveErr := table.ResolveRegion(name)
					if resolveErr != nil {
						return resolveErr
					}
					_, value, ok := row.Latest()
					if !ok {
						return fmt.Errorf("%s has no %s observation", name, weighted.metric)
					}
					candidates[i].Name = row.DisplayName()
					candidates[i].RegionID = row.RegionID
					candidates[i].Raw[weighted.metric] = value
					minValue, maxValue = math.Min(minValue, value), math.Max(maxValue, value)
				}
				for i := range candidates {
					normalized := 50.0
					if maxValue > minValue {
						normalized = (candidates[i].Raw[weighted.metric] - minValue) / (maxValue - minValue) * 100
					}
					if weighted.weight < 0 {
						normalized = 100 - normalized
					}
					component := normalized * math.Abs(weighted.weight)
					candidates[i].Components[weighted.metric] = component
					candidates[i].Score += component
				}
			}
			totalWeight := 0.0
			for _, weighted := range weights {
				totalWeight += math.Abs(weighted.weight)
			}
			for i := range candidates {
				candidates[i].Score /= totalWeight
			}
			sort.Slice(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })
			return emitMarket(cmd, flags, map[string]any{
				"ranked_regions": candidates, "weights": rawWeights,
				"method":   "Per-metric min-max normalization across supplied regions; negative weights prefer lower raw values.",
				"evidence": evidenceRows,
			}, nil)
		},
	}
	cmd.Flags().StringVar(&regions, "regions", "", "Comma-separated region names or IDs")
	cmd.Flags().StringSliceVar(&rawWeights, "weight", nil, "Metric weight as metric=weight; repeatable")
	_ = cmd.MarkFlagRequired("regions")
	return cmd
}

func newQualityCmd(flags *rootFlags) *cobra.Command {
	group := &cobra.Command{Use: "quality", Short: "Audit local or live Zillow dataset quality"}
	var jumpThreshold float64
	audit := &cobra.Command{
		Use: "audit <metric>", Args: cobra.ExactArgs(1), Short: "Find missing cells, duplicate regions, and large jumps",
		Example:     `  zillow-pp-cli quality audit zhvi --jump-threshold 20 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<metric>=zhvi"},
		RunE: func(cmd *cobra.Command, args []string) error {
			loader, err := marketLoader(flags)
			if err != nil {
				return err
			}
			table, err := loader.Load(cmd.Context(), args[0], flags.dataSource)
			if err != nil {
				return err
			}
			allDates := map[time.Time]struct{}{}
			seenIDs := map[int64]int{}
			for _, row := range table.Rows {
				seenIDs[row.RegionID]++
				for date := range row.Values {
					allDates[date] = struct{}{}
				}
			}
			missing, duplicateRegions := 0, 0
			var jumps []map[string]any
			for _, count := range seenIDs {
				if count > 1 {
					duplicateRegions++
				}
			}
			for _, row := range table.Rows {
				missing += len(allDates) - len(row.Values)
				dates := row.SortedDates()
				for i := 1; i < len(dates); i++ {
					before, after := row.Values[dates[i-1]], row.Values[dates[i]]
					if before == 0 {
						continue
					}
					change := (after/before - 1) * 100
					if math.Abs(change) >= jumpThreshold {
						jumps = append(jumps, map[string]any{
							"region_id": row.RegionID, "region": row.DisplayName(),
							"date": dates[i].Format("2006-01-02"), "change_percent": change,
						})
					}
				}
			}
			if len(jumps) > 50 {
				jumps = jumps[:50]
			}
			expected := len(table.Rows) * len(allDates)
			return emitMarket(cmd, flags, map[string]any{
				"metric": table.Dataset.Key, "regions": len(table.Rows), "observation_dates": len(allDates),
				"expected_cells": expected, "missing_cells": missing,
				"coverage_percent":     float64(expected-missing) / float64(expected) * 100,
				"duplicate_region_ids": duplicateRegions, "large_jumps": jumps,
				"jump_threshold_percent": jumpThreshold, "evidence": evidence(table),
			}, nil)
		},
	}
	audit.Flags().Float64Var(&jumpThreshold, "jump-threshold", 50, "Absolute month-over-month percentage threshold")
	group.AddCommand(audit)
	return group
}

func newBreadthCmd(flags *rootFlags) *cobra.Command {
	var groupBy string
	var months int
	cmd := &cobra.Command{
		Use: "breadth <metric>", Args: cobra.ExactArgs(1), Short: "Measure how broadly a metric is rising or falling",
		Example:     `  zillow-pp-cli breadth zhvi --group-by state --months 12 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<metric>=zhvi"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if groupBy != "state" && groupBy != "region-type" {
				return usageErr(fmt.Errorf("--group-by must be state or region-type"))
			}
			loader, err := marketLoader(flags)
			if err != nil {
				return err
			}
			table, err := loader.Load(cmd.Context(), args[0], flags.dataSource)
			if err != nil {
				return err
			}
			type aggregate struct {
				Rising, Falling, Unchanged int
				Changes                    []float64
			}
			groups := map[string]*aggregate{}
			for _, row := range table.Rows {
				change, _, _, ok := row.ChangeMonths(months)
				if !ok {
					continue
				}
				key := row.StateName
				if groupBy == "region-type" {
					key = row.RegionType
				}
				if key == "" {
					key = "unknown"
				}
				if groups[key] == nil {
					groups[key] = &aggregate{}
				}
				groups[key].Changes = append(groups[key].Changes, change)
				switch {
				case change > 0.01:
					groups[key].Rising++
				case change < -0.01:
					groups[key].Falling++
				default:
					groups[key].Unchanged++
				}
			}
			var out []map[string]any
			for key, group := range groups {
				sort.Float64s(group.Changes)
				median := group.Changes[len(group.Changes)/2]
				total := group.Rising + group.Falling + group.Unchanged
				out = append(out, map[string]any{
					"group": key, "rising": group.Rising, "falling": group.Falling,
					"unchanged": group.Unchanged, "total": total,
					"rising_share_percent":  float64(group.Rising) / float64(total) * 100,
					"median_change_percent": median,
				})
			}
			sort.Slice(out, func(i, j int) bool { return fmt.Sprint(out[i]["group"]) < fmt.Sprint(out[j]["group"]) })
			return emitMarket(cmd, flags, map[string]any{
				"metric": table.Dataset.Key, "change_months": months, "groups": out, "evidence": evidence(table),
			}, nil)
		},
	}
	cmd.Flags().StringVar(&groupBy, "group-by", "state", "Grouping: state or region-type")
	cmd.Flags().IntVar(&months, "months", 12, "Change window in months")
	return cmd
}

type buyRentAssumptions struct {
	InterestRate float64 `json:"interest_rate_percent"`
	DownPayment  float64 `json:"down_payment_percent"`
	TermYears    int     `json:"term_years"`
	HorizonYears int     `json:"horizon_years"`
	Appreciation float64 `json:"appreciation_percent"`
	RentGrowth   float64 `json:"rent_growth_percent"`
	PropertyTax  float64 `json:"property_tax_percent"`
	Insurance    float64 `json:"insurance_percent"`
	Maintenance  float64 `json:"maintenance_percent"`
	ClosingCosts float64 `json:"closing_cost_percent"`
	SellingCosts float64 `json:"selling_cost_percent"`
}

func monthlyMortgage(principal, annualRate float64, years int) float64 {
	if principal <= 0 || years <= 0 {
		return 0
	}
	monthlyRate := annualRate / 100 / 12
	payments := float64(years * 12)
	if monthlyRate == 0 {
		return principal / payments
	}
	factor := math.Pow(1+monthlyRate, payments)
	return principal * monthlyRate * factor / (factor - 1)
}

func remainingBalance(principal, annualRate float64, years, paidMonths int) float64 {
	if paidMonths <= 0 {
		return principal
	}
	if paidMonths >= years*12 {
		return 0
	}
	rate := annualRate / 100 / 12
	payment := monthlyMortgage(principal, annualRate, years)
	if rate == 0 {
		return math.Max(0, principal-payment*float64(paidMonths))
	}
	return principal*math.Pow(1+rate, float64(paidMonths)) -
		payment*(math.Pow(1+rate, float64(paidMonths))-1)/rate
}

func breakEvenValues(month int) (bool, any, any) {
	if month <= 0 {
		return false, nil, nil
	}
	return true, month, float64(month) / 12
}

func newBuyVsRentCmd(flags *rootFlags) *cobra.Command {
	assumptions := buyRentAssumptions{
		DownPayment: 20, TermYears: 30, HorizonYears: 10, Appreciation: 3,
		RentGrowth: 3, PropertyTax: 1.1, Insurance: 0.35, Maintenance: 1,
		ClosingCosts: 3, SellingCosts: 6,
	}
	cmd := &cobra.Command{
		Use: "buy-vs-rent <region>", Args: cobra.ExactArgs(1), Short: "Estimate buy-versus-rent break-even with visible assumptions",
		Example:     `  zillow-pp-cli buy-vs-rent "Austin, TX" --rate 6.5 --years 7 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<region>=Austin, TX;--rate=6.5;--years=7"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if assumptions.InterestRate < 0 || assumptions.DownPayment < 0 || assumptions.DownPayment >= 100 ||
				assumptions.TermYears <= 0 || assumptions.HorizonYears <= 0 {
				return usageErr(fmt.Errorf("invalid financial assumptions"))
			}
			zhviTable, zhvi, err := loadRegion(cmd.Context(), flags, "zhvi", args[0])
			if err != nil {
				return err
			}
			zoriTable, zori, err := loadRegion(cmd.Context(), flags, "zori", args[0])
			if err != nil {
				return err
			}
			date, values, ok := commonLatest(zhvi, zori)
			if !ok {
				return fmt.Errorf("no common ZHVI/ZORI observation")
			}
			homePrice, monthlyRent := values[0], values[1]
			down := homePrice * assumptions.DownPayment / 100
			principal := homePrice - down
			payment := monthlyMortgage(principal, assumptions.InterestRate, assumptions.TermYears)
			cumulativeOwnerOutflow := down + homePrice*assumptions.ClosingCosts/100
			cumulativeRent := 0.0
			breakEvenMonth := 0
			var finalOwnerNetCost, finalRentCost float64
			for month := 1; month <= assumptions.HorizonYears*12; month++ {
				yearFraction := float64(month-1) / 12
				currentHome := homePrice * math.Pow(1+assumptions.Appreciation/100, float64(month)/12)
				currentRent := monthlyRent * math.Pow(1+assumptions.RentGrowth/100, yearFraction)
				cumulativeRent += currentRent
				cumulativeOwnerOutflow += payment +
					currentHome*(assumptions.PropertyTax+assumptions.Insurance+assumptions.Maintenance)/100/12
				balance := remainingBalance(principal, assumptions.InterestRate, assumptions.TermYears, month)
				netSaleProceeds := currentHome*(1-assumptions.SellingCosts/100) - balance
				finalOwnerNetCost = cumulativeOwnerOutflow - netSaleProceeds
				finalRentCost = cumulativeRent
				if breakEvenMonth == 0 && finalOwnerNetCost <= cumulativeRent {
					breakEvenMonth = month
				}
			}
			breakEvenReached, breakEvenMonthValue, breakEvenYearValue := breakEvenValues(breakEvenMonth)
			return emitMarket(cmd, flags, map[string]any{
				"region": zhvi.DisplayName(), "data_date": date.Format("2006-01-02"),
				"typical_home_value": homePrice, "typical_monthly_rent": monthlyRent,
				"monthly_principal_interest": payment, "break_even_reached": breakEvenReached,
				"break_even_month":       breakEvenMonthValue,
				"break_even_year":        breakEvenYearValue,
				"horizon_owner_net_cost": finalOwnerNetCost, "horizon_rent_cost": finalRentCost,
				"assumptions": assumptions,
				"caveat":      "Scenario model using regional typical values; excludes opportunity cost, tax deductions, HOA, PMI, utilities, and property-specific conditions.",
				"evidence":    []marketEvidence{evidence(zhviTable), evidence(zoriTable)},
			}, nil)
		},
	}
	cmd.Flags().Float64Var(&assumptions.InterestRate, "rate", 0, "Annual mortgage rate percent; explicit input required")
	cmd.Flags().Float64Var(&assumptions.DownPayment, "down-payment", assumptions.DownPayment, "Down payment percent")
	cmd.Flags().IntVar(&assumptions.TermYears, "term-years", assumptions.TermYears, "Mortgage term")
	cmd.Flags().IntVar(&assumptions.HorizonYears, "years", assumptions.HorizonYears, "Comparison horizon")
	cmd.Flags().Float64Var(&assumptions.Appreciation, "appreciation", assumptions.Appreciation, "Annual home appreciation assumption")
	cmd.Flags().Float64Var(&assumptions.RentGrowth, "rent-growth", assumptions.RentGrowth, "Annual rent growth assumption")
	cmd.Flags().Float64Var(&assumptions.PropertyTax, "property-tax", assumptions.PropertyTax, "Annual property tax percent")
	cmd.Flags().Float64Var(&assumptions.Insurance, "insurance", assumptions.Insurance, "Annual insurance percent")
	cmd.Flags().Float64Var(&assumptions.Maintenance, "maintenance", assumptions.Maintenance, "Annual maintenance percent")
	cmd.Flags().Float64Var(&assumptions.ClosingCosts, "closing-cost", assumptions.ClosingCosts, "Buyer closing cost percent")
	cmd.Flags().Float64Var(&assumptions.SellingCosts, "selling-cost", assumptions.SellingCosts, "Seller transaction cost percent")
	_ = cmd.MarkFlagRequired("rate")
	return cmd
}

func normalizedFraction(value float64) float64 {
	if math.Abs(value) > 2 {
		return value / 100
	}
	return value
}

func newNegotiationCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use: "negotiation <region>", Args: cobra.ExactArgs(1), Short: "Compute explainable buyer negotiation leverage",
		Example:     `  zillow-pp-cli negotiation "Austin, TX" --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<region>=Austin, TX"},
		RunE: func(cmd *cobra.Command, args []string) error {
			keys := []string{"price_cut_share", "sale_to_list", "days_pending", "inventory"}
			tables := make([]*zillowdata.Table, 0, len(keys))
			rows := make([]zillowdata.Row, 0, len(keys))
			for _, key := range keys {
				table, row, err := loadRegion(cmd.Context(), flags, key, args[0])
				if err != nil {
					return err
				}
				tables, rows = append(tables, table), append(rows, row)
			}
			date, values, ok := commonLatest(rows...)
			if !ok {
				return fmt.Errorf("no common negotiation-data observation for %s", args[0])
			}
			priceCutPercent := normalizedFraction(values[0]) * 100
			saleToList := normalizedFraction(values[1])
			inventoryYoY, _ := changeAt(rows[3], date, 12)
			components := map[string]float64{
				"price_cut_share":  clamp((priceCutPercent-5)/25*100, 0, 100),
				"sale_to_list_gap": clamp((1-saleToList)/0.04*100, 0, 100),
				"days_pending":     clamp((values[2]-10)/50*100, 0, 100),
				"inventory_growth": clamp((inventoryYoY+10)/40*100, 0, 100),
			}
			score := 0.0
			for _, component := range components {
				score += component
			}
			score /= float64(len(components))
			evidenceRows := make([]marketEvidence, len(tables))
			for i, table := range tables {
				evidenceRows[i] = evidence(table)
			}
			return emitMarket(cmd, flags, map[string]any{
				"region": rows[0].DisplayName(), "date": date.Format("2006-01-02"),
				"buyer_leverage_score": score, "components": components,
				"raw": map[string]any{
					"price_cut_share_percent": priceCutPercent, "sale_to_list_ratio": saleToList,
					"days_pending": values[2], "inventory": values[3], "inventory_yoy_percent": inventoryYoY,
				},
				"formula":  "Equal-weight average of bounded price-cut, sale-to-list gap, days-pending, and inventory-growth components.",
				"caveat":   "Regional descriptive score, not property-specific offer advice.",
				"evidence": evidenceRows,
			}, nil)
		},
	}
}

func newTierSpreadCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use: "tier-spread <region>", Args: cobra.ExactArgs(1), Short: "Compare bottom-, middle-, and top-tier ZHVI growth",
		Example:     `  zillow-pp-cli tier-spread "Austin, TX" --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<region>=Austin, TX"},
		RunE: func(cmd *cobra.Command, args []string) error {
			keys := []string{"zhvi_bottom_tier", "zhvi", "zhvi_top_tier"}
			tables := make([]*zillowdata.Table, 0, len(keys))
			rows := make([]zillowdata.Row, 0, len(keys))
			for _, key := range keys {
				table, row, err := loadRegion(cmd.Context(), flags, key, args[0])
				if err != nil {
					return err
				}
				tables, rows = append(tables, table), append(rows, row)
			}
			date, values, ok := commonLatest(rows...)
			if !ok {
				return fmt.Errorf("no common tier observation")
			}
			changes := make([]float64, len(rows))
			for i, row := range rows {
				changes[i], _ = changeAt(row, date, 12)
			}
			evidenceRows := make([]marketEvidence, len(tables))
			for i, table := range tables {
				evidenceRows[i] = evidence(table)
			}
			return emitMarket(cmd, flags, map[string]any{
				"region": rows[0].DisplayName(), "date": date.Format("2006-01-02"),
				"bottom_tier":                    map[string]any{"value": values[0], "yoy_percent": changes[0]},
				"middle_tier":                    map[string]any{"value": values[1], "yoy_percent": changes[1]},
				"top_tier":                       map[string]any{"value": values[2], "yoy_percent": changes[2]},
				"bottom_minus_top_growth_points": changes[0] - changes[2],
				"entry_level_squeeze":            changes[0] > changes[2],
				"evidence":                       evidenceRows,
			}, nil)
		},
	}
}

func newDemandPressureCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use: "demand-pressure <region>", Args: cobra.ExactArgs(1), Short: "Combine rental demand and for-sale market momentum",
		Example:     `  zillow-pp-cli demand-pressure "Austin, TX" --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<region>=Austin, TX"},
		RunE: func(cmd *cobra.Command, args []string) error {
			keys := []string{"zordi", "inventory", "sales", "days_pending", "market_temperature"}
			tables := make([]*zillowdata.Table, 0, len(keys))
			rows := make([]zillowdata.Row, 0, len(keys))
			for _, key := range keys {
				table, row, err := loadRegion(cmd.Context(), flags, key, args[0])
				if err != nil {
					return err
				}
				tables, rows = append(tables, table), append(rows, row)
			}
			date, values, ok := commonLatest(rows...)
			if !ok {
				return fmt.Errorf("no common demand-pressure observation")
			}
			changes := make([]float64, len(rows))
			for i, row := range rows {
				changes[i], _ = changeAt(row, date, 12)
			}
			components := map[string]float64{
				"renter_demand":      clamp(50+changes[0]*3, 0, 100),
				"inventory":          clamp(50-changes[1]*2, 0, 100),
				"sales":              clamp(50+changes[2]*2, 0, 100),
				"market_speed":       clamp(50-changes[3]*2, 0, 100),
				"market_temperature": clamp(50+changes[4]*2, 0, 100),
			}
			score := 0.0
			for _, component := range components {
				score += component
			}
			score /= float64(len(components))
			evidenceRows := make([]marketEvidence, len(tables))
			for i, table := range tables {
				evidenceRows[i] = evidence(table)
			}
			return emitMarket(cmd, flags, map[string]any{
				"region": rows[0].DisplayName(), "date": date.Format("2006-01-02"),
				"demand_pressure_score": score, "components": components,
				"raw_values": map[string]float64{
					"zordi": values[0], "inventory": values[1], "sales": values[2],
					"days_pending": values[3], "market_temperature": values[4],
				},
				"yoy_change_percent": map[string]float64{
					"zordi": changes[0], "inventory": changes[1], "sales": changes[2],
					"days_pending": changes[3], "market_temperature": changes[4],
				},
				"formula":  "Equal-weight bounded components derived from year-over-year direction; inventory and days pending are inverted.",
				"caveat":   "Descriptive composite, not a forecast.",
				"evidence": evidenceRows,
			}, nil)
		},
	}
}

func newBuildGapCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use: "new-build-gap <region>", Args: cobra.ExactArgs(1), Short: "Compare new-construction pricing and activity with typical home value",
		Example:     `  zillow-pp-cli new-build-gap "Austin, TX" --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<region>=Austin, TX"},
		RunE: func(cmd *cobra.Command, args []string) error {
			keys := []string{"new_con_price", "new_con_price_per_sqft", "new_con_sales", "zhvi"}
			tables := make([]*zillowdata.Table, 0, len(keys))
			rows := make([]zillowdata.Row, 0, len(keys))
			for _, key := range keys {
				table, row, err := loadRegion(cmd.Context(), flags, key, args[0])
				if err != nil {
					return err
				}
				tables, rows = append(tables, table), append(rows, row)
			}
			date, values, ok := commonLatest(rows...)
			if !ok || values[3] == 0 {
				return fmt.Errorf("no common new-construction and ZHVI observation")
			}
			salesYoY, _ := changeAt(rows[2], date, 12)
			evidenceRows := make([]marketEvidence, len(tables))
			for i, table := range tables {
				evidenceRows[i] = evidence(table)
			}
			return emitMarket(cmd, flags, map[string]any{
				"region": rows[0].DisplayName(), "date": date.Format("2006-01-02"),
				"new_construction_median_sale_price": values[0],
				"new_construction_price_per_sqft":    values[1],
				"new_construction_sales":             values[2], "new_construction_sales_yoy_percent": salesYoY,
				"typical_home_value_zhvi":                values[3],
				"new_construction_price_minus_zhvi":      values[0] - values[3],
				"new_construction_price_to_zhvi_percent": (values[0]/values[3] - 1) * 100,
				"caveat":                                 "New-construction median sale price and ZHVI measure different populations; gap is a market-level comparison, not a like-for-like premium.",
				"evidence":                               evidenceRows,
			}, nil)
		},
	}
}

func newClientBriefCmd(flags *rootFlags) *cobra.Command {
	var income float64
	var format string
	cmd := &cobra.Command{
		Use: "client-brief <region>", Args: cobra.ExactArgs(1), Short: "Create deterministic client-ready market brief",
		Example:     `  zillow-pp-cli client-brief "Austin, TX" --income 120000 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<region>=Austin, TX;--income=120000"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "json" && format != "markdown" {
				return usageErr(fmt.Errorf("--format must be json or markdown"))
			}
			keys := []string{"zhvi", "zori", "inventory", "sales", "days_pending", "market_temperature", "homeowner_income", "price_cut_share", "sale_to_list"}
			values := map[string]metricValue{}
			var evidenceRows []marketEvidence
			for _, key := range keys {
				table, row, err := loadRegion(cmd.Context(), flags, key, args[0])
				if err != nil {
					continue
				}
				value, err := latestMetric(table, row)
				if err != nil {
					continue
				}
				values[key] = value
				evidenceRows = append(evidenceRows, evidence(table))
			}
			if len(values) == 0 {
				return fmt.Errorf("region %q was unavailable in every client-brief dataset", args[0])
			}
			brief := map[string]any{
				"region": args[0], "metrics": values, "generated_at": time.Now().UTC(),
				"evidence": evidenceRows, "attribution": zillowAttribution,
			}
			if income > 0 {
				if needed, ok := values["homeowner_income"]; ok {
					brief["affordability"] = map[string]any{
						"household_income": income, "income_needed": needed.Value,
						"gap_usd": income - needed.Value, "meets_zillow_threshold": income >= needed.Value,
					}
				}
			}
			if format == "json" {
				return emitMarket(cmd, flags, brief, nil)
			}
			var b strings.Builder
			fmt.Fprintf(&b, "# Zillow Market Brief: %s\n\n", args[0])
			fmt.Fprintf(&b, "Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339))
			for _, key := range keys {
				value, ok := values[key]
				if !ok {
					continue
				}
				fmt.Fprintf(&b, "- %s: %.2f %s as of %s", key, value.Value, value.Unit, value.Date)
				if value.YoY != nil {
					fmt.Fprintf(&b, " (%+.2f%% YoY)", *value.YoY)
				}
				b.WriteString("\n")
			}
			if affordability, ok := brief["affordability"].(map[string]any); ok {
				fmt.Fprintf(&b, "\nAffordability gap: $%.0f.\n", affordability["gap_usd"])
			}
			b.WriteString("\nData provided by Zillow Group. Regional metrics are not property-specific financial advice.\n")
			_, err := fmt.Fprint(cmd.OutOrStdout(), b.String())
			return err
		},
	}
	cmd.Flags().Float64Var(&income, "income", 0, "Optional annual household income")
	cmd.Flags().StringVar(&format, "format", "json", "Output: json or markdown")
	return cmd
}

type explanation struct {
	Command  string   `json:"command"`
	Formula  string   `json:"formula"`
	Datasets []string `json:"datasets"`
	Caveat   string   `json:"caveat,omitempty"`
}

var explanations = map[string]explanation{
	"affordability gap": {Command: "affordability gap", Formula: "household_income - homeowner_income_needed", Datasets: []string{"homeowner_income"}},
	"yield-proxy":       {Command: "yield-proxy", Formula: "12 * ZORI / ZHVI * 100", Datasets: []string{"zori", "zhvi"}, Caveat: "Gross regional proxy."},
	"supply-ratio":      {Command: "supply-ratio", Formula: "inventory / monthly_sales_nowcast", Datasets: []string{"inventory", "sales"}},
	"turning-points":    {Command: "turning-points", Formula: "Flag month where consecutive first differences change sign.", Datasets: []string{"market_temperature", "inventory", "days_pending", "zhvi"}},
	"shortlist":         {Command: "shortlist", Formula: "Weighted mean of per-metric min-max normalized values; negative weights invert preference.", Datasets: []string{"user-selected"}},
	"quality audit":     {Command: "quality audit", Formula: "Coverage, duplicate RegionID, and absolute month-over-month jump checks.", Datasets: []string{"user-selected"}},
	"breadth":           {Command: "breadth", Formula: "Count rising/falling/unchanged regions over selected window.", Datasets: []string{"user-selected"}},
	"buy-vs-rent":       {Command: "buy-vs-rent", Formula: "Cumulative ownership outflow minus net sale proceeds versus cumulative rent.", Datasets: []string{"zhvi", "zori"}, Caveat: "Scenario model."},
	"negotiation":       {Command: "negotiation", Formula: "Equal-weight bounded score from price-cut share, sale-to-list gap, days pending, and inventory growth.", Datasets: []string{"price_cut_share", "sale_to_list", "days_pending", "inventory"}},
	"tier-spread":       {Command: "tier-spread", Formula: "Bottom-tier YoY ZHVI growth minus top-tier YoY growth.", Datasets: []string{"zhvi_bottom_tier", "zhvi", "zhvi_top_tier"}},
	"demand-pressure":   {Command: "demand-pressure", Formula: "Equal-weight bounded YoY components; inventory and days pending inverted.", Datasets: []string{"zordi", "inventory", "sales", "days_pending", "market_temperature"}},
	"new-build-gap":     {Command: "new-build-gap", Formula: "New-construction median sale price minus ZHVI, plus new-construction sales growth.", Datasets: []string{"new_con_price", "new_con_price_per_sqft", "new_con_sales", "zhvi"}},
	"client-brief":      {Command: "client-brief", Formula: "Deterministic composition of latest regional metrics and optional affordability gap.", Datasets: []string{"zhvi", "zori", "inventory", "sales", "days_pending", "market_temperature", "homeowner_income", "price_cut_share", "sale_to_list"}},
	"mortgage":          {Command: "mortgage", Formula: "Standard fixed-rate amortization plus explicit taxes, insurance, and maintenance.", Datasets: nil},
}

func newExplainCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use: "explain <command>", Args: cobra.ExactArgs(1), Short: "Explain formula, datasets, and caveats for a compound command",
		Example:     `  zillow-pp-cli explain negotiation --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<command>=negotiation"},
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.ToLower(strings.TrimSpace(args[0]))
			item, ok := explanations[key]
			if !ok {
				keys := make([]string, 0, len(explanations))
				for name := range explanations {
					keys = append(keys, name)
				}
				sort.Strings(keys)
				return fmt.Errorf("unknown explain target %q; available: %s", key, strings.Join(keys, ", "))
			}
			return emitMarket(cmd, flags, map[string]any{
				"explanation": item, "freshness": "Each command reports source URL, fetch time, and cache/live source.",
				"attribution": zillowAttribution,
			}, nil)
		},
	}
}

func newMortgageCmd(flags *rootFlags) *cobra.Command {
	var price, downPayment, rate, tax, insurance, maintenance float64
	var years int
	cmd := &cobra.Command{
		Use: "mortgage", Short: "Calculate fixed-rate monthly housing payment from explicit inputs",
		Example:     `  zillow-pp-cli mortgage --price 450000 --rate 6.5 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--price=450000;--rate=6.5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if price <= 0 || downPayment < 0 || downPayment >= 100 || rate < 0 || years <= 0 {
				return usageErr(fmt.Errorf("price, down payment, rate, and years are invalid"))
			}
			downUSD := price * downPayment / 100
			principal := price - downUSD
			principalInterest := monthlyMortgage(principal, rate, years)
			taxMonthly := price * tax / 100 / 12
			insuranceMonthly := price * insurance / 100 / 12
			maintenanceMonthly := price * maintenance / 100 / 12
			return emitMarket(cmd, flags, map[string]any{
				"home_price": price, "down_payment_percent": downPayment, "down_payment_usd": downUSD,
				"loan_principal": principal, "interest_rate_percent": rate, "term_years": years,
				"principal_interest_monthly": principalInterest, "property_tax_monthly": taxMonthly,
				"insurance_monthly": insuranceMonthly, "maintenance_monthly": maintenanceMonthly,
				"total_monthly": principalInterest + taxMonthly + insuranceMonthly + maintenanceMonthly,
				"caveat":        "Excludes PMI, HOA, closing costs, utilities, and lender-specific fees.",
			}, nil)
		},
	}
	cmd.Flags().Float64Var(&price, "price", 0, "Home price in USD")
	cmd.Flags().Float64Var(&downPayment, "down-payment", 20, "Down payment percent")
	cmd.Flags().Float64Var(&rate, "rate", 0, "Annual interest rate percent")
	cmd.Flags().IntVar(&years, "years", 30, "Loan term")
	cmd.Flags().Float64Var(&tax, "property-tax", 1.1, "Annual property tax percent")
	cmd.Flags().Float64Var(&insurance, "insurance", 0.35, "Annual insurance percent")
	cmd.Flags().Float64Var(&maintenance, "maintenance", 1, "Annual maintenance percent")
	_ = cmd.MarkFlagRequired("price")
	_ = cmd.MarkFlagRequired("rate")
	return cmd
}

func newOpenCmd(flags *rootFlags) *cobra.Command {
	var launch bool
	var target string
	cmd := &cobra.Command{
		Use: "open [region]", Args: cobra.MaximumNArgs(1), Short: "Print a Zillow or Zillow Research URL; launch only with --launch",
		Example:     `  zillow-pp-cli open "Austin, TX" --target homes --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var targetURL string
			switch target {
			case "research":
				targetURL = "https://www.zillow.com/research/data/"
			case "homes":
				if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
					return usageErr(fmt.Errorf("region required for --target homes"))
				}
				slug := strings.ReplaceAll(strings.TrimSpace(args[0]), " ", "-")
				targetURL = "https://www.zillow.com/homes/" + url.PathEscape(slug) + "_rb/"
			default:
				return usageErr(fmt.Errorf("--target must be homes or research"))
			}
			if !launch {
				return emitMarket(cmd, flags, map[string]any{"url": targetURL, "launched": false}, nil)
			}
			var process *exec.Cmd
			switch runtime.GOOS {
			case "windows":
				process = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL)
			case "darwin":
				process = exec.Command("open", targetURL)
			default:
				process = exec.Command("xdg-open", targetURL)
			}
			if err := process.Start(); err != nil {
				return fmt.Errorf("launching browser: %w", err)
			}
			return emitMarket(cmd, flags, map[string]any{"url": targetURL, "launched": true}, nil)
		},
	}
	cmd.Flags().BoolVar(&launch, "launch", false, "Open URL in default browser")
	cmd.Flags().StringVar(&target, "target", "homes", "Target: homes or research")
	return cmd
}
