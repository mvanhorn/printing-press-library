package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/instacart"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/store"
)

// newOpsCmd exposes operations on the persisted GraphQL operation cache.
// Hidden from the main help because normal users don't need it.
func newOpsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "ops",
		Short:  "Inspect and manage the local GraphQL operation cache",
		Hidden: true,
	}
	cmd.AddCommand(newOpsListCmd(), newOpsSeedCmd())
	return cmd
}

func newOpsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show all persisted GraphQL operations known to the CLI",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			ops, err := app.Store.ListOps()
			if err != nil {
				return err
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(ops)
			}
			if len(ops) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no operations cached (run `instacart ops seed` or `instacart capture`)")
				return nil
			}
			for _, op := range ops {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %s\n", op.OperationName, op.Sha256Hash[:16]+"...")
			}
			return nil
		},
	}
}

func newOpsSeedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "seed",
		Short: "Load built-in GraphQL operation hashes into the local store",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			seeded := 0
			for _, name := range instacart.OpNames() {
				seed := instacart.DefaultOps[name]
				if seed.Hash == "" {
					continue
				}
				if err := app.Store.UpsertOp(store.Op{OperationName: name, Sha256Hash: seed.Hash}); err != nil {
					return err
				}
				seeded++
			}
			fmt.Fprintf(cmd.OutOrStdout(), "seeded %d operations\n", seeded)
			return nil
		},
	}
}
