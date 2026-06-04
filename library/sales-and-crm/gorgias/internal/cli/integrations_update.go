// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newIntegrationsUpdateCmd(flags *rootFlags) *cobra.Command {
	var bodyName string
	var bodyDeactivatedDatetime string
	var bodyDescription string
	var bodyHttp string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "update <id>",
		Short:       "Update an integration's config (`id`).",
		Example:     "  gorgias-pp-cli integrations update 123456789 --name example-resource",
		Annotations: map[string]string{"pp:endpoint": "integrations.update", "pp:method": "PUT", "pp:path": "/integrations/{id}"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if !stdinBody {
				if !cmd.Flags().Changed("name") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "name")
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/integrations/{id}"
			path = replacePathParam(path, "id", args[0])
			var body map[string]any
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
				body = map[string]any{}
				if bodyName != "" {
					body["name"] = bodyName
				}
				if bodyDeactivatedDatetime != "" {
					body["deactivated_datetime"] = bodyDeactivatedDatetime
				}
				if bodyDescription != "" {
					body["description"] = bodyDescription
				}
				if bodyHttp != "" {
					var parsedHttp any
					if err := json.Unmarshal([]byte(bodyHttp), &parsedHttp); err != nil {
						return fmt.Errorf("parsing --http JSON: %w", err)
					}
					body["http"] = parsedHttp
				}
			}
			data, statusCode, err := c.Put(path, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				// Check if response contains an array (directly or wrapped in "data")
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
				// Apply --compact and --select to the API response before wrapping.
				// --select wins when both are set: explicit field choice trumps the
				// generic high-gravity allow-list. Otherwise --compact still applies
				// when --agent is on but the user did not name fields.
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				envelope := map[string]any{
					"action":   "put",
					"resource": "integrations",
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
	cmd.Flags().StringVar(&bodyName, "name", "", "Name of the integration. Usually the email address, Facebook page name, etc. (example: My HTTP integration)")
	cmd.Flags().StringVar(&bodyDeactivatedDatetime, "deactivated-datetime", "", "When the integration was deactivated.")
	cmd.Flags().StringVar(&bodyDescription, "description", "", "Description about the integration.")
	cmd.Flags().StringVar(&bodyHttp, "http", "", "Only available for HTTP integrations, defines the configuration of the integration.")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
