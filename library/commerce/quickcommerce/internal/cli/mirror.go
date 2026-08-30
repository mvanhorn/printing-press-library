// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
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
		Short:       "Work with mirror",
		Example:     "  quickcommerce-pp-cli mirror coverage --location 12.9021,77.6639 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelMirrorCoverageCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelMirrorIngestCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelMirrorStaleCmd(flags))
	return cmd
}
