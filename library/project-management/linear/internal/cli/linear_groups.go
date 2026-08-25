// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// linear_groups.go provides Cobra parent groups for Linear-specific
// command families that have a transcendence subcommand. Each parent
// composes the existing "get a single X" promoted command (renamed to
// `get`) with the v3-ported transcendence subcommand.

package cli

import "github.com/spf13/cobra"

// newProjectsGroupCmd wires `projects get` + portfolio helpers.
func newProjectsGroupCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "projects",
		Short:       "Linear projects: get, list, search, resolve, and burndown projection",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	get := newProjectsPromotedCmd(flags)
	get.Use = "get <id>"
	get.Short = "Get a single project"
	cmd.AddCommand(get)
	cmd.AddCommand(newProjectsListCmd(flags))
	cmd.AddCommand(newProjectsSearchCmd(flags))
	cmd.AddCommand(newProjectsResolveCmd(flags))
	cmd.AddCommand(newProjectsBurndownCmd(flags))
	return cmd
}

// newCyclesGroupCmd wires `cycles compare` (no promoted singleton — v4
// dropped that emit).
func newCyclesGroupCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "cycles",
		Short:       "Linear cycles: cycle-over-cycle comparison",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCyclesCompareCmd(flags))
	return cmd
}

// newInitiativesGroupCmd wires `initiatives get` + portfolio helpers.
func newInitiativesGroupCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "initiatives",
		Short:       "Linear initiatives: get, list, search, resolve, and portfolio health rollup",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	get := newInitiativesPromotedCmd(flags)
	get.Use = "get <id>"
	get.Short = "Get a single initiative"
	cmd.AddCommand(get)
	cmd.AddCommand(newInitiativesListCmd(flags))
	cmd.AddCommand(newInitiativesSearchCmd(flags))
	cmd.AddCommand(newInitiativesResolveCmd(flags))
	cmd.AddCommand(newInitiativesHealthCmd(flags))
	return cmd
}
