// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newNovelPipelineCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "pipeline",
		Short:       "Every version in the review pipeline across all apps, ranked by how long it has waited",
		Long:        "pipeline lists every App Store version currently in flight (waiting for review, in review, or pending release) across all your apps, ranked by how long it has been sitting — so a submission stuck too long surfaces at the top.",
		Example:     "  asc-pp-cli pipeline --agent",
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
				App     string  `json:"app"`
				Version string  `json:"version"`
				State   string  `json:"state"`
				AgeDays float64 `json:"age_days"`
				Created string  `json:"created"`
			}
			rows := []row{}
			for _, a := range apps {
				vs, _ := appVersions(ctx, c, a.ID)
				for _, v := range vs {
					if !inFlightStates[v.state()] {
						continue
					}
					rows = append(rows, row{a.Name, v.VersionString, v.state(),
						round1(ageDays(v.CreatedDate)), v.CreatedDate})
				}
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].AgeDays > rows[j].AgeDays })
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{r.App, r.Version, r.State, fmt.Sprintf("%.1f", r.AgeDays)})
			}
			return renderFleet(cmd, flags, rows,
				[]string{"App", "Version", "State", "AgeDays"}, table,
				"nothing in the review pipeline right now")
		},
	}
	return cmd
}
