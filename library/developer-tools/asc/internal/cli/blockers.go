// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"github.com/spf13/cobra"
)

func newNovelBlockersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "blockers",
		Short:       "Every app that can't ship right now and why (failed builds, rejections, invalid binaries)",
		Long:        "blockers correlates each app's latest build processing state and latest version review state into a single fleet blocker list — showing only the apps that have something preventing a release, and the reason.",
		Example:     "  asc-pp-cli blockers --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if err := c.Config.ASCPreflight(); err != nil {
				return err
			}
			ctx := cmd.Context()
			apps, err := fleetApps(ctx, c)
			if err != nil {
				return err
			}
			type row struct {
				App         string `json:"app"`
				Reason      string `json:"reason"`
				ReviewState string `json:"review_state"`
				BuildState  string `json:"build_state"`
			}
			rows := []row{}
			for _, a := range apps {
				var state, buildState string
				fetchErr := false
				if v, ok, err := latestVersion(ctx, c, a.ID); err != nil {
					fetchErr = true
				} else if ok {
					state = v.state()
				}
				if b, ok, err := latestBuild(ctx, c, a.ID); err != nil {
					fetchErr = true
				} else if ok {
					buildState = b.ProcessingState
				}
				reason := blockedReason(state, buildState)
				if reason == "" && fetchErr {
					// A blocker tool must not silently hide an app it couldn't read.
					reason = "unknown — could not read state"
				}
				if reason != "" {
					rows = append(rows, row{a.Name, reason, state, buildState})
				}
			}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{r.App, r.Reason, dash(r.ReviewState), dash(r.BuildState)})
			}
			return renderFleet(cmd, flags, rows,
				[]string{"App", "Reason", "Review", "BuildState"}, table,
				"no blockers — every app can ship")
		},
	}
	return cmd
}
