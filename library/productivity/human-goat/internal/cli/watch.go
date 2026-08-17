// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/source/taskrabbit"

	"github.com/spf13/cobra"
)

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var flagOn string
	var flagMinRating float64
	var flagMaxRate float64
	var flagLat float64
	var flagLng float64
	var flagState string
	var flagLimit int
	var flagInterval time.Duration
	var flagMaxWait time.Duration

	cmd := &cobra.Command{
		Use:         "watch <job-query>",
		Short:       "Polls recommendations for a category and date (optionally a favorite or a rate ceiling) and alerts when a match opens.",
		Example:     "human-goat-pp-cli watch movers --on saturday --max-rate 60 --lat 37.7749 --lng -122.4194",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !commandHasChangedFlags(cmd) {
				return cmd.Help()
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would poll for a qualifying opening")
				return nil
			}
			if query == "" {
				return usageErr(fmt.Errorf("missing job-query"))
			}
			if !cmd.Flags().Changed("lat") || !cmd.Flags().Changed("lng") {
				return usageErr(fmt.Errorf("pass --lat and --lng for your location"))
			}
			if flagLimit < 0 {
				return usageErr(fmt.Errorf("--limit must be non-negative"))
			}
			if flagInterval <= 0 {
				return usageErr(fmt.Errorf("--interval must be positive"))
			}
			if flagMaxWait < 0 {
				return usageErr(fmt.Errorf("--max-wait must be non-negative"))
			}

			date, err := parseOnDate(flagOn)
			if err != nil {
				return usageErr(err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Bind the poll context to the --max-wait deadline so a hung API call
			// inside the loop is cancelled at the deadline, not just checked
			// between polls.
			deadline := time.Now().Add(flagMaxWait)
			pollCtx := cmd.Context()
			if flagMaxWait > 0 {
				var cancel context.CancelFunc
				pollCtx, cancel = context.WithDeadline(cmd.Context(), deadline)
				defer cancel()
			}

			category, err := resolveTaskRabbitCategory(pollCtx, c, query)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			tr := taskrabbit.New(c)
			opts := taskrabbitRankOptions{
				Date:      date,
				MinRating: flagMinRating,
				MaxRate:   flagMaxRate,
				Lat:       flagLat,
				Lng:       flagLng,
				State:     flagState,
				Limit:     flagLimit,
			}

		pollLoop:
			for {
				rows, err := rankedTaskRabbitRecommendations(pollCtx, tr, category, opts)
				if err != nil {
					// Our --max-wait deadline expiring mid-call is a clean timeout,
					// not a failure: fall through to the friendly message. A cancel
					// on the parent context (user Ctrl-C) is still surfaced.
					if pollDeadlineReached(cmd.Context(), err) {
						break pollLoop
					}
					return classifyAPIError(err, flags)
				}
				if len(rows) > 0 {
					return printTaskerRankRow(cmd, flags, rows[0])
				}
				if cliutil.IsDogfoodEnv() || !time.Now().Before(deadline) {
					break
				}

				wait := flagInterval
				if remaining := time.Until(deadline); remaining < wait {
					wait = remaining
				}
				if wait <= 0 {
					break
				}
				timer := time.NewTimer(wait)
				select {
				case <-pollCtx.Done():
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					if cmd.Context().Err() != nil {
						return cmd.Context().Err()
					}
					// Our --max-wait deadline elapsed during the wait.
					break pollLoop
				case <-timer.C:
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "no qualifying opening within %s\n", flagMaxWait)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagOn, "on", "", "Date to search: YYYY-MM-DD, today, tomorrow, or weekday")
	cmd.Flags().Float64Var(&flagMinRating, "min-rating", 0, "Minimum Tasker rating")
	cmd.Flags().Float64Var(&flagMaxRate, "max-rate", 0, "Maximum all-in hourly rate in dollars (0 for no ceiling)")
	cmd.Flags().Float64Var(&flagLat, "lat", 0, "Latitude for TaskRabbit recommendations")
	cmd.Flags().Float64Var(&flagLng, "lng", 0, "Longitude for TaskRabbit recommendations")
	cmd.Flags().StringVar(&flagState, "state", "", "State for CA/MA service-fee-only pricing rule")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum number of Taskers to consider")
	cmd.Flags().DurationVar(&flagInterval, "interval", 60*time.Second, "Polling interval")
	cmd.Flags().DurationVar(&flagMaxWait, "max-wait", 10*time.Minute, "Maximum time to wait for a qualifying opening")
	return cmd
}

// pollDeadlineReached reports whether err is the watch loop's own --max-wait
// deadline expiring (a clean timeout) rather than a real API error or a user
// cancellation on the parent context.
func pollDeadlineReached(parent context.Context, err error) bool {
	return errors.Is(err, context.DeadlineExceeded) && parent.Err() == nil
}
