// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/eia-energy/internal/config"
	"github.com/spf13/cobra"
)

type eiaPage struct {
	Total    string
	Rows     []map[string]any
	Warnings []any
}

func eiaFetch(ctx context.Context, flags *rootFlags, route string, params url.Values) (eiaPage, error) {
	route = strings.Trim(route, "/")
	if route == "" {
		return eiaPage{}, errors.New("--route is required")
	}
	if params == nil {
		params = url.Values{}
	}
	cloned := make(url.Values, len(params))
	for key, values := range params {
		cloned[key] = append([]string(nil), values...)
	}
	params = cloned
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return eiaPage{}, err
	}
	key := strings.TrimSpace(cfg.AuthHeader())
	if key == "" {
		key = "DEMO_KEY"
	}
	params.Set("api_key", key)
	c, err := flags.newClient()
	if err != nil {
		return eiaPage{}, err
	}
	raw, err := c.GetWithHeadersNoCacheValues(ctx, "/"+route+"/data/", params, nil)
	if err != nil {
		return eiaPage{}, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var envelope map[string]any
	if err := decoder.Decode(&envelope); err != nil {
		return eiaPage{}, err
	}
	response, ok := envelope["response"].(map[string]any)
	if !ok {
		return eiaPage{}, fmt.Errorf("EIA response missing response object: %v", envelope["error"])
	}
	page := eiaPage{Total: fmt.Sprint(response["total"])}
	if raw, ok := response["data"].([]any); ok {
		for _, item := range raw {
			if row, ok := item.(map[string]any); ok {
				page.Rows = append(page.Rows, row)
			}
		}
	}
	if warnings, ok := envelope["warnings"].([]any); ok {
		page.Warnings = warnings
	}
	return page, nil
}

func pageIsTruncated(page eiaPage) bool {
	total, err := strconv.Atoi(strings.TrimSpace(page.Total))
	return err == nil && total > len(page.Rows)
}

func consistentSeriesUnit(rows []map[string]any, data string) (string, error) {
	unit := ""
	for _, row := range rows {
		current := rowUnit(row, data)
		if current == "" {
			return "", fmt.Errorf("EIA did not report a unit for data column %q at period %v", data, row["period"])
		}
		if unit == "" {
			unit = current
		} else if current != unit {
			return "", fmt.Errorf("unit changes within series at period %v: %q vs %q", row["period"], current, unit)
		}
	}
	return unit, nil
}
func seriesParams(data, frequency, start, end string, facets map[string][]string, length int) url.Values {
	v := url.Values{"frequency": {frequency}, "data[0]": {data}, "length": {strconv.Itoa(length)}, "sort[0][column]": {"period"}, "sort[0][direction]": {"asc"}}
	if start != "" {
		v.Set("start", start)
	}
	if end != "" {
		v.Set("end", end)
	}
	for name, values := range facets {
		for _, value := range values {
			v.Add("facets["+name+"][]", value)
		}
	}
	return v
}
func parseFacet(raw string) (string, string, error) {
	parts := strings.SplitN(raw, "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("facet %q must be NAME=VALUE", raw)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}
func rowValue(row map[string]any, data string) (float64, bool) {
	value, err := strconv.ParseFloat(fmt.Sprint(row[data]), 64)
	return value, err == nil
}
func rowUnit(row map[string]any, data string) string {
	for _, key := range []string{data + "-units", "unit", "units"} {
		if value := strings.TrimSpace(fmt.Sprint(row[key])); value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}
func seriesStats(rows []map[string]any, data string) map[string]any {
	var values []float64
	for _, row := range rows {
		if value, ok := rowValue(row, data); ok {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return map[string]any{"count": 0}
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	variance := 0.0
	for _, v := range values {
		variance += (v - mean) * (v - mean)
	}
	return map[string]any{"count": len(values), "mean": mean, "standard_deviation": math.Sqrt(variance / float64(len(values))), "minimum": minFloat(values), "maximum": maxFloat(values)}
}
func minFloat(v []float64) float64 {
	m := v[0]
	for _, x := range v[1:] {
		if x < m {
			m = x
		}
	}
	return m
}
func maxFloat(v []float64) float64 {
	m := v[0]
	for _, x := range v[1:] {
		if x > m {
			m = x
		}
	}
	return m
}
func mapPeriods(rows []map[string]any, data string) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, row := range rows {
		out[fmt.Sprint(row["period"])] = row
	}
	return out
}
func sortedStringKeys(values map[string]map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func emitEIA(cmd *cobra.Command, flags *rootFlags, source string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, map[string]any{"source": source, "provider": "US EIA API v2", "retrieved_at": time.Now().UTC().Format(time.RFC3339)})
}
func eiaCaveats() []string {
	return []string{"All arithmetic retains EIA-reported units and frequency; incompatible units are never silently converted.", "Results are bounded and expose total rows so truncation remains visible.", "Anomalies and changes are descriptive source calculations, not forecasts or causal explanations."}
}
