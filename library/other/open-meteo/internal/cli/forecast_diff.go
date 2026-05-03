package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/open-meteo/internal/openmeteo"
)

func newForecastDiffCmd(flags *rootFlags) *cobra.Command {
	var (
		place     string
		latitude  string
		longitude string
		hourly    string
		daily     string
		current   string
		threshold float64
	)
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Diff the latest forecast against the previously cached forecast",
		Long: strings.TrimSpace(`
Pull the current forecast for a place and compare it against the snapshot saved
on the last invocation. Returns the changed hours/days for each variable that
moved by more than --threshold.

The first invocation for a place stores a baseline and reports "no prior
snapshot." Subsequent runs report deltas and update the baseline.
`),
		Example: strings.Trim(`
  open-meteo-pp-cli forecast diff --place Seattle --hourly temperature_2m,precipitation --json
  open-meteo-pp-cli forecast diff --place Berlin --daily temperature_2m_max,precipitation_sum --threshold 0.5 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			places, err := openmeteo.CoordsFromFlags(c, place, latitude, longitude)
			if err != nil {
				return err
			}
			p := places[0]
			if hourly == "" && daily == "" && current == "" {
				hourly = "temperature_2m,precipitation,weather_code"
			}
			params := map[string]string{
				"latitude":  strconv.FormatFloat(p.Latitude, 'f', -1, 64),
				"longitude": strconv.FormatFloat(p.Longitude, 'f', -1, 64),
				"timezone":  "auto",
			}
			if hourly != "" {
				params["hourly"] = hourly
			}
			if daily != "" {
				params["daily"] = daily
			}
			if current != "" {
				params["current"] = current
			}
			raw, err := c.Get("https://api.open-meteo.com/v1/forecast", params)
			if err != nil {
				return fmt.Errorf("fetching forecast: %w", err)
			}
			prev, _ := openmeteo.LoadSnapshot("forecast", p)
			paramsKey := hourly + "|" + daily + "|" + current
			view := map[string]any{
				"place":      p.Name,
				"latitude":   p.Latitude,
				"longitude":  p.Longitude,
				"fetched_at": time.Now().UTC().Format(time.RFC3339),
				"threshold":  threshold,
			}
			if prev == nil {
				view["status"] = "no_prior_snapshot"
				view["note"] = "Baseline saved. Re-run later to see what changed."
			} else if prev.ParamsKey != paramsKey {
				view["status"] = "params_changed"
				view["note"] = fmt.Sprintf("Snapshot params (%q) differ from current (%q); baseline rebuilt.", prev.ParamsKey, paramsKey)
			} else {
				view["status"] = "diffed"
				view["previous_stored_at"] = prev.StoredAt.Format(time.RFC3339)
				view["changes"] = diffForecastPayloads(prev.Payload, raw, threshold)
			}
			if err := openmeteo.SaveSnapshot("forecast", p, paramsKey, raw); err != nil {
				return fmt.Errorf("saving snapshot: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&place, "place", "", "Place name (e.g., \"Seattle\").")
	cmd.Flags().StringVar(&latitude, "latitude", "", "WGS84 latitude (alternative to --place).")
	cmd.Flags().StringVar(&longitude, "longitude", "", "WGS84 longitude (alternative to --place).")
	cmd.Flags().StringVar(&hourly, "hourly", "", "Comma-separated hourly variables (defaults to temperature_2m,precipitation,weather_code).")
	cmd.Flags().StringVar(&daily, "daily", "", "Comma-separated daily variables.")
	cmd.Flags().StringVar(&current, "current", "", "Comma-separated current variables.")
	cmd.Flags().Float64Var(&threshold, "threshold", 0.0, "Minimum absolute change to report. 0 reports any change.")
	return cmd
}

// diffForecastPayloads compares the variable arrays of two Open-Meteo forecast
// payloads and returns a slice of {variable, time, prev, curr, delta} entries
// for indices where the absolute change is >= threshold. Both hourly and daily
// blocks are inspected.
func diffForecastPayloads(prevRaw, currRaw json.RawMessage, threshold float64) []map[string]any {
	type bucket struct {
		Time map[string][]string              `json:"-"`
		Vars map[string]map[string][]*float64 `json:"-"`
	}
	parse := func(raw json.RawMessage) (map[string]map[string]json.RawMessage, error) {
		var p map[string]json.RawMessage
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		out := make(map[string]map[string]json.RawMessage)
		for _, key := range []string{"hourly", "daily", "current"} {
			if val, ok := p[key]; ok {
				var sub map[string]json.RawMessage
				if json.Unmarshal(val, &sub) == nil {
					out[key] = sub
				}
			}
		}
		return out, nil
	}
	prev, err := parse(prevRaw)
	if err != nil {
		return nil
	}
	curr, err := parse(currRaw)
	if err != nil {
		return nil
	}
	var changes []map[string]any
	for _, block := range []string{"hourly", "daily"} {
		prevBlock := prev[block]
		currBlock := curr[block]
		if prevBlock == nil || currBlock == nil {
			continue
		}
		prevTimes := decodeStringSlice(prevBlock["time"])
		currTimes := decodeStringSlice(currBlock["time"])
		// Index curr by time so deletions/insertions don't break alignment.
		currIdx := make(map[string]int, len(currTimes))
		for i, t := range currTimes {
			currIdx[t] = i
		}
		for varName := range currBlock {
			if varName == "time" {
				continue
			}
			prevVals := decodeFloatSlice(prevBlock[varName])
			currVals := decodeFloatSlice(currBlock[varName])
			for i, ts := range prevTimes {
				if i >= len(prevVals) || prevVals[i] == nil {
					continue
				}
				j, ok := currIdx[ts]
				if !ok || j >= len(currVals) || currVals[j] == nil {
					continue
				}
				delta := *currVals[j] - *prevVals[i]
				if abs(delta) < threshold {
					continue
				}
				changes = append(changes, map[string]any{
					"block":    block,
					"variable": varName,
					"time":     ts,
					"prev":     *prevVals[i],
					"curr":     *currVals[j],
					"delta":    delta,
				})
			}
		}
	}
	if curBlock := curr["current"]; curBlock != nil {
		if prBlock := prev["current"]; prBlock != nil {
			for varName, raw := range curBlock {
				if varName == "time" || varName == "interval" {
					continue
				}
				var c float64
				if json.Unmarshal(raw, &c) != nil {
					continue
				}
				prevRawVal, ok := prBlock[varName]
				if !ok {
					continue
				}
				var prevVal float64
				if json.Unmarshal(prevRawVal, &prevVal) != nil {
					continue
				}
				delta := c - prevVal
				if abs(delta) < threshold {
					continue
				}
				changes = append(changes, map[string]any{
					"block":    "current",
					"variable": varName,
					"prev":     prevVal,
					"curr":     c,
					"delta":    delta,
				})
			}
		}
	}
	return changes
}

func decodeFloatSlice(raw json.RawMessage) []*float64 {
	if len(raw) == 0 {
		return nil
	}
	var out []*float64
	_ = json.Unmarshal(raw, &out)
	return out
}

func decodeStringSlice(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var out []string
	_ = json.Unmarshal(raw, &out)
	return out
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
