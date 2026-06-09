package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon/internal/amazon"
)

func newCheckoutCmd() *cobra.Command {
	var yes, legacy bool
	cmd := &cobra.Command{
		Use:   "checkout",
		Short: "Place an order with default shipping + payment (--yes required)",
		Long: `Drives Amazon's checkout in a headless Chromium so JS executes and the
request fingerprint matches a real browser (the static-HTTP path trips Amazon's
robot check). Refuses to place the order without --yes. With --dry-run, walks
to the order-review page but does NOT click "Place order". With --legacy, uses
the old static POST path (faster but reliably trips CAPTCHA).

On a CAPTCHA / sign-in challenge, exits 9 with a deeplink in the JSON output so
the agent can hand the user back to her browser for the final tap.`,
		Example: "  amazon-pp-cli checkout --dry-run\n  amazon-pp-cli checkout --yes --profile personal",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			if !yes && !app.DryRun {
				return coded(ExitConfirm, "checkout requires --yes to actually place the order (use --dry-run to preview)")
			}
			if err := app.RequireSession(); err != nil {
				return err
			}
			if !legacy {
				ctx, cancel := contextWithTimeout(app.Ctx, 180*time.Second)
				defer cancel()
				placeOrder := yes && !app.DryRun
				res, herr := runBrowserHelper(ctx, "checkout", app.Cfg.CookiesPath(app.Profile.Name), placeOrder)
				if herr != nil {
					if app.JSON && res != nil {
						_ = json.NewEncoder(cmd.OutOrStdout()).Encode(res)
					}
					return herr
				}
				if app.JSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
				}
				switch res.Status {
				case "review_ready":
					fmt.Fprintln(cmd.OutOrStdout(), "Order review reached (dry-run). Re-run with --yes to place.")
					fmt.Fprint(cmd.OutOrStdout(), formatBrowserCart(res))
				case "placed":
					fmt.Fprintf(cmd.OutOrStdout(), "ORDER PLACED: %s\n", res.OrderID)
				case "placed_unconfirmed":
					fmt.Fprintf(cmd.OutOrStdout(), "place-order POST returned but no order ID parsed; check %s\n", res.ConfirmationURL)
				default:
					fmt.Fprintf(cmd.OutOrStdout(), "status=%s\n", res.Status)
				}
				return nil
			}
			// Legacy static path
			client, err := amazon.New(app.Profile, app.Session)
			if err != nil {
				return err
			}
			if app.DryRun {
				if app.JSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(amazon.CheckoutResult{
						Confirmed:  false,
						StatusNote: "dry-run (legacy): would GET /gp/buy/spc/handlers/display.html and POST place-order with --yes",
					})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "dry-run (legacy): would GET checkout page and POST place-order.")
				return nil
			}
			client.DryRun = false
			ctx, cancel := contextWithTimeout(app.Ctx, 60*time.Second)
			defer cancel()
			result, err := client.PlaceOrder(ctx, yes)
			if err != nil {
				return coded(ExitTransient, "checkout: %v", err)
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			if result.Confirmed {
				fmt.Fprintf(cmd.OutOrStdout(), "ORDER PLACED: %s\n", result.OrderID)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "not confirmed: %s\n", result.StatusNote)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm you want to actually place the order")
	cmd.Flags().BoolVar(&legacy, "legacy", false, "Use the static HTTP path instead of headless browser (faster but trips CAPTCHA).")
	return cmd
}
