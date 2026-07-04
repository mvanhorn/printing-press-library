// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature (NOT generated).
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type calCompareRow struct {
	APIID         string `json:"api_id"`
	Name          string `json:"name"`
	Slug          string `json:"slug,omitempty"`
	City          string `json:"city,omitempty"`
	UpcomingTotal int    `json:"upcoming_events"`
	UpcomingInWin int    `json:"upcoming_in_window,omitempty"`
	Error         string `json:"error,omitempty"`
}

type calCompareView struct {
	Window    string          `json:"window,omitempty"`
	Calendars []calCompareRow `json:"calendars"`
	Failures  int             `json:"fetch_failures"`
}

type calGetResp struct {
	Calendar struct {
		APIID   string `json:"api_id"`
		Name    string `json:"name"`
		Slug    string `json:"slug"`
		GeoCity string `json:"geo_city"`
	} `json:"calendar"`
	EventStartAts []string `json:"event_start_ats"`
}

func newNovelCalendarsCompareCmd(flags *rootFlags) *cobra.Command {
	var flagWindow string

	cmd := &cobra.Command{
		Use:   "compare [calendar-id...]",
		Short: "Side-by-side upcoming-event counts across several calendars (communities).",
		Long: "Fetch each calendar and compare upcoming-event counts side by side. The public API only\n" +
			"returns one calendar per call, so this aggregates them into a single ranked view.\n\n" +
			"Pass two or more calendar api_ids (cal-...). For one calendar's full detail use 'calendars get'.",
		Example:     "  luma-pp-cli calendars compare cal-AAA cal-BBB cal-CCC --window 14d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("compare needs at least two calendar api_ids (cal-...)"))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch and compare the given calendars")
				return nil
			}
			window, err := parseWindow(flagWindow)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --window %q: %w", flagWindow, err))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			now := time.Now()
			rowsOut := make([]calCompareRow, 0, len(args))
			failures := 0
			for _, id := range args {
				raw, gerr := c.Get(ctx, "/calendar/get", map[string]string{"api_id": id})
				if gerr != nil {
					rowsOut = append(rowsOut, calCompareRow{APIID: id, Error: gerr.Error()})
					failures++
					continue
				}
				var resp calGetResp
				if json.Unmarshal(raw, &resp) != nil {
					rowsOut = append(rowsOut, calCompareRow{APIID: id, Error: "unparseable response"})
					failures++
					continue
				}
				row := calCompareRow{
					APIID: id,
					Name:  resp.Calendar.Name,
					Slug:  resp.Calendar.Slug,
					City:  resp.Calendar.GeoCity,
				}
				for _, s := range resp.EventStartAts {
					t, perr := time.Parse(time.RFC3339, s)
					if perr != nil {
						continue
					}
					if t.Before(now.Add(-1 * time.Hour)) {
						continue
					}
					row.UpcomingTotal++
					if window > 0 && !t.After(now.Add(window)) {
						row.UpcomingInWin++
					}
				}
				rowsOut = append(rowsOut, row)
			}

			sort.SliceStable(rowsOut, func(i, j int) bool { return rowsOut[i].UpcomingTotal > rowsOut[j].UpcomingTotal })
			if failures > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d calendars failed to fetch\n", failures, len(args))
			}
			view := calCompareView{Calendars: rowsOut, Failures: failures}
			if window > 0 {
				view.Window = flagWindow
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagWindow, "window", "", "Also count events within this window from now (e.g. 14d)")
	return cmd
}
