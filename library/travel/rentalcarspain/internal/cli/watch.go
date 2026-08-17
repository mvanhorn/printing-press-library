// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: price-drop watch with target (cron-friendly typed exit codes).

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/carsource"
	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/store"

	"github.com/spf13/cobra"
)

type watchView struct {
	Search        string  `json:"search"`
	CheapestTotal float64 `json:"cheapest_total"`
	TargetPrice   float64 `json:"target_price"`
	Hit           bool    `json:"target_hit"`
	Supplier      string  `json:"cheapest_supplier"`
	Car           string  `json:"cheapest_car"`
	Currency      string  `json:"currency"`
	OfferCount    int     `json:"offer_count"`
}

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var targetPrice float64
	cmd := &cobra.Command{
		Use:         "watch <saved-name>",
		Short:       "Re-quote a saved search; exit 0 when at or below --target-price, 10 otherwise",
		Long:        "Re-run a saved search, record a price snapshot, and use a typed exit code so cron can act: exit 0 when the cheapest full-insurance total is at or below --target-price, exit 10 when it is still above. Without --target-price it always exits 0 after recording the snapshot.\n\nDo NOT use it for a one-off quote; use 'search'.",
		Example:     "  rentalcarspain-pp-cli watch agp-august --target-price 250 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,10"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("watch needs a <saved-name>"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, defaultDBPath("rentalcarspain-pp-cli"))
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			ss, err := db.GetSavedSearch(ctx, args[0])
			if err != nil {
				return apiErr(err)
			}
			if ss == nil {
				return notFoundErr(fmt.Errorf("no saved search named %q — add it with 'saved add'", args[0]))
			}
			dys := carsource.NewDoYouSpain(carHTTPClient(flags))
			offers, err := dys.Search(ctx, carsource.SearchQuery{ // pp:client-call
				LocationCode: ss.LocationCode, DropoffCode: ss.DropoffCode,
				Pickup: ss.Pickup, Dropoff: ss.Dropoff, DriverAge: ss.DriverAge,
			})
			if err != nil {
				return apiErr(err)
			}
			key := searchKey(ss.LocationCode, ss.DropoffCode, ss.Pickup, ss.Dropoff, ss.DriverAge)
			recordSnapshotFor(ctx, flags, key, offers)

			best, ok := cheapest(offers)
			view := watchView{
				Search: args[0], TargetPrice: targetPrice, Currency: "EUR", OfferCount: len(offers),
			}
			if ok {
				view.CheapestTotal = best.Total
				view.Supplier = best.Supplier
				view.Car = best.Car
			}
			view.Hit = watchTargetHit(targetPrice, best.Total, ok)

			if wantsMachineOutput(flags) || flags.asJSON {
				b, _ := json.Marshal(view)
				if err := printOutputWithFlags(cmd.OutOrStdout(), b, flags); err != nil {
					return err
				}
			} else {
				w := cmd.OutOrStdout()
				if !ok {
					fmt.Fprintf(w, "No offers found for %q.\n", args[0])
				} else if view.Hit && targetPrice > 0 {
					fmt.Fprintf(w, "HIT: %s cheapest is %.2f %s (%s, %s) — at or below target %.2f\n",
						args[0], best.Total, view.Currency, best.Supplier, truncate(best.Car, 24), targetPrice)
				} else if targetPrice > 0 {
					fmt.Fprintf(w, "above target: %s cheapest is %.2f %s (target %.2f)\n",
						args[0], best.Total, view.Currency, targetPrice)
				} else {
					fmt.Fprintf(w, "%s cheapest is %.2f %s (%s)\n", args[0], best.Total, view.Currency, best.Supplier)
				}
			}
			if !ok {
				// No offers at all: never exit 0. A missing quote must read as a
				// genuine failure to cron, not a silent "target reached".
				return apiErr(fmt.Errorf("no offers returned for saved search %q", args[0]))
			}
			if targetPrice > 0 && !view.Hit {
				// Typed exit code 10: still above target (control flow, not an error).
				return &cliError{code: 10, err: fmt.Errorf("cheapest %.2f above target %.2f", view.CheapestTotal, targetPrice)}
			}
			return nil
		},
	}
	cmd.Flags().Float64Var(&targetPrice, "target-price", 0, "Alert threshold: exit 0 when cheapest total is at or below this")
	return cmd
}

// watchTargetHit reports whether a watch run counts as a "hit" (exit 0). With no
// target (<= 0) every run hits; otherwise a hit needs a real cheapest total
// (haveOffer, positive) at or below the target. A zero/absent price never hits a
// positive target — a missing quote must not read as "target reached".
func watchTargetHit(targetPrice, cheapestTotal float64, haveOffer bool) bool {
	if !haveOffer {
		return false
	}
	return targetPrice <= 0 || (cheapestTotal > 0 && cheapestTotal <= targetPrice)
}
