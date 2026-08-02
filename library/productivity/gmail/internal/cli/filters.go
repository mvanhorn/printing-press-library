// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelFiltersCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "filters",
		Short:       "Filters as code: diff and apply a declarative filters.yaml",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	// Registered directly rather than through addNovelCommandIfAbsent: the
	// indirection hides these leaves from static command-tree analysis, so
	// `filters diff` reads as a positional arg on `filters` to the verifier.
	cmd.AddCommand(newNovelFiltersDiffCmd(flags))
	cmd.AddCommand(newNovelFiltersApplyCmd(flags))
	return cmd
}
