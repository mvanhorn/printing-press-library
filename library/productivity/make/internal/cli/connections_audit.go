// Copyright 2026 Wade Carpenter and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type connAuditRow struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	TeamID      int64    `json:"teamId"`
	AccountName string   `json:"accountName,omitempty"`
	AccountType string   `json:"accountType,omitempty"`
	Expire      string   `json:"expire,omitempty"`
	ExpireIn    string   `json:"expireIn,omitempty"`
	Issues      []string `json:"issues"`
	UsedBy      int      `json:"usedByScenarios"`
	ScenarioIDs []int64  `json:"scenarioIds,omitempty"`
}

func newNovelConnectionsAuditCmd(flags *rootFlags) *cobra.Command {
	var flagTeam string
	var flagAllTeams bool
	var flagUnused bool
	var flagExpiring time.Duration
	var flagErrored time.Duration

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit connections for unused, expiring, or repeatedly errored credentials",
		Example: strings.Trim(`
  make-pp-cli connections audit --all-teams --unused --expiring 168h --json
  make-pp-cli connections audit --team 588013 --expiring 168h --json --select rows.id,rows.name,rows.expire,rows.issues
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// No-input invocation: show help (verify-friendly).
			if flagTeam == "" && !flagAllTeams && !flagUnused && flagExpiring == 0 && flagErrored == 0 {
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

			now := time.Now()
			var rows []connAuditRow

			for _, tid := range teamIDs {
				conns, err := listConnections(ctx, c, tid)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warn: list connections for team %d failed: %v\n", tid, err)
					continue
				}
				scenarios, err := listScenarios(ctx, c, tid)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warn: list scenarios for team %d failed: %v\n", tid, err)
				}
				usage := map[int64][]int64{}
				for _, s := range scenarios {
					sid := int64(asFloat(s["id"]))
					if sid == 0 {
						continue
					}
					bp, err := getBlueprint(ctx, c, sid)
					if err != nil {
						continue
					}
					for _, connID := range walkBlueprintConnectionRefs(bp) {
						usage[connID] = append(usage[connID], sid)
					}
				}
				for _, conn := range conns {
					cid := int64(asFloat(conn["id"]))
					if cid == 0 {
						continue
					}
					r := connAuditRow{
						ID:          cid,
						Name:        stringOf(conn["name"]),
						TeamID:      tid,
						AccountName: stringOf(conn["accountName"]),
						AccountType: stringOf(conn["accountType"]),
						Expire:      stringOf(conn["expire"]),
					}
					if walked, ok := usage[cid]; ok {
						r.ScenarioIDs = walked
					}
					if hint, ok := conn["scenarioUsages"].([]any); ok {
						for _, h := range hint {
							if hm, ok := h.(map[string]any); ok {
								id := int64(asFloat(hm["id"]))
								if id != 0 {
									r.ScenarioIDs = append(r.ScenarioIDs, id)
								}
							}
						}
					}
					r.ScenarioIDs = uniqueInt64(r.ScenarioIDs)
					r.UsedBy = len(r.ScenarioIDs)

					if r.Expire != "" {
						if t, err := time.Parse(time.RFC3339Nano, r.Expire); err == nil {
							until := t.Sub(now)
							r.ExpireIn = humanDuration(until)
							if until <= 0 {
								r.Issues = append(r.Issues, "expired")
							} else if flagExpiring > 0 && until <= flagExpiring {
								r.Issues = append(r.Issues, "expiring")
							}
						}
					}
					if r.UsedBy == 0 && flagUnused {
						r.Issues = append(r.Issues, "unused")
					}
					rows = append(rows, r)
				}
			}

			withIssues := 0
			for _, r := range rows {
				if len(r.Issues) > 0 {
					withIssues++
				}
			}
			out := map[string]any{
				"teamsScanned": len(teamIDs),
				"totalIssues":  withIssues,
				"rows":         rows,
			}
			b, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&flagTeam, "team", "", "Team ID to audit (omit to require --all-teams)")
	cmd.Flags().BoolVar(&flagAllTeams, "all-teams", false, "Audit every team the token can see")
	cmd.Flags().BoolVar(&flagUnused, "unused", false, "Flag connections not referenced by any scenario blueprint")
	cmd.Flags().DurationVar(&flagExpiring, "expiring", 0, "Flag connections expiring within this duration (e.g. 168h for 7 days)")
	cmd.Flags().DurationVar(&flagErrored, "errored", 0, "Reserved: flag connections with recent execution errors (planned for executions sync)")
	return cmd
}

func stringOf(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func humanDuration(d time.Duration) string {
	if d < 0 {
		return "expired " + humanDuration(-d) + " ago"
	}
	days := int(d.Hours() / 24)
	if days >= 2 {
		return fmt.Sprintf("%dd", days)
	}
	if d.Hours() >= 2 {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return d.Round(time.Minute).String()
}
