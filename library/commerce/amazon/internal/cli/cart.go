package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon/internal/amazon"
)

func newCartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "cart",
		Short:       "Show current Amazon cart for the active profile",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newCartShowCmd())
	return cmd
}

func newCartShowCmd() *cobra.Command {
	var legacy bool
	cmd := &cobra.Command{
		Use:         "show",
		Short:       "Render the Amazon cart (via headless browser, with default address + card last-4)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Drives the Amazon cart in a headless Chromium so JS-rendered cells (cart
items, default ship-to, default payment last-4) are visible — the static-HTTP
parser can only see skeleton placeholders. Pass --legacy to use the old static
path (faster, but no defaults surfaced).

On a CAPTCHA / sign-in challenge, exits 9 with a deeplink in the JSON output so
the caller can hand the user back to their browser.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			if err := app.RequireSession(); err != nil {
				return err
			}
			if app.DryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "would render /gp/cart/view.html in headless browser")
				return nil
			}
			if !legacy {
				ctx, cancel := contextWithTimeout(app.Ctx, 90*time.Second)
				defer cancel()
				res, herr := runBrowserHelper(ctx, "cart-show", app.Cfg.CookiesPath(app.Profile.Name), false)
				if herr != nil {
					if app.JSON && res != nil {
						_ = json.NewEncoder(cmd.OutOrStdout()).Encode(res)
					}
					return herr
				}
				// Merge stored defaults (card / address labels) into the response so
				// the agent can confirm payment info Amazon's cart page won't expose.
				if d, derr := LoadProfileDefaults(app.Cfg.ProfileDir(app.Profile.Name)); derr == nil {
					if res.DefaultCardLast4 == "" {
						res.DefaultCardLast4 = d.CardLast4
					}
					if res.DefaultAddress == "" {
						res.DefaultAddress = d.AddressLabel
					}
				}
				if app.JSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
				}
				if len(res.Items) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "(cart is empty)")
					return nil
				}
				fmt.Fprint(cmd.OutOrStdout(), formatBrowserCart(res))
				return nil
			}
			// Legacy static path
			client, err := amazon.New(app.Profile, app.Session)
			if err != nil {
				return err
			}
			ctx, cancel := contextWithTimeout(app.Ctx, 30*time.Second)
			defer cancel()
			lines, err := client.CartView(ctx)
			if err != nil {
				return coded(ExitTransient, "cart view: %v", err)
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(lines)
			}
			if len(lines) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(cart is empty)")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "QTY\tASIN\tTITLE")
			for _, l := range lines {
				fmt.Fprintf(tw, "%d\t%s\t%s\n", l.Quantity, l.ASIN, truncate(l.Title, 60))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&legacy, "legacy", false, "Use the static HTTP parser instead of the headless browser (faster, no defaults surfaced).")
	return cmd
}
