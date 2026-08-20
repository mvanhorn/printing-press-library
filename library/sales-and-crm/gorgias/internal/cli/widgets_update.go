// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newWidgetsUpdateCmd(flags *rootFlags) *cobra.Command {
	var bodyContext string
	var bodyDeactivatedDatetime string
	var bodyIntegrationId string
	var bodyAppId string
	var bodyOrder int
	var bodyTemplate string
	var bodyType string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "update <id>",
		Short:       "Update a sidebar widget (`id`).",
		Example:     "  gorgias-pp-cli widgets update 123456789",
		Annotations: map[string]string{"pp:endpoint": "widgets.update", "pp:method": "PUT", "pp:path": "/widgets/{id}"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/widgets/{id}"
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
				if bodyContext != "" {
					body["context"] = bodyContext
				}
				if bodyDeactivatedDatetime != "" {
					body["deactivated_datetime"] = bodyDeactivatedDatetime
				}
				if bodyIntegrationId != "" {
					body["integration_id"] = bodyIntegrationId
				}
				if bodyAppId != "" {
					body["app_id"] = bodyAppId
				}
				if bodyOrder != 0 {
					body["order"] = bodyOrder
				}
				if bodyTemplate != "" {
					var parsedTemplate any
					if err := json.Unmarshal([]byte(bodyTemplate), &parsedTemplate); err != nil {
						return fmt.Errorf("parsing --template JSON: %w", err)
					}
					body["template"] = parsedTemplate
				}
				if bodyType != "" {
					body["type"] = bodyType
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
					"resource": "widgets",
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
	cmd.Flags().StringVar(&bodyContext, "context", "", "The context to display this widget in. (enum: ticket, customer, user) (example: ticket)")
	cmd.Flags().StringVar(&bodyDeactivatedDatetime, "deactivated-datetime", "", "When the widget was deactivated. (example: 2020-12-03T13:00:00.123456)")
	cmd.Flags().StringVar(&bodyIntegrationId, "integration-id", "", "ID of the integration that the widget's data is attached to.")
	cmd.Flags().StringVar(&bodyAppId, "app-id", "", "The ID of the 3rd party app that  the widget's data is attached to.")
	cmd.Flags().IntVar(&bodyOrder, "order", 0, "Order of precedence of the widget. Widgets with lower order are shown first. (example: 3)")
	cmd.Flags().StringVar(&bodyTemplate, "template", "", "Template to render the data of the widget.")
	cmd.Flags().StringVar(&bodyType, "type", "", "Type of data the widget is attached to.")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
