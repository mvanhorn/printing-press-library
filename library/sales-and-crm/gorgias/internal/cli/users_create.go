// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newUsersCreateCmd(flags *rootFlags) *cobra.Command {
	var bodyEmail string
	var bodyName string
	var bodyRole string
	var bodyBio string
	var bodyCountry string
	var bodyExternalId string
	var bodyLanguage string
	var bodyMeta string
	var bodyTimezone string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "create",
		Short:       "Create a new user (Gorgias agent/operator).",
		Example:     "  gorgias-pp-cli users create --email customer-lookup-placeholder",
		Annotations: map[string]string{"pp:endpoint": "users.create", "pp:method": "POST", "pp:path": "/users"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
				if !cmd.Flags().Changed("email") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "email")
				}
				if !cmd.Flags().Changed("name") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "name")
				}
				if !cmd.Flags().Changed("role") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "role")
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/users"
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
				if bodyEmail != "" {
					body["email"] = bodyEmail
				}
				if bodyName != "" {
					body["name"] = bodyName
				}
				if bodyRole != "" {
					var parsedRole any
					if err := json.Unmarshal([]byte(bodyRole), &parsedRole); err != nil {
						return fmt.Errorf("parsing --role JSON: %w", err)
					}
					body["role"] = parsedRole
				}
				if bodyBio != "" {
					body["bio"] = bodyBio
				}
				if bodyCountry != "" {
					body["country"] = bodyCountry
				}
				if bodyExternalId != "" {
					body["external_id"] = bodyExternalId
				}
				if bodyLanguage != "" {
					body["language"] = bodyLanguage
				}
				if bodyMeta != "" {
					var parsedMeta any
					if err := json.Unmarshal([]byte(bodyMeta), &parsedMeta); err != nil {
						return fmt.Errorf("parsing --meta JSON: %w", err)
					}
					body["meta"] = parsedMeta
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
					"resource": "users",
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
	cmd.Flags().StringVar(&bodyEmail, "email", "", "Email address of the user.")
	cmd.Flags().StringVar(&bodyName, "name", "", "Full name of the user. (example: Steve Frazelli)")
	cmd.Flags().StringVar(&bodyRole, "role", "", "The role of the user. (example: {'name': 'admin'})")
	cmd.Flags().StringVar(&bodyBio, "bio", "", "Short biography of the user. (example: Full stack developer at Gorgias)")
	cmd.Flags().StringVar(&bodyCountry, "country", "", "Country of the user (example: FR)")
	cmd.Flags().StringVar(&bodyExternalId, "external-id", "", "ID of the user in a foreign system (Stripe, Aircall, etc...).")
	cmd.Flags().StringVar(&bodyLanguage, "language", "", "User's preferred UI language (ISO 639-1 code, e.g. en, fr, es, de, ja).")
	cmd.Flags().StringVar(&bodyMeta, "meta", "", "Free-form metadata associated with the user (JSON object).")
	cmd.Flags().StringVar(&bodyTimezone, "timezone", "", "User's timezone (IANA tz name, e.g. America/Los_Angeles, Europe/Paris, UTC).")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
