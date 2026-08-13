// Copyright 2026 Ryan Gravette and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: aggregate a course's structure into rollup stats.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type courseStatsView struct {
	ID                   string  `json:"id"`
	Title                string  `json:"title"`
	Status               string  `json:"status"`
	Sections             int     `json:"sections"`
	Instructions         int     `json:"instructions"`
	Interactions         int     `json:"interactions"`
	TotalPoints          int     `json:"total_points"`
	TotalDurationSeconds float64 `json:"total_duration_seconds"`
	TotalDurationHours   float64 `json:"total_duration_hours"`
}

// pp:data-source computed
func newNovelCourseStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "stats <id>",
		Short:       "Aggregate total instruction time, points, and section/instruction/interaction counts for a course.",
		Long:        "Aggregate total instruction time, points, and section/instruction/interaction counts for a course.\n\nUse for total seat-time and content-size metrics of one course.",
		Example:     "  agilix-dawn-pp-cli course stats c_f4bff87c0cab456984f2860af3e427d0 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "id=c_7666e422b4b94b3f840a250cb923a1a6"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch concept structure")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("id is required (a concept id, c_...)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			concept, err := fetchConcept(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			view := courseStatsView{ID: concept.ID, Title: concept.Title, Status: concept.Status}
			view.Sections = len(concept.Section)
			for _, s := range concept.Section {
				view.Instructions += len(s.Instruction)
				for _, in := range s.Instruction {
					view.Interactions += in.interactionCount()
					view.TotalPoints += in.Points
					view.TotalDurationSeconds += in.Duration
				}
			}
			view.TotalDurationHours = view.TotalDurationSeconds / 3600.0
			if flags.asJSON {
				return flags.printJSON(cmd, view)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s  (%s)  status=%s\n", view.Title, view.ID, view.Status)
			fmt.Fprintf(w, "  sections:      %d\n", view.Sections)
			fmt.Fprintf(w, "  instructions:  %d\n", view.Instructions)
			fmt.Fprintf(w, "  interactions:  %d\n", view.Interactions)
			fmt.Fprintf(w, "  total points:  %d\n", view.TotalPoints)
			fmt.Fprintf(w, "  total time:    %.1f hours (%.0f seconds)\n", view.TotalDurationHours, view.TotalDurationSeconds)
			return nil
		},
	}
	return cmd
}
