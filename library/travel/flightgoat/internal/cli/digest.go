// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
// digest is a transcendence command that joins multiple AeroAPI endpoints
// (departures, delays, weather) into a single daily brief. Extracted from
// transcend.go so each novel compound command lives in its own file.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// ----- T13: digest -----

func newDigestCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "digest <airport>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "One-command daily brief: departures, delays, weather, disruptions",
		Example: `  flightgoat-pp-cli digest SEA`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			airport := upperCode(args[0])

			if flags.dryRun {
				fmt.Fprintf(cmd.ErrOrStderr(), "GET /airports/%s/flights/departures?start=<today>&end=<eod>&max_pages=3\n", airport)
				fmt.Fprintf(cmd.ErrOrStderr(), "GET /airports/%s/delays\n", airport)
				fmt.Fprintf(cmd.ErrOrStderr(), "GET /airports/%s/weather/observations\n", airport)
				fmt.Fprintln(cmd.ErrOrStderr(), "\n(dry run - no requests sent; would aggregate departures, delays, and weather into a daily brief)")
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			type digest struct {
				Airport         string          `json:"airport"`
				DeparturesToday int             `json:"departures_today"`
				ActiveDelays    json.RawMessage `json:"active_delays,omitempty"`
				Weather         json.RawMessage `json:"weather,omitempty"`
				TopDestinations []string        `json:"top_destinations,omitempty"`
			}
			d := digest{Airport: airport}

			now := time.Now().UTC()
			start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
			end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC).Format(time.RFC3339)
			if raw, err := c.Get(fmt.Sprintf("/airports/%s/flights/departures", airport),
				map[string]string{"start": start, "end": end, "max_pages": "3"}); err == nil {
				var page scheduledDeparturesPage
				_ = json.Unmarshal(raw, &page)
				items := page.items()
				d.DeparturesToday = len(items)
				counts := map[string]int{}
				for _, it := range items {
					dest := it.Destination.Best()
					if dest != "" {
						counts[dest]++
					}
				}
				type kv struct {
					k string
					v int
				}
				top := make([]kv, 0, len(counts))
				for k, v := range counts {
					top = append(top, kv{k, v})
				}
				sort.SliceStable(top, func(i, j int) bool { return top[i].v > top[j].v })
				// Compute share percentage for each top destination so the output
				// shows destination dominance, not just raw counts.
				totalForPct := float64(len(items))
				for i := 0; i < 5 && i < len(top); i++ {
					var sharePct float64
					if totalForPct > 0 {
						sharePct = float64(top[i].v) * 100.0 / totalForPct
					}
					d.TopDestinations = append(d.TopDestinations, fmt.Sprintf("%s (%d, %.1f%%)", top[i].k, top[i].v, sharePct))
				}
			}

			if raw, err := c.Get(fmt.Sprintf("/airports/%s/delays", airport), nil); err == nil {
				d.ActiveDelays = raw
			}
			if raw, err := c.Get(fmt.Sprintf("/airports/%s/weather/observations", airport), nil); err == nil {
				d.Weather = raw
			}

			bts, _ := json.MarshalIndent(d, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(bts))
			return nil
		},
	}
	return cmd
}
