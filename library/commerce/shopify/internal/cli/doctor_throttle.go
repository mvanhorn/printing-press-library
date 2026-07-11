// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live

// throttleProbeQuery is the cheapest possible authenticated GraphQL call —
// Shopify reports extensions.cost.throttleStatus on every response,
// including this one, so a single shop{id} round-trip is enough to read
// the current query-cost budget.
const throttleProbeQuery = `query { shop { id } }`

type throttleStatusView struct {
	MaximumAvailable   float64   `json:"maximum_available"`
	CurrentlyAvailable float64   `json:"currently_available"`
	RestoreRate        float64   `json:"restore_rate"`
	LastUpdated        time.Time `json:"last_updated"`
	Note               string    `json:"note,omitempty"`
}

func newNovelDoctorThrottleCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "throttle",
		Short:       "Check remaining GraphQL query-cost budget before a heavy batch or agent session.",
		Long:        "Use this command to check remaining GraphQL query-cost budget before a heavy batch or agent session. Do NOT use this command for general auth/connectivity health; use 'doctor' instead.",
		Example:     "  shopify-pp-cli doctor throttle --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would check GraphQL throttle status")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if _, err := c.Query(ctx, throttleProbeQuery, nil); err != nil {
				return classifyAPIError(err, flags)
			}

			status, seeded := c.Throttle.Status()
			view := throttleStatusView{
				MaximumAvailable:   status.MaximumAvailable,
				CurrentlyAvailable: status.CurrentlyAvailable,
				RestoreRate:        status.RestoreRate,
				LastUpdated:        status.LastUpdated,
			}
			if !seeded {
				view.Note = "Shopify did not report extensions.cost.throttleStatus on this response; budget unknown."
			}

			if flags.asJSON || wantsMachineOutput(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if !seeded {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "currently_available=%.1f\tmaximum_available=%.1f\trestore_rate=%.1f/s\n",
				view.CurrentlyAvailable, view.MaximumAvailable, view.RestoreRate)
			return nil
		},
	}
	return cmd
}
