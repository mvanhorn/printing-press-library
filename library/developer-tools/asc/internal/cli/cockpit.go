// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"github.com/spf13/cobra"
)

func newNovelCockpitCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "cockpit",
		Short:       "One-glance board across all your apps: review state, latest build, and an action-needed flag",
		Long:        "cockpit rolls up every app in the team into one row: its latest App Store version and review state, its latest TestFlight build and processing state, and a derived action-needed flag. It answers 'what across all my apps needs attention right now' in one call.",
		Example:     "  asc-pp-cli cockpit --agent",
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
				BundleID    string `json:"bundle_id"`
				Version     string `json:"version"`
				ReviewState string `json:"review_state"`
				Build       string `json:"build"`
				BuildState  string `json:"build_state"`
				Action      string `json:"action"`
			}
			rows := make([]row, 0, len(apps))
			table := make([][]string, 0, len(apps))
			for _, a := range apps {
				var state, ver, buildNo, buildState string
				fetchErr := false
				if v, ok, err := latestVersion(ctx, c, a.ID); err != nil {
					fetchErr = true
				} else if ok {
					state, ver = v.state(), v.VersionString
				}
				if b, ok, err := latestBuild(ctx, c, a.ID); err != nil {
					fetchErr = true
				} else if ok {
					buildNo, buildState = b.Version, b.ProcessingState
				}
				action := actionFlag(state, buildState)
				if fetchErr {
					// Don't report an app we couldn't fully read as "ok".
					action = "unknown — fetch error"
				}
				rows = append(rows, row{a.Name, a.BundleID, ver, state, buildNo, buildState, action})
				table = append(table, []string{a.Name, dash(ver), dash(state), dash(buildNo), dash(buildState), action})
			}
			return renderFleet(cmd, flags, rows,
				[]string{"App", "Version", "Review", "Build", "BuildState", "Action"}, table,
				"no apps found for this team")
		},
	}
	return cmd
}
