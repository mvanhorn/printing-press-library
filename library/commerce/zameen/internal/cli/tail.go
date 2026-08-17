// tail: show the most recently-listed properties from the local store.
// pp:data-source local
package cli

import (
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newTailCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Show the most recently-listed synced properties (newest first)",
		Long: "List the newest listings in the local store by creation time. " +
			"Run 'zameen-pp-cli pull ...' first to populate it.",
		Example:     "  zameen-pp-cli tail --limit 10",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list the newest stored listings")
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("zameen-pp-cli")
			}
			listings, err := loadStoredListings(cmd.Context(), dbPath)
			if err != nil {
				if errors.Is(err, errNoMirror) {
					return emitEmptyMirrorHint(cmd, flags, dbPath)
				}
				return err
			}
			sort.SliceStable(listings, func(i, j int) bool { return listings[i].CreatedAt > listings[j].CreatedAt })
			if limit > 0 && len(listings) > limit {
				listings = listings[:limit]
			}
			return emitListings(cmd, flags, listings)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum listings to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard data dir)")
	return cmd
}
