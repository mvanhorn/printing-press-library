// Copyright 2026 daniel-larson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

// newNewsletterGroupCmd is the visible top-level "newsletter" parent.
// Newsletter assembly is the canonical Hermes-agent workflow: fan out across
// N section chatflows, concat the responses into a single markdown document,
// record provenance.
func newNewsletterGroupCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "newsletter",
		Short: "Compound newsletter workflows: compose multi-section drafts, audit prior runs",
		Long: `Compound newsletter workflows for an agent that drives Flowise as a marketing
manager.

` + "`newsletter compose`" + ` reads a YAML plan listing section name + chatflowId +
question, fires each prediction in sequence, and concatenates the text fields
into a single markdown document. Every chatId is recorded so a downstream
audit step can trace what was generated.

` + "`newsletter audit`" + ` joins the local predictions and chatmessages tables for a
time window and produces a per-chatId audit row: flow name, source doc count,
tools invoked.`,
	}
	cmd.AddCommand(newNewsletterComposeCmd(flags))
	cmd.AddCommand(newNewsletterAuditCmd(flags))
	return cmd
}
