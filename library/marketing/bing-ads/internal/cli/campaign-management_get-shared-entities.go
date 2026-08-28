// Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newCampaignManagementGetSharedEntitiesCmd(flags *rootFlags) *cobra.Command {
	var bodySharedEntityScope string
	var bodySharedEntityType string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "get-shared-entities",
		Short:       "get_shared_entities",
		Example:     "  bing-ads-pp-cli campaign-management get-shared-entities",
		Annotations: map[string]string{"pp:endpoint": "campaign-management.get-shared-entities", "pp:method": "POST", "pp:path": "/CampaignManagement/v13/SharedEntities/Query", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
			}
			path := "/CampaignManagement/v13/SharedEntities/Query"
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{}
			var body any
			if stdinBody {
				stdinData, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				var jsonBody map[string]any
				if err := json.Unmarshal(stdinData, &jsonBody); err != nil {
					return fmt.Errorf("parsing stdin JSON: %w", err)
				}
				body = jsonBody
			} else {
				bodyMap := map[string]any{}
				body = bodyMap
				if cmd.Flags().Changed("shared-entity-scope") || bodySharedEntityScope != "" {
					bodyMap["SharedEntityScope"] = bodySharedEntityScope
				}
				if cmd.Flags().Changed("shared-entity-type") || bodySharedEntityType != "" {
					bodyMap["SharedEntityType"] = bodySharedEntityType
				}
			}
			data, statusCode, err := c.PostQueryWithParams(cmd.Context(), path, params, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if isDryRunResponse(c.IsDryRun(), data) {
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "dry-run"}, nil)
				}
				return nil
			}
			_ = statusCode
			if !flags.dryRun {
				data = applyResponsePath(data, "SharedEntities")
			}
			outputData := data
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(outputData, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
					}
					return nil
				}
			}
			formatData := data
			if flags.csv || flags.plain {
				formatData = outputData
			}
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), formatData, flags, map[string]any{"source": "live"}, nil)
		},
	}
	cmd.Flags().StringVar(&bodySharedEntityScope, "shared-entity-scope", "", "Shared entity scope")
	cmd.Flags().StringVar(&bodySharedEntityType, "shared-entity-type", "", "Shared entity type")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
