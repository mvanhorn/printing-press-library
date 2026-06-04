// Copyright 2026 omarshahine. Licensed under Apache-2.0. See LICENSE.
// Hand-authored booking handoff (not generator output).
// PATCH: `book` assembles a booking (geocode + live quote for the chosen class)
// and hands payment off to the browser. It NEVER calls createBooking — that
// mutation requires a Braintree-minted payment nonce + 3-D Secure that can only
// be produced in a real browser, so the human completes payment + confirm there.

package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/blacklane/internal/cliutil"
	"github.com/spf13/cobra"
)

// openInBrowser opens a URL in the user's default browser.
func openInBrowser(url string) error {
	var bin string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		bin, args = "open", []string{url}
	case "windows":
		bin, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		bin, args = "xdg-open", []string{url}
	}
	return exec.Command(bin, args...).Start()
}

func newNovelBookCmd(flags *rootFlags) *cobra.Command {
	var at string
	var hourly int
	var class string
	var confirmOpen bool

	cmd := &cobra.Command{
		Use:   "book <pickup> [dropoff]",
		Short: "Assemble a booking and hand payment off to your browser (never books directly)",
		Long: "Quotes a ride for the chosen vehicle class, prints a booking summary, and — with --confirm —\n" +
			"opens Blacklane's site so you complete payment (Braintree + 3-D Secure) and click Book yourself.\n\n" +
			"This command NEVER places a booking or charges anything on its own. The actual booking\n" +
			"happens in your browser, under your control. By default it only prints the plan (dry-run).",
		Example: strings.Trim(`
  blacklane-pp-cli book "JFK Airport" "Times Square New York" --at 2026-06-25T15:00 --class business
  blacklane-pp-cli book "JFK Airport" "Times Square New York" --at 2026-06-25T15:00 --class business --confirm`, "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,4"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			departAt, err := normalizeDepartAt(at)
			if err != nil {
				return err
			}
			pickup, err := resolveLocation(args[0], 0, 0, false, flags.timeout)
			if err != nil {
				return err
			}
			var dropoff *geoPoint
			st, secs := "transfer", 0
			if hourly > 0 {
				s, err := hourlySeconds(hourly)
				if err != nil {
					return err
				}
				st, secs = "hourly", s
			} else {
				if len(args) < 2 {
					return fmt.Errorf("transfer booking needs a dropoff (or use --hourly <hours>)")
				}
				d, err := resolveLocation(args[1], 0, 0, false, flags.timeout)
				if err != nil {
					return err
				}
				dropoff = &d
			}
			if dryRunOK(flags) {
				return nil
			}

			// Live quote to validate serviceability and show the real price.
			r, err := doQuote(flags, st, departAt, secs, pickup, dropoff)
			if err != nil {
				return err
			}
			chosen := r.Packages[0] // cheapest by default
			if class != "" {
				found := false
				for _, p := range r.Packages {
					if strings.EqualFold(p.PackageSlug, class) || strings.EqualFold(p.Title, class) {
						chosen = p
						found = true
						break
					}
				}
				if !found {
					avail := make([]string, 0, len(r.Packages))
					for _, p := range r.Packages {
						avail = append(avail, p.PackageSlug)
					}
					return fmt.Errorf("class %q not available for this route; available: %s", class, strings.Join(avail, ", "))
				}
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "Booking plan (nothing has been booked or charged):")
			route := pickup.Address
			if dropoff != nil {
				route += "  →  " + dropoff.Address
			}
			fmt.Fprintf(out, "  Route:   %s\n", route)
			fmt.Fprintf(out, "  When:    %s%s\n", departAt, map[bool]string{true: fmt.Sprintf("  (%dh hourly)", hourly), false: ""}[hourly > 0])
			fmt.Fprintf(out, "  Class:   %s  (%s %s)\n", chosen.Title, chosen.GrossAmount, chosen.Currency)

			// Side effect: open the browser only with explicit --confirm, and never under verify.
			bookingURL := "https://www.blacklane.com/en/"
			if cliutil.IsVerifyEnv() || !confirmOpen {
				fmt.Fprintln(out, "")
				fmt.Fprintf(out, "To complete this booking, run again with --confirm to open Blacklane in your browser,\n")
				fmt.Fprintf(out, "then enter the route above, pick %s, and complete payment yourself.\n", chosen.Title)
				fmt.Fprintf(out, "would open: %s\n", bookingURL)
				return nil
			}
			fmt.Fprintln(out, "")
			fmt.Fprintf(out, "Opening Blacklane checkout in your browser — enter the route above, pick %s, and pay.\n", chosen.Title)
			if err := openInBrowser(bookingURL); err != nil {
				return fmt.Errorf("opening browser: %w (visit %s manually)", err, bookingURL)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&at, "at", "", "Pickup datetime, e.g. 2026-06-25T15:00 (required)")
	cmd.Flags().IntVar(&hourly, "hourly", 0, "Hourly booking: number of hours (min 2). Omit dropoff.")
	cmd.Flags().StringVar(&class, "class", "", "Vehicle class to book (e.g. business, first, van); default cheapest")
	cmd.Flags().BoolVar(&confirmOpen, "confirm", false, "Open the browser checkout to complete payment (default: print plan only)")
	return cmd
}
