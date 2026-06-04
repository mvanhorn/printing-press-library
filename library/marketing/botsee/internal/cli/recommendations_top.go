// Copyright 2026 grahac and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// newRecommendationsCmd is the top-level promotion of
// `POST /api/v1/analysis/{uuid}/recommendations`. The spec also exposes it
// nested under `analysis recommendations`; this top-level alias makes it
// agent-discoverable from the root help.
func newRecommendationsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recommendations <analysis_uuid>",
		Short: "Generate next-step recommendations for an analysis (top-level promotion of `analysis recommendations`).",
		Long: `Generate next-step recommendations for an analysis.

Promotes POST /api/v1/analysis/{uuid}/recommendations to the top level so
agents discover it from the root help. Same backing endpoint as
` + "`botsee-pp-cli analysis recommendations <uuid>`" + ` — use whichever reads
better in your script.`,
		Example: "  botsee-pp-cli recommendations $ANALYSIS_UUID --agent\n" +
			"  botsee-pp-cli recommendations $ANALYSIS_UUID --json",
		Annotations: map[string]string{
			"mcp:read-only": "false",
			"pp:novel":      "recommendations-promoted",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			analysisUUID := args[0]
			path := "/api/v1/analysis/" + analysisUUID + "/recommendations"
			resp, status, err := c.Post(cmd.Context(), path, map[string]any{})
			if err != nil || status < 200 || status >= 300 {
				if err == nil {
					err = fmt.Errorf("HTTP %d: %s", status, string(resp))
				}
				return fmt.Errorf("generating recommendations: %w", err)
			}
			out := cmd.OutOrStdout()
			if flags.asJSON || flags.agent || !isTerminal(out) {
				return printOutputWithFlags(out, resp, flags)
			}
			var parsed any
			if err := json.Unmarshal(resp, &parsed); err == nil {
				b, _ := json.MarshalIndent(parsed, "", "  ")
				fmt.Fprintln(out, string(b))
				return nil
			}
			fmt.Fprintln(out, string(resp))
			return nil
		},
	}
	return cmd
}
