// Copyright 2026 matthew.martin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelWebhookCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "webhook",
		Short:       "Record and inspect locally received Square webhooks.",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelWebhookHealthCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelWebhookIngestCmd(flags))
	return cmd
}
