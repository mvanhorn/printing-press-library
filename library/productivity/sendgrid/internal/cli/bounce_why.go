// Hand-coded — do not add "DO NOT EDIT" header. Persists across regenerations.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newBounceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "bounce",
		Short:  "Investigate why a recipient is suppressed, including blocks, invalids, and spam reports",
		Hidden: true,
		RunE:   parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newBounceWhyCmd(flags))
	return cmd
}

type bounceWhyResult struct {
	Email         string `json:"email"`
	BounceReason  string `json:"bounce_reason,omitempty"`
	BounceCreated string `json:"bounce_created_at,omitempty"`
	IsBlocked     bool   `json:"is_blocked"`
	IsInvalid     bool   `json:"is_invalid"`
	IsSpam        bool   `json:"is_spam"`
	ActivityCount int    `json:"activity_count"`
	Narrative     string `json:"narrative"`
}

func newBounceWhyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "why <email>",
		Short: "Explain why an email address keeps bouncing",
		Long: `Joins suppression state (bounces, blocks, invalid_emails, spam_reports),
email activity (messages search by to_email), and recent stats context to
produce a human-readable narrative explaining why a specific address bounces.
Output: structured JSON or a narrative prose block.`,
		Example:     "  sendgrid-pp-cli bounce why user@example.com",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			email := strings.ToLower(strings.TrimSpace(args[0]))
			if email == "" {
				return usageErr(fmt.Errorf("email address is required"))
			}

			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			result := bounceWhyResult{Email: email}
			var narrativeParts []string

			// Check bounces
			bounceData, err := c.Get("/v3/suppression/bounces/"+email, nil)
			if err == nil {
				var bounce map[string]any
				if json.Unmarshal(bounceData, &bounce) == nil {
					if reason, ok := bounce["reason"].(string); ok && reason != "" {
						result.BounceReason = reason
					}
					if created, ok := bounce["created"].(float64); ok && created > 0 {
						result.BounceCreated = fmt.Sprintf("%d", int64(created))
					}
					narrativeParts = append(narrativeParts,
						fmt.Sprintf("Email %s is on the bounces suppression list (reason: %q, since: %s).",
							email, result.BounceReason, result.BounceCreated))
				}
			}

			// Check blocks
			blocksData, err := c.Get("/v3/suppression/blocks/"+email, nil)
			if err == nil {
				var blocks []map[string]any
				if json.Unmarshal(blocksData, &blocks) == nil && len(blocks) > 0 {
					result.IsBlocked = true
					narrativeParts = append(narrativeParts, fmt.Sprintf("Address is also on the blocks list (%d entry/entries).", len(blocks)))
				}
			}

			// Check invalid emails
			invalidData, err := c.Get("/v3/suppression/invalid_emails/"+email, nil)
			if err == nil {
				var invalids []map[string]any
				if json.Unmarshal(invalidData, &invalids) == nil && len(invalids) > 0 {
					result.IsInvalid = true
					narrativeParts = append(narrativeParts, "Address is marked as invalid.")
				}
			}

			// Check spam reports
			spamData, err := c.Get("/v3/suppression/spam_reports/"+email, nil)
			if err == nil {
				var spam []map[string]any
				if json.Unmarshal(spamData, &spam) == nil && len(spam) > 0 {
					result.IsSpam = true
					narrativeParts = append(narrativeParts, "Address has reported spam.")
				}
			}

			// Check email activity
			query := fmt.Sprintf("to_email=%s", email)
			activityData, err := c.Get("/v3/messages", map[string]string{
				"query": query,
				"limit": "25",
			})
			if err == nil {
				var activity map[string]json.RawMessage
				if json.Unmarshal(activityData, &activity) == nil {
					if msgsRaw, ok := activity["messages"]; ok {
						var msgs []json.RawMessage
						if json.Unmarshal(msgsRaw, &msgs) == nil {
							result.ActivityCount = len(msgs)
							narrativeParts = append(narrativeParts,
								fmt.Sprintf("Found %d message(s) in email activity for this address.", len(msgs)))
						}
					}
				}
			}

			if len(narrativeParts) == 0 {
				narrativeParts = append(narrativeParts,
					fmt.Sprintf("No suppression or activity records found for %s.", email))
			}

			result.Narrative = strings.Join(narrativeParts, " ")

			if flags.asJSON {
				raw, _ := json.Marshal(result)
				return printOutput(cmd.OutOrStdout(), raw, true)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), result.Narrative)
			return nil
		},
	}

	return cmd
}
