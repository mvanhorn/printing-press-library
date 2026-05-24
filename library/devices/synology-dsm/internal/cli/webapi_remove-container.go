package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

func newWebapiRemoveContainerCmd(flags *rootFlags) *cobra.Command {
	var bodyApi string
	var bodyMethod string
	var bodyName string
	var bodyVersion string

	cmd := &cobra.Command{
		Use:         "remove-container",
		Short:       "Remove (delete) a stopped container",
		Example:     "  synology-dsm-pp-cli webapi remove-container --name my-container",
		Annotations: map[string]string{"pp:endpoint": "webapi.remove-container", "pp:method": "POST", "pp:path": "/webapi/entry.cgi/docker/container/delete"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("name") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "name")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/webapi/entry.cgi/docker/container/delete"
			params := map[string]string{}
			fields := url.Values{}
			if bodyApi != "" {
				fields.Set("api", bodyApi)
			}
			if bodyMethod != "" {
				fields.Set("method", bodyMethod)
			}
			if bodyName != "" {
				fields.Set("name", bodyName)
			}
			if bodyVersion != "" {
				fields.Set("version", bodyVersion)
			}

			data, statusCode, err := c.PostFormWithParams(path, params, fields)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
					} else {
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
							return nil
						}
					}
				}
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				if flags.quiet {
					return nil
				}
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				envelope := map[string]any{
					"action":   "post",
					"resource": "webapi",
					"path":     path,
					"status":   statusCode,
					"success":  statusCode >= 200 && statusCode < 300,
				}
				if flags.dryRun {
					envelope["dry_run"] = true
					envelope["status"] = 0
					envelope["success"] = false
				}
				if len(filtered) > 0 {
					var parsed any
					if err := json.Unmarshal(filtered, &parsed); err == nil {
						envelope["data"] = parsed
					}
				}
				envelopeJSON, err := json.Marshal(envelope)
				if err != nil {
					return err
				}
				return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&bodyApi, "api", "SYNO.Docker.Container", "DSM API name")
	cmd.Flags().StringVar(&bodyMethod, "method", "delete", "API method")
	cmd.Flags().StringVar(&bodyName, "name", "", "Container name (required)")
	cmd.Flags().StringVar(&bodyVersion, "version", "1", "API version")

	return cmd
}
