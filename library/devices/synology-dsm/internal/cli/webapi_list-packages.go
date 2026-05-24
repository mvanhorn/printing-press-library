package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newWebapiListPackagesCmd(flags *rootFlags) *cobra.Command {
	var flagApi string
	var flagVersion string
	var flagMethod string
	var flagLimit int
	var flagOffset string
	var flagAll bool

	cmd := &cobra.Command{
		Use:         "list-packages",
		Short:       "List all installed DSM packages with version and status",
		Example:     "  synology-dsm-pp-cli webapi list-packages",
		Annotations: map[string]string{"pp:endpoint": "webapi.list-packages", "pp:method": "GET", "pp:path": "/webapi/entry.cgi/core/package/list", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/webapi/entry.cgi/core/package/list"
			data, prov, err := resolvePaginatedRead(cmd.Context(), c, flags, "webapi", path, map[string]string{
				"api":     fmt.Sprintf("%v", flagApi),
				"version": fmt.Sprintf("%v", flagVersion),
				"method":  fmt.Sprintf("%v", flagMethod),
				"limit":   fmt.Sprintf("%v", flagLimit),
				"offset":  fmt.Sprintf("%v", flagOffset),
			}, nil, flagAll, "offset", "", "")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var countItems []json.RawMessage
				_ = json.Unmarshal(data, &countItems)
				printProvenance(cmd, len(countItems), prov)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
					}
					return nil
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&flagApi, "api", "SYNO.Core.Package", "DSM API name")
	cmd.Flags().StringVar(&flagVersion, "version", "2", "API version")
	cmd.Flags().StringVar(&flagMethod, "method", "list", "API method")
	cmd.Flags().IntVar(&flagLimit, "limit", -1, "Max results (-1 for all)")
	cmd.Flags().StringVar(&flagOffset, "offset", "0", "Offset")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Fetch all pages")

	return cmd
}
