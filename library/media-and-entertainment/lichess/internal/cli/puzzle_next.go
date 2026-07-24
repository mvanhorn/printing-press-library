// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newPuzzleNextCmd(flags *rootFlags) *cobra.Command {
	var angle string

	cmd := &cobra.Command{
		Use:         "next",
		Short:       "Fetch exactly one official puzzle for a required training theme.",
		Example:     "  lichess-pp-cli puzzle next --angle pin",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			angle = strings.TrimSpace(angle)
			if angle == "" {
				return usageErr(fmt.Errorf("--angle is required; use the one follow-up emitted by training-brief"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// This command deliberately makes one request only. It offers no paging,
			// batching, difficulty sweep, or repeated-fetch mode, so it remains the
			// bounded follow-up selected by training-brief rather than an enumerator.
			data, err := c.Get(cmd.Context(), "/api/puzzle/next", map[string]string{"angle": angle})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&angle, "angle", "", "Required puzzle theme from training-brief; fetches one puzzle")
	return cmd
}
