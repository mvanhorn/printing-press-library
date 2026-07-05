// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/internal/airbnb"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/internal/cliutil"
	"github.com/spf13/cobra"
)

func newContactCmd(flags *rootFlags) *cobra.Command {
	var p airbnb.ContactParams
	var confirm bool
	cmd := &cobra.Command{
		Use:   "contact [listing-id]",
		Short: "Start a conversation with a host / property owner",
		Long: `Open a new conversation with a listing's host and send an initial message —
the core outreach action for contacting property owners. Guarded: without
--confirm the message is previewed only.`,
		Example: "  airbnb-outreach-pp-cli contact 400704 --message \"Hi, I'm interested in a longer stay — is that possible?\" --confirm",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			p.ListingID = args[0]
			if p.Message == "" {
				return usageErr(fmt.Errorf("--message is required"))
			}
			if flags.dryRun || cliutil.IsVerifyEnv() || !(confirm || flags.yes) {
				return previewWrite(cmd, flags, "contact", map[string]any{
					"listing_id": p.ListingID,
					"message":    p.Message,
					"checkin":    p.Checkin,
					"checkout":   p.Checkout,
				})
			}
			c := newAirbnbClient(flags)
			data, err := c.Contact(p)
			if err != nil {
				return classifyAirbnb(err, flags)
			}
			return reportWrite(cmd, flags, "contacted", data)
		},
	}
	cmd.Flags().StringVar(&p.Message, "message", "", "Message to send to the host")
	cmd.Flags().StringVar(&p.Checkin, "checkin", "", "Optional check-in date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&p.Checkout, "checkout", "", "Optional check-out date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&p.Adults, "adults", 0, "Optional number of guests")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually send (without this, the message is only previewed)")
	return cmd
}
