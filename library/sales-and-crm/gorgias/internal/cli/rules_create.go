// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newRulesCreateCmd(flags *rootFlags) *cobra.Command {
	var bodyCode string
	var bodyName string
	var bodyCodeAst string
	var bodyDeactivatedDatetime string
	var bodyDescription string
	var bodyEventTypes string
	var bodyPriority int
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "create",
		Short:       "Create a new automation rule.",
		Example:     "  gorgias-pp-cli rules create --code example-value",
		Annotations: map[string]string{"pp:endpoint": "rules.create", "pp:method": "POST", "pp:path": "/rules"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
				if !cmd.Flags().Changed("code") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "code")
				}
				if !cmd.Flags().Changed("name") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "name")
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/rules"
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
				if bodyCode != "" {
					body["code"] = bodyCode
				}
				if bodyName != "" {
					body["name"] = bodyName
				}
				if bodyCodeAst != "" {
					var parsedCodeAst any
					if err := json.Unmarshal([]byte(bodyCodeAst), &parsedCodeAst); err != nil {
						return fmt.Errorf("parsing --code-ast JSON: %w", err)
					}
					body["code_ast"] = parsedCodeAst
				}
				if bodyDeactivatedDatetime != "" {
					body["deactivated_datetime"] = bodyDeactivatedDatetime
				}
				if bodyDescription != "" {
					body["description"] = bodyDescription
				}
				if bodyEventTypes != "" {
					body["event_types"] = bodyEventTypes
				}
				if bodyPriority != 0 {
					body["priority"] = bodyPriority
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
					"resource": "rules",
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
	cmd.Flags().StringVar(&bodyCode, "code", "", "The logic of the rule (as JavaScript code).")
	cmd.Flags().StringVar(&bodyName, "name", "", "The name of the rule. (example: Auto-close all tickets from Amazon)")
	cmd.Flags().StringVar(&bodyCodeAst, "code-ast", "", "The logic of the rule (as an ESTree [AST](https://en.wikipedia.org/wiki/Abstract_syntax_tree) representation).")
	cmd.Flags().StringVar(&bodyDeactivatedDatetime, "deactivated-datetime", "", "When the rule was deactivated. (example: 2019-11-28T15:59:41.966927)")
	cmd.Flags().StringVar(&bodyDescription, "description", "", "The description of the rule. (example: Automatically close all tickets from Amazon because there is nothing to do fo...)")
	cmd.Flags().StringVar(&bodyEventTypes, "event-types", "", "A list of comma separated events that this rule will be executed on.")
	cmd.Flags().IntVar(&bodyPriority, "priority", 0, "Order of execution of the rule. Rules with higher priorities are executed first. (example: 100)")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
