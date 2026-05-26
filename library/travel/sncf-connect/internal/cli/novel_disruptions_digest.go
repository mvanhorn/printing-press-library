// Copyright 2026 jmbernabotto. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newDisruptionsDigestCmd(flags *rootFlags) *cobra.Command {
	var coverage, lines string
	var all bool

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Morning brief: all disruptions ranked by severity",
		Long: `Fetches disruptions for a coverage region and ranks them by severity:
blocked > delayed > informational.

Optionally filter to specific line IDs with --lines.
By default shows only active disruptions; use --all to include future and past.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  sncf-connect-pp-cli disruptions digest
  sncf-connect-pp-cli disruptions digest --coverage sncf --lines "line:OCE:TGV"
  sncf-connect-pp-cli disruptions digest --all --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := fmt.Sprintf("/coverage/%s/disruptions", coverage)
			params := map[string]string{
				"count": "100",
			}

			data, _, err := resolveRead(cmd.Context(), c, flags, "disruptions", true, path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			allDisruptions := navitiaItems(data, "disruptions")

			// Filter by line IDs if requested
			if lines != "" {
				lineSet := make(map[string]bool)
				for _, l := range strings.Split(lines, ",") {
					lineSet[strings.TrimSpace(l)] = true
				}
				filtered := allDisruptions[:0]
				for _, d := range allDisruptions {
					if disruptionMatchesLines(d, lineSet) {
						filtered = append(filtered, d)
					}
				}
				allDisruptions = filtered
			}

			// Filter to active only (unless --all)
			if !all {
				active := allDisruptions[:0]
				for _, d := range allDisruptions {
					if status, _ := d["status"].(string); status == "active" {
						active = append(active, d)
					}
				}
				allDisruptions = active
			}

			// Rank by severity
			ranked := rankDisruptionsBySeverity(allDisruptions)

			if flags.asJSON {
				type digestEntry struct {
					Severity string         `json:"severity"`
					ID       string         `json:"id"`
					Status   string         `json:"status"`
					Cause    string         `json:"cause"`
					Message  string         `json:"message"`
					Lines    []string       `json:"lines,omitempty"`
					Raw      map[string]any `json:"raw,omitempty"`
				}
				var entries []digestEntry
				for _, d := range ranked {
					e := digestEntry{
						Severity: disruptionSeverity(d),
						ID:       getString(d, "id"),
						Status:   getString(d, "status"),
						Cause:    getString(d, "cause"),
						Message:  extractDisruptionMessage(d),
						Lines:    disruptionLineNames(d),
					}
					entries = append(entries, e)
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"coverage":   coverage,
					"total":      len(entries),
					"disruptions": entries,
				})
			}

			if len(ranked) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No %sdisruptions on %s.\n",
					map[bool]string{false: "active ", true: ""}[all], coverage)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Disruption digest — %s (%d total)\n\n", coverage, len(ranked))

			currentSev := ""
			for _, d := range ranked {
				sev := disruptionSeverity(d)
				if sev != currentSev {
					currentSev = sev
					fmt.Fprintf(cmd.OutOrStdout(), "[%s]\n", strings.ToUpper(sev))
				}
				msg := extractDisruptionMessage(d)
				lineNames := disruptionLineNames(d)
				linesStr := ""
				if len(lineNames) > 0 {
					linesStr = " [" + strings.Join(lineNames, ", ") + "]"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  • %s%s\n", msg, linesStr)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&coverage, "coverage", "sncf", "Navitia coverage region")
	cmd.Flags().StringVar(&lines, "lines", "", "Comma-separated line URIs to filter (e.g. line:OCE:TGV)")
	cmd.Flags().BoolVar(&all, "all", false, "Include future and past disruptions, not just active")
	return cmd
}

func severityRank(sev string) int {
	switch sev {
	case "blocking":
		return 0
	case "delayed":
		return 1
	default:
		return 2
	}
}

func disruptionSeverity(d map[string]any) string {
	if sev, ok := d["severity"].(map[string]any); ok {
		if effect, ok := sev["effect"].(string); ok {
			switch strings.ToUpper(effect) {
			case "NO_SERVICE", "SIGNIFICANT_DELAYS":
				return "blocking"
			case "REDUCED_SERVICE":
				return "delayed"
			}
		}
		if name, ok := sev["name"].(string); ok {
			lower := strings.ToLower(name)
			if strings.Contains(lower, "block") || strings.Contains(lower, "supprim") {
				return "blocking"
			}
			if strings.Contains(lower, "retard") || strings.Contains(lower, "delay") {
				return "delayed"
			}
		}
	}
	return "info"
}

func rankDisruptionsBySeverity(disruptions []map[string]any) []map[string]any {
	out := make([]map[string]any, len(disruptions))
	copy(out, disruptions)
	for i := 0; i < len(out)-1; i++ {
		for j := i + 1; j < len(out); j++ {
			if severityRank(disruptionSeverity(out[i])) > severityRank(disruptionSeverity(out[j])) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func disruptionLineNames(d map[string]any) []string {
	var names []string
	impacted, _ := d["impacted_objects"].([]any)
	for _, obj := range impacted {
		o, _ := obj.(map[string]any)
		pt, _ := o["pt_object"].(map[string]any)
		if pt == nil {
			continue
		}
		if line, ok := pt["line"].(map[string]any); ok {
			if name, ok := line["name"].(string); ok {
				names = append(names, name)
			} else if code, ok := line["code"].(string); ok {
				names = append(names, code)
			}
		}
	}
	return names
}

func disruptionMatchesLines(d map[string]any, lineSet map[string]bool) bool {
	impacted, _ := d["impacted_objects"].([]any)
	for _, obj := range impacted {
		o, _ := obj.(map[string]any)
		pt, _ := o["pt_object"].(map[string]any)
		if pt == nil {
			continue
		}
		if line, ok := pt["line"].(map[string]any); ok {
			if id, ok := line["id"].(string); ok && lineSet[id] {
				return true
			}
		}
	}
	return false
}

func getString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}
