// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil"
)

// Commands that deliver email print a request preview unless --send is given,
// so an agent or a copy-pasted example cannot deliver mail by accident.
// --sandbox (POSTMARK_API_TEST token) validates without delivering, so it runs
// without --send.
var postmarkGatedSendCommands = []string{
	"send",
	"send-with-template",
	"send-batch",
	"send-batch-with-templates",
	"send-bulk",
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		emailCmd, _, err := root.Find([]string{"email"})
		if err != nil || emailCmd == root {
			return
		}
		for _, name := range postmarkGatedSendCommands {
			for _, sub := range emailCmd.Commands() {
				if sub.Name() == name {
					gatePostmarkSend(sub, flags)
				}
			}
		}
	})
}

func gatePostmarkSend(cmd *cobra.Command, flags *rootFlags) {
	if cmd.Flags().Lookup("send") != nil || cmd.RunE == nil {
		return
	}
	var send bool
	cmd.Flags().BoolVar(&send, "send", false, "Deliver the email. Without it the command prints the request it would make and sends nothing (not needed with --sandbox)")
	inner := cmd.RunE
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if len(args) == 0 && c.Flags().NFlag() == 0 {
			return c.Help()
		}
		if send && !postmarkSelection.sandbox && !flags.dryRun && cliutil.IsAnyHarness() {
			return writeHarnessRefusal(c.OutOrStdout(), flags, "send email")
		}
		if !send && !postmarkSelection.sandbox && !flags.dryRun {
			flags.dryRun = true
			fmt.Fprintln(c.ErrOrStderr(), "preview only: nothing was sent. Add --send to deliver, or --sandbox to validate with Postmark's test token.")
		}
		if send && !flags.dryRun {
			postmarkSendAllowed.Store(true)
			defer postmarkSendAllowed.Store(false)
		}
		return inner(c, args)
	}
	if cmd.Long == "" {
		cmd.Long = cmd.Short
	}
	cmd.Long += "\n\nPrints the request without sending unless --send is given. --sandbox validates the payload with Postmark's POSTMARK_API_TEST token and never delivers."
}
