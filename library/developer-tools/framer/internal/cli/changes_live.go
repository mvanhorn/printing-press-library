// Copyright 2026 ioncom. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/client"
	"github.com/spf13/cobra"
)

func newChangesLiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "changes",
		Short: "List changed paths since last publish via the Server API",
		Long: `Fetch added, removed, and modified paths from the Framer Server API
(WebSocket bridge) and display them as a table or JSON.

Requires FRAMER_PROJECT_URL and FRAMER_API_KEY environment variables.`,
		Example: `  # Table output
  framer-pp-cli changes

  # JSON output for agents
  framer-pp-cli changes --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch changed paths from Framer API")
				return nil
			}

			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}

			raw, err := bc.Call("changes-list")
			if err != nil {
				return fmt.Errorf("changes-list failed: %w", err)
			}

			// Try to parse as array of change objects.
			var changes []struct {
				Path   string `json:"path"`
				Status string `json:"status"`
			}
			if err := json.Unmarshal(raw, &changes); err != nil {
				// If it doesn't parse as expected, output raw.
				if flags.asJSON {
					return flags.printJSON(cmd, json.RawMessage(raw))
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}

			if flags.asJSON {
				return flags.printJSON(cmd, changes)
			}

			// Table output.
			headers := []string{"PATH", "STATUS"}
			rows := make([][]string, len(changes))
			for i, c := range changes {
				rows[i] = []string{c.Path, c.Status}
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	return cmd
}
