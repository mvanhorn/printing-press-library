// Copyright 2026 mikemarqq and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelMirrorCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "mirror",
		Short:       "Collect local Voice history, inspect coverage and export observed messages",
		Example:     "  google-voice-pp-cli mirror coverage --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:typed-exit-codes": "0,2", "pp:parent-group": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelMirrorCoverageCmd(flags))
	return cmd
}
