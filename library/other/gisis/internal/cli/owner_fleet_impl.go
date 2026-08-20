// Hand-authored — NOT generated. Implements `gisis-pp-cli owner fleet <owner>`:
// list every cached vessel for a registered-owner string. The GISIS Companies
// module is out of v1 scope, so this groups the locally cached ships by their
// registered_owner column rather than querying GISIS. newOwnerParentCmd is the
// hand-authored replacement wired from root.go (mirrors newShipParentCmd).
package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/gisis/internal/store"

	"github.com/spf13/cobra"
)

// newOwnerParentCmd reuses the generated owner parent (keeping its annotations
// and the generated stub constructors live for the linter) but swaps its
// subcommands for the hand-authored implementations. Wired from root.go.
func newOwnerParentCmd(flags *rootFlags) *cobra.Command {
	cmd := newNovelOwnerCmd(flags)
	cmd.Short = "Owner-centric views over your locally cached vessels."
	cmd.Long = "Group cached vessels by registered owner. The GISIS Companies module is not in v1, so 'owner fleet' reads the local cache populated by 'ship get' / 'ship batch'."
	cmd.ResetCommands()
	cmd.AddCommand(newOwnerFleetCmd(flags))
	return cmd
}

func newOwnerFleetCmd(flags *rootFlags) *cobra.Command {
	var flagLike bool

	cmd := &cobra.Command{
		Use:         "fleet <owner>",
		Short:       "List every cached vessel for a registered-owner string.",
		Long:        "Returns all locally cached vessels whose registered owner matches the given string. By default the match is exact (case-insensitive); pass --like for a substring match (useful when owner names vary in punctuation or suffixes).",
		Example:     "  gisis-pp-cli owner fleet \"KIVIK SHIPPING LTD\" --json\n  gisis-pp-cli owner fleet kivik --like",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			owner := strings.TrimSpace(strings.Join(args, " "))
			if owner == "" {
				return usageErr(fmt.Errorf("owner name is required"))
			}
			db, err := openStoreForRead(cmd.Context(), "gisis-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if db == nil {
				return printJSONFiltered(cmd.OutOrStdout(), []store.ShipRow{}, flags)
			}
			defer db.Close()

			rows, err := db.OwnerFleet(owner, flagLike)
			if err != nil {
				return fmt.Errorf("querying local ship cache: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().BoolVar(&flagLike, "like", false, "Substring match on owner instead of exact")
	return cmd
}
