// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "stats",
		Short:       "Your reading stats — want/reading/read counts, average rating, reflections",
		Long:        "stats aggregates your local reading log: how many books you want to read, are reading, and have read, your average rating, and how many reflections you've written.",
		Example:     "  passage stats",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			p, err := openPractice(cmd)
			if err != nil {
				return err
			}
			defer p.Close()
			s, err := p.Stats(cmd.Context())
			if err != nil {
				return err
			}
			table := [][]string{
				{"want", fmt.Sprintf("%d", s.Want)},
				{"reading", fmt.Sprintf("%d", s.Reading)},
				{"read", fmt.Sprintf("%d", s.Read)},
				{"avg_rating", fmt.Sprintf("%.1f", s.AvgRating)},
				{"reflections", fmt.Sprintf("%d", s.Reflections)},
			}
			return bookRender(cmd, flags, s, []string{"Metric", "Value"}, table)
		},
	}
	return cmd
}
