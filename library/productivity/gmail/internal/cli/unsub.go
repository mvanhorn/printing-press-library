// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written `unsub` family: the RFC 8058 one-click unsubscribe engine.
// audit (classify senders, local store) -> plan (freeze sender list + URLs
// + one-time token) -> run (re-verify everything live, hardened HTTPS
// POSTs, ledger every attempt) -> verify (who kept mailing anyway).
// mailto: unsubscribe entries are surfaced as a desk list and NEVER acted
// on — no mail ever leaves this binary (grill contract: structurally
// send-free).

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelUnsubCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unsub",
		Short: "One-click unsubscribe engine: audit classifies senders, plan freezes who to leave, run POSTs (hardened), verify catches senders that keep mailing",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelUnsubAuditCmd(flags))
	cmd.AddCommand(newNovelUnsubPlanCmd(flags))
	cmd.AddCommand(newNovelUnsubRunCmd(flags))
	cmd.AddCommand(newNovelUnsubVerifyCmd(flags))
	return cmd
}
