// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
// PATCH(library): hand-written `watch` command group for monitoring
// purchased-flight prices. Mirrors the primary.go extension pattern —
// register a top-level command via registerWatchCommands(rootCmd, flags)
// rather than reaching into the generated per-endpoint files.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/flight-goat/internal/watch"
	"github.com/spf13/cobra"
)

// registerWatchCommands wires the `watch` command group: add, list, show,
// check, remove, alert-test. The group persists state in its own SQLite
// file (see internal/watch/store.go) so users can register purchased
// flights and re-check their prices later without re-supplying the
// fields.
//
// Called from root.go after registerPrimaryCommands so `watch` appears in
// --help under the primary headlines but above the generated commands.
func registerWatchCommands(rootCmd *cobra.Command, flags *rootFlags) {
	watchCmd := &cobra.Command{
		Use:   "watch",
		Short: "Monitor purchased-flight prices and alert when they drop",
		Long: `watch tracks flights you have already booked and re-checks their live
prices via the Google Flights backend. When the same flight's fare drops
by more than your threshold you receive an alert (stdout or webhook) with
a safety notice covering refundability, change fees, and credit handling.

Alerts trigger only on EXACT matches (same airline + flight number + date
+ route + cabin). The route-cheapest fare is surfaced for context but is
never used to fire an alert — cheapest-on-route on a different flight
won't help you if your current ticket can't be moved without losing it.`,
	}
	watchCmd.AddCommand(newWatchAddCmd(flags))
	watchCmd.AddCommand(newWatchListCmd(flags))
	watchCmd.AddCommand(newWatchShowCmd(flags))
	watchCmd.AddCommand(newWatchCheckCmd(flags))
	watchCmd.AddCommand(newWatchRemoveCmd(flags))
	watchCmd.AddCommand(newWatchAlertTestCmd(flags))
	rootCmd.AddCommand(watchCmd)
}

// openWatchStore is the single helper for resolving the watch DB path
// from --watch-db or the FLIGHT_GOAT_WATCH_DB env var (the env var is the
// default via watch.DefaultDBPath).
func openWatchStore(ctx context.Context, dbPath string) (*watch.Store, error) {
	return watch.Open(ctx, dbPath)
}

// ----- watch add ------------------------------------------------------

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var (
		from, to, date  string
		departureTime   string
		airline, fno    string
		cabin           string
		fareBrand       string
		includeBasic    bool
		passengers      int
		paid, threshold float64
		currency        string
		notify          string
		bookingRef      string
		notes           string
		watchDB         string
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Register a purchased flight to monitor for price drops",
		Long: `Add registers a flight you already hold. All fields except --cabin,
--passengers, --notify, --booking-ref, and --notes are required.

The watch is exact-match: alerts only fire when the same airline +
flight number + date + route + cabin combination appears at a lower
price. Cheaper itineraries on the same route via a different flight
number are surfaced for context but never alert.`,
		Example: `  # SFO -> JFK on Delta 669, paid $428.20, alert at $50 off
  flight-goat-pp-cli watch add \
    --from SFO --to JFK --date 2026-06-21 \
    --airline DL --flight-number 669 \
    --paid 428.20 --threshold 50 --currency USD

  # With a webhook alert sink
  flight-goat-pp-cli watch add \
    --from SFO --to JFK --date 2026-06-21 \
    --airline DL --flight-number 669 --cabin economy \
    --paid 428.20 --threshold 50 \
    --notify webhook:https://hooks.example.com/flight-drops`,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := &watch.Watch{
				Origin:        from,
				Destination:   to,
				DepartureDate: date,
				DepartureTime: departureTime,
				Airline:       airline,
				FlightNumber:  fno,
				Cabin:         cabin,
				FareBrand:     fareBrand,
				ExcludeBasic:  !includeBasic,
				Passengers:    passengers,
				OriginalPrice: paid,
				Threshold:     threshold,
				Currency:      currency,
				Notify:        notify,
				BookingRef:    bookingRef,
				Notes:         notes,
			}
			if err := w.Validate(); err != nil {
				return usageErr(err)
			}
			if flags.dryRun {
				return printWatchResult(cmd, flags, w, "watch (dry-run, not persisted)")
			}
			ctx := cmd.Context()
			s, err := openWatchStore(ctx, watchDB)
			if err != nil {
				return err
			}
			defer s.Close()
			if _, err := s.Insert(ctx, w); err != nil {
				return err
			}
			return printWatchResult(cmd, flags, w, "watch added")
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Origin airport (3-letter IATA, e.g. SFO) [required]")
	cmd.Flags().StringVar(&to, "to", "", "Destination airport (3-letter IATA, e.g. JFK) [required]")
	cmd.Flags().StringVar(&date, "date", "", "Departure date YYYY-MM-DD [required]")
	cmd.Flags().StringVar(&departureTime, "departure-time", "", "Optional HH:MM (24h) departure time; matcher rejects candidates that drift more than ±30 min")
	cmd.Flags().StringVar(&airline, "airline", "", "Carrier code (e.g. DL, AA) [required]")
	cmd.Flags().StringVar(&fno, "flight-number", "", "Flight number, digits with optional trailing letter (e.g. 669) [required]")
	cmd.Flags().StringVar(&cabin, "cabin", "", "Cabin: economy, premium_economy, business, first")
	cmd.Flags().StringVar(&fareBrand, "fare-brand", "", "Fare brand for your reference (e.g. 'Main Cabin', 'Comfort+', 'Polaris'); shown in alerts so you can eyeball fare comparability")
	cmd.Flags().BoolVar(&includeBasic, "include-basic", false, "Include basic-economy results in the price comparison (default excludes basic, since basic-vs-main is a misleading match)")
	cmd.Flags().IntVar(&passengers, "passengers", 1, "Number of passengers")
	cmd.Flags().Float64Var(&paid, "paid", 0, "Price paid for the ticket [required]")
	cmd.Flags().Float64Var(&threshold, "threshold", 0, "Alert when paid - found >= threshold (in --currency) [required]")
	cmd.Flags().StringVar(&currency, "currency", "USD", "ISO 4217 currency code")
	cmd.Flags().StringVar(&notify, "notify", "", "Alert sink: stdout, json, or webhook:<https://url>")
	cmd.Flags().StringVar(&bookingRef, "booking-ref", "", "Optional booking reference (PNR / confirmation number) for your records")
	cmd.Flags().StringVar(&notes, "notes", "", "Free-form notes attached to the watch")
	cmd.Flags().StringVar(&watchDB, "watch-db", "", "Override watch SQLite DB path (default $FLIGHT_GOAT_WATCH_DB or ~/.local/share/flight-goat-pp-cli/watches.db)")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
	_ = cmd.MarkFlagRequired("date")
	_ = cmd.MarkFlagRequired("airline")
	_ = cmd.MarkFlagRequired("flight-number")
	_ = cmd.MarkFlagRequired("paid")
	_ = cmd.MarkFlagRequired("threshold")
	return cmd
}

// ----- watch list -----------------------------------------------------

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	var statusFilter, watchDB string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered watches",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := openWatchStore(ctx, watchDB)
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := s.List(ctx, statusFilter)
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return writeJSON(cmd, map[string]any{
					"count":   len(rows),
					"watches": rows,
				})
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tROUTE\tDATE\tFLIGHT\tPAID\tTHRESHOLD\tLAST SEEN\tSTATUS")
			for _, w := range rows {
				lastSeen := "-"
				if w.LastSeenPrice != nil {
					lastSeen = fmt.Sprintf("%s %.2f", w.Currency, *w.LastSeenPrice)
				}
				fmt.Fprintf(tw, "%s\t%s-%s\t%s\t%s%s\t%s %.2f\t%s %.2f\t%s\t%s\n",
					w.ID, w.Origin, w.Destination, w.DepartureDate,
					w.Airline, w.FlightNumber, w.Currency, w.OriginalPrice,
					w.Currency, w.Threshold, lastSeen, w.Status)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status: active, paused, archived")
	cmd.Flags().StringVar(&watchDB, "watch-db", "", "Override watch SQLite DB path")
	return cmd
}

// ----- watch show -----------------------------------------------------

func newWatchShowCmd(flags *rootFlags) *cobra.Command {
	var watchDB string
	cmd := &cobra.Command{
		Use:   "show <watch_id>",
		Short: "Show one watch's full state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := openWatchStore(ctx, watchDB)
			if err != nil {
				return err
			}
			defer s.Close()
			w, err := s.Get(ctx, args[0])
			if err != nil {
				return err
			}
			return printWatchResult(cmd, flags, w, "")
		},
	}
	cmd.Flags().StringVar(&watchDB, "watch-db", "", "Override watch SQLite DB path")
	return cmd
}

// ----- watch check ----------------------------------------------------

func newWatchCheckCmd(flags *rootFlags) *cobra.Command {
	var (
		watchDB     string
		forceAlert  bool
		repeatDelta float64
	)
	cmd := &cobra.Command{
		Use:   "check [watch_id]",
		Short: "Re-check live prices and dispatch alerts",
		Long: `check looks up the current price of one watched flight (or every active
watch if no ID is given), compares it to the paid price, and dispatches
an alert through the watch's notify sink when the threshold is crossed
on an EXACT-match itinerary.

The JSON output schema is stable:

  {
    "schema": "flight-goat.watch.check.v1",
    "watch_id": "watch_…",
    "found_price": 354.10,
    "route_cheapest_price": 312.00,
    "confidence": "high" | "medium" | "low",
    "threshold_crossed": true,
    "alert_dispatched": true,
    "alert_suppressed": false,
    "matched_flight": {…},
    "safety_notice": "Same flight appears cheaper. Verify fare rules…"
  }

Webhook alerts POST the same envelope as the body.`,
		Args: cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := openWatchStore(ctx, watchDB)
			if err != nil {
				return err
			}
			defer s.Close()
			var targets []*watch.Watch
			if len(args) == 1 {
				w, err := s.Get(ctx, args[0])
				if err != nil {
					return err
				}
				targets = []*watch.Watch{w}
			} else {
				rows, err := s.List(ctx, watch.StatusActive)
				if err != nil {
					return err
				}
				targets = rows
			}
			// Resolve the dispatcher writer once. In JSON mode the
			// envelope itself carries the full alert payload, so we
			// route the stdout dispatcher to io.Discard to avoid
			// mixing two formats on the same stream — anyone parsing
			// the output (the flight-watch-check cron, agents) gets
			// clean JSON.
			dispWriter := cmd.OutOrStdout()
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				dispWriter = io.Discard
			}
			results := make([]watch.CheckResult, 0, len(targets))
			for _, w := range targets {
				disp := watch.DispatcherFor(w.Notify)
				if setter, ok := disp.(watch.StdoutWriterSetter); ok {
					setter.SetStdoutWriter(dispWriter)
				}
				res, err := watch.Check(ctx, s, w, watch.CheckOptions{
					ForceAlert:  forceAlert,
					RepeatDelta: repeatDelta,
					Dispatcher:  disp,
				})
				if err != nil {
					return err
				}
				results = append(results, *res)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if len(results) == 1 {
					return writeJSON(cmd, results[0])
				}
				return writeJSON(cmd, map[string]any{
					"count":   len(results),
					"results": results,
				})
			}
			for i, r := range results {
				if i > 0 {
					fmt.Fprintln(cmd.OutOrStdout())
				}
				fmt.Fprintln(cmd.OutOrStdout(), watch.FormatAlertText(r))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&watchDB, "watch-db", "", "Override watch SQLite DB path")
	cmd.Flags().BoolVar(&forceAlert, "force-alert", false, "Dispatch the alert even if the threshold isn't crossed or the price already alerted")
	cmd.Flags().Float64Var(&repeatDelta, "repeat-delta", 0, "Re-alert only if the price has dropped by at least this much since the last alert (default 0 = dedup any equal-or-higher price)")
	return cmd
}

// ----- watch remove ---------------------------------------------------

func newWatchRemoveCmd(flags *rootFlags) *cobra.Command {
	var watchDB string
	cmd := &cobra.Command{
		Use:   "remove <watch_id>",
		Short: "Delete a watch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := openWatchStore(ctx, watchDB)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.Delete(ctx, args[0]); err != nil {
				return err
			}
			if flags.asJSON {
				return writeJSON(cmd, map[string]string{"removed": args[0]})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&watchDB, "watch-db", "", "Override watch SQLite DB path")
	return cmd
}

// ----- watch alert-test -----------------------------------------------

func newWatchAlertTestCmd(flags *rootFlags) *cobra.Command {
	var watchDB string
	cmd := &cobra.Command{
		Use:   "alert-test <watch_id>",
		Short: "Send a synthetic alert through the watch's notify sink",
		Long: `alert-test posts a sample CheckResult through the watch's configured
notify sink (stdout / json / webhook). The check does NOT hit Google
Flights and does NOT update last_alerted_price — it's purely for
verifying that the alert path works end-to-end before relying on it.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := openWatchStore(ctx, watchDB)
			if err != nil {
				return err
			}
			defer s.Close()
			w, err := s.Get(ctx, args[0])
			if err != nil {
				return err
			}
			res := watch.SampleResult(w, time.Now())
			disp := watch.DispatcherFor(w.Notify)
			if setter, ok := disp.(watch.StdoutWriterSetter); ok {
				setter.SetStdoutWriter(cmd.OutOrStdout())
			}
			if err := disp.Dispatch(ctx, res); err != nil {
				return err
			}
			res.AlertDispatched = true
			res.AlertDispatchedTo = disp.Name()
			if flags.asJSON {
				return writeJSON(cmd, res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "sent test alert for %s via %s\n", w.ID, disp.Name())
			return nil
		},
	}
	cmd.Flags().StringVar(&watchDB, "watch-db", "", "Override watch SQLite DB path")
	return cmd
}

// ----- shared helpers -------------------------------------------------

// printWatchResult renders a Watch row either as JSON or as a labeled
// stdout block, matching the rest of the CLI's --json toggle behavior.
func printWatchResult(cmd *cobra.Command, flags *rootFlags, w *watch.Watch, banner string) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return writeJSON(cmd, w)
	}
	if banner != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", banner, w.ID)
	}
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "ID:\t%s\n", w.ID)
	fmt.Fprintf(tw, "Route:\t%s -> %s\n", w.Origin, w.Destination)
	fmt.Fprintf(tw, "Date:\t%s\n", w.DepartureDate)
	if w.DepartureTime != "" {
		fmt.Fprintf(tw, "Departs:\t%s\n", w.DepartureTime)
	}
	fmt.Fprintf(tw, "Flight:\t%s%s\n", w.Airline, w.FlightNumber)
	if w.Cabin != "" {
		fmt.Fprintf(tw, "Cabin:\t%s\n", w.Cabin)
	}
	if w.FareBrand != "" {
		fmt.Fprintf(tw, "Fare brand:\t%s\n", w.FareBrand)
	}
	excludeStr := "no (basic-economy included)"
	if w.ExcludeBasic {
		excludeStr = "yes (basic-economy filtered out)"
	}
	fmt.Fprintf(tw, "Exclude basic:\t%s\n", excludeStr)
	fmt.Fprintf(tw, "Passengers:\t%d\n", w.Passengers)
	fmt.Fprintf(tw, "Paid:\t%s %.2f\n", w.Currency, w.OriginalPrice)
	fmt.Fprintf(tw, "Threshold:\t%s %.2f\n", w.Currency, w.Threshold)
	if w.Notify != "" {
		fmt.Fprintf(tw, "Notify:\t%s\n", w.Notify)
	}
	fmt.Fprintf(tw, "Status:\t%s\n", w.Status)
	if w.LastSeenPrice != nil {
		fmt.Fprintf(tw, "Last seen:\t%s %.2f\n", w.Currency, *w.LastSeenPrice)
	}
	if w.LastAlertedPrice != nil {
		fmt.Fprintf(tw, "Last alerted:\t%s %.2f\n", w.Currency, *w.LastAlertedPrice)
	}
	return tw.Flush()
}

func writeJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
