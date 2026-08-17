// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: browser-handoff commands for delivery management, subscription
// control, and payment/card management.
//
// These flows (reschedule, skip, suspend, pause, resume, boleto, card management)
// use Django server-rendered forms at <subdomain>.shopper.com.br. They require
// a session cookie and CSRF token that the siteapi Bearer token cannot supply.
// The CLI opens the correct storefront page in the system browser rather than
// attempting to replay the form — no sensitive session data is handled here.
// pp:data-source computed

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/shopper/internal/cliutil"
	"github.com/spf13/cobra"
)

// newDeliveryManageCmd adds reschedule/skip/suspend/boleto subcommands to the
// generated delivery parent. Each opens the relevant storefront page in the browser.
func newDeliveryManageCmd(flags *rootFlags) []*cobra.Command {
	reschedule := &cobra.Command{
		Use:   "reschedule",
		Short: "Open the delivery reschedule calendar in the browser (requires storefront login)",
		Long: `Opens https://<store>.shopper.com.br/shop-cliente/minha-conta/calendario-entregas
in the system browser.

Delivery rescheduling uses POST /shop/minha-conta/alterar-data with a server-computed
form body (data_entrega, botao) that requires a live Django session cookie. The CLI
cannot supply that cookie — reschedule is browser-required.

Only available for subscription stores (programada, fresh, pet).`,
		Example: "  shopper-pp-cli delivery reschedule --store programada\n  shopper-pp-cli delivery reschedule --store fresh",
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return openStorefrontPage(cmd, flags, "/shop-cliente/minha-conta/calendario-entregas")
		},
	}

	skip := &cobra.Command{
		Use:   "skip",
		Short: "Open the skip-delivery page in the browser — subscription stores only (requires storefront login)",
		Long: `Opens https://<store>.shopper.com.br/shop-cliente/minha-conta/pular-entrega/
in the system browser.

Skipping a delivery uses POST /shop/minha-conta/pular-entrega/ with a Django session
cookie. Browser-required.`,
		Example: "  shopper-pp-cli delivery skip --store programada",
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return openStorefrontPage(cmd, flags, "/shop-cliente/minha-conta/pular-entrega/")
		},
	}

	suspend := &cobra.Command{
		Use:   "suspend",
		Short: "Open the suspend-subscription page in the browser — subscription stores only (requires storefront login)",
		Long: `Opens https://<store>.shopper.com.br/shop-cliente/minha-conta/suspender-entrega
in the system browser.

Suspending a subscription uses POST /shop/minha-conta/suspender-entrega with a
Django session cookie. Browser-required.`,
		Example: "  shopper-pp-cli delivery suspend --store programada",
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return openStorefrontPage(cmd, flags, "/shop-cliente/minha-conta/suspender-entrega")
		},
	}

	boleto := &cobra.Command{
		Use:   "boleto",
		Short: "Open the boleto-retrieval page in the browser to get the bank slip for the current order",
		Long: `Opens https://<store>.shopper.com.br/shop-cliente/minha-conta/recuperar-boleto/
in the system browser.

Boleto retrieval uses GET /shop-cliente/minha-conta/recuperar-boleto/ which requires
a live Django session. Browser-required.`,
		Example: "  shopper-pp-cli delivery boleto --store programada",
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return openStorefrontPage(cmd, flags, "/shop-cliente/minha-conta/recuperar-boleto/")
		},
	}

	return []*cobra.Command{reschedule, skip, suspend, boleto}
}

// newSubscriptionCmd adds a subscription parent with pause/resume subcommands.
func newSubscriptionCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "subscription",
		Short:       "subscription subcommands: pause, resume",
		Long:        "Manage Shopper recurring basket subscription. Pause and resume require a browser session.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}

	pause := &cobra.Command{
		Use:   "pause",
		Short: "Open the subscription-pause page in the browser (requires storefront login)",
		Long: `Opens https://<store>.shopper.com.br/shop-cliente/ in the system browser.

Pausing a subscription uses POST /shop/carrinho/pause/ with a Django session cookie.
Browser-required. Only available for subscription stores (programada, fresh, pet).`,
		Example: "  shopper-pp-cli subscription pause --store programada",
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return openStorefrontPage(cmd, flags, "/shop-cliente/")
		},
	}

	resume := &cobra.Command{
		Use:   "resume",
		Short: "Open the subscription-resume page in the browser (requires storefront login)",
		Long: `Opens https://<store>.shopper.com.br/shop-cliente/ in the system browser.

Resuming a subscription uses POST /shop/carrinho/play/ with a Django session cookie.
Browser-required.`,
		Example: "  shopper-pp-cli subscription resume --store programada",
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return openStorefrontPage(cmd, flags, "/shop-cliente/")
		},
	}

	cmd.AddCommand(pause, resume)
	return cmd
}

// newPaymentCmd adds a payment parent with a cards subcommand.
func newPaymentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "payment",
		Short:       "payment subcommands: cards",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}

	var openBrowserFlag bool
	cards := &cobra.Command{
		Use:   "cards",
		Short: "Show payment card info and open card-management in the browser (card add/delete is browser-required)",
		Long: `Fetches basic card-slot availability from /features/stores and shows the
payment parameters for the active store (minimum days for credit card, boleto, PIX).

Card add/delete/set-default uses Django server-rendered forms at
/shop-cliente/minha-conta/cartoes/ with a session cookie and CSRF token.
The CLI never handles raw card numbers.

Use --open to launch the card management page in the system browser.`,
		Example: "  shopper-pp-cli payment cards --store programada\n  shopper-pp-cli payment cards --store programada --open",
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"would":"fetch /features/stores for payment parameters"}`)
				return nil
			}

			storeName := flags.store
			if storeName == "" {
				storeName = "programada"
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			storesData, err := c.Get(cmd.Context(), "/features/stores", nil)
			params := extractPaymentParams(storesData, storeName)
			if err != nil {
				params = nil
			}

			cardPageURL := "https://" + resolveSubdomain(storeName) + ".shopper.com.br/shop-cliente/minha-conta/cartoes/"

			result := map[string]any{
				"store":               storeName,
				"payment_params":      params,
				"card_management_url": cardPageURL,
				"browser_required":    "Card add/delete/set-default requires a browser session.",
				"security_note":       "The CLI never handles raw card numbers. Use the browser page to manage saved cards.",
			}

			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
				return err
			}

			if openBrowserFlag {
				fmt.Fprintf(cmd.ErrOrStderr(), "Opening card management: %s\n", cardPageURL)
				return openBrowser(cardPageURL)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Card management: %s (use --open to launch in browser)\n", cardPageURL)
			return nil
		},
	}
	cards.Flags().BoolVar(&openBrowserFlag, "open", false, "Open the card management page in the system browser")
	cmd.AddCommand(cards)
	return cmd
}

// newStoresTopLevelCmd is a promoted top-level alias for 'features stores' with
// friendlier output — shows all 6 storefronts with IDs, cluster IDs, and flags.
func newStoresTopLevelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stores",
		Short: "List all available Shopper storefronts with IDs, cluster IDs, and payment parameters",
		Long: `Fetches the live storefront list from GET /features/stores.

Displays all available storefronts with their store IDs, cluster IDs, recurrence
type (subscription vs one-time), ultra-fast flag, and payment parameters.

Use this to discover available stores before switching --store.`,
		Example: "  shopper-pp-cli stores\n  shopper-pp-cli stores --json",
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"would":"GET /features/stores"}`)
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(cmd.Context(), "/features/stores", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}

// openStorefrontPage opens a storefront page path in the system browser.
// storeName is resolved from --store flag; path is the page path under the subdomain.
func openStorefrontPage(cmd *cobra.Command, flags *rootFlags, path string) error {
	storeName := flags.store
	if storeName == "" {
		storeName = "programada"
	}
	url := "https://" + resolveSubdomain(storeName) + ".shopper.com.br" + path

	if cliutil.IsVerifyEnv() {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"would_open":       url,
			"browser_required": true,
		}, flags)
	}
	if dryRunOK(flags) {
		return writeDryRun(cmd.OutOrStdout(), flags, "open-browser")
	}

	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		if err := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"action":           "opening",
			"url":              url,
			"browser_required": true,
			"note":             "Log in if prompted. This action requires a browser session.",
		}, flags); err != nil {
			return err
		}
		return openBrowser(url)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Opening: %s\n", url)
	fmt.Fprintln(cmd.OutOrStdout(), "Log in if prompted. This action requires a browser session.")
	return openBrowser(url)
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		// Add reschedule/skip/suspend/boleto to the generated delivery command.
		var delivCmd *cobra.Command
		for _, c := range root.Commands() {
			if c.Name() == "delivery" {
				delivCmd = c
				break
			}
		}
		if delivCmd != nil {
			for _, sub := range newDeliveryManageCmd(flags) {
				addNovelCommandIfAbsent(delivCmd, sub)
			}
		}

		// Add subscription command.
		addNovelCommandIfAbsent(root, newSubscriptionCmd(flags))

		// Add payment command.
		addNovelCommandIfAbsent(root, newPaymentCmd(flags))

		// Add stores top-level command.
		addNovelCommandIfAbsent(root, newStoresTopLevelCmd(flags))
	})
}
