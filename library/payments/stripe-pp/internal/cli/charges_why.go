// Copyright 2026 primiasolutions. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newChargesWhyCmd implements `stripe-pp-pp-cli charges why <ch_id>`.
// Returns decline_code + network message + that customer's recent activity +
// similar failures in the last 7d.
func newChargesWhyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "why [charge-id]",
		Short: "Diagnose a failed charge: decline code, customer recent activity, similar 7d failures",
		Long: strings.TrimSpace(`
Charge diagnosis. For a failed charge, returns one JSON document with:
  - the charge's failure_code, decline_code, and outcome.network_status / .reason
  - the affected customer's last 10 charges (success/failure mix)
  - count of similar failures (same decline_code) in the last 7 days

Compresses what is normally 5+ dashboard clicks into a single CLI call.
`),
		Example: strings.Trim(`
  stripe-pp-pp-cli charges why ch_3Nabc --json
  stripe-pp-pp-cli charges why ch_3Nabc --json --select decline_code,network_message,similar_failures_7d
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			chargeID := args[0]

			// 1) Get the charge.
			raw, err := c.Get("/v1/charges/"+chargeID, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var charge map[string]any
			if err := json.Unmarshal(raw, &charge); err != nil {
				return apiErr(err)
			}

			declineCode, _ := charge["failure_code"].(string)
			if declineCode == "" {
				if oc, ok := charge["outcome"].(map[string]any); ok {
					if v, ok := oc["reason"].(string); ok {
						declineCode = v
					}
				}
			}
			networkMessage, _ := charge["failure_message"].(string)
			if outcome, ok := charge["outcome"].(map[string]any); ok {
				if networkMessage == "" {
					networkMessage, _ = outcome["seller_message"].(string)
				}
			}
			custID, _ := charge["customer"].(string)

			// 2) Customer recent activity (last 10 charges).
			var customerRecent []map[string]any
			if custID != "" {
				recRaw, err := c.Get("/v1/charges", map[string]string{"customer": custID, "limit": "10"})
				if err == nil {
					customerRecent = unmarshalList(recRaw)
				}
			}

			// 3) Similar failures last 7d. Stripe's Charge list filters by created date.
			weekAgo := time.Now().Add(-7 * 24 * time.Hour).Unix()
			simParams := map[string]string{
				"created[gte]": fmt.Sprintf("%d", weekAgo),
				"limit":        "100",
			}
			simRaw, _ := c.Get("/v1/charges", simParams)
			similar := unmarshalList(simRaw)
			similarSameCode := 0
			for _, ch := range similar {
				if v, ok := ch["failure_code"].(string); ok && v == declineCode && declineCode != "" {
					similarSameCode++
					continue
				}
				if oc, ok := ch["outcome"].(map[string]any); ok {
					if v, ok := oc["reason"].(string); ok && v == declineCode && declineCode != "" {
						similarSameCode++
					}
				}
			}

			out := map[string]any{
				"id":                       charge["id"],
				"amount":                   charge["amount"],
				"currency":                 charge["currency"],
				"paid":                     charge["paid"],
				"status":                   charge["status"],
				"customer":                 custID,
				"decline_code":             declineCode,
				"network_message":          networkMessage,
				"outcome":                  charge["outcome"],
				"customer_recent_activity": customerRecent,
				"similar_failures_7d":      similarSameCode,
				"similar_failures_window":  "last 7 days",
			}
			return flags.printJSON(cmd, out)
		},
	}
	return cmd
}
