package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/store"

	"github.com/spf13/cobra"
)

func newStartersPromotedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "starters",
		Short:       "List starter list items",
		Long:        "Shortcut for 'starters list'. List starter list items",
		Example:     "  anylist-pp-cli starters",
		Annotations: map[string]string{"pp:endpoint": "starters.list", "pp:method": "POST", "pp:path": "/data/user-data/get", "mcp:read-only": "true"},
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

			items, err := st.GetStarterItems("starter")
			if err != nil {
				return fmt.Errorf("reading starters: %w", err)
			}

			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No starter items found — run 'anylist-pp-cli sync' first")
				return nil
			}

			if flags.asJSON {
				type starterJSON struct {
					Name     string `json:"name"`
					Quantity string `json:"quantity,omitempty"`
					Category string `json:"category,omitempty"`
				}
				out := make([]starterJSON, len(items))
				for i, it := range items {
					out[i] = starterJSON{Name: it.Name, Quantity: it.Quantity, Category: it.Category}
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "NAME\tQTY\tCATEGORY")
			for _, it := range items {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", it.Name, it.Quantity, it.Category)
			}
			return tw.Flush()
		},
	}
	return cmd
}
