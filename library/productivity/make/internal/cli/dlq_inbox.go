// Copyright 2026 Wade Carpenter and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type dlqRow struct {
	ID         string `json:"id"`
	ScenarioID int64  `json:"scenarioId"`
	Scenario   string `json:"scenarioName,omitempty"`
	TeamID     int64  `json:"teamId"`
	Reason     string `json:"reason"`
	Status     int    `json:"status"`
	Created    string `json:"created"`
	AgeHours   int    `json:"ageHours"`
}

func newNovelDlqInboxCmd(flags *rootFlags) *cobra.Command {
	var flagTeam string
	var flagAllTeams bool
	var flagAge time.Duration
	var flagGroupBy string
	var flagRetryAll bool
	var flagResolveAll bool
	var flagMatchReason string

	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "Cross-scenario DLQ inbox: list incomplete executions across teams, group by error fingerprint, bulk retry/resolve",
		Example: strings.Trim(`
  make-pp-cli dlq inbox --all-teams --age 24h --group-by reason --json
  make-pp-cli dlq inbox --team 588013 --group-by reason --retry-all --match-reason 'rate.*limit'
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagTeam == "" && !flagAllTeams {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			teamIDs, err := teamIDsFromFlags(ctx, c, flagTeam, flagAllTeams)
			if err != nil {
				return err
			}
			if len(teamIDs) == 0 {
				return usageErr(fmt.Errorf("specify --team <id> or --all-teams"))
			}

			ageCutoff := time.Time{}
			if flagAge > 0 {
				ageCutoff = time.Now().Add(-flagAge)
			}

			var rows []dlqRow
			for _, tid := range teamIDs {
				scenarios, err := listScenarios(ctx, c, tid)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warn: list scenarios for team %d failed: %v\n", tid, err)
					continue
				}
				for _, s := range scenarios {
					sid := int64(asFloat(s["id"]))
					if sid == 0 {
						continue
					}
					name, _ := s["name"].(string)
					dlqList, err := listDLQs(ctx, c, sid, tid)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warn: list DLQs scenario %d failed: %v\n", sid, err)
						continue
					}
					for _, d := range dlqList {
						created, _ := d["created"].(string)
						if !ageCutoff.IsZero() {
							if t, err := time.Parse(time.RFC3339Nano, created); err == nil && t.Before(ageCutoff) {
								continue
							}
						}
						ageH := 0
						if t, err := time.Parse(time.RFC3339Nano, created); err == nil {
							ageH = int(time.Since(t).Hours())
						}
						id, _ := d["id"].(string)
						reason, _ := d["reason"].(string)
						rows = append(rows, dlqRow{
							ID:         id,
							ScenarioID: sid,
							Scenario:   name,
							TeamID:     tid,
							Reason:     reason,
							Status:     int(asFloat(d["status"])),
							Created:    created,
							AgeHours:   ageH,
						})
					}
				}
			}

			var actioned []map[string]any
			if flagRetryAll || flagResolveAll {
				matcher, err := compileReasonMatcher(flagMatchReason)
				if err != nil {
					return usageErr(err)
				}
				for _, r := range rows {
					if matcher != nil && !matcher.MatchString(r.Reason) {
						continue
					}
					action := "retry"
					path := "/dlqs/" + r.ID + "/retry"
					if flagResolveAll {
						action = "resolve"
						path = "/dlqs/" + r.ID + "/resolve"
					}
					_, sc, err := c.PostWithParams(ctx, path, nil, nil)
					ok := err == nil && sc >= 200 && sc < 300
					actioned = append(actioned, map[string]any{
						"id":         r.ID,
						"scenarioId": r.ScenarioID,
						"action":     action,
						"ok":         ok,
						"status":     sc,
						"error":      errString(err),
					})
				}
			}

			out := map[string]any{
				"teamsScanned": len(teamIDs),
				"totalDlqs":    len(rows),
				"rows":         rows,
			}
			if flagGroupBy == "reason" {
				out["groups"] = groupByReason(rows)
			}
			if len(actioned) > 0 {
				out["actions"] = actioned
			}
			b, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&flagTeam, "team", "", "Team ID to scan (omit to require --all-teams)")
	cmd.Flags().BoolVar(&flagAllTeams, "all-teams", false, "Scan every team the token can see")
	cmd.Flags().DurationVar(&flagAge, "age", 0, "Only include DLQs newer than this duration (e.g. 24h, 168h for 7 days)")
	cmd.Flags().StringVar(&flagGroupBy, "group-by", "", "Group DLQs by reason fingerprint (only 'reason' is supported)")
	cmd.Flags().BoolVar(&flagRetryAll, "retry-all", false, "Retry every matched DLQ via POST /dlqs/{id}/retry")
	cmd.Flags().BoolVar(&flagResolveAll, "resolve-all", false, "Resolve every matched DLQ via POST /dlqs/{id}/resolve")
	cmd.Flags().StringVar(&flagMatchReason, "match-reason", "", "Regex on the reason field, required when --retry-all or --resolve-all is used")
	return cmd
}

func groupByReason(rows []dlqRow) []map[string]any {
	type bucket struct {
		Pattern   string
		Count     int
		Scenarios map[int64]bool
	}
	buckets := map[string]*bucket{}
	for _, r := range rows {
		key := reasonFingerprint(r.Reason)
		if buckets[key] == nil {
			buckets[key] = &bucket{Pattern: key, Scenarios: map[int64]bool{}}
		}
		buckets[key].Count++
		buckets[key].Scenarios[r.ScenarioID] = true
	}
	var out []map[string]any
	for _, b := range buckets {
		out = append(out, map[string]any{
			"reason":        b.Pattern,
			"count":         b.Count,
			"scenarioCount": len(b.Scenarios),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i]["count"].(int) > out[j]["count"].(int)
	})
	return out
}

var (
	reasonNumRE  = regexp.MustCompile(`\b[0-9]+\b`)
	reasonUUIDRE = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
)

func reasonFingerprint(reason string) string {
	r := strings.TrimSpace(reason)
	r = reasonUUIDRE.ReplaceAllString(r, "<uuid>")
	r = reasonNumRE.ReplaceAllString(r, "N")
	if len(r) > 200 {
		r = r[:200] + "…"
	}
	return r
}

func compileReasonMatcher(pat string) (*regexp.Regexp, error) {
	if pat == "" {
		return nil, fmt.Errorf("--retry-all/--resolve-all requires --match-reason <regex> to avoid accidental fan-out")
	}
	re, err := regexp.Compile("(?i)" + pat)
	if err != nil {
		return nil, fmt.Errorf("invalid --match-reason regex: %w", err)
	}
	return re, nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}
