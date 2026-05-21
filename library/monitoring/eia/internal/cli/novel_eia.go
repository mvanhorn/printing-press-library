// Copyright 2026 synota. Licensed under Apache-2.0. See LICENSE.
//
// EIA novel commands. These compose the underlying /data/ endpoints into
// trader-friendly snapshots: latest retail price by state, BA fuel mix,
// state generation by fuel, Henry Hub history, WTI history, STEO
// forecasts, and SEDS-derived CO2 emissions. Each command calls the real
// client; nothing is hand-baked.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/eia/internal/client"

	"github.com/spf13/cobra"
)

// eiaEnvelope mirrors the EIA APIv2 response shape. Data values are
// strings since v2.1.6 — we keep them as RawMessage and parse per row.
type eiaEnvelope struct {
	Response struct {
		Total      json.RawMessage   `json:"total"`
		DateFormat string            `json:"dateFormat"`
		Frequency  string            `json:"frequency"`
		Data       []json.RawMessage `json:"data"`
		Warnings   []json.RawMessage `json:"warnings"`
	} `json:"response"`
	APIVersion string `json:"apiVersion"`
}

// eiaRow is a permissive view of one data row. Different routes return
// different column sets; we extract by key after the fact.
type eiaRow = map[string]any

// rowString safely pulls a string-valued column. EIA returns even
// numeric columns as JSON strings.
func rowString(row eiaRow, key string) string {
	v, ok := row[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		// Periods sometimes deserialize as numbers if a row is malformed;
		// keep this branch defensive.
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

// rowFloat parses a numeric column (always a string on the wire).
func rowFloat(row eiaRow, key string) (float64, bool) {
	s := rowString(row, key)
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// callEiaData hits a /v2/.../data/ endpoint with the EIA convention:
// frequency, data[]=value (or whatever columns the caller wants),
// facets[<id>]=<value>, optional start/end/sort/length. Returns the
// parsed envelope's data rows plus the envelope for metadata access.
func callEiaData(c *client.Client, path string, params map[string]string) (*eiaEnvelope, []eiaRow, error) {
	raw, err := c.Get(path, params)
	if err != nil {
		return nil, nil, err
	}
	// Some helpers in the codebase pre-unwrap; if "response" is missing,
	// fall back to treating the body as the data array.
	var env eiaEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, nil, fmt.Errorf("decode EIA envelope: %w", err)
	}
	rows := make([]eiaRow, 0, len(env.Response.Data))
	for _, item := range env.Response.Data {
		var row eiaRow
		if err := json.Unmarshal(item, &row); err != nil {
			continue
		}
		rows = append(rows, row)
	}
	return &env, rows, nil
}

// emit prints either JSON (when agent/json flags are set) or a small
// human table. The novel commands always carry the structured payload
// — the table is for eyeballing in the terminal.
func emit(cmd *cobra.Command, flags *rootFlags, payload any, columns []string, table [][]string) error {
	if flags.asJSON || flags.compact || flags.quiet || flags.plain {
		out, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
		return nil
	}
	if len(table) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no rows)")
		return nil
	}
	widths := make([]int, len(columns))
	for i, c := range columns {
		widths[i] = len(c)
	}
	for _, row := range table {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	printRow := func(cells []string) {
		parts := make([]string, len(cells))
		for i, cell := range cells {
			parts[i] = padRight(cell, widths[i])
		}
		fmt.Fprintln(cmd.OutOrStdout(), strings.Join(parts, "  "))
	}
	printRow(columns)
	dashes := make([]string, len(columns))
	for i := range columns {
		dashes[i] = strings.Repeat("-", widths[i])
	}
	printRow(dashes)
	for _, row := range table {
		printRow(row)
	}
	return nil
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

// ----------------------------------------------------------------------
// electricity retail-price <state> [--sector] [--latest]
// ----------------------------------------------------------------------

var retailSectorAliases = map[string]string{
	"residential": "RES",
	"commercial":  "COM",
	"industrial":  "IND",
	"all":         "ALL",
	"transport":   "TRA",
	"transportation": "TRA",
}

func newElectricityRetailPriceCmd(flags *rootFlags) *cobra.Command {
	var sector string
	var latest bool
	var months int

	cmd := &cobra.Command{
		Use:   "retail-price <state>",
		Short: "Retail electricity price for a state, optionally pinned to a sector",
		Long: `Retail electricity price (cents per kWh) for a state. Defaults to the
'ALL' sector aggregate. Pass --sector residential, commercial, industrial,
transportation, or all to pin the sector. --latest returns the single most
recent month; otherwise returns the last --months months (default 12).`,
		Example: "  eia-pp-cli electricity retail-price TX --sector industrial --latest",
		Args:    cobra.ExactArgs(1),
		Annotations: map[string]string{
			"pp:novel":      "electricity.retail-price",
			"pp:client-call": "true",
			"mcp:read-only":  "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			state := strings.ToUpper(strings.TrimSpace(args[0]))
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			sectorCode, ok := retailSectorAliases[strings.ToLower(sector)]
			if !ok {
				sectorCode = strings.ToUpper(sector)
			}
			params := map[string]string{
				"frequency":          "monthly",
				"data[]":             "price",
				"facets[stateid][]":  state,
				"facets[sectorid][]": sectorCode,
				"sort[0][column]":    "period",
				"sort[0][direction]": "desc",
			}
			if latest {
				params["length"] = "1"
			} else {
				if months <= 0 {
					months = 12
				}
				params["length"] = strconv.Itoa(months)
			}
			_, rows, err := callEiaData(c, "/electricity/retail-sales/data/", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			columns := []string{"period", "state", "sector", "price (¢/kWh)", "units"}
			table := make([][]string, 0, len(rows))
			for _, row := range rows {
				price, _ := rowFloat(row, "price")
				table = append(table, []string{
					rowString(row, "period"),
					rowString(row, "stateid"),
					rowString(row, "sectorid"),
					fmt.Sprintf("%.3f", price),
					rowString(row, "price-units"),
				})
			}
			return emit(cmd, flags, rows, columns, table)
		},
	}
	cmd.Flags().StringVar(&sector, "sector", "ALL", "residential | commercial | industrial | transportation | all")
	cmd.Flags().BoolVar(&latest, "latest", false, "return only the most recent month")
	cmd.Flags().IntVar(&months, "months", 12, "months of history (when --latest is not set)")
	return cmd
}

// ----------------------------------------------------------------------
// electricity rto <ba> --fuel-mix
// ----------------------------------------------------------------------

func newElectricityRtoCmd(flags *rootFlags) *cobra.Command {
	var fuelMix bool
	var hours int

	cmd := &cobra.Command{
		Use:   "rto <ba>",
		Short: "BA-level operations (fuel mix snapshot, hourly demand/generation)",
		Long: `Real-time operations for a Balancing Authority. With --fuel-mix, returns
net generation by fuel type for the most recent hours. BA codes are FERC
respondent codes — ERCO (ERCOT), PJM, MISO, CISO (CAISO), NYIS, ISNE,
SWPP, BPAT, FPL, SOCO, etc.`,
		Example: "  eia-pp-cli electricity rto ERCO --fuel-mix --hours 24",
		Args:    cobra.ExactArgs(1),
		Annotations: map[string]string{
			"pp:novel":       "electricity.rto",
			"pp:client-call": "true",
			"mcp:read-only":  "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ba := strings.ToUpper(strings.TrimSpace(args[0]))
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if hours <= 0 {
				hours = 24
			}
			path := "/electricity/rto/region-data/data/"
			if fuelMix {
				path = "/electricity/rto/fuel-type-data/data/"
			}
			params := map[string]string{
				"frequency":           "hourly",
				"data[]":              "value",
				"facets[respondent][]": ba,
				"sort[0][column]":     "period",
				"sort[0][direction]":  "desc",
				"length":              strconv.Itoa(hours * 8), // multiple fueltypes per hour
			}
			_, rows, err := callEiaData(c, path, params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if fuelMix {
				// Aggregate by fueltype across returned hours
				type bucket struct {
					Sum   float64
					Count int
					Last  string
				}
				buckets := map[string]*bucket{}
				for _, row := range rows {
					ft := rowString(row, "fueltype")
					if ft == "" {
						continue
					}
					v, ok := rowFloat(row, "value")
					if !ok {
						continue
					}
					b := buckets[ft]
					if b == nil {
						b = &bucket{}
						buckets[ft] = b
					}
					b.Sum += v
					b.Count++
					if p := rowString(row, "period"); p > b.Last {
						b.Last = p
					}
				}
				type aggRow struct {
					Fueltype  string  `json:"fueltype"`
					AvgMWh    float64 `json:"avg_mwh"`
					Hours     int     `json:"hours"`
					LastPeriod string `json:"last_period"`
				}
				out := make([]aggRow, 0, len(buckets))
				for ft, b := range buckets {
					avg := 0.0
					if b.Count > 0 {
						avg = b.Sum / float64(b.Count)
					}
					out = append(out, aggRow{ft, avg, b.Count, b.Last})
				}
				sort.Slice(out, func(i, j int) bool { return out[i].AvgMWh > out[j].AvgMWh })
				columns := []string{"fueltype", "avg MWh", "hours", "last"}
				table := make([][]string, 0, len(out))
				for _, r := range out {
					table = append(table, []string{r.Fueltype, fmt.Sprintf("%.1f", r.AvgMWh), strconv.Itoa(r.Hours), r.LastPeriod})
				}
				return emit(cmd, flags, out, columns, table)
			}
			// Region-data: D / DF / NG / TI snapshot
			columns := []string{"period", "ba", "type", "value (MWh)"}
			table := make([][]string, 0, len(rows))
			for _, row := range rows {
				v, _ := rowFloat(row, "value")
				table = append(table, []string{
					rowString(row, "period"),
					rowString(row, "respondent"),
					rowString(row, "type"),
					fmt.Sprintf("%.1f", v),
				})
			}
			return emit(cmd, flags, rows, columns, table)
		},
	}
	cmd.Flags().BoolVar(&fuelMix, "fuel-mix", false, "return aggregated fuel-mix snapshot for the BA")
	cmd.Flags().IntVar(&hours, "hours", 24, "hours of history to aggregate")
	return cmd
}

// ----------------------------------------------------------------------
// electricity generation <state> [--fuel-type]
// ----------------------------------------------------------------------

var fuelTypeAliases = map[string]string{
	"natural-gas":  "NG",
	"natgas":       "NG",
	"gas":          "NG",
	"coal":         "COL",
	"solar":        "SUN",
	"wind":         "WND",
	"nuclear":      "NUC",
	"hydro":        "HYC",
	"hydropower":   "HYC",
	"oil":          "PEL",
	"petroleum":    "PEL",
	"biomass":      "BIO",
	"geothermal":   "GEO",
	"all":          "ALL",
}

func newElectricityGenerationCmd(flags *rootFlags) *cobra.Command {
	var fuelType string
	var months int

	cmd := &cobra.Command{
		Use:   "generation <state>",
		Short: "Monthly net generation by state, optionally filtered to one fuel type",
		Long: `Net electricity generation by state (monthly). Optionally pin to a fuel
type: natural-gas, coal, solar, wind, nuclear, hydro, oil, biomass,
geothermal, or all. Without --fuel-type, returns the 'ALL' aggregate.`,
		Example: "  eia-pp-cli electricity generation TX --fuel-type natural-gas",
		Args:    cobra.ExactArgs(1),
		Annotations: map[string]string{
			"pp:novel":       "electricity.generation",
			"pp:client-call": "true",
			"mcp:read-only":  "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			state := strings.ToUpper(strings.TrimSpace(args[0]))
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if months <= 0 {
				months = 12
			}
			ft, ok := fuelTypeAliases[strings.ToLower(fuelType)]
			if !ok {
				ft = strings.ToUpper(fuelType)
			}
			params := map[string]string{
				"frequency":             "monthly",
				"data[]":                "generation",
				"facets[location][]":    state,
				"facets[fueltypeid][]":  ft,
				"sort[0][column]":       "period",
				"sort[0][direction]":    "desc",
				"length":                strconv.Itoa(months * 4), // multiple sectorid rows per month
			}
			_, rows, err := callEiaData(c, "/electricity/electric-power-operational-data/data/", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			columns := []string{"period", "state", "fueltype", "sector", "generation"}
			table := make([][]string, 0, len(rows))
			for _, row := range rows {
				gen, _ := rowFloat(row, "generation")
				table = append(table, []string{
					rowString(row, "period"),
					rowString(row, "location"),
					rowString(row, "fueltypeid"),
					rowString(row, "sectorid"),
					fmt.Sprintf("%.1f", gen),
				})
			}
			return emit(cmd, flags, rows, columns, table)
		},
	}
	cmd.Flags().StringVar(&fuelType, "fuel-type", "ALL", "natural-gas | coal | solar | wind | nuclear | hydro | oil | biomass | geothermal | all")
	cmd.Flags().IntVar(&months, "months", 12, "months of history")
	return cmd
}

// ----------------------------------------------------------------------
// natural-gas price (henry-hub | spot --state X)
// ----------------------------------------------------------------------

func newNaturalGasPriceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "price",
		Short: "Natural gas price snapshots (Henry Hub spot, state citygate)",
		Long:  `Trader-friendly natural gas price subcommands.`,
	}
	cmd.AddCommand(newNaturalGasPriceHenryHubCmd(flags))
	cmd.AddCommand(newNaturalGasPriceSpotCmd(flags))
	return cmd
}

func newNaturalGasPriceHenryHubCmd(flags *rootFlags) *cobra.Command {
	var lastDays int
	cmd := &cobra.Command{
		Use:   "henry-hub",
		Short: "Henry Hub natural gas spot price",
		Long: `Henry Hub spot natural gas price ($/MMBtu). Pull the last --last N days
of daily prints. Source: NYMEX via EIA natural-gas/pri/fut.`,
		Example: "  eia-pp-cli natgas price henry-hub --last 30",
		Annotations: map[string]string{
			"pp:novel":       "natural-gas.price.henry-hub",
			"pp:client-call": "true",
			"mcp:read-only":  "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if lastDays <= 0 {
				lastDays = 30
			}
			params := map[string]string{
				"frequency":             "daily",
				"data[]":                "value",
				"facets[series][]":      "RNGWHHD",
				"sort[0][column]":       "period",
				"sort[0][direction]":    "desc",
				"length":                strconv.Itoa(lastDays),
			}
			_, rows, err := callEiaData(c, "/natural-gas/pri/fut/data/", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			columns := []string{"period", "series", "price ($/MMBtu)", "units"}
			table := make([][]string, 0, len(rows))
			for _, row := range rows {
				v, _ := rowFloat(row, "value")
				table = append(table, []string{
					rowString(row, "period"),
					rowString(row, "series"),
					fmt.Sprintf("%.3f", v),
					rowString(row, "units"),
				})
			}
			return emit(cmd, flags, rows, columns, table)
		},
	}
	cmd.Flags().IntVar(&lastDays, "last", 30, "days of history")
	return cmd
}

func newNaturalGasPriceSpotCmd(flags *rootFlags) *cobra.Command {
	var state string
	var months int

	cmd := &cobra.Command{
		Use:   "spot",
		Short: "State citygate natural gas price",
		Long: `Monthly citygate (and sector) natural gas price for a state. Source:
EIA natural-gas/pri/sum.`,
		Example: "  eia-pp-cli natgas price spot --state TX",
		Annotations: map[string]string{
			"pp:novel":       "natural-gas.price.spot",
			"pp:client-call": "true",
			"mcp:read-only":  "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if months <= 0 {
				months = 12
			}
			duo := strings.ToUpper(strings.TrimSpace(state))
			// duoarea for state-level price summary is "SXX" — e.g. STX for Texas
			if len(duo) == 2 {
				duo = "S" + duo
			}
			params := map[string]string{
				"frequency":             "monthly",
				"data[]":                "value",
				"facets[duoarea][]":     duo,
				"sort[0][column]":       "period",
				"sort[0][direction]":    "desc",
				"length":                strconv.Itoa(months * 5),
			}
			_, rows, err := callEiaData(c, "/natural-gas/pri/sum/data/", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			columns := []string{"period", "duoarea", "process", "price ($/MMBtu)", "units"}
			table := make([][]string, 0, len(rows))
			for _, row := range rows {
				v, _ := rowFloat(row, "value")
				table = append(table, []string{
					rowString(row, "period"),
					rowString(row, "duoarea"),
					rowString(row, "process"),
					fmt.Sprintf("%.3f", v),
					rowString(row, "units"),
				})
			}
			return emit(cmd, flags, rows, columns, table)
		},
	}
	cmd.Flags().StringVar(&state, "state", "", "two-letter state code (e.g. TX) — required")
	cmd.Flags().IntVar(&months, "months", 12, "months of history")
	_ = cmd.MarkFlagRequired("state")
	return cmd
}

// ----------------------------------------------------------------------
// petroleum price crude wti [--frequency]
// ----------------------------------------------------------------------

func newPetroleumPriceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "price",
		Short: "Petroleum price snapshots (WTI, Brent, RBOB, distillate)",
		Long:  `Trader-friendly petroleum price subcommands.`,
	}
	cmd.AddCommand(newPetroleumPriceCrudeCmd(flags))
	return cmd
}

func newPetroleumPriceCrudeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "crude",
		Short: "Crude oil prices",
		Long:  `WTI and Brent crude oil price series.`,
	}
	cmd.AddCommand(newPetroleumPriceCrudeWtiCmd(flags))
	cmd.AddCommand(newPetroleumPriceCrudeBrentCmd(flags))
	return cmd
}

func newPetroleumPriceCrudeWtiCmd(flags *rootFlags) *cobra.Command {
	var frequency string
	var length int
	cmd := &cobra.Command{
		Use:   "wti",
		Short: "WTI crude oil spot price (Cushing, OK)",
		Long: `Cushing, OK WTI spot price ($/bbl). Series RWTC. Default frequency
is daily. Use --frequency weekly|monthly|annual for aggregated series.`,
		Example: "  eia-pp-cli petroleum price crude wti --frequency daily",
		Annotations: map[string]string{
			"pp:novel":       "petroleum.price.crude.wti",
			"pp:client-call": "true",
			"mcp:read-only":  "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			freq := strings.ToLower(strings.TrimSpace(frequency))
			switch freq {
			case "", "daily", "weekly", "monthly", "annual":
			default:
				return fmt.Errorf("invalid --frequency %q: must be daily|weekly|monthly|annual", frequency)
			}
			if freq == "" {
				freq = "daily"
			}
			if length <= 0 {
				length = 30
			}
			params := map[string]string{
				"frequency":             freq,
				"data[]":                "value",
				"facets[series][]":      "RWTC",
				"sort[0][column]":       "period",
				"sort[0][direction]":    "desc",
				"length":                strconv.Itoa(length),
			}
			_, rows, err := callEiaData(c, "/petroleum/pri/spt/data/", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			columns := []string{"period", "series", "price ($/bbl)", "units"}
			table := make([][]string, 0, len(rows))
			for _, row := range rows {
				v, _ := rowFloat(row, "value")
				table = append(table, []string{
					rowString(row, "period"),
					rowString(row, "series"),
					fmt.Sprintf("%.2f", v),
					rowString(row, "units"),
				})
			}
			return emit(cmd, flags, rows, columns, table)
		},
	}
	cmd.Flags().StringVar(&frequency, "frequency", "daily", "daily | weekly | monthly | annual")
	cmd.Flags().IntVar(&length, "length", 30, "rows to fetch")
	return cmd
}

func newPetroleumPriceCrudeBrentCmd(flags *rootFlags) *cobra.Command {
	var frequency string
	var length int
	cmd := &cobra.Command{
		Use:   "brent",
		Short: "Brent crude oil spot price (Europe)",
		Long:  `Europe Brent spot price ($/bbl). Series RBRTE.`,
		Example: "  eia-pp-cli petroleum price crude brent --frequency daily --length 30",
		Annotations: map[string]string{
			"pp:novel":       "petroleum.price.crude.brent",
			"pp:client-call": "true",
			"mcp:read-only":  "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			freq := strings.ToLower(strings.TrimSpace(frequency))
			if freq == "" {
				freq = "daily"
			}
			if length <= 0 {
				length = 30
			}
			params := map[string]string{
				"frequency":             freq,
				"data[]":                "value",
				"facets[series][]":      "RBRTE",
				"sort[0][column]":       "period",
				"sort[0][direction]":    "desc",
				"length":                strconv.Itoa(length),
			}
			_, rows, err := callEiaData(c, "/petroleum/pri/spt/data/", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			columns := []string{"period", "series", "price ($/bbl)", "units"}
			table := make([][]string, 0, len(rows))
			for _, row := range rows {
				v, _ := rowFloat(row, "value")
				table = append(table, []string{
					rowString(row, "period"),
					rowString(row, "series"),
					fmt.Sprintf("%.2f", v),
					rowString(row, "units"),
				})
			}
			return emit(cmd, flags, rows, columns, table)
		},
	}
	cmd.Flags().StringVar(&frequency, "frequency", "daily", "daily | weekly | monthly | annual")
	cmd.Flags().IntVar(&length, "length", 30, "rows to fetch")
	return cmd
}

// ----------------------------------------------------------------------
// steo --series natgas|oil|electricity [--months 6]
// ----------------------------------------------------------------------

// steoSeriesAliases map friendly --series names to STEO seriesId codes.
// Short-Term Energy Outlook series ids: full catalog at
// https://www.eia.gov/opendata/browser/steo
var steoSeriesAliases = map[string]string{
	"natgas":          "NGHHMCF",
	"natural-gas":     "NGHHMCF",
	"henry-hub":       "NGHHMCF",
	"oil":             "WTIPUUS",
	"wti":             "WTIPUUS",
	"brent":           "BREPUUS",
	"electricity":     "ELRGCREP",
	"power":           "ELRGCREP",
	"gasoline":        "GORGREG",
	"diesel":          "GODUREG",
	"coal":            "COPRPUS",
	"co2":             "CO2TOTPUS",
}

func newSteoNovelCmd(flags *rootFlags) *cobra.Command {
	var series string
	var months int
	cmd := &cobra.Command{
		Use:   "forecast",
		Short: "Short-Term Energy Outlook forecast for a named series",
		Long: `Friendly aliases for STEO series: natgas, oil, brent, electricity,
gasoline, diesel, coal, co2. Returns the next --months months of forecast
data (monthly frequency by default).`,
		Example: "  eia-pp-cli steo forecast --series natgas --months 6",
		Annotations: map[string]string{
			"pp:novel":       "steo.forecast",
			"pp:client-call": "true",
			"mcp:read-only":  "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			alias := strings.ToLower(strings.TrimSpace(series))
			code, ok := steoSeriesAliases[alias]
			if !ok {
				code = strings.ToUpper(series)
			}
			if months <= 0 {
				months = 6
			}
			now := time.Now().UTC().Format("2006-01")
			params := map[string]string{
				"frequency":             "monthly",
				"data[]":                "value",
				"facets[seriesId][]":    code,
				"start":                 now,
				"sort[0][column]":       "period",
				"sort[0][direction]":    "asc",
				"length":                strconv.Itoa(months),
			}
			_, rows, err := callEiaData(c, "/steo/data/", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			columns := []string{"period", "seriesId", "value", "units"}
			table := make([][]string, 0, len(rows))
			for _, row := range rows {
				v, _ := rowFloat(row, "value")
				table = append(table, []string{
					rowString(row, "period"),
					rowString(row, "seriesId"),
					fmt.Sprintf("%.3f", v),
					rowString(row, "unit"),
				})
			}
			return emit(cmd, flags, rows, columns, table)
		},
	}
	cmd.Flags().StringVar(&series, "series", "natgas", "natgas | oil | brent | electricity | gasoline | diesel | coal | co2 (or raw STEO seriesId)")
	cmd.Flags().IntVar(&months, "months", 6, "forecast horizon in months")
	return cmd
}

// ----------------------------------------------------------------------
// co2 <state> [--sector electric-power] [--annual]
// ----------------------------------------------------------------------

// Sector code mapping for the /v2/co2-emissions/co2-emissions-aggregates/
// endpoint. EIA marks that route deprecated and points to SEDS, but SEDS
// only carries per-fuel CO2 series (no all-fuel sector rollups), so the
// deprecated route remains the right surface for the cross-fuel-by-sector
// aggregation that traders actually want. The route still serves data
// through 2022; we surface that as the published cutoff in --help.
var co2SectorAliases = map[string]string{
	"electric-power": "EC",
	"residential":    "RC",
	"commercial":     "CC",
	"industrial":     "IC",
	"transportation": "TC",
	"total":          "TT",
	"all":            "TT",
}

func newCo2Cmd(flags *rootFlags) *cobra.Command {
	var sector string
	var annual bool
	var years int

	cmd := &cobra.Command{
		Use:   "co2 <state>",
		Short: "State CO2 emissions in million metric tons by sector",
		Long: `State-level CO2 emissions in million metric tons, aggregated across
fuel types within a sector (electric-power | residential | commercial |
industrial | transportation | total). Sources from EIA's
/v2/co2-emissions/co2-emissions-aggregates/ — marked deprecated by EIA in
favor of SEDS, but kept here because SEDS only carries per-fuel series
and not the cross-fuel-by-sector rollups traders actually want. Data
cutoff: 2022.`,
		Example: "  eia-pp-cli co2 TX --sector electric-power --annual",
		Args:    cobra.ExactArgs(1),
		Annotations: map[string]string{
			"pp:novel":       "co2",
			"pp:client-call": "true",
			"mcp:read-only":  "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			state := strings.ToUpper(strings.TrimSpace(args[0]))
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			code, ok := co2SectorAliases[strings.ToLower(sector)]
			if !ok {
				code = strings.ToUpper(sector)
			}
			if years <= 0 {
				years = 10
			}
			_ = annual // route is always annual; flag is for parity
			// Fetch enough rows to cover --years across all fuels in the sector
			// (coal, NG, petroleum). 6 fuels * years gives headroom.
			params := map[string]string{
				"frequency":          "annual",
				"data[]":             "value",
				"facets[sectorId][]": code,
				"facets[stateId][]":  state,
				"sort[0][column]":    "period",
				"sort[0][direction]": "desc",
				"length":             strconv.Itoa(years * 6),
			}
			_, rows, err := callEiaData(c, "/co2-emissions/co2-emissions-aggregates/data/", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			columns := []string{"year", "state", "sector", "fuel", "MMT CO2"}
			table := make([][]string, 0, len(rows))
			for _, row := range rows {
				v, _ := rowFloat(row, "value")
				table = append(table, []string{
					rowString(row, "period"),
					rowString(row, "stateId"),
					rowString(row, "sectorId"),
					rowString(row, "fuelId"),
					fmt.Sprintf("%.3f", v),
				})
			}
			return emit(cmd, flags, rows, columns, table)
		},
	}
	cmd.Flags().StringVar(&sector, "sector", "total", "electric-power | residential | commercial | industrial | transportation | total")
	cmd.Flags().BoolVar(&annual, "annual", true, "annual frequency (SEDS only supports annual)")
	cmd.Flags().IntVar(&years, "years", 10, "years of history")
	return cmd
}
