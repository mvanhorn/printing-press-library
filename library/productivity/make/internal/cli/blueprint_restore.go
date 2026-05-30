// Copyright 2026 Wade Carpenter and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelBlueprintRestoreCmd(flags *rootFlags) *cobra.Command {
	var flagSnapshot string

	cmd := &cobra.Command{
		Use:   "restore <scenarioId>",
		Short: "Restore a blueprint snapshot file to the live API for an existing scenario",
		Example: strings.Trim(`
  make-pp-cli blueprint restore 3041366 --snapshot ./repo/team-588013/3041366-buzzsprout.blueprint.json --dry-run
  make-pp-cli blueprint restore 3041366 --snapshot ./blueprint.json --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			scenarioID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("scenarioId must be an integer: %q", args[0]))
			}
			if flagSnapshot == "" {
				return usageErr(fmt.Errorf("--snapshot <path> is required"))
			}
			raw, err := os.ReadFile(flagSnapshot)
			if err != nil {
				return fmt.Errorf("reading snapshot: %w", err)
			}
			var bpAny any
			if err := json.Unmarshal(raw, &bpAny); err != nil {
				return fmt.Errorf("snapshot is not valid JSON: %w", err)
			}
			bpStr, _ := json.Marshal(bpAny)
			result := map[string]any{
				"scenarioId": scenarioID,
				"snapshot":   flagSnapshot,
			}
			if flags.dryRun {
				result["dryRun"] = true
				result["bytes"] = len(bpStr)
				b, _ := json.Marshal(result)
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{"blueprint": string(bpStr)}
			respRaw, status, err := c.PatchWithParams(cmd.Context(), "/scenarios/"+strconv.FormatInt(scenarioID, 10), nil, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if status < 200 || status >= 300 {
				return apiErr(fmt.Errorf("restore returned HTTP %d: %s", status, truncate(string(respRaw), 200)))
			}
			var inner any
			_ = json.Unmarshal(respRaw, &inner)
			result["response"] = inner
			b, _ := json.Marshal(result)
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&flagSnapshot, "snapshot", "", "Path to the blueprint JSON file to restore")
	return cmd
}
