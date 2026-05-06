package cli

import (
	"errors"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/cartstore"

	"github.com/spf13/cobra"
)

// newReorderCmd replays the user's most recent saved template (or the
// active cart). Templates are the local stand-in for synced order
// history: each one is a remembered order shape that can be replayed.
// When no template and no active cart exist, the command returns a
// structured zero envelope with a hint rather than failing.
func newReorderCmd(flags *rootFlags) *cobra.Command {
	var fromTemplate string
	var substitute bool
	var confirm bool

	cmd := &cobra.Command{
		Use:   "reorder",
		Short: "Replay your most recent saved template or active cart",
		Long: `Replay your most recent saved template (or the active cart if no
templates exist).

Modes:
  - default: reads the most recent template by created_at; falls back to
    the active cart if no templates are saved.
  - --from-template <name>: reads a specific template by name.
  - --confirm: actually attempt to place the order (default: dry-run).
  - --substitute-unavailable: stub for FTS-substituting unavailable items
    against the current menu (not yet wired).

Returns a structured envelope describing the replayed cart (store, items,
service) so agents can decide whether to confirm.`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		Example:     "  dominos-pp-cli reorder\n  dominos-pp-cli reorder --from-template friday\n  dominos-pp-cli reorder --confirm",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = substitute

			// 1) Specific template by name takes priority.
			if fromTemplate != "" {
				t, err := cartstore.LoadTemplate(fromTemplate)
				if err != nil {
					if errors.Is(err, cartstore.ErrNotFound) {
						return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
							"action": "reorder",
							"placed": false,
							"hint":   "template not found: " + fromTemplate,
						}, flags)
					}
					return err
				}
				return emitReorderEnvelope(cmd, flags, "template:"+t.Name, t.StoreID, t.Service, t.Address, t.Items, confirm)
			}

			// 2) Pick the most recent saved template.
			names, err := cartstore.ListTemplates()
			if err != nil {
				return err
			}
			var newest *cartstore.Template
			var newestAt time.Time
			var newestName string
			for _, n := range names {
				t, err := cartstore.LoadTemplate(n)
				if err != nil {
					if errors.Is(err, cartstore.ErrNotFound) {
						continue
					}
					return err
				}
				if newest == nil || t.CreatedAt.After(newestAt) {
					newest = t
					newestAt = t.CreatedAt
					newestName = n
				}
			}
			if newest != nil {
				return emitReorderEnvelope(cmd, flags, "template:"+newestName, newest.StoreID, newest.Service, newest.Address, newest.Items, confirm)
			}

			// 3) Fall back to the active cart.
			active, err := cartstore.LoadActive()
			if err != nil {
				if errors.Is(err, cartstore.ErrNotFound) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"action": "reorder",
						"placed": false,
						"hint":   "no saved templates and no active cart; build one with 'cart new' or 'template save <name>'",
					}, flags)
				}
				return err
			}
			return emitReorderEnvelope(cmd, flags, "active-cart", active.StoreID, active.Service, active.Address, active.Items, confirm)
		},
	}
	var last bool
	cmd.Flags().StringVar(&fromTemplate, "from-template", "", "Replay a specific named template instead of the most recent")
	cmd.Flags().BoolVar(&last, "last", false, "Use the most recent saved template/cart (default behavior; this flag is for narrative parity)")
	cmd.Flags().BoolVar(&substitute, "substitute-unavailable", false, "FTS-substitute unavailable items against the current menu (not yet wired)")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually attempt to place the replayed order (default: dry-run)")
	_ = last
	return cmd
}

// emitReorderEnvelope writes the reorder JSON envelope describing the
// cart that would be replayed. We intentionally do not call the place
// endpoint here — placing requires building a fully validated/priced
// order body, which is the responsibility of `orders place-order`.
// Reorder's job is to surface the reusable cart shape.
func emitReorderEnvelope(cmd *cobra.Command, flags *rootFlags, source, storeID, service, address string, items []cartstore.CartItem, confirm bool) error {
	itemRows := make([]map[string]any, 0, len(items))
	for _, it := range items {
		itemRows = append(itemRows, map[string]any{
			"code":     it.Code,
			"qty":      it.Qty,
			"size":     it.Size,
			"toppings": it.Toppings,
		})
	}
	sort.SliceStable(itemRows, func(i, j int) bool {
		ci, _ := itemRows[i]["code"].(string)
		cj, _ := itemRows[j]["code"].(string)
		return ci < cj
	})
	envelope := map[string]any{
		"action":   "reorder",
		"source":   source,
		"store_id": storeID,
		"service":  service,
		"address":  address,
		"items":    itemRows,
		"placed":   false,
	}
	if confirm {
		envelope["next_step"] = "use 'orders place-order' with the rendered Order body to place this cart"
	} else {
		envelope["dry_run"] = true
	}
	return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
}
