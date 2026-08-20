// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newMacrosUpdateCmd(flags *rootFlags) *cobra.Command {
	var bodyExternalId string
	var bodyName string
	var bodyIntent string
	var bodyLanguage string
	var bodyActions string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "update <id>",
		Short:       "Update a macro (`id`) — edit its body, variables, tags-to-add, or action list.",
		Example:     "  gorgias-pp-cli macros update 123456789",
		Annotations: map[string]string{"pp:endpoint": "macros.update", "pp:method": "PUT", "pp:path": "/macros/{id}"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/macros/{id}"
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
				if bodyName != "" {
					body["name"] = bodyName
				}
				if bodyIntent != "" {
					body["intent"] = bodyIntent
				}
				if bodyLanguage != "" {
					body["language"] = bodyLanguage
				}
				if bodyActions != "" {
					parsed, err := parseBodyJSONField("actions", bodyActions)
					if err != nil {
						return err
					}
					body["actions"] = parsed
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
					"resource": "macros",
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
	cmd.Flags().StringVar(&bodyExternalId, "external-id", "", "External ID of the macro in a foreign system.")
	cmd.Flags().StringVar(&bodyName, "name", "", "The name of the Macro. Tips: choose a name that can be easily searched. (example: Order status inquiry)")
	cmd.Flags().StringVar(&bodyIntent, "intent", "", "The intention of the macro should be used for.")
	cmd.Flags().StringVar(&bodyLanguage, "language", "", "The language of the macro in ISO 639-1 format. (example: en)")
	cmd.Flags().StringVar(&bodyActions, "actions", "", "A list of [actions](#the-macroaction-object) to be applied on tickets.")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
