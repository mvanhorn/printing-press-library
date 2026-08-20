// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
//
// `status` — report generation status for one or more clips. GET
// /api/feed/?ids= (batched in pairs of 2). --wait polls until every clip is
// terminal. Read-only.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newSunoStatusCmd(flags *rootFlags) *cobra.Command {
	var wait bool

	cmd := &cobra.Command{
		Use:   "status <clip_id> [<clip_id>...]",
		Short: "Show generation status (and audio URL) for one or more clips",
		Long: `Fetch the current status, title, and audio URL for one or more clips.

IDs are fetched in pairs of two (Suno's feed endpoint returns malformed
results when 4+ IDs are requested at once).

With --wait, polls every ~3 seconds until every clip reaches a terminal state
(complete or error).`,
		Example: `  suno-pp-cli status 550e8400-e29b-41d4-a716-446655440000
  suno-pp-cli status <id1> <id2> --wait --json`,
		Annotations: map[string]string{"pp:method": "GET", "pp:path": "/api/feed/", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("at least one clip_id is required"))
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var clips []json.RawMessage
			if wait {
				clips, err = waitForClips(cmd.Context(), c, args, cmd)
			} else {
				clips, err = fetchClips(cmd.Context(), c, args)
			}
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), clips, flags)
			}
			for _, raw := range clips {
				var cs clipStatus
				if json.Unmarshal(raw, &cs) != nil {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %-30s [%s]  %s\n", cs.ID, cs.Title, cs.Status, cs.AudioURL)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&wait, "wait", false, "Poll until all clips reach a terminal state")
	return cmd
}
