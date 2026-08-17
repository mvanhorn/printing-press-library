// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Cost journaling: every live Exa response that carries
// costDollars is appended to a local JSONL journal by the HTTP client, and
// `spend` aggregates that journal into a per-day, per-resource report.
// pp:data-source local

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/exa/internal/client"
)

// costEntry is one journaled API call. The client writes these; spend reads them.
type costEntry struct {
	TS        string  `json:"ts"`
	Method    string  `json:"method"`
	Path      string  `json:"path"`
	RequestID string  `json:"requestId,omitempty"`
	Cost      float64 `json:"cost"`
}

// resourceFromPath maps an API path to the logical resource a user would
// group spend by. Unknown paths fall back to a cleaned path segment.
func resourceFromPath(p string) string {
	p = strings.TrimPrefix(p, "/")
	switch {
	case strings.HasPrefix(p, "search"):
		return "searches"
	case strings.HasPrefix(p, "contents"):
		return "contents"
	case strings.HasPrefix(p, "answer"):
		return "answers"
	case strings.HasPrefix(p, "findSimilar"):
		return "find-similar"
	case strings.HasPrefix(p, "monitors") || strings.HasPrefix(p, "v0/monitors"):
		return "monitors"
	case strings.HasPrefix(p, "agent/runs"):
		return "agents"
	case strings.HasPrefix(p, "v0/websets") || strings.HasPrefix(p, "websets"):
		return "websets"
	case strings.HasPrefix(p, "v0/webhooks"):
		return "webhooks"
	case strings.HasPrefix(p, "v0/imports"):
		return "imports"
	case strings.HasPrefix(p, "v0/events"):
		return "events"
	default:
		seg := strings.Split(p, "/")[0]
		if seg == "" {
			return "other"
		}
		return seg
	}
}

func newNovelSpendCmd(flags *rootFlags) *cobra.Command {
	var flagDays string
	var flagResource string

	cmd := &cobra.Command{
		Use:   "spend",
		Short: "See cumulative API spend across every Exa call, broken down by day and resource.",
		Long: `Use this command to understand cumulative spend across every Exa call.
Do NOT use it for counts or groupings of synced records; use 'analytics' instead.`,
		Example:     "  exa-pp-cli spend --days 30 --resource searches",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "spend")
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			days := 30
			if flagDays != "" {
				d, err := strconv.Atoi(flagDays)
				if err != nil || d < 1 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--days must be a positive integer, got %q", flagDays))
				}
				days = d
			}
			resource := ""
			if flagResource != "" {
				resource = strings.ToLower(flagResource)
			}

			journal := client.CostJournalPath()
			// #nosec G304 -- journal path is derived from cliutil.DataDir(),
			// not user input; there is no untrusted path here.
			f, err := os.Open(journal)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no cost journal found at %s\nrun any live exa-pp-cli command (search, contents, answer, ...) to record spend\n", journal)
					if flags.asJSON || flags.agent {
						_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{
							"days": days, "resource": resource, "entries": []any{},
							"totalCost": 0, "totalCostUsd": "$0.0000", "journal": journal,
							"note":   "no cost journal yet; run any live exa-pp-cli command to record spend",
							"source": "local",
						}, flags)
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), "No spend recorded yet.")
					}
					return nil
				}
				return fmt.Errorf("opening cost journal: %w", err)
			}
			defer f.Close()

			cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)

			type dayResource struct {
				day      string
				resource string
			}
			byKey := map[dayResource]float64{}
			var lines, skipped int
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				line := strings.TrimSpace(sc.Text())
				if line == "" {
					continue
				}
				var e costEntry
				if err := json.Unmarshal([]byte(line), &e); err != nil {
					skipped++
					continue
				}
				ts, err := time.Parse(time.RFC3339, e.TS)
				if err != nil {
					skipped++
					continue
				}
				if ts.Before(cutoff) {
					continue
				}
				res := resourceFromPath(e.Path)
				if resource != "" && res != resource {
					continue
				}
				byKey[dayResource{day: ts.Format("2006-01-02"), resource: res}] += e.Cost
				lines++
			}
			if err := sc.Err(); err != nil {
				return fmt.Errorf("reading cost journal: %w", err)
			}
			if skipped > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d malformed journal line(s) skipped in %s\n", skipped, journal)
			}
			if lines == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no cost entries in the last %d days\n", days)
				return nil
			}

			type row struct {
				Day      string  `json:"day"`
				Resource string  `json:"resource"`
				Cost     float64 `json:"cost"`
				CostUSD  string  `json:"costUsd,omitempty"`
			}
			rows := make([]row, 0, len(byKey))
			total := 0.0
			for k, v := range byKey {
				rows = append(rows, row{Day: k.day, Resource: k.resource, Cost: v})
				total += v
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Day != rows[j].Day {
					return rows[i].Day < rows[j].Day
				}
				return rows[i].Resource < rows[j].Resource
			})

			if flags.asJSON || flags.agent {
				view := struct {
					Days     int     `json:"days"`
					Resource string  `json:"resource,omitempty"`
					Entries  []row   `json:"entries"`
					Total    float64 `json:"totalCost"`
					TotalUSD string  `json:"totalCostUsd"`
					Journal  string  `json:"journal"`
					Note     string  `json:"note,omitempty"`
					Skipped  int     `json:"skippedLines,omitempty"`
					Source   string  `json:"source"`
				}{
					Days:     days,
					Resource: resource,
					Entries:  rows,
					Total:    total,
					TotalUSD: fmt.Sprintf("$%.4f", total),
					Journal:  journal,
					Note:     "cost journal records costDollars.total from every live API response; cached and dry-run calls are not billed",
					Skipped:  skipped,
					Source:   "local",
				}
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			// Human table
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-16s %10s\n", "DAY", "RESOURCE", "COST (USD)")
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-16s %10.4f\n", r.Day, r.Resource, r.Cost)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-16s %10s\n", "TOTAL", fmt.Sprintf("%d days", days), fmt.Sprintf("%.4f", total))
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDays, "days", "30", "Number of days of cost history to aggregate")
	cmd.Flags().StringVar(&flagResource, "resource", "", "Only show spend for one resource (searches, contents, answers, find-similar, monitors, agents, websets, webhooks, imports, events)")
	return cmd
}
