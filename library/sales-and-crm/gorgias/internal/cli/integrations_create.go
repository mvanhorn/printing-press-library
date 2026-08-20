// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newIntegrationsCreateCmd(flags *rootFlags) *cobra.Command {
	var bodyName string
	var bodyType string
	var bodyDeactivatedDatetime string
	var bodyDescription string
	var bodyHttp string
	var bodyManaged bool
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "create",
		Short:       "Install a new third-party integration on the Gorgias account.",
		Example:     "  gorgias-pp-cli integrations create --name example-resource",
		Annotations: map[string]string{"pp:endpoint": "integrations.create", "pp:method": "POST", "pp:path": "/integrations"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
				if !cmd.Flags().Changed("name") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "name")
				}
				if !cmd.Flags().Changed("type") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "type")
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/integrations"
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
				if bodyType != "" {
					body["type"] = bodyType
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
				if bodyManaged != false {
					body["managed"] = bodyManaged
				}
			}
			data, statusCode, err := c.Post(path, body)
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
					"action":   "post",
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
	cmd.Flags().StringVar(&bodyType, "type", "", "Type of integration. spec-sources/gorgias-crowd.yaml lists only 'http'; the live Gorgias API also supports shopify, magento, instagram, facebook, sms, and more — see the Gorgias integrations docs for the full enum.")
	cmd.Flags().StringVar(&bodyDeactivatedDatetime, "deactivated-datetime", "", "When the integration was deactivated.")
	cmd.Flags().StringVar(&bodyDescription, "description", "", "Description about the integration.")
	cmd.Flags().StringVar(&bodyHttp, "http", "", "Only available for HTTP integrations, defines the configuration of the integration.")
	cmd.Flags().BoolVar(&bodyManaged, "managed", false, "Whether integration is managed by Gorgias.")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
