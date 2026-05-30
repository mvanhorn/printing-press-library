// Copyright 2026 Wade Carpenter and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type hookMapRow struct {
	HookID        int64    `json:"hookId"`
	Name          string   `json:"name"`
	TypeName      string   `json:"typeName,omitempty"`
	URL           string   `json:"url,omitempty"`
	TeamID        int64    `json:"teamId"`
	UsedBy        int      `json:"usedByScenarios"`
	Scenarios     []int64  `json:"scenarioIds,omitempty"`
	ScenarioNames []string `json:"scenarioNames,omitempty"`
	Status        string   `json:"status"` // active | orphan | shared
}

func newNovelHooksMapCmd(flags *rootFlags) *cobra.Command {
	var flagTeam string
	var flagAllTeams bool
	var flagOrphans bool
	var flagShared bool

	cmd := &cobra.Command{
		Use:   "map",
		Short: "Webhook → scenario routing map: walks blueprints to find which scenarios consume each hook",
		Example: strings.Trim(`
  make-pp-cli hooks map --all-teams --json
  make-pp-cli hooks map --team 588013 --orphans --json
  make-pp-cli hooks map --team 588013 --shared --json --select rows.hookId,rows.name,rows.scenarioIds
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
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

			var rows []hookMapRow
			for _, tid := range teamIDs {
				hooks, err := listHooks(ctx, c, tid)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warn: list hooks for team %d failed: %v\n", tid, err)
					continue
				}
				scenarios, err := listScenarios(ctx, c, tid)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warn: list scenarios for team %d failed: %v\n", tid, err)
				}
				hookUsage := map[int64][]int64{}
				scenarioNames := map[int64]string{}
				for _, s := range scenarios {
					sid := int64(asFloat(s["id"]))
					if sid == 0 {
						continue
					}
					scenarioNames[sid] = stringOf(s["name"])
					if hid := int64(asFloat(s["hookId"])); hid != 0 {
						hookUsage[hid] = append(hookUsage[hid], sid)
					}
					bp, err := getBlueprint(ctx, c, sid)
					if err != nil {
						continue
					}
					for _, hid := range walkBlueprintWebhookRefs(bp) {
						hookUsage[hid] = append(hookUsage[hid], sid)
					}
				}
				for _, h := range hooks {
					hid := int64(asFloat(h["id"]))
					if hid == 0 {
						continue
					}
					usedBy := uniqueInt64(hookUsage[hid])
					r := hookMapRow{
						HookID:    hid,
						Name:      stringOf(h["name"]),
						TypeName:  stringOf(h["typeName"]),
						URL:       stringOf(h["url"]),
						TeamID:    tid,
						UsedBy:    len(usedBy),
						Scenarios: usedBy,
					}
					for _, sid := range usedBy {
						if n, ok := scenarioNames[sid]; ok && n != "" {
							r.ScenarioNames = append(r.ScenarioNames, n)
						}
					}
					switch {
					case len(usedBy) == 0:
						r.Status = "orphan"
					case len(usedBy) > 1:
						r.Status = "shared"
					default:
						r.Status = "active"
					}
					if flagOrphans && r.Status != "orphan" {
						continue
					}
					if flagShared && r.Status != "shared" {
						continue
					}
					rows = append(rows, r)
				}
			}

			out := map[string]any{
				"teamsScanned": len(teamIDs),
				"totalHooks":   len(rows),
				"rows":         rows,
			}
			b, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&flagTeam, "team", "", "Team ID to map (omit to require --all-teams)")
	cmd.Flags().BoolVar(&flagAllTeams, "all-teams", false, "Map hooks across every team the token can see")
	cmd.Flags().BoolVar(&flagOrphans, "orphans", false, "Show only hooks with zero scenario consumers")
	cmd.Flags().BoolVar(&flagShared, "shared", false, "Show only hooks consumed by more than one scenario")
	return cmd
}
