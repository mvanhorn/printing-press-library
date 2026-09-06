// Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mvanhorn/printing-press-library/library/marketing/bing-ads/internal/cliutil"
	"github.com/spf13/cobra"
)

func newCampaignManagementGetLabelAssociationsByEntityIdsCmd(flags *rootFlags) *cobra.Command {
	var bodyEntityIds string
	var bodyEntityType string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "get-label-associations-by-entity-ids",
		Short:       "get_label_associations_by_entity_ids",
		Example:     "  bing-ads-pp-cli campaign-management get-label-associations-by-entity-ids",
		Annotations: map[string]string{"pp:endpoint": "campaign-management.get-label-associations-by-entity-ids", "pp:method": "POST", "pp:path": "/CampaignManagement/v13/LabelAssociations/QueryByEntityIds", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
			}
			path := "/CampaignManagement/v13/LabelAssociations/QueryByEntityIds"
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
				if cmd.Flags().Changed("entity-ids") {
					parsedEntityIds, parseErr := cliutil.ParseStringList(bodyEntityIds)
					if parseErr != nil {
						return fmt.Errorf("parsing --entity-ids list: %w", parseErr)
					}
					bodyMap["EntityIds"] = parsedEntityIds
				}
				if cmd.Flags().Changed("entity-type") || bodyEntityType != "" {
					bodyMap["EntityType"] = bodyEntityType
				}
			}
			data, statusCode, err := c.PostQueryWithParams(cmd.Context(), path, params, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if isDryRunResponse(c.IsDryRun(), data) {
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "dry-run"}, map[string]bool{"LabelAssociations": true, "PartialErrors": true})
				}
				return nil
			}
			_ = statusCode
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
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), formatData, flags, map[string]any{"source": "live"}, map[string]bool{"LabelAssociations": true, "PartialErrors": true})
		},
	}
	cmd.Flags().StringVar(&bodyEntityIds, "entity-ids", "", "Entity ids")
	cmd.Flags().StringVar(&bodyEntityType, "entity-type", "", "Entity type")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
