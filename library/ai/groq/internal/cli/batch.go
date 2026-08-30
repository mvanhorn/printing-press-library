// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelBatchCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "batch",
		Short:       "Batch workflow guardrails",
		Example:     "  groq-pp-cli batch diagnose batch_abc123 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelBatchDiagnoseCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelBatchValidateCmd(flags))
	return cmd
}
