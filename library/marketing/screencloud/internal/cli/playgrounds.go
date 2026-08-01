// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelPlaygroundsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "playgrounds",
		Short:       "playgrounds subcommands: config-drift, contract-check, create-reconcile, impact, preview-drift, readiness",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelPlaygroundsConfigDriftCmd(flags))
	cmd.AddCommand(newNovelPlaygroundsContractCheckCmd(flags))
	cmd.AddCommand(newNovelPlaygroundsCreateReconcileCmd(flags))
	cmd.AddCommand(newNovelPlaygroundsImpactCmd(flags))
	cmd.AddCommand(newNovelPlaygroundsPreviewDriftCmd(flags))
	cmd.AddCommand(newNovelPlaygroundsReadinessCmd(flags))
	return cmd
}
