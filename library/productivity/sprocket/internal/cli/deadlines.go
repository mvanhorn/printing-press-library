// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

func newNovelDeadlinesCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:   "deadlines",
		Short: "Open registration programs sorted by how soon they close, flagging ones closing within N days.",
		Long: "List programs currently open for registration, sorted by close date, flagging those closing within " +
			"N days.\n\n" +
			"Use this command for upcoming registration close dates. Do NOT use it to list all open programs " +
			"unsorted; use 'programs open'.",
		Example:     "  sprocket-pp-cli deadlines\n  sprocket-pp-cli deadlines --days 30 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := rejectLocalDataSource(flags); err != nil {
				return err
			}
			if days < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--days must be at least 1"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(ctx, "/api/players/open-programs", map[string]string{"includeCredits": "true"})
			if err != nil {
				return fmt.Errorf("fetching open programs: %w", err)
			}
			views, err := buildDeadlines(data, time.Now(), days)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return flags.printJSON(cmd, views)
			}
			w := cmd.OutOrStdout()
			if len(views) == 0 {
				fmt.Fprintln(w, "No open programs found.")
				return nil
			}
			headers := []string{"PROGRAM", "CLOSES", "DAYS LEFT", "SOON"}
			rows := make([][]string, 0, len(views))
			for _, v := range views {
				closes := v.ClosesAt
				daysLeft := ""
				if v.ClosesAt != "" {
					daysLeft = fmt.Sprintf("%d", v.DaysUntilClose)
				} else {
					closes = "(no close date)"
				}
				soon := ""
				if v.ClosingSoon {
					soon = "★"
				}
				rows = append(rows, []string{v.Name, closes, daysLeft, soon})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	cmd.Flags().IntVar(&days, "days", 14, "flag programs closing within this many days")
	return cmd
}

type deadlineView struct {
	Name           string `json:"name"`
	ClosesAt       string `json:"closesAt,omitempty"`
	DaysUntilClose int    `json:"daysUntilClose,omitempty"`
	ClosingSoon    bool   `json:"closingSoon"`
}

// buildDeadlines parses open-program objects, extracts a registration close
// date, and sorts ascending by close date (programs without a date sort last).
// Returns an error when a non-empty response cannot be interpreted, matching
// buildOwedReport, so an unexpected API shape surfaces as an error rather than
// silently reporting "No open programs". Pure and unit-tested.
func buildDeadlines(data json.RawMessage, now time.Time, withinDays int) ([]deadlineView, error) {
	objs, ok := asObjects(data)
	if !ok {
		return nil, fmt.Errorf("could not interpret open-programs response (unexpected shape)")
	}
	views := make([]deadlineView, 0, len(objs))
	nowDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, m := range objs {
		name := firstString(m, "name", "programName", "title")
		if name == "" {
			name = "(unnamed program)"
		}
		v := deadlineView{Name: name}
		closeStr := firstString(m, "registrationEndDate", "registrationCloseDate", "registrationDeadline",
			"closeDate", "regEndDate", "endDate")
		if t, ok := parseSprocketTime(closeStr); ok {
			v.ClosesAt = t.Format("2006-01-02")
			// Compare on calendar dates in a single zone so partial days don't
			// truncate the count (a close "tomorrow" reads as 1 day, not 0).
			closeDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, now.Location())
			d := int(closeDay.Sub(nowDay).Hours() / 24)
			v.DaysUntilClose = d
			v.ClosingSoon = d >= 0 && d <= withinDays
		}
		views = append(views, v)
	}
	sort.SliceStable(views, func(i, j int) bool {
		hi, hj := views[i].ClosesAt != "", views[j].ClosesAt != ""
		if hi != hj {
			return hi // dated programs first
		}
		return views[i].DaysUntilClose < views[j].DaysUntilClose
	})
	return views, nil
}

// firstString returns the first key whose value is a non-empty string.
func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}
