// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelSttJobCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "stt-job",
		Short:       "stt-job subcommands: report, retry",
		Example:     "  sarvam-pp-cli stt-job report 20260707_9f1c2b3a-4d5e-6f70-8a9b-c0d1e2f3a4b5 --json\n  sarvam-pp-cli stt-job retry 20260707_9f1c2b3a-4d5e-6f70-8a9b-c0d1e2f3a4b5 --failed-only --dir ./audio/",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,5,6"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelSttJobReportCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelSttJobRetryCmd(flags))
	return cmd
}
