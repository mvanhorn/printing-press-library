// Hand-authored (Slice 2): the `orders` parent group (not generated).
// pp:data-source local

package cli

import (
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newS2OrdersCmd(flags))
	})
}

func newS2OrdersCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "orders",
		Short:       "Order views built from your synced receipts",
		Example:     "  etsy-pp-cli orders unshipped",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
}
