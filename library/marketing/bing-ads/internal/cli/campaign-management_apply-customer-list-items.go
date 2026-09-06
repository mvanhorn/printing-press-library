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

func newCampaignManagementApplyCustomerListItemsCmd(flags *rootFlags) *cobra.Command {
	var bodyCustomerListAudienceAudienceNetworkSize string
	var bodyCustomerListAudienceCustomerShareCustomerAccountShares string
	var bodyCustomerListAudienceCustomerShareOwnerCustomerId string
	var bodyCustomerListAudienceDescription string
	var bodyCustomerListAudienceForwardCompatibilityMap string
	var bodyCustomerListAudienceId string
	var bodyCustomerListAudienceMembershipDuration int
	var bodyCustomerListAudienceName string
	var bodyCustomerListAudienceParentId string
	var bodyCustomerListAudienceScope string
	var bodyCustomerListAudienceSearchSize string
	var bodyCustomerListAudienceSupportedCampaignTypes string
	var bodyCustomerListAudienceType string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "apply-customer-list-items",
		Short:       "apply_customer_list_items",
		Example:     "  bing-ads-pp-cli campaign-management apply-customer-list-items",
		Annotations: map[string]string{"pp:endpoint": "campaign-management.apply-customer-list-items", "pp:method": "POST", "pp:path": "/CampaignManagement/v13/CustomerListItems/Apply"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
			}
			path := "/CampaignManagement/v13/CustomerListItems/Apply"
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
				body = map[string]any{"CustomerListAudience": bodyMap}
				if cmd.Flags().Changed("customer-list-audience-audience-network-size") || bodyCustomerListAudienceAudienceNetworkSize != "" {
					bodyMap["AudienceNetworkSize"] = bodyCustomerListAudienceAudienceNetworkSize
				}
				{
					nestedCustomerListAudienceCustomerShare := map[string]any{}
					if cmd.Flags().Changed("customer-list-audience-customer-share-customer-account-shares") || bodyCustomerListAudienceCustomerShareCustomerAccountShares != "" {
						var parsedCustomerListAudienceCustomerShareCustomerAccountShares any
						if err := json.Unmarshal([]byte(bodyCustomerListAudienceCustomerShareCustomerAccountShares), &parsedCustomerListAudienceCustomerShareCustomerAccountShares); err != nil {
							return fmt.Errorf("parsing --customer-list-audience-customer-share-customer-account-shares JSON: %w", err)
						}
						asArray, ok := parsedCustomerListAudienceCustomerShareCustomerAccountShares.([]any)
						if !ok {
							return fmt.Errorf("--customer-list-audience-customer-share-customer-account-shares must be a JSON array, got JSON %T", parsedCustomerListAudienceCustomerShareCustomerAccountShares)
						}
						nestedCustomerListAudienceCustomerShare["CustomerAccountShares"] = asArray
					}
					if cmd.Flags().Changed("customer-list-audience-customer-share-owner-customer-id") || bodyCustomerListAudienceCustomerShareOwnerCustomerId != "" {
						nestedCustomerListAudienceCustomerShare["OwnerCustomerId"] = bodyCustomerListAudienceCustomerShareOwnerCustomerId
					}
					if len(nestedCustomerListAudienceCustomerShare) > 0 {
						bodyMap["CustomerShare"] = nestedCustomerListAudienceCustomerShare
					}
				}
				if cmd.Flags().Changed("customer-list-audience-description") || bodyCustomerListAudienceDescription != "" {
					bodyMap["Description"] = bodyCustomerListAudienceDescription
				}
				if cmd.Flags().Changed("customer-list-audience-forward-compatibility-map") || bodyCustomerListAudienceForwardCompatibilityMap != "" {
					var parsedCustomerListAudienceForwardCompatibilityMap any
					if err := json.Unmarshal([]byte(bodyCustomerListAudienceForwardCompatibilityMap), &parsedCustomerListAudienceForwardCompatibilityMap); err != nil {
						return fmt.Errorf("parsing --customer-list-audience-forward-compatibility-map JSON: %w", err)
					}
					asArray, ok := parsedCustomerListAudienceForwardCompatibilityMap.([]any)
					if !ok {
						return fmt.Errorf("--customer-list-audience-forward-compatibility-map must be a JSON array, got JSON %T", parsedCustomerListAudienceForwardCompatibilityMap)
					}
					bodyMap["ForwardCompatibilityMap"] = asArray
				}
				if cmd.Flags().Changed("customer-list-audience-id") || bodyCustomerListAudienceId != "" {
					bodyMap["Id"] = bodyCustomerListAudienceId
				}
				if cmd.Flags().Changed("customer-list-audience-membership-duration") || bodyCustomerListAudienceMembershipDuration != 0 {
					bodyMap["MembershipDuration"] = bodyCustomerListAudienceMembershipDuration
				}
				if cmd.Flags().Changed("customer-list-audience-name") || bodyCustomerListAudienceName != "" {
					bodyMap["Name"] = bodyCustomerListAudienceName
				}
				if cmd.Flags().Changed("customer-list-audience-parent-id") || bodyCustomerListAudienceParentId != "" {
					bodyMap["ParentId"] = bodyCustomerListAudienceParentId
				}
				if cmd.Flags().Changed("customer-list-audience-scope") || bodyCustomerListAudienceScope != "" {
					bodyMap["Scope"] = bodyCustomerListAudienceScope
				}
				if cmd.Flags().Changed("customer-list-audience-search-size") || bodyCustomerListAudienceSearchSize != "" {
					bodyMap["SearchSize"] = bodyCustomerListAudienceSearchSize
				}
				if cmd.Flags().Changed("customer-list-audience-supported-campaign-types") {
					parsedCustomerListAudienceSupportedCampaignTypes, parseErr := cliutil.ParseStringList(bodyCustomerListAudienceSupportedCampaignTypes)
					if parseErr != nil {
						return fmt.Errorf("parsing --customer-list-audience-supported-campaign-types list: %w", parseErr)
					}
					bodyMap["SupportedCampaignTypes"] = parsedCustomerListAudienceSupportedCampaignTypes
				}
				if cmd.Flags().Changed("customer-list-audience-type") || bodyCustomerListAudienceType != "" {
					var parsedCustomerListAudienceType any
					if err := json.Unmarshal([]byte(bodyCustomerListAudienceType), &parsedCustomerListAudienceType); err != nil {
						return fmt.Errorf("parsing --customer-list-audience-type JSON: %w", err)
					}
					asMap, ok := parsedCustomerListAudienceType.(map[string]any)
					if !ok {
						return fmt.Errorf("--customer-list-audience-type must be a JSON object, got JSON %T", parsedCustomerListAudienceType)
					}
					bodyMap["Type"] = asMap
				}
			}
			data, statusCode, err := c.PostWithParams(cmd.Context(), path, params, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			// Inspect the mutate response body for a partial-failure-shaped
			// field (e.g. Google Ads `partialFailureError`). Several Google
			// APIs return 200 OK with a partial-failure field when some
			// operations in the batch failed; ignoring it silently swallows
			// real failures. Detection runs before output-mode selection so
			// the exit code is consistent regardless of how stdout is
			// rendered. --dry-run short-circuits because no real request
			// was sent.
			var partialFailure *partialFailureReport
			if !flags.dryRun && statusCode >= 200 && statusCode < 300 {
				partialFailure = detectPartialFailure(data)
				if partialFailure != nil {
					fmt.Fprintf(os.Stderr, "warning: partial failure detected in %s response: %s\n", "campaign-management", partialFailure.Message)
					if len(partialFailure.ResourceNames) > 0 {
						fmt.Fprintf(os.Stderr, "         succeeded: %d operation(s)\n", len(partialFailure.ResourceNames))
					}
				}
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				// Check if response contains an array (directly or wrapped in "data")
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
					} else {
						if partialFailure != nil && !flags.allowPartialFailure {
							return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "campaign-management", partialFailure.Message))
						}
						return nil
					}
				} else {
					var wrapped struct {
						Data []map[string]any `json:"data"`
					}
					if json.Unmarshal(data, &wrapped) == nil && len(wrapped.Data) > 0 {
						if err := printAutoTable(cmd.OutOrStdout(), wrapped.Data); err != nil {
							fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
						} else {
							if partialFailure != nil && !flags.allowPartialFailure {
								return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "campaign-management", partialFailure.Message))
							}
							return nil
						}
					}
				}
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				if flags.quiet {
					if partialFailure != nil && !flags.allowPartialFailure {
						return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "campaign-management", partialFailure.Message))
					}
					return nil
				}
				envelope := map[string]any{
					"action":   "post",
					"resource": "campaign-management",
					"path":     path,
					"status":   statusCode,
					"success":  statusCode >= 200 && statusCode < 300 && (partialFailure == nil || flags.allowPartialFailure),
				}
				if flags.agent {
					envelope["meta"] = map[string]any{"source": "live"}
				}
				if partialFailure != nil {
					envelope["partial_failure"] = partialFailure
				}
				if flags.dryRun {
					envelope["dry_run"] = true
					envelope["status"] = 0
					envelope["success"] = false
				}
				// Verify-mode synthetic envelope detection runs against RAW data
				// (before --compact/--select filtering) so the sentinel field is
				// guaranteed to be visible even if the operator passes a filter
				// flag that would otherwise strip it. Surfaces a top-level
				// verify_noop signal + flips success to false. Mirrors the dry_run
				// shape above.
				if len(data) > 0 {
					var rawParsed any
					if err := json.Unmarshal(data, &rawParsed); err == nil {
						if m, ok := rawParsed.(map[string]any); ok {
							if v, ok := m["__pp_verify_synthetic__"].(bool); ok && v {
								envelope["verify_noop"] = true
								envelope["success"] = false
							}
						}
					}
				}
				// Apply --compact and --select to the API response before wrapping.
				// --select wins when both are set: explicit field choice trumps the
				// generic high-gravity allow-list. Otherwise --compact still applies
				// when --agent is on but the user did not name fields.
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered, nil)
				}
				if len(filtered) > 0 {
					var parsed any
					if err := json.Unmarshal(filtered, &parsed); err == nil {
						if flags.agent {
							envelope["results"] = parsed
						} else {
							envelope["data"] = parsed
						}
					}
				}
				envelopeJSON, err := json.Marshal(envelope)
				if err != nil {
					return err
				}
				resultKey := "data"
				if flags.agent {
					resultKey = "results"
				}
				structured, err := wrapPlatformStructuredOutput(json.RawMessage(envelopeJSON), flags, resultKey, true)
				if err != nil {
					return err
				}
				if perr := printOutput(cmd.OutOrStdout(), structured, true); perr != nil {
					return perr
				}
				if partialFailure != nil && !flags.allowPartialFailure {
					return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "campaign-management", partialFailure.Message))
				}
				return nil
			}
			// Fall-through for mutate paths that did not hit the table or
			// asJSON branches: --quiet, --csv, --plain, and default terminal
			// raw output. printOutputWithFlags renders the body, then the
			// typed partial-failure exit fires unless --allow-partial-failure
			// downgrades it. Without this guard a partial failure would exit
			// 0 for these output modes — the exact silent-swallow regression
			// the surrounding patch is preventing for asJSON / piped output.
			if perr := printOutputWithFlags(cmd.OutOrStdout(), data, flags); perr != nil {
				return perr
			}
			if partialFailure != nil && !flags.allowPartialFailure {
				return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "campaign-management", partialFailure.Message))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&bodyCustomerListAudienceAudienceNetworkSize, "customer-list-audience-audience-network-size", "", "Audience network size")
	cmd.Flags().StringVar(&bodyCustomerListAudienceCustomerShareCustomerAccountShares, "customer-list-audience-customer-share-customer-account-shares", "", "Customer account shares")
	cmd.Flags().StringVar(&bodyCustomerListAudienceCustomerShareOwnerCustomerId, "customer-list-audience-customer-share-owner-customer-id", "", "Owner customer id")
	cmd.Flags().StringVar(&bodyCustomerListAudienceDescription, "customer-list-audience-description", "", "Description")
	cmd.Flags().StringVar(&bodyCustomerListAudienceForwardCompatibilityMap, "customer-list-audience-forward-compatibility-map", "", "Forward compatibility map")
	cmd.Flags().StringVar(&bodyCustomerListAudienceId, "customer-list-audience-id", "", "Id")
	cmd.Flags().IntVar(&bodyCustomerListAudienceMembershipDuration, "customer-list-audience-membership-duration", 0, "Membership duration")
	cmd.Flags().StringVar(&bodyCustomerListAudienceName, "customer-list-audience-name", "", "Name")
	cmd.Flags().StringVar(&bodyCustomerListAudienceParentId, "customer-list-audience-parent-id", "", "Parent id")
	cmd.Flags().StringVar(&bodyCustomerListAudienceScope, "customer-list-audience-scope", "", "Scope")
	cmd.Flags().StringVar(&bodyCustomerListAudienceSearchSize, "customer-list-audience-search-size", "", "Search size")
	cmd.Flags().StringVar(&bodyCustomerListAudienceSupportedCampaignTypes, "customer-list-audience-supported-campaign-types", "", "Supported campaign types")
	cmd.Flags().StringVar(&bodyCustomerListAudienceType, "customer-list-audience-type", "", "Type")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
