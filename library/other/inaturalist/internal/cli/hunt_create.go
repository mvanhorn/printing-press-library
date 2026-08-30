// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// pp:data-source live
func newNovelHuntCreateCmd(flags *rootFlags) *cobra.Command {
	var placeID, iconicTaxa string
	var limit int
	cmd := &cobra.Command{
		Use:         "create",
		Short:       "Create a factual balanced scavenger-hunt checklist from observed taxa.",
		Example:     "  inaturalist-pp-cli hunt create --place-id 97394 --iconic-taxa Aves,Plantae --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runHuntCreate(cmd, flags, placeID, iconicTaxa, limit)
		},
	}
	cmd.Flags().StringVar(&placeID, "place-id", "", "iNaturalist place ID to use for the hunt")
	cmd.Flags().StringVar(&iconicTaxa, "iconic-taxa", "", "Comma-separated iconic taxa to include, for example Aves,Plantae")
	cmd.Flags().IntVar(&limit, "limit", 12, "Maximum number of hunt items")
	return cmd
}
