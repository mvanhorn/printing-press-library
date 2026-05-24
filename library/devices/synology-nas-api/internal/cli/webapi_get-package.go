package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newWebapiGetPackageCmd(flags *rootFlags) *cobra.Command {
	var flagApi string
	var flagVersion string
	var flagMethod string
	var flagPackage string

	cmd := &cobra.Command{
		Use:         "get-package",
		Short:       "Get info for a specific installed DSM package",
		Long:        `Shows package info by filtering the package list. The DSM API does not have a single-package GET endpoint, so this fetches the full list and filters client-side.`,
		Example:     "  synology-nas-api-pp-cli webapi get-package --package ContainerManager",
		Annotations: map[string]string{"pp:endpoint": "webapi.get-package", "pp:method": "GET", "pp:path": "/webapi/entry.cgi/core/package/get", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("package") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "package")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/webapi/entry.cgi/core/package/get"
			params := map[string]string{
				"api":     flagApi,
				"version": flagVersion,
				"method":  flagMethod,
				"limit":   "-1",
				"offset":  "0",
			}
			data, prov, err := resolveRead(cmd.Context(), c, flags, "webapi", false, path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var resp struct {
				Data struct {
					Packages []map[string]any `json:"packages"`
				} `json:"data"`
			}
			if json.Unmarshal(data, &resp) == nil {
				for _, p := range resp.Data.Packages {
					if fmt.Sprintf("%v", p["id"]) == flagPackage {
						filtered, _ := json.Marshal(p)
						if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
							wrapped, wrapErr := wrapWithProvenance(filtered, prov)
							if wrapErr != nil {
								return wrapErr
							}
							return printOutput(cmd.OutOrStdout(), wrapped, true)
						}
						return printOutputWithFlags(cmd.OutOrStdout(), filtered, flags)
					}
				}
				return fmt.Errorf("package %q not found", flagPackage)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				wrapped, wrapErr := wrapWithProvenance(data, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&flagApi, "api", "SYNO.Core.Package", "DSM API name")
	cmd.Flags().StringVar(&flagVersion, "version", "2", "API version")
	cmd.Flags().StringVar(&flagMethod, "method", "list", "API method")
	cmd.Flags().StringVar(&flagPackage, "package", "", "Package ID (e.g. ContainerManager, Tailscale) (required)")

	return cmd
}
