// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: sold-out watch loop. Hand-authored.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/internal/parking"

	"github.com/spf13/cobra"
)

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var entry, exit, interval, promo string
	var notify bool
	var untilPrice float64
	var maxPolls int

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Poll a sold-out or target date range until Reserved opens up (or drops below a price), then notify.",
		Long: "Poll one entry/exit range on an interval until Reserved becomes available (or\n" +
			"its price drops below --until-price), then stop and notify. Every poll is\n" +
			"persisted. This is one foreground loop, not a background daemon — leave it\n" +
			"running in a terminal or under a supervisor.\n\n" +
			"For a one-shot check use 'quote'; to search many ranges use 'sweep'.",
		Example: "  sea-airport-parking-pp-cli watch --entry 2026-11-25T11:00 --exit 2026-11-30T11:00 --interval 30m --notify",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would poll the range until Reserved becomes available")
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
			pollEvery, err := cliutil.ParseDurationLoose(interval)
			if err != nil || pollEvery <= 0 {
				return usageErr(fmt.Errorf("invalid --interval %q (use e.g. 30m, 1h)", interval))
			}

			pc, err := newParkingClient(flags)
			if err != nil {
				return err
			}

			dogfood := cliutil.IsDogfoodEnv()
			jsonMode := flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout())
			w := cmd.OutOrStdout()
			for poll := 1; ; poll++ {
				q, qerr := pc.Quote(ctx, entryT, exitT, promo)
				if qerr != nil {
					return fmt.Errorf("polling: %w", qerr)
				}
				if perr := persistQuote(ctx, defaultDBPath(dbName), q); perr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not persist poll: %v\n", perr)
				}

				met := watchConditionMet(q, untilPrice)
				if met {
					if jsonMode {
						if err := flags.printJSON(cmd, q); err != nil {
							return err
						}
					} else {
						renderQuoteHuman(cmd, q)
						if notify {
							fmt.Fprintf(w, "\aNOTIFY: SEA Reserved is available for %s → %s at $%.2f\n", q.EntryStr, q.ExitStr, q.TotalPrice)
						}
					}
					return nil
				}

				stopping := dogfood || (maxPolls > 0 && poll >= maxPolls)
				if jsonMode {
					// In machine mode only emit JSON, and only once: the latest
					// poll's quote when we're about to stop. Intermediate polls
					// stay silent so stdout is always valid JSON.
					if stopping {
						return flags.printJSON(cmd, q)
					}
				} else {
					fmt.Fprintf(w, "[poll %d] %s — %s\n", poll, time.Now().Format("15:04:05"), watchStatus(q))
					if stopping && maxPolls > 0 && !dogfood {
						fmt.Fprintf(w, "reached --max-polls (%d) without meeting the condition\n", maxPolls)
					}
				}
				if stopping {
					return nil
				}
				if err := sleepCtx(ctx, pollEvery); err != nil {
					return err
				}
			}
		},
	}
	cmd.Flags().StringVar(&entry, "entry", "", "Entry date/time, e.g. 2026-11-25T11:00")
	cmd.Flags().StringVar(&exit, "exit", "", "Exit date/time, e.g. 2026-11-30T11:00")
	cmd.Flags().StringVar(&interval, "interval", "15m", "Poll interval (e.g. 30m, 1h)")
	cmd.Flags().BoolVar(&notify, "notify", false, "Emit a terminal bell + NOTIFY line when the condition is met")
	cmd.Flags().Float64Var(&untilPrice, "until-price", 0, "Stop when available at or below this total price (0 = any availability)")
	cmd.Flags().IntVar(&maxPolls, "max-polls", 0, "Stop after this many polls (0 = unlimited)")
	cmd.Flags().StringVar(&promo, "promo", "", "Optional promo code to apply")
	return cmd
}

func watchConditionMet(q *parking.Quote, untilPrice float64) bool {
	if !q.Available {
		return false
	}
	if untilPrice > 0 {
		return q.TotalPrice > 0 && q.TotalPrice <= untilPrice
	}
	return true
}

func watchStatus(q *parking.Quote) string {
	switch {
	case q.Invalid:
		return "invalid: " + q.Reason
	case q.SoldOut:
		return "sold out"
	case q.Available:
		return fmt.Sprintf("available at $%.2f (above target)", q.TotalPrice)
	default:
		return "unavailable"
	}
}

// sleepCtx sleeps for d unless ctx is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
