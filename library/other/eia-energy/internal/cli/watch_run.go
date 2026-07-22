// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through eiaFetch in eia_novel_support.go

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type eiaWatchRule struct {
	Name      string  `json:"name"`
	Route     string  `json:"route"`
	Facet     string  `json:"facet"`
	Data      string  `json:"data"`
	Frequency string  `json:"frequency"`
	Operator  string  `json:"operator"`
	Threshold float64 `json:"threshold"`
}

func newNovelWatchRunCmd(flags *rootFlags) *cobra.Command {
	var rulesFile string
	cmd := &cobra.Command{
		Use:         "run",
		Short:       "Evaluate a bounded JSON rule set once for thresholds, freshness, missing data, and changes while retaining source rows.",
		Example:     "  eia-energy-pp-cli watch run --rules ./energy-watch-rules.json --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			if strings.TrimSpace(rulesFile) == "" {
				return fmt.Errorf("--rules is required")
			}
			// #nosec G304 -- path is an explicit operator-supplied CLI input.
			raw, err := os.ReadFile(rulesFile)
			if err != nil {
				return err
			}
			var rules []eiaWatchRule
			if err := json.Unmarshal(raw, &rules); err != nil {
				return fmt.Errorf("parse --rules: %w", err)
			}
			if len(rules) == 0 || len(rules) > 25 {
				return fmt.Errorf("--rules must contain 1-25 rules")
			}
			end := time.Now().UTC().Truncate(time.Hour)
			start := end.Add(-23 * time.Hour)
			results := make([]map[string]any, 0, len(rules))
			triggered := 0
			for i, rule := range rules {
				if rule.Name == "" {
					rule.Name = fmt.Sprintf("rule-%d", i+1)
				}
				if rule.Route == "" {
					rule.Route = "electricity/rto/region-data"
				}
				if rule.Data == "" {
					rule.Data = "value"
				}
				if rule.Frequency == "" {
					rule.Frequency = "hourly"
				}
				facetName, facetValue, err := parseFacet(rule.Facet)
				if err != nil {
					return fmt.Errorf("rule %q: %w", rule.Name, err)
				}
				params := seriesParams(rule.Data, rule.Frequency, start.Format("2006-01-02T15"), end.Format("2006-01-02T15"), map[string][]string{facetName: {facetValue}}, 100)
				ctx, cancel := boundCtx(cmd.Context(), flags)
				page, fetchErr := eiaFetch(ctx, flags, rule.Route, params)
				cancel()
				if fetchErr != nil {
					return fmt.Errorf("rule %q: %w", rule.Name, fetchErr)
				}
				result := map[string]any{"name": rule.Name, "operator": rule.Operator, "threshold": rule.Threshold, "returned_rows": len(page.Rows), "total": page.Total, "missing": len(page.Rows) == 0}
				if len(page.Rows) > 0 {
					latest := page.Rows[len(page.Rows)-1]
					value, ok := rowValue(latest, rule.Data)
					result["latest_period"] = latest["period"]
					result["latest_value"] = latest[rule.Data]
					result["unit"] = rowUnit(latest, rule.Data)
					result["source_row"] = latest
					if !ok {
						return fmt.Errorf("rule %q latest value is not numeric", rule.Name)
					}
					fired, err := compareThreshold(value, rule.Operator, rule.Threshold)
					if err != nil {
						return fmt.Errorf("rule %q: %w", rule.Name, err)
					}
					result["triggered"] = fired
					if fired {
						triggered++
					}
				}
				results = append(results, result)
			}
			return emitEIA(cmd, flags, "live", map[string]any{"evaluated_at": time.Now().UTC().Format(time.RFC3339), "rule_count": len(rules), "triggered_count": triggered, "results": results, "caveats": eiaCaveats()})
		},
	}
	cmd.Flags().StringVar(&rulesFile, "rules", "", "JSON file containing 1-25 threshold rules")
	return cmd
}

func compareThreshold(value float64, operator string, threshold float64) (bool, error) {
	switch operator {
	case ">":
		return value > threshold, nil
	case ">=":
		return value >= threshold, nil
	case "<":
		return value < threshold, nil
	case "<=":
		return value <= threshold, nil
	default:
		return false, fmt.Errorf("operator must be one of >, >=, <, <=")
	}
}
