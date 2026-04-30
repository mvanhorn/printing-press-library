package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	srcebay "github.com/mvanhorn/printing-press-library/library/commerce/ebay/internal/source/ebay"

	"github.com/spf13/cobra"
)

func newSnipeCmd(flags *rootFlags) *cobra.Command {
	var (
		max      float64
		lead     time.Duration
		simulate bool
		group    string
		now      bool
	)
	cmd := &cobra.Command{
		Use:   "snipe <itemId>",
		Short: "Schedule a max bid that fires through the user's session in the final seconds",
		Long: `Place a sniper bid: hold a max client-side, bid through your authenticated
eBay session at lead-seconds before the auction ends. Other bidders only see
the bid when it's too late to react.

Default lead is 25s. Use --simulate to dry-run without placing. Use --now to
fire immediately (skip the wait).

Examples:
  ebay-pp-cli snipe 123456789012 --max 50.00
  ebay-pp-cli snipe 123456789012 --max 50.00 --lead 8s
  ebay-pp-cli snipe 123456789012 --max 50.00 --simulate`,
		Example: `  ebay-pp-cli snipe 123456789012 --max 50.00
  ebay-pp-cli snipe 123456789012 --max 50.00 --simulate`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			itemID := args[0]
			if !cmd.Flags().Changed("max") && !flags.dryRun && !simulate {
				return fmt.Errorf("required flag \"--max\" not set")
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := context.Background()
			plan := srcebay.BidPlan{
				ItemID:      itemID,
				MaxAmount:   max,
				Currency:    "USD",
				LeadSeconds: int(lead.Seconds()),
				Group:       group,
				Simulate:    simulate,
			}
			placer := srcebay.NewBidPlacer(c)

			// In simulate mode just plan + return.
			if simulate {
				res, err := placer.Plan(ctx, plan)
				if err != nil && res == nil {
					return err
				}
				return emit(cmd, flags, res, err)
			}

			// Look up the item end time so we can wait until lead-seconds before close.
			src := srcebay.New(c)
			listing, err := src.FetchItem(ctx, itemID)
			if err != nil {
				return fmt.Errorf("fetching item %s: %w", itemID, err)
			}
			endsAt := listing.EndsAt
			if now || endsAt.IsZero() {
				// Fire immediately when --now or item end time is unknown.
				res, err := placer.Place(ctx, plan)
				return emit(cmd, flags, res, err)
			}
			fireAt := endsAt.Add(-lead)
			waitFor := time.Until(fireAt)
			if waitFor < 0 {
				// Auction is already past lead window; place now.
				res, err := placer.Place(ctx, plan)
				return emit(cmd, flags, res, err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "snipe scheduled: item %s ends ~%s, firing in %s (lead %s)\n",
				itemID, endsAt.Format(time.RFC3339), waitFor.Round(time.Second), lead)
			res, err := placer.PlaceAt(ctx, endsAt, int(lead.Seconds()), plan)
			return emit(cmd, flags, res, err)
		},
	}
	cmd.Flags().Float64Var(&max, "max", 0, "Maximum bid amount you authorize (held client-side)")
	cmd.Flags().DurationVar(&lead, "lead", 25*time.Second, "Lead time before auction end to place the bid (e.g. 8s, 25s, 1m)")
	cmd.Flags().BoolVar(&simulate, "simulate", false, "Dry-run the snipe without placing a bid")
	cmd.Flags().BoolVar(&now, "now", false, "Place the bid immediately rather than waiting for lead time")
	cmd.Flags().StringVar(&group, "group", "", "Optional bid-group name for coordinated multi-item snipes")
	return cmd
}

func emit(cmd *cobra.Command, flags *rootFlags, res *srcebay.BidResult, err error) error {
	if res == nil {
		if err != nil {
			return err
		}
		return nil
	}
	data, jerr := json.Marshal(res)
	if jerr != nil {
		return jerr
	}
	if !flags.asJSON && !flags.agent && flags.selectFields == "" {
		w := cmd.OutOrStdout()
		fmt.Fprintf(w, "Item:    %s\n", res.ItemID)
		fmt.Fprintf(w, "Amount:  %s%.2f\n", currencySymbol(res.Currency), res.Amount)
		fmt.Fprintf(w, "Status:  %s\n", res.Status)
		if res.Message != "" {
			fmt.Fprintf(w, "Note:    %s\n", res.Message)
		}
		if res.WaitedSecs > 0 {
			fmt.Fprintf(w, "Waited:  %ds\n", res.WaitedSecs)
		}
		if !res.PlacedAt.IsZero() {
			fmt.Fprintf(w, "Placed:  %s\n", res.PlacedAt.Format(time.RFC3339))
		}
		fmt.Fprintf(w, "URL:     %s\n", res.BidURL)
	} else {
		_ = printOutputWithFlags(cmd.OutOrStdout(), data, flags)
	}
	return err
}

func currencySymbol(code string) string {
	switch code {
	case "USD":
		return "$"
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	case "JPY":
		return "¥"
	default:
		return code + " "
	}
}
