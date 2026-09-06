// Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newCustomerBillingGetBillingDocumentsCmd(flags *rootFlags) *cobra.Command {
	var bodyBillingDocumentsInfo string
	var bodyType string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "get-billing-documents",
		Short:       "get_billing_documents",
		Example:     "  bing-ads-pp-cli customer-billing get-billing-documents",
		Annotations: map[string]string{"pp:endpoint": "customer-billing.get-billing-documents", "pp:method": "POST", "pp:path": "/CustomerBilling/v13/BillingDocuments/Query", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
			}
			path := "/CustomerBilling/v13/BillingDocuments/Query"
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
				if cmd.Flags().Changed("billing-documents-info") || bodyBillingDocumentsInfo != "" {
					var parsedBillingDocumentsInfo any
					if err := json.Unmarshal([]byte(bodyBillingDocumentsInfo), &parsedBillingDocumentsInfo); err != nil {
						return fmt.Errorf("parsing --billing-documents-info JSON: %w", err)
					}
					asArray, ok := parsedBillingDocumentsInfo.([]any)
					if !ok {
						return fmt.Errorf("--billing-documents-info must be a JSON array, got JSON %T", parsedBillingDocumentsInfo)
					}
					bodyMap["BillingDocumentsInfo"] = asArray
				}
				if cmd.Flags().Changed("type") || bodyType != "" {
					bodyMap["Type"] = bodyType
				}
			}
			data, statusCode, err := c.PostQueryWithParams(cmd.Context(), path, params, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if isDryRunResponse(c.IsDryRun(), data) {
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "dry-run"}, map[string]bool{"Data": true, "Id": true, "Number": true, "Type": true})
				}
				return nil
			}
			_ = statusCode
			if !flags.dryRun {
				data = applyResponsePath(data, "BillingDocuments")
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
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), formatData, flags, map[string]any{"source": "live"}, map[string]bool{"Data": true, "Id": true, "Number": true, "Type": true})
		},
	}
	cmd.Flags().StringVar(&bodyBillingDocumentsInfo, "billing-documents-info", "", "Billing documents info")
	cmd.Flags().StringVar(&bodyType, "type", "", "Type")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
