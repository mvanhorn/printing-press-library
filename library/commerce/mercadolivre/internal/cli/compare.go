// Copyright 2026 wandreis and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: compare N products side-by-side from the LOCAL store.
// pp:data-source local

package cli

import (
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var flagDiff bool

	cmd := &cobra.Command{
		Use:   "compare <catalog_id> <catalog_id> [catalog_id...]",
		Short: "Compare N products side-by-side as a normalized attribute matrix with a price row",
		Long: "Compare two or more locally-stored products side-by-side. Rows are the union " +
			"of their technical-spec attributes plus a price row; columns are the products. " +
			"Reads only the local store — run 'products get <id>' first to populate it.",
		Example:     "  mercadolivre-pp-cli compare MLB51764304 MLB40287816 --diff --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			s, err := openLocalStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			ids := args
			// Gather each product's attributes + price.
			attrsByID := make(map[string]map[string]string, len(ids))
			priceByID := make(map[string]string, len(ids))
			nameSet := map[string]bool{}
			for _, id := range ids {
				attrs, err := s.ProductAttributes(id)
				if err != nil {
					return err
				}
				attrsByID[id] = attrs
				for name := range attrs {
					nameSet[name] = true
				}
				if pb, ok, err := s.GetProductBasic(id); err != nil {
					return err
				} else if ok && pb.Price > 0 {
					priceByID[id] = strconv.FormatFloat(pb.Price, 'f', -1, 64)
				}
			}

			names := make([]string, 0, len(nameSet))
			for n := range nameSet {
				names = append(names, n)
			}
			sort.Strings(names)

			// Build rows: one per attribute name, plus a trailing price row.
			var rows []map[string]any
			appendRow := func(attribute string, valueFor func(id string) string) {
				vals := make([]string, 0, len(ids))
				row := map[string]any{"attribute": attribute}
				for _, id := range ids {
					v := valueFor(id)
					row[id] = v
					vals = append(vals, v)
				}
				if flagDiff && allEqual(vals) {
					return // hide rows every product agrees on
				}
				rows = append(rows, row)
			}

			for _, name := range names {
				name := name
				appendRow(name, func(id string) string { return attrsByID[id][name] })
			}
			appendRow("price", func(id string) string { return priceByID[id] })

			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().BoolVar(&flagDiff, "diff", false, "Hide rows where every product shares the identical value")
	return cmd
}

// allEqual reports whether every element of vals is identical.
func allEqual(vals []string) bool {
	if len(vals) < 2 {
		return true
	}
	first := vals[0]
	for _, v := range vals[1:] {
		if v != first {
			return false
		}
	}
	return true
}
