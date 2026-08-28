// Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newCustomerManagementGetCustomersInfoCmd(flags *rootFlags) *cobra.Command {
	var bodyCustomerNameFilter string
	var bodyTopN int
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "get-customers-info",
		Short:       "get_customers_info",
		Example:     "  bing-ads-pp-cli customer-management get-customers-info",
		Annotations: map[string]string{"pp:endpoint": "customer-management.get-customers-info", "pp:method": "POST", "pp:path": "/CustomerManagement/v13/CustomersInfo/Query", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
			}
			path := "/CustomerManagement/v13/CustomersInfo/Query"
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
				if cmd.Flags().Changed("customer-name-filter") || bodyCustomerNameFilter != "" {
					bodyMap["CustomerNameFilter"] = bodyCustomerNameFilter
				}
				if cmd.Flags().Changed("top-n") || bodyTopN != 0 {
					bodyMap["TopN"] = bodyTopN
				}
			}
			data, statusCode, err := c.PostQueryWithParams(cmd.Context(), path, params, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if isDryRunResponse(c.IsDryRun(), data) {
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "dry-run"}, map[string]bool{"Id": true, "Name": true})
				}
				return nil
			}
			_ = statusCode
			if !flags.dryRun {
				data = applyResponsePath(data, "CustomersInfo")
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
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), formatData, flags, map[string]any{"source": "live"}, map[string]bool{"Id": true, "Name": true})
		},
	}
	cmd.Flags().StringVar(&bodyCustomerNameFilter, "customer-name-filter", "", "Customer name filter")
	cmd.Flags().IntVar(&bodyTopN, "top-n", 0, "Top n")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
