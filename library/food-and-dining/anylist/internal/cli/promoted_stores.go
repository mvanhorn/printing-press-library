package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/store"

	"github.com/spf13/cobra"
)

func newStoresPromotedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "stores",
		Short:       "List all stores and store filters",
		Long:        "Shortcut for 'stores list'. List all stores and store filters",
		Example:     "  anylist-pp-cli stores",
		Annotations: map[string]string{"pp:endpoint": "stores.list", "pp:method": "POST", "pp:path": "/data/user-data/get", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			st, err := store.Open(cfg)
			if err != nil {
				return fmt.Errorf("no local data found — run 'anylist-pp-cli sync' first")
			}
			defer st.Close()

			stores, err := st.GetStores()
			if err != nil {
				return fmt.Errorf("reading stores: %w", err)
			}

			if len(stores) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No stores found — run 'anylist-pp-cli sync' first")
				return nil
			}

			if flags.asJSON {
				type storeJSON struct {
					ID        string `json:"id"`
					Name      string `json:"name"`
					ListID    string `json:"list_id,omitempty"`
					SortIndex int    `json:"sort_index"`
				}
				out := make([]storeJSON, len(stores))
				for i, s := range stores {
					out[i] = storeJSON{ID: s.ID, Name: s.Name, ListID: s.ListID, SortIndex: s.SortIndex}
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "NAME\tLIST ID\tSORT")
			for _, s := range stores {
				fmt.Fprintf(tw, "%s\t%s\t%d\n", s.Name, s.ListID, s.SortIndex)
			}
			return tw.Flush()
		},
	}
	return cmd
}
