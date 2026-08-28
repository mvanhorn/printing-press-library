// Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mvanhorn/printing-press-library/library/marketing/bing-ads/internal/cliutil"
	"github.com/spf13/cobra"
)

func newCustomerManagementSendUserInvitationCmd(flags *rootFlags) *cobra.Command {
	var bodyUserInvitationAccountIds string
	var bodyUserInvitationCustomerId string
	var bodyUserInvitationEmail string
	var bodyUserInvitationExpirationDate string
	var bodyUserInvitationFirstName string
	var bodyUserInvitationId string
	var bodyUserInvitationLastName string
	var bodyUserInvitationLcid string
	var bodyUserInvitationRoleId int
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "send-user-invitation",
		Short:       "send_user_invitation",
		Example:     "  bing-ads-pp-cli customer-management send-user-invitation",
		Annotations: map[string]string{"pp:endpoint": "customer-management.send-user-invitation", "pp:method": "POST", "pp:path": "/CustomerManagement/v13/UserInvitation/Send"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
			}
			path := "/CustomerManagement/v13/UserInvitation/Send"
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{}
			var body any
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
				bodyMap := map[string]any{}
				body = map[string]any{"UserInvitation": bodyMap}
				if cmd.Flags().Changed("user-invitation-account-ids") {
					parsedUserInvitationAccountIds, parseErr := cliutil.ParseStringList(bodyUserInvitationAccountIds)
					if parseErr != nil {
						return fmt.Errorf("parsing --user-invitation-account-ids list: %w", parseErr)
					}
					bodyMap["AccountIds"] = parsedUserInvitationAccountIds
				}
				if cmd.Flags().Changed("user-invitation-customer-id") || bodyUserInvitationCustomerId != "" {
					bodyMap["CustomerId"] = bodyUserInvitationCustomerId
				}
				if cmd.Flags().Changed("user-invitation-email") || bodyUserInvitationEmail != "" {
					bodyMap["Email"] = bodyUserInvitationEmail
				}
				if cmd.Flags().Changed("user-invitation-expiration-date") || bodyUserInvitationExpirationDate != "" {
					bodyMap["ExpirationDate"] = bodyUserInvitationExpirationDate
				}
				if cmd.Flags().Changed("user-invitation-first-name") || bodyUserInvitationFirstName != "" {
					bodyMap["FirstName"] = bodyUserInvitationFirstName
				}
				if cmd.Flags().Changed("user-invitation-id") || bodyUserInvitationId != "" {
					bodyMap["Id"] = bodyUserInvitationId
				}
				if cmd.Flags().Changed("user-invitation-last-name") || bodyUserInvitationLastName != "" {
					bodyMap["LastName"] = bodyUserInvitationLastName
				}
				if cmd.Flags().Changed("user-invitation-lcid") || bodyUserInvitationLcid != "" {
					bodyMap["Lcid"] = bodyUserInvitationLcid
				}
				if cmd.Flags().Changed("user-invitation-role-id") || bodyUserInvitationRoleId != 0 {
					bodyMap["RoleId"] = bodyUserInvitationRoleId
				}
			}
			data, statusCode, err := c.PostWithParams(cmd.Context(), path, params, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			// Inspect the mutate response body for a partial-failure-shaped
			// field (e.g. Google Ads `partialFailureError`). Several Google
			// APIs return 200 OK with a partial-failure field when some
			// operations in the batch failed; ignoring it silently swallows
			// real failures. Detection runs before output-mode selection so
			// the exit code is consistent regardless of how stdout is
			// rendered. --dry-run short-circuits because no real request
			// was sent.
			var partialFailure *partialFailureReport
			if !flags.dryRun && statusCode >= 200 && statusCode < 300 {
				partialFailure = detectPartialFailure(data)
				if partialFailure != nil {
					fmt.Fprintf(os.Stderr, "warning: partial failure detected in %s response: %s\n", "customer-management", partialFailure.Message)
					if len(partialFailure.ResourceNames) > 0 {
						fmt.Fprintf(os.Stderr, "         succeeded: %d operation(s)\n", len(partialFailure.ResourceNames))
					}
				}
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				// Check if response contains an array (directly or wrapped in "data")
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
					} else {
						if partialFailure != nil && !flags.allowPartialFailure {
							return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "customer-management", partialFailure.Message))
						}
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
							if partialFailure != nil && !flags.allowPartialFailure {
								return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "customer-management", partialFailure.Message))
							}
							return nil
						}
					}
				}
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				if flags.quiet {
					if partialFailure != nil && !flags.allowPartialFailure {
						return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "customer-management", partialFailure.Message))
					}
					return nil
				}
				envelope := map[string]any{
					"action":   "post",
					"resource": "customer-management",
					"path":     path,
					"status":   statusCode,
					"success":  statusCode >= 200 && statusCode < 300 && (partialFailure == nil || flags.allowPartialFailure),
				}
				if flags.agent {
					envelope["meta"] = map[string]any{"source": "live"}
				}
				if partialFailure != nil {
					envelope["partial_failure"] = partialFailure
				}
				if flags.dryRun {
					envelope["dry_run"] = true
					envelope["status"] = 0
					envelope["success"] = false
				}
				// Verify-mode synthetic envelope detection runs against RAW data
				// (before --compact/--select filtering) so the sentinel field is
				// guaranteed to be visible even if the operator passes a filter
				// flag that would otherwise strip it. Surfaces a top-level
				// verify_noop signal + flips success to false. Mirrors the dry_run
				// shape above.
				if len(data) > 0 {
					var rawParsed any
					if err := json.Unmarshal(data, &rawParsed); err == nil {
						if m, ok := rawParsed.(map[string]any); ok {
							if v, ok := m["__pp_verify_synthetic__"].(bool); ok && v {
								envelope["verify_noop"] = true
								envelope["success"] = false
							}
						}
					}
				}
				// Apply --compact and --select to the API response before wrapping.
				// --select wins when both are set: explicit field choice trumps the
				// generic high-gravity allow-list. Otherwise --compact still applies
				// when --agent is on but the user did not name fields.
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered, map[string]bool{"UserInvitationId": true})
				}
				if len(filtered) > 0 {
					var parsed any
					if err := json.Unmarshal(filtered, &parsed); err == nil {
						if flags.agent {
							envelope["results"] = parsed
						} else {
							envelope["data"] = parsed
						}
					}
				}
				envelopeJSON, err := json.Marshal(envelope)
				if err != nil {
					return err
				}
				resultKey := "data"
				if flags.agent {
					resultKey = "results"
				}
				structured, err := wrapPlatformStructuredOutput(json.RawMessage(envelopeJSON), flags, resultKey, true)
				if err != nil {
					return err
				}
				if perr := printOutput(cmd.OutOrStdout(), structured, true); perr != nil {
					return perr
				}
				if partialFailure != nil && !flags.allowPartialFailure {
					return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "customer-management", partialFailure.Message))
				}
				return nil
			}
			// Fall-through for mutate paths that did not hit the table or
			// asJSON branches: --quiet, --csv, --plain, and default terminal
			// raw output. printOutputWithFlags renders the body, then the
			// typed partial-failure exit fires unless --allow-partial-failure
			// downgrades it. Without this guard a partial failure would exit
			// 0 for these output modes — the exact silent-swallow regression
			// the surrounding patch is preventing for asJSON / piped output.
			if perr := printOutputWithFlags(cmd.OutOrStdout(), data, flags); perr != nil {
				return perr
			}
			if partialFailure != nil && !flags.allowPartialFailure {
				return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "customer-management", partialFailure.Message))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&bodyUserInvitationAccountIds, "user-invitation-account-ids", "", "Account ids")
	cmd.Flags().StringVar(&bodyUserInvitationCustomerId, "user-invitation-customer-id", "", "Customer id")
	cmd.Flags().StringVar(&bodyUserInvitationEmail, "user-invitation-email", "", "Email")
	cmd.Flags().StringVar(&bodyUserInvitationExpirationDate, "user-invitation-expiration-date", "", "Expiration date")
	cmd.Flags().StringVar(&bodyUserInvitationFirstName, "user-invitation-first-name", "", "First name")
	cmd.Flags().StringVar(&bodyUserInvitationId, "user-invitation-id", "", "Id")
	cmd.Flags().StringVar(&bodyUserInvitationLastName, "user-invitation-last-name", "", "Last name")
	cmd.Flags().StringVar(&bodyUserInvitationLcid, "user-invitation-lcid", "", "Lcid")
	cmd.Flags().IntVar(&bodyUserInvitationRoleId, "user-invitation-role-id", 0, "Role id")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
