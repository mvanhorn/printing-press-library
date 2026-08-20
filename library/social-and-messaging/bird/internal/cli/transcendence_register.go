// Copyright 2026 Stephan Stoeber and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"github.com/spf13/cobra"
)

// newSmsParentCmd builds a fresh `sms` parent that owns the flat-flag send
// command plus transcendence subcommands (search, send-batch, reconcile).
// It replaces the generator's promoted shortcut so the user experience is
// consistent across the SMS surface.
func newSmsParentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sms",
		Short: "Send and inspect SMS messages",
		Long:  "Send SMS messages, search the local body index, and bulk-send + reconcile from a CSV.",
	}
	cmd.AddCommand(newSmsFlatSendCmd(flags))
	cmd.AddCommand(newSmsSearchCmd(flags))
	cmd.AddCommand(newSmsSendBatchCmd(flags))
	cmd.AddCommand(newSmsReconcileCmd(flags))
	return cmd
}

// newTenantCmd hosts the tenant-readiness checklist as `bird-pp-cli tenant doctor`.
func newTenantCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenant",
		Short: "Tenant readiness and onboarding helpers",
	}
	cmd.AddCommand(newTenantDoctorCmd(flags))
	return cmd
}

// addTranscendenceCommands wires the hand-written novel features into existing
// generator-emitted parents. Called from root.go after the standard parents
// are registered.
func addTranscendenceCommands(rootCmd *cobra.Command, flags *rootFlags) {
	for _, c := range rootCmd.Commands() {
		switch c.Use {
		case "messages":
			c.AddCommand(newMessagesAuditCmd(flags))
			c.AddCommand(newMessagesFailuresCmd(flags))
			c.AddCommand(newMessagesFromCmd(flags))
		case "conversations":
			c.AddCommand(newConversationsTimelineCmd(flags))
		case "compliance":
			c.AddCommand(newComplianceAutoBlockCmd(flags))
		}
	}
}

func findCommand(parent *cobra.Command, use string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Use == use || c.Name() == use {
			return c
		}
	}
	return nil
}
