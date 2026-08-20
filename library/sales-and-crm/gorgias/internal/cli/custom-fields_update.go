// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newCustomFieldsUpdateCmd(flags *rootFlags) *cobra.Command {
	var bodyExternalId string
	var bodyObjectType string
	var bodyLabel string
	var bodyDescription string
	var bodyPriority int
	var bodyRequired bool
	var bodyManagedType string
	var bodyDefinition string
	var bodyDeactivatedDatetime string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "update <id>",
		Short:       "Update one custom field definition by `id`.",
		Example:     "  gorgias-pp-cli custom-fields update 123456789",
		Annotations: map[string]string{"pp:endpoint": "custom-fields.update", "pp:method": "PUT", "pp:path": "/custom-fields/{id}"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/custom-fields/{id}"
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
				if bodyExternalId != "" {
					body["external_id"] = bodyExternalId
				}
				if bodyObjectType != "" {
					parsed, err := parseBodyJSONField("object_type", bodyObjectType)
					if err != nil {
						return err
					}
					body["object_type"] = parsed
				}
				if bodyLabel != "" {
					body["label"] = bodyLabel
				}
				if bodyDescription != "" {
					body["description"] = bodyDescription
				}
				if bodyPriority != 0 {
					body["priority"] = bodyPriority
				}
				if bodyRequired != false {
					body["required"] = bodyRequired
				}
				if bodyManagedType != "" {
					body["managed_type"] = bodyManagedType
				}
				if bodyDefinition != "" {
					parsed, err := parseBodyJSONField("definition", bodyDefinition)
					if err != nil {
						return err
					}
					body["definition"] = parsed
				}
				if bodyDeactivatedDatetime != "" {
					body["deactivated_datetime"] = bodyDeactivatedDatetime
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
					"resource": "custom-fields",
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
	cmd.Flags().StringVar(&bodyExternalId, "external-id", "", "ID of the custom field in a foreign system (ex: Zendesk). (example: custom-field-84203241)")
	cmd.Flags().StringVar(&bodyObjectType, "object-type", "", "Type of entity on which to use this custom field. (example: Ticket)")
	cmd.Flags().StringVar(&bodyLabel, "label", "", "The name of the custom field. (example: Test field)")
	cmd.Flags().StringVar(&bodyDescription, "description", "", "The description of the custom field. (example: An amazing field description.)")
	cmd.Flags().IntVar(&bodyPriority, "priority", 0, "Order in which custom fields are displayed. (example: 1)")
	cmd.Flags().BoolVar(&bodyRequired, "required", false, "Whether this custom field is required. (example: True)")
	cmd.Flags().StringVar(&bodyManagedType, "managed-type", "", "The type of the managed field. (example: contact_reason)")
	cmd.Flags().StringVar(&bodyDefinition, "definition", "", "The settings for this custom field, dependent on the data type.")
	cmd.Flags().StringVar(&bodyDeactivatedDatetime, "deactivated-datetime", "", "When the custom field was deactivated. (example: 2021-01-02T03:04:05.123456)")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
