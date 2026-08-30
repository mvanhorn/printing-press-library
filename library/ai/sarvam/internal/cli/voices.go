// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelVoicesCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "voices",
		Short:       "voices subcommands: preview",
		Example:     "  sarvam-pp-cli voices preview --lang hi-IN --sample 'नमस्ते, स्वागत है' --speakers shubh,ritu,priya --output ./voices",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelVoicesPreviewCmd(flags))
	return cmd
}
