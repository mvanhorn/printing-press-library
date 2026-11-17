package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func init() {
	novelCommandHooks = append(novelCommandHooks, func(root *cobra.Command, flags *rootFlags) {
		if order, _, err := root.Find([]string{"order"}); err == nil && order != root {
			order.AddCommand(newOrderListCmd(flags), newOrderShowCmd(flags), newOrderCancelCmd(flags))
		}
	})
}

func newOrderListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List your online shop orders with status and pickup slot.",
		Example:     "  rewe-pp-cli order list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "order list")
			}
			sh, _, err := openShop(cmd, flags)
			if err != nil {
				return err
			}
			orders, err := sh.Orders(cmd.Context())
			if err != nil {
				return reweErr(err)
			}
			return emit(cmd, flags, orders, func(w io.Writer) {
				if len(orders) == 0 {
					fmt.Fprintln(w, "No online orders.")
				}
				for _, o := range orders {
					fmt.Fprintf(w, "%-16s %-28s %s\n", o.ID, o.Status, o.Slot)
				}
			})
		},
	}
}

func newOrderShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "show [order-id]",
		Short:       "Show status and pickup slot of one order.",
		Example:     "  rewe-pp-cli order show B-AAA-BBB-CCC --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "order show")
			}
			sh, _, err := openShop(cmd, flags)
			if err != nil {
				return err
			}
			o, err := sh.Order(cmd.Context(), args[0])
			if err != nil {
				return reweErr(err)
			}
			return emit(cmd, flags, o, func(w io.Writer) { fmt.Fprintf(w, "%s: %s, %s\n", o.ID, o.Status, o.Slot) })
		},
	}
}

func newOrderCancelCmd(flags *rootFlags) *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:     "cancel [order-id]",
		Short:   "Cancel an online order (only with --confirm; otherwise shows what would be cancelled).",
		Example: "  rewe-pp-cli order cancel B-AAA-BBB-CCC --confirm",
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := verifyDryRun(cmd, flags, "order cancel"); done {
				return err
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			sh, _, err := openShop(cmd, flags)
			if err != nil {
				return err
			}
			before, err := sh.Order(cmd.Context(), args[0])
			if err != nil {
				return reweErr(err)
			}
			if !confirm || flags.dryRun {
				return emit(cmd, flags, map[string]any{"order": before, "cancelled": false, "needs_confirm": true}, func(w io.Writer) {
					fmt.Fprintf(w, "Would cancel %s (%s, %s). Re-run with --confirm.\n", before.ID, before.Status, before.Slot)
				})
			}
			msg, err := sh.CancelOrder(cmd.Context(), args[0])
			if err != nil {
				return reweErr(err)
			}
			after, err := sh.Order(cmd.Context(), args[0])
			if err != nil {
				return reweErr(err)
			}
			out := map[string]any{"order": after, "cancelled": true, "message": msg, "status_before": before.Status}
			return emit(cmd, flags, out, func(w io.Writer) { fmt.Fprintf(w, "%s: %s -> %s. %s\n", after.ID, before.Status, after.Status, msg) })
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Really cancel the order")
	return cmd
}
