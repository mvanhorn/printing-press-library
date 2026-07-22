// Copyright 2026 Harvey The AI Guy and contributors. Licensed under Apache-2.0. See LICENSE.
// Habit streaks: current/longest streaks and at-risk flags computed from live
// checkin history (the checkin query endpoint is POST-only, so no local sync).

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelHabitsStreaksCmd(flags *rootFlags) *cobra.Command {
	var flagDays int

	cmd := &cobra.Command{
		Use:   "streaks",
		Short: "Current streak, longest streak, and at-risk-today flags for every habit",
		Long: "Fetches habits and their checkin history live, then computes per-habit current streak, longest streak in the window, and whether the habit is at risk today (not yet checked in).\n" +
			"Streak math counts consecutive days with a completed checkin, ending today or yesterday.",
		Example:     "  ticktick-pp-cli habits streaks --json --select name,current_streak,at_risk",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch habits and checkins to compute streaks")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			habitsRaw, err := c.Get(ctx, "/habits", nil)
			if err != nil {
				return apiErr(fmt.Errorf("fetching habits: %w", err))
			}
			var habits []map[string]json.RawMessage
			if err := json.Unmarshal(habitsRaw, &habits); err != nil {
				return apiErr(fmt.Errorf("parsing habits: %w", err))
			}
			ids := make([]string, 0, len(habits))
			names := map[string]string{}
			for _, h := range habits {
				var status int
				_ = json.Unmarshal(h["status"], &status)
				if status != 0 {
					continue
				}
				id := rawStr(h["id"])
				ids = append(ids, id)
				names[id] = rawStr(h["name"])
			}
			if len(ids) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), []any{}, flags)
			}

			afterStamp, _ := strconv.Atoi(time.Now().AddDate(0, 0, -flagDays).Format("20060102"))
			body := map[string]any{"habitIds": ids, "afterStamp": afterStamp}
			checkinsRaw, status, err := c.Post(ctx, "/habitCheckins/query", body)
			if err != nil {
				return apiErr(fmt.Errorf("querying checkins: %w", err))
			}
			if status < 200 || status >= 300 {
				return apiErr(fmt.Errorf("checkin query rejected (HTTP %d)", status))
			}
			var resp struct {
				Checkins map[string][]struct {
					CheckinStamp int     `json:"checkinStamp"`
					Status       int     `json:"status"`
					Value        float64 `json:"value"`
				} `json:"checkins"`
			}
			if err := json.Unmarshal(checkinsRaw, &resp); err != nil {
				return apiErr(fmt.Errorf("parsing checkins: %w", err))
			}

			today, _ := strconv.Atoi(time.Now().Format("20060102"))
			yesterday, _ := strconv.Atoi(time.Now().AddDate(0, 0, -1).Format("20060102"))
			results := make([]map[string]any, 0, len(ids))
			for _, id := range ids {
				stamps := map[int]bool{}
				for _, ck := range resp.Checkins[id] {
					if ck.Status == 2 { // 2 = completed
						stamps[ck.CheckinStamp] = true
					}
				}
				current, longest := computeStreaks(stamps, today, yesterday)
				results = append(results, map[string]any{
					"id":             id,
					"name":           names[id],
					"current_streak": current,
					"longest_streak": longest,
					"checked_today":  stamps[today],
					"at_risk":        current > 0 && !stamps[today],
				})
			}
			sort.Slice(results, func(i, j int) bool {
				return results[i]["current_streak"].(int) > results[j]["current_streak"].(int)
			})
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			for _, r := range results {
				risk := ""
				if r["at_risk"].(bool) {
					risk = "  ⚠ at risk today"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30v current %3d  best %3d%s\n", r["name"], r["current_streak"], r["longest_streak"], risk)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 90, "History window in days for streak computation")
	return cmd
}

// computeStreaks walks day-stamps backward from today (or yesterday, if today
// is not yet checked) to find the current run, and scans the window for the
// longest run.
func computeStreaks(stamps map[int]bool, today, yesterday int) (current, longest int) {
	dayOf := func(stamp int) time.Time {
		t, _ := time.Parse("20060102", strconv.Itoa(stamp))
		return t
	}
	// Current streak: anchor on today if checked, else yesterday.
	anchor := 0
	if stamps[today] {
		anchor = today
	} else if stamps[yesterday] {
		anchor = yesterday
	}
	if anchor != 0 {
		d := dayOf(anchor)
		for {
			s, _ := strconv.Atoi(d.Format("20060102"))
			if !stamps[s] {
				break
			}
			current++
			d = d.AddDate(0, 0, -1)
		}
	}
	// Longest streak in window.
	if len(stamps) > 0 {
		days := make([]int, 0, len(stamps))
		for s := range stamps {
			days = append(days, s)
		}
		sort.Ints(days)
		run := 1
		longest = 1
		for i := 1; i < len(days); i++ {
			prev := dayOf(days[i-1]).AddDate(0, 0, 1)
			if dayOf(days[i]).Equal(prev) {
				run++
			} else {
				run = 1
			}
			if run > longest {
				longest = run
			}
		}
	}
	return current, longest
}
