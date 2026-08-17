// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: checkout parent command wiring + checkout open browser-handoff.
// Checkout confirmation requires a Django session cookie — this CLI opens the
// storefront browser page and does not place orders directly.
// pp:data-source computed

package cli

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/mvanhorn/printing-press-library/library/commerce/shopper/internal/cliutil"
	"github.com/spf13/cobra"
)

func newNovelCheckoutCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "checkout",
		Short:       "checkout subcommands: preview, open",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelCheckoutPreviewCmd(flags))
	cmd.AddCommand(newCheckoutOpenCmd(flags))
	return cmd
}

func newCheckoutOpenCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "open",
		Short: "Open the Shopper checkout page in the system browser",
		Long: `Opens https://<store>.shopper.com.br/shop/checkout in the system browser.

Checkout requires a browser session (Django server-rendered form with CSRF and
session cookies). The CLI cannot place orders directly — card details, PIX, and
boleto payment all happen through the web UI.

Precondition: you must be logged in to the storefront in your browser.
Run 'shopper-pp-cli checkout preview --store <store>' first to verify basket
totals and delivery date before opening the checkout page.`,
		Example: "  shopper-pp-cli checkout open --store programada\n  shopper-pp-cli checkout open --store now",
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			storeName := flags.store
			if storeName == "" {
				storeName = "programada"
			}
			url := "https://" + resolveSubdomain(storeName) + ".shopper.com.br/shop/checkout"

			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"would_open":       url,
					"browser_required": true,
				}, flags)
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "open-checkout")
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":           "opening",
					"url":              url,
					"browser_required": true,
					"note":             "Log in if prompted. Payment is completed in the browser.",
				}, flags); err != nil {
					return err
				}
				return openBrowser(url)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Opening checkout: %s\n", url)
			fmt.Fprintln(cmd.OutOrStdout(), "Log in if prompted. Payment is completed in the browser.")
			return openBrowser(url)
		},
	}
	return cmd
}

// openBrowser opens a URL in the system default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url) // #nosec G204 -- standard OS browser-open; command name is hardcoded, URL is CLI-derived and intentional
	case "linux":
		cmd = exec.Command("xdg-open", url) // #nosec G204 -- standard OS browser-open; command name is hardcoded, URL is CLI-derived and intentional
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url) // #nosec G204 -- standard OS browser-open; command name is hardcoded, URL is CLI-derived and intentional
	default:
		return fmt.Errorf("cannot open browser on %s: open %s manually", runtime.GOOS, url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("opening browser: %w (open %s manually)", err, url)
	}
	return nil
}
