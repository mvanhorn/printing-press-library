// Copyright 2026 Milos Mladenovic and contributors. Licensed under Apache-2.0. See LICENSE.

// Novel feature: gear mileage rollup. Fetches the athlete's gear and
// components from the live API and rolls up distance/time, flagging components
// whose usage has passed a reminder threshold. The rollup-with-status view is
// the value-add; the API exposes gear and reminders piecemeal.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/other/intervals-icu/internal/cliutil"

	"github.com/spf13/cobra"
)

type gearItem struct {
	ID         string  `json:"id,omitempty"`
	Name       string  `json:"name"`
	Type       string  `json:"type,omitempty"`
	DistanceKm float64 `json:"distance_km"`
	TimeHrs    float64 `json:"time_hrs"`
	Retired    bool    `json:"retired"`
	Status     string  `json:"status"`
	DueReason  string  `json:"due_reason,omitempty"`
}

type gearStatusView struct {
	AthleteID string     `json:"athlete_id"`
	Gear      []gearItem `json:"gear"`
	DueCount  int        `json:"due_count"`
	Note      string     `json:"note,omitempty"`
}

func newNovelGearStatusCmd(flags *rootFlags) *cobra.Command {
	var flagAll bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Roll up gear distance/time and flag components due for service.",
		Long: "Fetch gear and components and report distance/time per item, flagging\n" +
			"any whose usage has passed a reminder threshold. Use to catch\n" +
			"chain/tyre/shoe replacement before it is overdue.",
		Example:     "  intervals-icu-pp-cli gear status --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch gear and roll up usage vs reminders")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			id := athleteID(flags)
			path := replacePathParam("/api/v1/athlete/{id}/gear", "id", id)
			data, err := c.Get(cmd.Context(), path, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var gears []map[string]json.RawMessage
			if err := json.Unmarshal(data, &gears); err != nil {
				return fmt.Errorf("parsing gear: %w", err)
			}
			items := make([]gearItem, 0, len(gears))
			due := 0
			for _, g := range gears {
				retired := jsonBool(g, "retired")
				if retired && !flagAll {
					continue
				}
				distM, _ := cliutil.ExtractNumber(g, "distance")
				timeS, _ := cliutil.ExtractNumber(g, "time")
				it := gearItem{
					ID:         jsonStr(g, "id"),
					Name:       jsonStr(g, "name"),
					Type:       jsonStr(g, "type"),
					DistanceKm: round1(distM / 1000),
					TimeHrs:    round1(timeS / 3600),
					Retired:    retired,
					Status:     "ok",
				}
				if reason, isDue := gearDue(g, distM); isDue {
					it.Status = "due"
					it.DueReason = reason
					due++
				}
				items = append(items, it)
			}
			sort.SliceStable(items, func(i, j int) bool { return items[i].DistanceKm > items[j].DistanceKm })
			view := gearStatusView{AthleteID: id, Gear: items, DueCount: due}
			if len(items) == 0 {
				view.Note = "no gear found (add --all to include retired components)"
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, view)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%-24s %-8s %10s %8s  %s\n", "GEAR", "TYPE", "KM", "HRS", "STATUS")
			for _, it := range items {
				status := it.Status
				if it.DueReason != "" {
					status += " (" + it.DueReason + ")"
				}
				fmt.Fprintf(out, "%-24s %-8s %10.1f %8.1f  %s\n", trunc(it.Name, 24), trunc(it.Type, 8), it.DistanceKm, it.TimeHrs, status)
			}
			if view.Note != "" {
				fmt.Fprintln(out, view.Note)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagAll, "all", false, "Include retired gear/components")
	return cmd
}

// gearDue inspects a gear's reminders array for a distance threshold the gear's
// usage has passed. Reminder shape varies; this reads the common numeric keys
// defensively and is silent (returns false) when none are recognized.
func gearDue(g map[string]json.RawMessage, distanceM float64) (string, bool) {
	raw, ok := g["reminders"]
	if !ok {
		return "", false
	}
	var reminders []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &reminders); err != nil {
		return "", false
	}
	for _, r := range reminders {
		threshold, hasT := firstNumber(r, "distance", "atDistance", "intervalDistance")
		if !hasT || threshold <= 0 {
			continue
		}
		if distanceM >= threshold {
			label := jsonStr(r, "text")
			if label == "" {
				label = jsonStr(r, "name")
			}
			if label == "" {
				label = "service"
			}
			return label, true
		}
	}
	return "", false
}

func jsonBool(m map[string]json.RawMessage, key string) bool {
	raw, ok := m[key]
	if !ok {
		return false
	}
	var b bool
	_ = json.Unmarshal(raw, &b)
	return b
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
