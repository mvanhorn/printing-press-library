// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/rewe"
)

var phoneRe = regexp.MustCompile(`^\+?[0-9 /-]{8,20}$`)

func newNovelOrderPlaceCmd(flags *rootFlags) *cobra.Command {
	var slot, phone, message string
	var confirm bool
	cmd := &cobra.Command{
		Use:   "place",
		Short: "Place the pickup order: refresh basket, reserve slot, pay in market, submit - only with --confirm.",
		Long: "Walks the REWE checkout for the current basket: re-validates prices, reserves the slot\n" +
			"(--slot earliest or a slot id), chooses payment at pickup (MARKET_PAYMENT) and submits the order.\n" +
			"Without --confirm (or with --dry-run) it only prints the preview: items, total, slot, payment.\n" +
			"REWE asks for a mobile number for the pickup. This command spends money: agents must show the\n" +
			"preview to the person and pass --confirm only after an explicit yes.",
		Example:     "  rewe-pp-cli order place --slot earliest --phone +4915112345678\n  rewe-pp-cli order place --slot earliest --phone +4915112345678 --confirm",
		Annotations: map[string]string{"pp:data-source": "live", "pp:happy-args": "--slot=earliest;--phone=+4915112345678", "pp:preview-happy-path": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := verifyDryRun(cmd, flags, "order place"); done {
				return err
			}
			if slot == "" {
				slot = "earliest"
			}
			if phone != "" && !phoneRe.MatchString(phone) {
				return usageErr(fmt.Errorf("--phone %q is not a phone number", phone))
			}
			sh, c, err := openShop(cmd, flags)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			id, err := sh.BasketID(ctx)
			if errors.Is(err, rewe.ErrEmptyBasket) && !confirm {
				// A preview of an empty basket is an answer, not a failure.
				empty := map[string]any{"basket": basketView{Items: []basketViewItem{}, Total: rewe.FormatCents(0)}, "placed": false, "needs_confirm": true, "message": "the basket is empty; add items first"}
				return emit(cmd, flags, empty, func(w io.Writer) { fmt.Fprintln(w, "The basket is empty; add items first.") })
			}
			if err != nil {
				return reweErr(err)
			}
			b, err := sh.RefreshBasket(ctx, id)
			if err != nil {
				return reweErr(err)
			}
			if len(b.LineItems) == 0 {
				if !confirm {
					empty := map[string]any{"basket": viewBasket(b), "placed": false, "needs_confirm": true, "message": "the basket is empty; add items first"}
					return emit(cmd, flags, empty, func(w io.Writer) { fmt.Fprintln(w, "The basket is empty; add items first.") })
				}
				return usageErr(errors.New("the basket is empty"))
			}
			slots, err := sh.Slots(ctx)
			if err != nil {
				return reweErr(err)
			}
			var chosen rewe.Slot
			if slot == "earliest" {
				s, ok := earliestSlot(slots, time.Now(), 0)
				if !ok {
					return notFoundErr(errors.New("no pickup slot available"))
				}
				chosen = s
			} else {
				for _, s := range slots {
					if s.ID == slot {
						chosen = s
					}
				}
				if chosen.ID == "" {
					return notFoundErr(fmt.Errorf("slot %s is not offered (any more); run `slots list`", slot))
				}
			}
			pay, err := sh.PaymentOptions(ctx, b.Summary.TotalPrice)
			if err != nil {
				return reweErr(err)
			}
			preview := map[string]any{
				"basket":         viewBasket(b),
				"slot":           chosen,
				"slot_local":     chosen.StartTime.Local().Format("Mon 02.01. 15:04") + " - " + chosen.EndTime.Local().Format("15:04"),
				"market":         c.SelectedMarket,
				"payment":        "MARKET_PAYMENT",
				"market_payment": rewe.MarketPaymentEnabled(pay),
				"placed":         false,
				"needs_confirm":  !confirm,
				"phone_required": phone == "",
			}
			if !rewe.MarketPaymentEnabled(pay) {
				return apiErr(errors.New("payment at pickup (MARKET_PAYMENT) is not enabled for this basket; order not placed"))
			}
			if !confirm || flags.dryRun || phone == "" {
				return emit(cmd, flags, preview, func(w io.Writer) {
					printBasket(w, viewBasket(b))
					fmt.Fprintf(w, "Pickup: %s at market %s, pay at pickup.\n", preview["slot_local"], c.SelectedMarket)
					if phone == "" {
						fmt.Fprintln(w, "A mobile number is required: add --phone.")
					}
					fmt.Fprintln(w, "Preview only. Re-run with --confirm to place the order.")
				})
			}
			if _, err := sh.Reserve(ctx, chosen.ID, b.CustomerUUID, c.SelectedMarket, c.CustomerZipCode); err != nil {
				return reweErr(fmt.Errorf("reserving slot: %w", err))
			}
			if err := sh.SubmitCheckout(ctx, b.ID, chosen.ID); err != nil {
				return reweErr(err)
			}
			res, err := sh.PlaceOrder(ctx, b.ID, phone, message)
			if err != nil {
				return reweErr(err)
			}
			_ = sh.Persist()
			preview["placed"] = true
			preview["needs_confirm"] = false
			preview["result"] = res
			return emit(cmd, flags, preview, func(w io.Writer) {
				fmt.Fprintf(w, "Order placed (HTTP %d). Pickup %s. %s\n", res.Status, preview["slot_local"], res.Location)
			})
		},
	}
	cmd.Flags().StringVar(&slot, "slot", "earliest", "Slot id from `slots list`, or earliest")
	cmd.Flags().StringVar(&phone, "phone", "", "Mobile number REWE uses for pickup questions")
	cmd.Flags().StringVar(&message, "message", "", "Note for the market (Bemerkung)")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Really place the order (spends money)")
	return cmd
}
