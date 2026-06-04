// Hand-authored — NOT generated. Implements `gisis-pp-cli ship list`: browse
// the local cache of vessels accumulated by ship get / batch / refresh, with
// filters on flag/owner/type, a name-or-owner substring search, and a
// pinned-only view. Read-only against the local SQLite store.
package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/gisis/internal/store"

	"github.com/spf13/cobra"
)

func newShipListCmd(flags *rootFlags) *cobra.Command {
	var flagFlag, flagOwner, flagType, flagNameLike string
	var flagPinned bool
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Browse vessels you have already fetched, filtered by flag/owner/type or name search.",
		Long:        "Queries the local cache populated by 'ship get', 'ship batch', and 'ship refresh'. Combine filters to narrow the set; --name-like does a case-insensitive substring match on name or registered owner. Returns nothing if you have not fetched any vessels yet.",
		Example:     "  gisis-pp-cli ship list --flag Panama --json\n  gisis-pp-cli ship list --name-like sider --pinned",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openStoreForRead(cmd.Context(), "gisis-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if db == nil {
				return printJSONFiltered(cmd.OutOrStdout(), []store.ShipRow{}, flags)
			}
			defer db.Close()

			rows, err := db.ListShips(store.ListShipsOptions{
				Flag:       flagFlag,
				Owner:      flagOwner,
				ShipType:   flagType,
				NameLike:   flagNameLike,
				PinnedOnly: flagPinned,
				Limit:      flagLimit,
			})
			if err != nil {
				return fmt.Errorf("querying local ship cache: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&flagFlag, "flag", "", "Filter by flag state (exact, case-insensitive)")
	cmd.Flags().StringVar(&flagOwner, "owner", "", "Filter by registered owner (substring, case-insensitive)")
	cmd.Flags().StringVar(&flagType, "type", "", "Filter by ship type (exact, case-insensitive)")
	cmd.Flags().StringVar(&flagNameLike, "name-like", "", "Substring search on vessel name or registered owner")
	cmd.Flags().BoolVar(&flagPinned, "pinned", false, "Only list pinned (watchlisted) vessels")
	cmd.Flags().IntVar(&flagLimit, "limit", 200, "Maximum number of rows to return")
	return cmd
}
