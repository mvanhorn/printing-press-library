// Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newCampaignManagementGetCampaignsByAccountIdCmd(flags *rootFlags) *cobra.Command {
	var bodyAccountId string
	var bodyCampaignType string
	var bodyReturnAdditionalFields string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "get-campaigns-by-account-id",
		Short:       "get_campaigns_by_account_id",
		Example:     "  bing-ads-pp-cli campaign-management get-campaigns-by-account-id",
		Annotations: map[string]string{"pp:endpoint": "campaign-management.get-campaigns-by-account-id", "pp:method": "POST", "pp:path": "/CampaignManagement/v13/Campaigns/QueryByAccountId", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
			}
			path := "/CampaignManagement/v13/Campaigns/QueryByAccountId"
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
				if cmd.Flags().Changed("account-id") || bodyAccountId != "" {
					bodyMap["AccountId"] = bodyAccountId
				}
				if cmd.Flags().Changed("campaign-type") || bodyCampaignType != "" {
					var parsedCampaignType any
					if err := json.Unmarshal([]byte(bodyCampaignType), &parsedCampaignType); err != nil {
						return fmt.Errorf("parsing --campaign-type JSON: %w", err)
					}
					asMap, ok := parsedCampaignType.(map[string]any)
					if !ok {
						return fmt.Errorf("--campaign-type must be a JSON object, got JSON %T", parsedCampaignType)
					}
					bodyMap["CampaignType"] = asMap
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
					return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "dry-run"}, map[string]bool{"AdScheduleUseSearcherTimeZone": true, "AudienceAdsBidAdjustment": true, "BidStrategyId": true, "BidStrategyScope": true, "BiddingScheme": true, "BudgetId": true, "BudgetType": true, "CampaignType": true, "DailyBudget": true, "DealIds": true, "EndDate": true, "ExperimentId": true, "FinalUrlSuffix": true, "ForwardCompatibilityMap": true, "GoalIds": true, "Id": true, "IsDealCampaign": true, "IsPolitical": true, "Languages": true, "MultimediaAdsBidAdjustment": true, "Name": true, "Settings": true, "StartDate": true, "Status": true, "SubType": true, "TimeZone": true, "TrackingUrlTemplate": true, "UrlCustomParameters": true, "UseCampaignLevelDates": true})
				}
				return nil
			}
			_ = statusCode
			if !flags.dryRun {
				data = applyResponsePath(data, "Campaigns")
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
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), formatData, flags, map[string]any{"source": "live"}, map[string]bool{"AdScheduleUseSearcherTimeZone": true, "AudienceAdsBidAdjustment": true, "BidStrategyId": true, "BidStrategyScope": true, "BiddingScheme": true, "BudgetId": true, "BudgetType": true, "CampaignType": true, "DailyBudget": true, "DealIds": true, "EndDate": true, "ExperimentId": true, "FinalUrlSuffix": true, "ForwardCompatibilityMap": true, "GoalIds": true, "Id": true, "IsDealCampaign": true, "IsPolitical": true, "Languages": true, "MultimediaAdsBidAdjustment": true, "Name": true, "Settings": true, "StartDate": true, "Status": true, "SubType": true, "TimeZone": true, "TrackingUrlTemplate": true, "UrlCustomParameters": true, "UseCampaignLevelDates": true})
		},
	}
	cmd.Flags().StringVar(&bodyAccountId, "account-id", "", "Account id")
	cmd.Flags().StringVar(&bodyCampaignType, "campaign-type", "", "Campaign type")
	cmd.Flags().StringVar(&bodyReturnAdditionalFields, "return-additional-fields", "", "Return additional fields")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
