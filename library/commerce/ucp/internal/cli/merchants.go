package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/ucp/internal/registry"
	"github.com/spf13/cobra"
)

func newMerchantsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merchants",
		Short: "UCP merchant directory",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newMerchantsListCmd(flags))
	return cmd
}

func newMerchantsListCmd(flags *rootFlags) *cobra.Command {
	var categoryFilter string
	var ropeToysOnly bool

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List known UCP merchants (seeded registry of 58 Grade-A merchants)",
		Example: `  ucp-pp-cli merchants list --rope-toys`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			all := registry.Default()

			// Sort: pet first, then alphabetic within category.
			sort.SliceStable(all, func(i, j int) bool {
				ci, cj := all[i].Category, all[j].Category
				iPet := strings.HasPrefix(ci, "pet")
				jPet := strings.HasPrefix(cj, "pet")
				if iPet != jPet {
					return iPet
				}
				if ci != cj {
					return ci < cj
				}
				return all[i].Domain < all[j].Domain
			})

			// Apply filters.
			var filtered []registry.Merchant
			for _, m := range all {
				if categoryFilter != "" && !strings.HasPrefix(m.Category, categoryFilter) {
					continue
				}
				if ropeToysOnly && !m.HasRopeToys {
					continue
				}
				filtered = append(filtered, m)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(filtered)
			}

			if len(filtered) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No merchants match the given filters.")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DOMAIN\tGRADE\tCATEGORY\tROPE-TOYS")
			for _, m := range filtered {
				ropeToys := ""
				if m.HasRopeToys {
					ropeToys = "yes"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", m.Domain, m.Grade, m.Category, ropeToys)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&categoryFilter, "category", "", "Filter by category prefix (e.g. pet, fashion, beauty)")
	cmd.Flags().BoolVar(&ropeToysOnly, "rope-toys", false, "Show only merchants known to carry rope/tug toys")
	return cmd
}
