// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: booking handoff (quote + prefill, never auto-pays). Not generated.
// pp:data-source live

package cli

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"

	"github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/internal/parking"

	"github.com/spf13/cobra"
)

type bookHandoff struct {
	Available   bool     `json:"available"`
	Entry       string   `json:"entry"`
	Exit        string   `json:"exit"`
	Nights      int      `json:"nights"`
	TotalPrice  float64  `json:"total_price"`
	ProductName string   `json:"product_name,omitempty"`
	BookingURL  string   `json:"booking_url"`
	Steps       []string `json:"steps"`
	Reason      string   `json:"reason,omitempty"`
	Note        string   `json:"note"`
}

func newBookCmd(flags *rootFlags) *cobra.Command {
	var entry, exit, promo string
	var launch bool

	cmd := &cobra.Command{
		Use:   "book",
		Short: "Prepare a guest-checkout booking handoff (quotes, then stops before payment)",
		Long: "Confirm availability and price for a range, then hand off to the official\n" +
			"guest-checkout flow with the exact dates to enter. This NEVER submits payment —\n" +
			"it prepares the booking and stops so you complete the card step yourself.\n\n" +
			"Pass --launch to open the booking page in your browser.",
		Example: "  sea-airport-parking-pp-cli book --quote --entry 2026-08-15T11:00 --exit 2026-08-18T11:00",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would prepare a booking handoff (no payment is ever submitted)")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if entry == "" || exit == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--entry and --exit are required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			entryT, err := parseWhen(entry)
			if err != nil {
				return usageErr(err)
			}
			exitT, err := parseWhen(exit)
			if err != nil {
				return usageErr(err)
			}
			if err := validateRange(entryT, exitT); err != nil {
				return usageErr(err)
			}

			pc, err := newParkingClient(flags)
			if err != nil {
				return err
			}
			q, err := pc.Quote(ctx, entryT, exitT, promo)
			if err != nil {
				return fmt.Errorf("quoting before booking: %w", err)
			}
			if perr := persistQuote(ctx, defaultDBPath(dbName), q); perr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not persist quote: %v\n", perr)
			}

			cfg, _ := config.Load(flags.configPath)
			base := "https://reservesea.portseattle.org"
			if cfg != nil && cfg.BaseURL != "" {
				base = cfg.BaseURL
			}
			bookingURL := base + fmt.Sprintf("/book/%s/Parking?parkingCmd=collectParkingDetails", parking.Airport)

			h := bookHandoff{
				Available:   q.Available,
				Entry:       q.EntryStr,
				Exit:        q.ExitStr,
				Nights:      q.Nights,
				TotalPrice:  q.TotalPrice,
				ProductName: q.ProductName,
				BookingURL:  bookingURL,
				Note:        "This CLI never submits payment. Complete the card step yourself in the browser.",
			}
			if !q.Available {
				h.Reason = q.Reason
				if h.Reason == "" {
					h.Reason = "not available for these dates"
				}
			} else {
				h.Steps = []string{
					fmt.Sprintf("Open %s", bookingURL),
					fmt.Sprintf("Enter entry %s and exit %s", q.EntryStr, q.ExitStr),
					"Choose Reserved Parking, skip or add travel extras",
					"Enter your details (name, email, phone, license plate + vehicle)",
					fmt.Sprintf("Confirm the total (~$%.2f) and complete payment yourself", q.TotalPrice),
				}
			}

			if launch {
				if cliutil.IsVerifyEnv() {
					fmt.Fprintln(cmd.OutOrStdout(), "would launch:", bookingURL)
				} else if err := openBrowser(bookingURL); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not open browser: %v\n", err)
				}
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, h)
			}
			renderBookHuman(cmd, h)
			return nil
		},
	}
	cmd.Flags().StringVar(&entry, "entry", "", "Entry date/time, e.g. 2026-08-15T11:00")
	cmd.Flags().StringVar(&exit, "exit", "", "Exit date/time, e.g. 2026-08-18T11:00")
	cmd.Flags().StringVar(&promo, "promo", "", "Optional promo code to apply")
	cmd.Flags().BoolVar(&launch, "launch", false, "Open the booking page in your browser")
	// --quote is accepted as a no-op alias so the documented `book --quote` form
	// works; the command always quotes before handing off.
	var quoteAlias bool
	cmd.Flags().BoolVar(&quoteAlias, "quote", false, "Quote before handing off (default behavior)")
	return cmd
}

func renderBookHuman(cmd *cobra.Command, h bookHandoff) {
	w := cmd.OutOrStdout()
	if !h.Available {
		fmt.Fprintf(w, "Cannot book %s → %s: %s\n", h.Entry, h.Exit, h.Reason)
		fmt.Fprintf(w, "Try 'watch' to be alerted when it opens, or 'sweep' for nearby dates.\n")
		return
	}
	fmt.Fprintf(w, "%s is available for %s → %s at $%.2f (%d night(s)).\n", nameOr(h.ProductName), h.Entry, h.Exit, h.TotalPrice, h.Nights)
	fmt.Fprintln(w, "To book (you finish the payment step):")
	for i, s := range h.Steps {
		fmt.Fprintf(w, "  %d. %s\n", i+1, s)
	}
	fmt.Fprintf(w, "\n%s\n", h.Note)
}

func nameOr(name string) string {
	if name == "" {
		return "Reserved Parking"
	}
	return name
}

// openBrowser opens rawURL in the platform default browser.
//
// rawURL must be an absolute http/https URL. The scheme guard is a security
// invariant, not a convenience check: it guarantees the string can never begin
// with "-" (so it can't be reinterpreted as a flag by open/xdg-open) and can't
// carry a shell-executable scheme, which is what makes handing it to a
// subprocess safe. exec.Command does not invoke a shell, so metacharacters in
// the query string are inert.
func openBrowser(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("refusing to open malformed URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("refusing to open non-http(s) URL: %q", rawURL)
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", rawURL).Start() // #nosec G204 -- rawURL validated to be an absolute http/https URL above; no shell invoked
	case "windows":
		return exec.Command("cmd", "/c", "start", rawURL).Start() // #nosec G204 -- rawURL validated to be an absolute http/https URL above
	default:
		return exec.Command("xdg-open", rawURL).Start() // #nosec G204 -- rawURL validated to be an absolute http/https URL above; no shell invoked
	}
}
