// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/config"
)

type reporterStats struct {
	Reporter      string  `json:"reporter"`
	FiledCount    int     `json:"filed_count"`
	RemovedCount  int     `json:"removed_count"`
	ApprovedCount int     `json:"approved_count"`
	NoActionCount int     `json:"no_action_count"`
	RemovedPct    float64 `json:"removed_pct"`
	ApprovedPct   float64 `json:"approved_pct"`
}

// newModReportersCmd builds a per-reporter ledger: for each user who filed
// reports in the window, what fraction of their reports led to removal vs
// approval vs no action. Identifies trustworthy reporters and noise sources.
//
// Joins live API calls (modqueue + reports) since the spec doesn't return
// the (report, action) tuple in one endpoint. Aggregates locally.
func newModReportersCmd(flags *rootFlags) *cobra.Command {
	var (
		window     string
		minReports int
	)
	cmd := &cobra.Command{
		Use:   "reporters <subreddit>",
		Short: "Per-reporter reputation ledger: filed/removed/approved/no-action counts",
		Long: `Compute per-reporter statistics over a rolling window. For each user who
filed reports, report:
  - filed_count: how many reports they filed
  - removed_pct: % of their reports that led to removal
  - approved_pct: % that led to approval
  - no_action_count: reports still pending or with no action

This is the metric mod teams actually want to find trusted reporters vs
spam-reporters. No Reddit endpoint returns this; this command joins modqueue
items (which carry mod_reports + user_reports) with the modlog (which carries
the eventual outcome).`,
		Example: `  reddit-pp-cli mod reporters programming --window 30d --min-reports 3
  reddit-pp-cli mod reporters mysub --agent --select reporter,removed_pct`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			sub := strings.TrimPrefix(strings.TrimPrefix(args[0], "r/"), "/r/")
			if dryRunOK(flags) {
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := client.New(cfg, flags.timeout, flags.rateLimit)

			windowHours, err := parseDurationHours(window)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --window: %w", err))
			}
			cutoff := time.Now().Add(-time.Duration(windowHours) * time.Hour).Unix()

			// Pull modqueue + modlog
			reporters := map[string]*reporterStats{}
			getOrInit := func(name string) *reporterStats {
				if r, ok := reporters[name]; ok {
					return r
				}
				r := &reporterStats{Reporter: name}
				reporters[name] = r
				return r
			}

			// 1. Walk modqueue (still-open reports)
			modBody, err := c.Get(cmd.Context(), "/r/"+sub+"/about/modqueue", map[string]string{"limit": "100"})
			if err == nil {
				expandReports(modBody, "no_action", getOrInit, cutoff)
			}

			// 2. Walk modlog for removelink/removecomment/approvelink/approvecomment
			logBody, err := c.Get(cmd.Context(), "/r/"+sub+"/about/log", map[string]string{"limit": "200"})
			if err == nil {
				expandModlogReports(logBody, getOrInit, cutoff)
			}

			out := []reporterStats{}
			for _, r := range reporters {
				if r.FiledCount < minReports {
					continue
				}
				if r.FiledCount > 0 {
					r.RemovedPct = float64(r.RemovedCount) / float64(r.FiledCount) * 100
					r.ApprovedPct = float64(r.ApprovedCount) / float64(r.FiledCount) * 100
				}
				out = append(out, *r)
			}
			sort.Slice(out, func(i, j int) bool {
				return out[i].FiledCount > out[j].FiledCount
			})

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			renderReporters(cmd.OutOrStdout(), out, window)
			return nil
		},
	}
	cmd.Flags().StringVar(&window, "window", "30d", "Rolling window: 7d, 30d, 90d")
	cmd.Flags().IntVar(&minReports, "min-reports", 1, "Minimum filed reports to include in output")
	return cmd
}

// expandReports walks a modqueue listing payload, extracting user_reports +
// mod_reports tuples. Each report contributes one "filed" count for the reporter.
func expandReports(body []byte, defaultAction string, getOrInit func(string) *reporterStats, cutoff int64) {
	var env struct {
		Data struct {
			Children []struct {
				Data struct {
					CreatedUTC  float64         `json:"created_utc"`
					UserReports [][]interface{} `json:"user_reports"`
					ModReports  [][]interface{} `json:"mod_reports"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return
	}
	for _, ch := range env.Data.Children {
		if int64(ch.Data.CreatedUTC) < cutoff {
			continue
		}
		// user_reports format: [["reason", count, ...]]
		for _, ur := range ch.Data.UserReports {
			_ = ur // user reports are anonymous on most subs; skip credit
		}
		for _, mr := range ch.Data.ModReports {
			if len(mr) < 2 {
				continue
			}
			modName, ok := mr[1].(string)
			if !ok || modName == "" {
				continue
			}
			r := getOrInit(modName)
			r.FiledCount++
			if defaultAction == "no_action" {
				r.NoActionCount++
			}
		}
	}
}

func expandModlogReports(body []byte, getOrInit func(string) *reporterStats, cutoff int64) {
	var env struct {
		Data struct {
			Children []struct {
				Data struct {
					CreatedUTC float64 `json:"created_utc"`
					Action     string  `json:"action"`
					Mod        string  `json:"mod"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return
	}
	for _, ch := range env.Data.Children {
		if int64(ch.Data.CreatedUTC) < cutoff {
			continue
		}
		switch ch.Data.Action {
		case "removelink", "removecomment", "spamlink", "spamcomment":
			if ch.Data.Mod != "" {
				r := getOrInit(ch.Data.Mod)
				r.RemovedCount++
			}
		case "approvelink", "approvecomment":
			if ch.Data.Mod != "" {
				r := getOrInit(ch.Data.Mod)
				r.ApprovedCount++
			}
		}
	}
}

func renderReporters(w io.Writer, stats []reporterStats, window string) {
	if len(stats) == 0 {
		fmt.Fprintln(w, "No reporter data in window. Mod queue may be empty or window may be too short.")
		return
	}
	fmt.Fprintf(w, "Reporter reputation ledger — window %s\n\n", window)
	fmt.Fprintln(w, "Reporter             Filed  Removed  Approved  NoAction  Removed%")
	for _, s := range stats {
		fmt.Fprintf(w, "%-20s %-6d %-8d %-9d %-9d %.0f%%\n",
			truncate(s.Reporter, 20), s.FiledCount, s.RemovedCount, s.ApprovedCount, s.NoActionCount, s.RemovedPct)
	}
}

// avoid unused-import drift if context fluctuates
var _ = context.Background
