// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newCustomersCreateCmd(flags *rootFlags) *cobra.Command {
	var bodyChannels string
	var bodyEmail string
	var bodyExternalId string
	var bodyLanguage string
	var bodyName string
	var bodyTimezone string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "create",
		Short:       "Create a new customer record.",
		Example:     "  gorgias-pp-cli customers create",
		Annotations: map[string]string{"pp:endpoint": "customers.create", "pp:method": "POST", "pp:path": "/customers"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
				if !cmd.Flags().Changed("channels") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "channels")
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/customers"
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
				if bodyChannels != "" {
					parsed, err := parseBodyJSONField("channels", bodyChannels)
					if err != nil {
						return err
					}
					body["channels"] = parsed
				}
				if bodyEmail != "" {
					body["email"] = bodyEmail
				}
				if bodyExternalId != "" {
					body["external_id"] = bodyExternalId
				}
				if bodyLanguage != "" {
					body["language"] = bodyLanguage
				}
				if bodyName != "" {
					body["name"] = bodyName
				}
				if bodyTimezone != "" {
					body["timezone"] = bodyTimezone
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
					"resource": "customers",
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
	cmd.Flags().StringVar(&bodyChannels, "channels", "", "The [customer's contact channels](#the-customerchannel-object).")
	cmd.Flags().StringVar(&bodyEmail, "email", "", "Primary email address of the customer. (example: example-email-value)")
	cmd.Flags().StringVar(&bodyExternalId, "external-id", "", "ID of the customer in a foreign system (Stripe, Aircall, etc...).")
	cmd.Flags().StringVar(&bodyLanguage, "language", "", "The customer's preferred language (format: ISO_639-1). (ISO 639-1 code, e.g. en, fr, es, de, ja).")
	cmd.Flags().StringVar(&bodyName, "name", "", "Full name of the customer. (example: John Smith)")
	cmd.Flags().StringVar(&bodyTimezone, "timezone", "", "The customer's preferred timezone (format: IANA timezone name).")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
