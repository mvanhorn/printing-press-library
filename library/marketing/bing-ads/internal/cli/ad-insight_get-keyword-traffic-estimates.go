// Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newAdInsightGetKeywordTrafficEstimatesCmd(flags *rootFlags) *cobra.Command {
	var bodyCampaignEstimators string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "get-keyword-traffic-estimates",
		Short:       "get_keyword_traffic_estimates",
		Example:     "  bing-ads-pp-cli ad-insight get-keyword-traffic-estimates",
		Annotations: map[string]string{"pp:endpoint": "ad-insight.get-keyword-traffic-estimates", "pp:method": "POST", "pp:path": "/AdInsight/v13/KeywordTrafficEstimates/query", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
			}
			path := "/AdInsight/v13/KeywordTrafficEstimates/query"
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
				if cmd.Flags().Changed("campaign-estimators") || bodyCampaignEstimators != "" {
					var parsedCampaignEstimators any
					if err := json.Unmarshal([]byte(bodyCampaignEstimators), &parsedCampaignEstimators); err != nil {
						return fmt.Errorf("parsing --campaign-estimators JSON: %w", err)
					}
					asArray, ok := parsedCampaignEstimators.([]any)
					if !ok {
						return fmt.Errorf("--campaign-estimators must be a JSON array, got JSON %T", parsedCampaignEstimators)
					}
					bodyMap["CampaignEstimators"] = asArray
				}
			}
			data, statusCode, err := c.PostQueryWithParams(cmd.Context(), path, params, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if isDryRunResponse(c.IsDryRun(), data) {
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "dry-run"}, map[string]bool{"AdGroupEstimates": true, "CampaignId": true})
				}
				return nil
			}
			_ = statusCode
			if !flags.dryRun {
				data = applyResponsePath(data, "CampaignEstimates")
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
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), formatData, flags, map[string]any{"source": "live"}, map[string]bool{"AdGroupEstimates": true, "CampaignId": true})
		},
	}
	cmd.Flags().StringVar(&bodyCampaignEstimators, "campaign-estimators", "", "Campaign estimators")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
