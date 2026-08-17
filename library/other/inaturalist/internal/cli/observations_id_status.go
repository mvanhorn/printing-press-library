// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// pp:data-source live
// pp:client-call
// runIdentificationStatus reaches the generated client through the shared
// privacy-redacting novelGet helper.
func newNovelObservationsIdStatusCmd(flags *rootFlags) *cobra.Command {
	var user, since string
	cmd := &cobra.Command{
		Use:         "id-status",
		Short:       "See which recent observations are identified, need IDs, disagree, or have no taxon.",
		Example:     "  inaturalist-pp-cli observations id-status --user inaturalist --since 30d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        func(cmd *cobra.Command, _ []string) error { return runIdentificationStatus(cmd, flags, user, since) },
	}
	cmd.Flags().StringVar(&user, "user", "", "iNaturalist login whose public observations to inspect")
	cmd.Flags().StringVar(&since, "since", "30d", "Observation window as whole days, for example 30d")
	return cmd
}
