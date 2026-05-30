// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelAlertsImpactCmd(flags *rootFlags) *cobra.Command {
	var flagLines string
	var flagStops string

	cmd := &cobra.Command{
		Use:   "impact",
		Short: "Filter active alerts to only those affecting specific lines or stops",
		Example: strings.Trim(`
  # Show alerts affecting tram line A
  infotbm-pp-cli alerts impact --lines A

  # Show alerts affecting multiple lines
  infotbm-pp-cli alerts impact --lines A,B,C

  # Show alerts affecting a specific stop
  infotbm-pp-cli alerts impact --stops stop_point:BP:SA:3136

  # Show alerts affecting line A or specific stops
  infotbm-pp-cli alerts impact --lines A --stops stop_point:BP:SA:3136,stop_point:BP:SA:3120
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch active alerts and filter by specified lines/stops")
				return nil
			}
			if flagLines == "" && flagStops == "" {
				return usageErr(fmt.Errorf("at least one of --lines or --stops is required"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fetch active alerts
			data, err := c.Get(cmd.Context(), "/alerts/active/bordeaux", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Parse the alerts response — SIRI alerts come in various envelope shapes
			var rawAlerts []json.RawMessage
			if json.Unmarshal(data, &rawAlerts) != nil {
				// Try unwrapping common envelopes
				rawAlerts = extractSearchResults(data)
			}

			// Build filter sets
			lineFilter := make(map[string]bool)
			if flagLines != "" {
				for _, l := range strings.Split(flagLines, ",") {
					l = strings.TrimSpace(l)
					if l != "" {
						lineFilter[strings.ToUpper(l)] = true
					}
				}
			}
			stopFilter := make(map[string]bool)
			if flagStops != "" {
				for _, s := range strings.Split(flagStops, ",") {
					s = strings.TrimSpace(s)
					if s != "" {
						stopFilter[s] = true
					}
				}
			}

			// Filter alerts by matching affected lines or stops
			type alertView struct {
				ID             string   `json:"id,omitempty"`
				Title          string   `json:"title,omitempty"`
				Description    string   `json:"description,omitempty"`
				Severity       string   `json:"severity,omitempty"`
				AffectedLines  []string `json:"affected_lines"`
				AffectedStops  []string `json:"affected_stops"`
				ValidityStart  string   `json:"validity_start,omitempty"`
				ValidityEnd    string   `json:"validity_end,omitempty"`
				LastUpdateTime string   `json:"last_update_time,omitempty"`
			}

			results := make([]alertView, 0)
			for _, rawAlert := range rawAlerts {
				var alert map[string]any
				if json.Unmarshal(rawAlert, &alert) != nil {
					continue
				}

				// Extract affected lines and stops from alert structure
				affectedLines := make([]string, 0)
				affectedStops := make([]string, 0)

				// Check various SIRI/GTFS-RT alert structures for affected entities
				extractRefs(alert, "AffectedLineRef", &affectedLines)
				extractRefs(alert, "LineRef", &affectedLines)
				extractRefs(alert, "affectedLines", &affectedLines)
				extractRefs(alert, "lines", &affectedLines)
				extractRefs(alert, "AffectedStopPointRef", &affectedStops)
				extractRefs(alert, "StopPointRef", &affectedStops)
				extractRefs(alert, "affectedStops", &affectedStops)
				extractRefs(alert, "stops", &affectedStops)

				// Check nested AffectedEntity arrays
				if entities, ok := alert["AffectedEntity"].([]any); ok {
					for _, e := range entities {
						if em, ok := e.(map[string]any); ok {
							if lr, ok := em["LineRef"].(string); ok {
								affectedLines = append(affectedLines, lr)
							}
							if sp, ok := em["StopPointRef"].(string); ok {
								affectedStops = append(affectedStops, sp)
							}
						}
					}
				}

				// Check if this alert matches any of our filters
				match := false
				if len(lineFilter) > 0 {
					for _, l := range affectedLines {
						// Match by short name (e.g., "A") or full ref
						shortName := strings.ToUpper(l)
						// Extract short name from refs like "line:BM:A"
						if parts := strings.Split(l, ":"); len(parts) > 0 {
							short := parts[len(parts)-1]
							if len(parts) >= 4 {
								short = parts[len(parts)-2]
							}
							shortName = strings.ToUpper(short)
						}
						if lineFilter[shortName] || lineFilter[strings.ToUpper(l)] {
							match = true
							break
						}
					}
				}
				if !match && len(stopFilter) > 0 {
					for _, s := range affectedStops {
						if stopFilter[s] {
							match = true
							break
						}
					}
				}
				if !match {
					continue
				}

				view := alertView{
					AffectedLines: affectedLines,
					AffectedStops: affectedStops,
				}
				if v, ok := alert["id"].(string); ok {
					view.ID = v
				} else if v, ok := alert["SituationNumber"].(string); ok {
					view.ID = v
				}
				if v, ok := alert["title"].(string); ok {
					view.Title = v
				} else if v, ok := alert["Summary"].(string); ok {
					view.Title = v
				}
				if v, ok := alert["description"].(string); ok {
					view.Description = v
				} else if v, ok := alert["Description"].(string); ok {
					view.Description = v
				}
				if v, ok := alert["severity"].(string); ok {
					view.Severity = v
				} else if v, ok := alert["Severity"].(string); ok {
					view.Severity = v
				}
				if v, ok := alert["ValidityPeriod"].(map[string]any); ok {
					if s, ok := v["StartTime"].(string); ok {
						view.ValidityStart = s
					}
					if e, ok := v["EndTime"].(string); ok {
						view.ValidityEnd = e
					}
				}
				if v, ok := alert["validityStart"].(string); ok {
					view.ValidityStart = v
				}
				if v, ok := alert["validityEnd"].(string); ok {
					view.ValidityEnd = v
				}
				if v, ok := alert["lastUpdateTime"].(string); ok {
					view.LastUpdateTime = v
				}
				results = append(results, view)
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&flagLines, "lines", "", "Comma-separated line short names to filter (e.g. A,B,C)")
	cmd.Flags().StringVar(&flagStops, "stops", "", "Comma-separated stop IDs to filter")
	return cmd
}

// extractRefs pulls string references from various alert field shapes (string, []string, []any).
func extractRefs(alert map[string]any, key string, out *[]string) {
	v, ok := alert[key]
	if !ok {
		return
	}
	switch val := v.(type) {
	case string:
		*out = append(*out, val)
	case []any:
		for _, item := range val {
			if s, ok := item.(string); ok {
				*out = append(*out, s)
			}
		}
	}
}
