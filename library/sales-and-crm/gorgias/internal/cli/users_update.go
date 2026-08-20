// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newUsersUpdateCmd(flags *rootFlags) *cobra.Command {
	var bodyBio string
	var bodyCountry string
	var bodyEmail string
	var bodyExternalId string
	var bodyLanguage string
	var bodyMeta string
	var bodyName string
	var bodyNewPassword string
	var bodyOldPassword string
	var bodyPasswordConfirmation string
	var bodyRole string
	var bodyTimezone string
	var bodyTwoFaCode string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "update <id>",
		Short:       "Update a user (`id`) — change role, name, team membership, or active state.",
		Example:     "  gorgias-pp-cli users update 123456789",
		Annotations: map[string]string{"pp:endpoint": "users.update", "pp:method": "PUT", "pp:path": "/users/{id}"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/users/{id}"
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
				if bodyBio != "" {
					body["bio"] = bodyBio
				}
				if bodyCountry != "" {
					body["country"] = bodyCountry
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
				if bodyMeta != "" {
					var parsedMeta any
					if err := json.Unmarshal([]byte(bodyMeta), &parsedMeta); err != nil {
						return fmt.Errorf("parsing --meta JSON: %w", err)
					}
					body["meta"] = parsedMeta
				}
				if bodyName != "" {
					body["name"] = bodyName
				}
				if bodyNewPassword != "" {
					body["new_password"] = bodyNewPassword
				}
				if bodyOldPassword != "" {
					body["old_password"] = bodyOldPassword
				}
				if bodyPasswordConfirmation != "" {
					body["password_confirmation"] = bodyPasswordConfirmation
				}
				if bodyRole != "" {
					var parsedRole any
					if err := json.Unmarshal([]byte(bodyRole), &parsedRole); err != nil {
						return fmt.Errorf("parsing --role JSON: %w", err)
					}
					body["role"] = parsedRole
				}
				if bodyTimezone != "" {
					body["timezone"] = bodyTimezone
				}
				if bodyTwoFaCode != "" {
					body["two_fa_code"] = bodyTwoFaCode
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
	cmd.Flags().StringVar(&bodyBio, "bio", "", "Short biography of the user. (example: Full stack developer at Gorgias)")
	cmd.Flags().StringVar(&bodyCountry, "country", "", "Country of the user (example: FR)")
	cmd.Flags().StringVar(&bodyEmail, "email", "", "Email address of the user.")
	cmd.Flags().StringVar(&bodyExternalId, "external-id", "", "ID of the user in a foreign system (Stripe, Aircall, etc...).")
	cmd.Flags().StringVar(&bodyLanguage, "language", "", "User's preferred UI language (ISO 639-1 code, e.g. en, fr, es, de, ja).")
	cmd.Flags().StringVar(&bodyMeta, "meta", "", "Data associated with the user.")
	cmd.Flags().StringVar(&bodyName, "name", "", "Full name of the user. (example: Steve Frazelli)")
	cmd.Flags().StringVar(&bodyNewPassword, "new-password", "", "New password of the user. `old_password` field must be provided when changing the password. (example: NewPassword6502!)")
	cmd.Flags().StringVar(&bodyOldPassword, "old-password", "", "Current password of the user. (example: OldPassword5031!)")
	cmd.Flags().StringVar(&bodyPasswordConfirmation, "password-confirmation", "", "Current password of the user. (example: OldPassword5031!)")
	cmd.Flags().StringVar(&bodyRole, "role", "", "The role of the user. (example: {'name': 'admin'})")
	cmd.Flags().StringVar(&bodyTimezone, "timezone", "", "Timezone of the user.")
	cmd.Flags().StringVar(&bodyTwoFaCode, "two-fa-code", "", "")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
