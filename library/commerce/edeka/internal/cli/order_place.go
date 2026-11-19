// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call (calls the market shop through internal/edeka.Shop)

package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/edeka"
)

func newNovelOrderPlaceCmd(flags *rootFlags) *cobra.Command {
	var market, payment, slot, typ string
	var confirm bool
	var expectTotal string
	cmd := &cobra.Command{
		Use:   "place",
		Short: "Preview the order (slot, payment, items, total) and place it only with --confirm.",
		Long: "Use this command to preview or place the order. Do NOT use it to list past orders; use 'order list'.\n\n" +
			"Without --confirm it only previews: it may choose the slot (--slot) and the payment method in your shop\n" +
			"session, walks the checkout to the shop's final review page and prints slot, payment, items and total.\n" +
			"With --confirm and --expect-total (the preview's total) it presses the shop's final order button; it refuses\n" +
			"when the total changed since the preview. Only pay-at-handover methods are supported: mxcash (cash) and\n" +
			"mxlocalcashcard (card at handover). Ask the customer before using --confirm. --dry-run sends nothing.",
		Example:     "  edeka-pp-cli order place --payment mxcash --dry-run\n  edeka-pp-cli order place --slot earliest --payment mxcash\n  edeka-pp-cli order place --slot '2030-01-02 10:00' --payment mxcash --confirm --expect-total 597",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live", "pp:happy-args": "--payment=mxcash;--slot=earliest", "pp:preview-happy-path": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if done, err := verifyDryRun(cmd, flags, "order place"); done {
				return err
			}
			if payment != "mxcash" && payment != "mxlocalcashcard" {
				return usageErr(fmt.Errorf("payment %q is not supported: use mxcash (cash) or mxlocalcashcard (card at handover); online card and direct debit need data this CLI does not handle", payment))
			}
			if confirm && flags.dryRun {
				return usageErr(errors.New("--confirm and --dry-run contradict each other"))
			}
			if confirm && strings.EqualFold(strings.TrimSpace(slot), "earliest") {
				return usageErr(errors.New("--confirm needs the exact slot the customer accepted: pass --slot 'YYYY-MM-DD HH:MM' from the preview's confirm_slot, or leave --slot out to keep the slot reserved in the session ('earliest' can pick a different time)"))
			}
			if flags.dryRun {
				plan := map[string]any{"dry_run": true, "payment": payment, "slot": slot, "would": "check the basket, choose the slot if --slot is set, walk the checkout to the shop's review page and print the preview; no order"}
				return emit(cmd, flags, plan, func(w io.Writer) {
					fmt.Fprintln(w, "Dry run: would preview the order (no request sent). Run without --dry-run for the shop's real preview; add --confirm only after a yes.")
				})
			}
			ctx = cmd.Context()
			sh, err := openShop(cmd, flags, market)
			if err != nil {
				return err
			}
			defer func() { _ = sh.Persist() }()
			b, err := sh.Basket(ctx)
			if err != nil {
				return edekaErr(err)
			}
			if len(b.Items) == 0 {
				return usageErr(errors.New("the basket is empty: add items first (basket add, list add)"))
			}
			var chosen *edeka.Slot
			if slot != "" {
				s, err := pickSlot(ctx, sh, slot, typ)
				if err != nil {
					return err
				}
				if err := sh.ReserveSlot(ctx, s); err != nil {
					return edekaErr(err)
				}
				chosen = &s
			} else if txt, err := sh.SelectedSlotText(ctx); err != nil || strings.TrimSpace(txt) == "" {
				return usageErr(errors.New("no slot chosen in this session: pass --slot earliest or --slot 'YYYY-MM-DD HH:MM' (see 'slots list')"))
			}
			review, form, err := sh.PreviewOrder(ctx, payment)
			if err != nil {
				return edekaErr(err)
			}
			out := map[string]any{"market": sh.Host, "review": review, "placed": false}
			if chosen != nil {
				out["slot"] = chosen
				out["confirm_slot"] = chosen.Date + " " + chosen.From
			}
			if !confirm {
				out["next_step"] = fmt.Sprintf("show this preview to the customer; only after a clear yes run again with --confirm --expect-total %d", review.TotalCents)
				if chosen != nil {
					out["next_step"] = fmt.Sprintf("%s --slot '%s'", out["next_step"], out["confirm_slot"])
				}
				return emit(cmd, flags, out, func(w io.Writer) { printReview(w, review, false) })
			}
			if expectTotal == "" {
				return usageErr(fmt.Errorf("--confirm needs --expect-total with the total of the preview the customer accepted (now %s, %d cents)", review.Total, review.TotalCents))
			}
			want, ok := parseTotal(expectTotal)
			if !ok {
				return usageErr(fmt.Errorf("--expect-total %q: give cents (597) or euros (5,97)", expectTotal))
			}
			if want != review.TotalCents {
				out["placed"] = false
				return emitThen(cmd, flags, out, usageErr(fmt.Errorf("the basket total changed since the preview: now %s, expected %s; show the new preview to the customer", review.Total, edeka.FormatCents(want))))
			}
			if !review.Ready {
				return apiErr(errors.New("the shop's review page has no order button; open the shop to see why"))
			}
			nr, err := sh.PlaceOrder(ctx, form)
			if err != nil {
				return edekaErr(err)
			}
			out["placed"] = true
			out["order_number"] = nr
			return emit(cmd, flags, out, func(w io.Writer) {
				printReview(w, review, true)
				fmt.Fprintf(w, "Order placed. Order number: %s\n", nr)
			})
		},
	}
	cmd.Flags().StringVar(&payment, "payment", "mxcash", "Payment method id: mxcash or mxlocalcashcard (see 'payment options')")
	cmd.Flags().StringVar(&slot, "slot", "", "Choose a slot first: 'earliest' or 'YYYY-MM-DD HH:MM' (default: keep the slot chosen in the session)")
	cmd.Flags().StringVar(&typ, "type", "pickup", "Slot type for --slot: pickup or delivery")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Really place the order (after the customer confirmed the preview)")
	cmd.Flags().StringVar(&expectTotal, "expect-total", "", "Total of the accepted preview, in cents (597) or euros (5,97); required with --confirm")
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}

// parseTotal reads "597" (cents) or "5,97" / "5.97" (euros).
func parseTotal(s string) (int, bool) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "€"))
	if strings.Contains(s, ",") {
		return edeka.ParseCents(s)
	}
	if strings.Contains(s, ".") {
		return edeka.ParseCents(strings.ReplaceAll(s, ".", ","))
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, s != ""
}

func printReview(w io.Writer, r *edeka.OrderReview, placed bool) {
	if !placed {
		fmt.Fprintln(w, "Order preview (not placed):")
	}
	fmt.Fprintf(w, "  Slot:     %s %s\n", r.Slot, r.ShippingMethod)
	fmt.Fprintf(w, "  Payment:  %s\n", r.Payment)
	for _, it := range r.Items {
		fmt.Fprintf(w, "  %2dx %-46s %9s\n", it.Quantity, truncate(it.Title, 46), edeka.FormatCents(it.TotalCents))
	}
	fmt.Fprintf(w, "  Fee:      %s\n  Total:    %s\n", edeka.FormatCents(r.FeeCents), r.Total)
}
