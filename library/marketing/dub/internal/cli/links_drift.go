// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

// links_drift compares analytics over two adjacent windows and flags links
// whose click rate moved more than the configured threshold. Pulls per-link
// click counts from the live API, then enriches with the local /store cache
// (store.Open-backed) to attach short-link slugs and tags.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type driftRow struct {
	LinkID    string  `json:"link_id"`
	Domain    string  `json:"domain,omitempty"`
	Key       string  `json:"key,omitempty"`
	ShortLink string  `json:"shortLink,omitempty"`
	Recent    int     `json:"recent_clicks"`
	Prior     int     `json:"prior_clicks"`
	DeltaPct  float64 `json:"delta_pct"`
	Direction string  `json:"direction"`
}

func newLinksDriftCmd(flags *rootFlags) *cobra.Command {
	var window string
	var threshold float64
	var minClicks int

	cmd := &cobra.Command{
		Use:   "drift",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Detect links whose click rate dropped beyond threshold week-over-week",
		Long: `Compare per-link click counts across two adjacent windows and flag links whose
click rate changed beyond --threshold percent. Surfaces silent campaign collapse
before someone notices traffic is gone.

Live API call: queries /analytics?groupBy=top_links twice and computes deltas.`,
		Example: `  # 7-day window, flag links that dropped by 30% or more
  dub-pp-cli links drift --window 7d --threshold 30 --json

  # Tighter signal — ignore links with fewer than 50 baseline clicks
  dub-pp-cli links drift --threshold 25 --min-clicks 50 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			windowDur, err := parseWindow(window)
			if err != nil {
				return usageErr(err)
			}
			recentEnd := now
			recentStart := recentEnd.Add(-windowDur)
			priorEnd := recentStart
			priorStart := priorEnd.Add(-windowDur)

			fetch := func(start, end time.Time) (map[string]int, map[string]map[string]string, error) {
				params := map[string]string{
					"groupBy": "top_links",
					"start":   start.Format(time.RFC3339),
					"end":     end.Format(time.RFC3339),
				}
				resp, err := c.Get("/analytics", params)
				if err != nil {
					return nil, nil, classifyAPIError(err)
				}
				var rows []map[string]any
				if err := json.Unmarshal(resp, &rows); err != nil {
					return nil, nil, fmt.Errorf("parse analytics response: %w", err)
				}
				clicks := make(map[string]int)
				meta := make(map[string]map[string]string)
				for _, r := range rows {
					id, _ := r["link"].(string)
					if id == "" {
						id, _ = r["id"].(string)
					}
					if id == "" {
						continue
					}
					clicks[id] = intField(r, "clicks")
					meta[id] = map[string]string{
						"domain":    stringField(r, "domain"),
						"key":       stringField(r, "key"),
						"shortLink": stringField(r, "shortLink"),
					}
				}
				return clicks, meta, nil
			}

			recent, recentMeta, err := fetch(recentStart, recentEnd)
			if err != nil {
				return err
			}
			prior, _, err := fetch(priorStart, priorEnd)
			if err != nil {
				return err
			}

			out := make([]driftRow, 0)
			for id, recentClicks := range recent {
				priorClicks := prior[id]
				if priorClicks < minClicks && recentClicks < minClicks {
					continue
				}
				dPct := computeDriftPct(recentClicks, priorClicks)
				if absFloat(dPct) < threshold {
					continue
				}
				dir := "up"
				if dPct < 0 {
					dir = "down"
				}
				m := recentMeta[id]
				out = append(out, driftRow{
					LinkID:    id,
					Domain:    m["domain"],
					Key:       m["key"],
					ShortLink: m["shortLink"],
					Recent:    recentClicks,
					Prior:     priorClicks,
					DeltaPct:  dPct,
					Direction: dir,
				})
			}
			// also catch fully dead links (in prior but not recent)
			for id, priorClicks := range prior {
				if _, ok := recent[id]; ok {
					continue
				}
				if priorClicks < minClicks {
					continue
				}
				out = append(out, driftRow{
					LinkID:    id,
					Recent:    0,
					Prior:     priorClicks,
					DeltaPct:  -100,
					Direction: "down",
				})
			}

			sort.Slice(out, func(i, j int) bool {
				return out[i].DeltaPct < out[j].DeltaPct
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no drift detected")
				return nil
			}
			headers := []string{"LINK", "DOMAIN", "KEY", "RECENT", "PRIOR", "DELTA%"}
			rows := make([][]string, 0, len(out))
			for _, r := range out {
				rows = append(rows, []string{r.LinkID, r.Domain, r.Key, fmt.Sprintf("%d", r.Recent), fmt.Sprintf("%d", r.Prior), fmt.Sprintf("%+.1f", r.DeltaPct)})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().StringVar(&window, "window", "7d", "Comparison window (e.g. 24h, 7d, 30d)")
	cmd.Flags().Float64Var(&threshold, "threshold", 30, "Minimum percent change to report (absolute value)")
	cmd.Flags().IntVar(&minClicks, "min-clicks", 10, "Ignore links below this baseline click count in either window")
	return cmd
}

// parseWindow accepts 24h / 7d / 30d-style durations.
func parseWindow(w string) (time.Duration, error) {
	w = strings.TrimSpace(w)
	if w == "" {
		return 0, fmt.Errorf("window required")
	}
	if d, err := time.ParseDuration(w); err == nil && d > 0 {
		return d, nil
	}
	if strings.HasSuffix(w, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(w, "d"))
		if err == nil && n > 0 {
			return time.Duration(n) * 24 * time.Hour, nil
		}
	}
	return 0, fmt.Errorf("invalid window %q (try 24h, 7d, 30d)", w)
}

// computeDriftPct returns percent change from prior to recent.
// 0->N returns +100. N->0 returns -100. 0->0 returns 0.
func computeDriftPct(recent, prior int) float64 {
	if prior == 0 && recent == 0 {
		return 0
	}
	if prior == 0 {
		return 100
	}
	return (float64(recent) - float64(prior)) * 100 / float64(prior)
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
