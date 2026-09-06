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

func newCampaignManagementGetKeywordsByIdsCmd(flags *rootFlags) *cobra.Command {
	var bodyAdGroupId string
	var bodyKeywordIds string
	var bodyReturnAdditionalFields string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "get-keywords-by-ids",
		Short:       "get_keywords_by_ids",
		Example:     "  bing-ads-pp-cli campaign-management get-keywords-by-ids",
		Annotations: map[string]string{"pp:endpoint": "campaign-management.get-keywords-by-ids", "pp:method": "POST", "pp:path": "/CampaignManagement/v13/Keywords/QueryByIds", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
			}
			path := "/CampaignManagement/v13/Keywords/QueryByIds"
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
				if cmd.Flags().Changed("ad-group-id") || bodyAdGroupId != "" {
					bodyMap["AdGroupId"] = bodyAdGroupId
				}
				if cmd.Flags().Changed("keyword-ids") {
					parsedKeywordIds, parseErr := cliutil.ParseStringList(bodyKeywordIds)
					if parseErr != nil {
						return fmt.Errorf("parsing --keyword-ids list: %w", parseErr)
					}
					bodyMap["KeywordIds"] = parsedKeywordIds
				}
				if cmd.Flags().Changed("return-additional-fields") || bodyReturnAdditionalFields != "" {
					var parsedReturnAdditionalFields any
					if err := json.Unmarshal([]byte(bodyReturnAdditionalFields), &parsedReturnAdditionalFields); err != nil {
						return fmt.Errorf("parsing --return-additional-fields JSON: %w", err)
					}
					asMap, ok := parsedReturnAdditionalFields.(map[string]any)
					if !ok {
						return fmt.Errorf("--return-additional-fields must be a JSON object, got JSON %T", parsedReturnAdditionalFields)
					}
					bodyMap["ReturnAdditionalFields"] = asMap
				}
			}
			data, statusCode, err := c.PostQueryWithParams(cmd.Context(), path, params, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if isDryRunResponse(c.IsDryRun(), data) {
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "dry-run"}, map[string]bool{"Keywords": true, "PartialErrors": true})
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
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), formatData, flags, map[string]any{"source": "live"}, map[string]bool{"Keywords": true, "PartialErrors": true})
		},
	}
	cmd.Flags().StringVar(&bodyAdGroupId, "ad-group-id", "", "Ad group id")
	cmd.Flags().StringVar(&bodyKeywordIds, "keyword-ids", "", "Keyword ids")
	cmd.Flags().StringVar(&bodyReturnAdditionalFields, "return-additional-fields", "", "Return additional fields")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
