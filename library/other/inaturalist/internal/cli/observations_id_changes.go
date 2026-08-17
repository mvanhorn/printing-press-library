// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// pp:data-source auto
// pp:client-call
// runIdentificationChanges reaches the generated client through the shared
// privacy-redacting novelGet helper before it writes a redacted local snapshot.
func newNovelObservationsIdChangesCmd(flags *rootFlags) *cobra.Command {
	var user, since string
	cmd := &cobra.Command{
		Use:         "id-changes",
		Short:       "Report identification changes since the previous privacy-safe local snapshot.",
		Example:     "  inaturalist-pp-cli observations id-changes --user inaturalist --since 30d --agent",
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE:        func(cmd *cobra.Command, _ []string) error { return runIdentificationChanges(cmd, flags, user, since) },
	}
	cmd.Flags().StringVar(&user, "user", "", "iNaturalist login whose public observation states to compare")
	cmd.Flags().StringVar(&since, "since", "30d", "Observation window as whole days, for example 30d")
	return cmd
}
