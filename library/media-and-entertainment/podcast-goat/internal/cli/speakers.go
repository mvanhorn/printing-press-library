// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 `speakers list`.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newSpeakersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "speakers",
		Short: "Aggregate speakers across the cached corpus",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newSpeakersListCmd(flags))
	return cmd
}

func newSpeakersListCmd(flags *rootFlags) *cobra.Command {
	var (
		flagShow string
		flagMin  int
	)
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List speakers with episode + segment counts",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			ps, err := openPodcastStore(cmd.Context())
			if err != nil {
				return err
			}
			rows, err := ps.ListSpeakers(cmd.Context(), flagShow, flagMin)
			if err != nil {
				return err
			}
			if flags.asJSON {
				out, _ := json.MarshalIndent(rows, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no speakers yet — cache some episodes first")
				return nil
			}
			headers := []string{"speaker", "episodes", "segments", "shows"}
			var data [][]string
			for _, r := range rows {
				data = append(data, []string{
					r.Speaker,
					fmt.Sprintf("%d", r.EpisodeCount),
					fmt.Sprintf("%d", r.SegmentCount),
					strings.Join(r.Shows, ", "),
				})
			}
			return flags.printTable(cmd, headers, data)
		},
	}
	cmd.Flags().StringVar(&flagShow, "show", "", "Filter to one show slug")
	cmd.Flags().IntVar(&flagMin, "min-segments", 1, "Minimum segments to include a speaker")
	return cmd
}
