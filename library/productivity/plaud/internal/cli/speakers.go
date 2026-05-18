// Copyright 2026 jnalv414. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/plaud/internal/store"
)

func newSpeakersCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagSort string

	cmd := &cobra.Command{
		Use:   "speakers",
		Short: "List speakers across the corpus with appearance counts and last-heard timestamps",
		Long: "Returns the materialized speakers table — one row per distinct speaker\n" +
			"name across all transcripts. Includes appearance count, first/last seen\n" +
			"timestamps, and the underlying original_speaker label from ASR.\n\n" +
			"Useful for picking a name to pass to `about`, `cross-meeting`, or\n" +
			"`silence`.",
		Example: `  plaud-pp-cli speakers
  plaud-pp-cli speakers --sort count
  plaud-pp-cli speakers --json --select name,appearance_count,last_seen`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			s, err := openPlaudStore(cmd.Context())
			var _ *store.Store = s
			if err != nil {
				return err
			}
			defer s.Close()

			orderBy := "last_seen DESC"
			switch flagSort {
			case "count":
				orderBy = "appearance_count DESC"
			case "name":
				orderBy = "name ASC"
			case "first":
				orderBy = "first_seen ASC"
			}

			query := fmt.Sprintf(`
				SELECT name, original_speaker, appearance_count, first_seen, last_seen
				FROM speakers
				ORDER BY %s
				LIMIT ?
			`, orderBy)
			rows, err := queryRowsToMaps(cmd.Context(), s.DB(), query, flagLimit)
			if err != nil {
				return apiErr(fmt.Errorf("speakers query: %w", err))
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Max speakers to return")
	cmd.Flags().StringVar(&flagSort, "sort", "last", "Sort by: last (default), count, name, first")
	return cmd
}
