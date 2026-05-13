// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var interval time.Duration
	var autoBook bool
	var maxDuration time.Duration
	var paymentOption string

	cmd := &cobra.Command{
		Use:   "watch [class-id]",
		Short: "Poll a sold-out class and emit a structured event the moment a spot opens",
		Long: `Watch synthesizes the waitlist signal that Mariana Tek's consumer scope does not
expose. It polls GET /classes/{id} on an interval, diffs available-spot count against
the previous tick, and emits an NDJSON event when a spot opens. With --auto-book, it
calls POST /me/reservations in the same tick a spot opens and exits 0 on success.

The command prints NDJSON to stdout (one JSON object per line). Pipe to your own
notifier of choice — desktop alert, SMS gateway, calendar create.`,
		Example: `  # Watch a class, print NDJSON events to stdout
  marianatek-pp-cli watch 84212 --interval 60s

  # Auto-book the moment a spot opens
  marianatek-pp-cli watch 84212 --interval 60s --auto-book

  # Stop after 4 hours of polling
  marianatek-pp-cli watch 84212 --interval 30s --max-duration 4h`,
		Annotations: map[string]string{
			"pp:novel": "watch",
			// PATCH(greptile #487): do not mark this MCP tool read-only;
			// --auto-book can create a reservation.
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			classID := strings.TrimSpace(args[0])
			if classID == "" {
				return usageErr(fmt.Errorf("class-id is required"))
			}
			if dryRunOK(flags) {
				_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"event":    "watch_dry_run",
					"class_id": classID,
					"interval": interval.String(),
				})
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/classes/%s", classID)

			ctx := cmd.Context()
			deadline := time.Now().Add(maxDuration)
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			var prevSpots = -1
			var retryAutoBook bool
			enc := json.NewEncoder(cmd.OutOrStdout())

			emit := func(event string, payload map[string]any) {
				payload["event"] = event
				payload["ts"] = time.Now().UTC().Format(time.RFC3339)
				payload["class_id"] = classID
				_ = enc.Encode(payload)
			}

			attemptAutoBook := func() bool {
				resBody := newReservationCreateBody(classID, paymentOption, "")
				body, status, perr := c.Post("/me/reservations", resBody)
				if perr != nil || status >= 400 {
					retryAutoBook = true
					emit("auto_book_failed", map[string]any{
						"status": status,
						"error":  fmt.Sprintf("%v", perr),
						"body":   string(body),
					})
					return false
				}
				retryAutoBook = false
				emit("booked", map[string]any{
					"status": status,
					"body":   json.RawMessage(body),
				})
				return true
			}

			tick := func() (bool, error) {
				data, err := c.Get(path, nil)
				if err != nil {
					emit("watch_error", map[string]any{"error": err.Error()})
					return false, nil
				}
				spots := extractSpotsLeft(data)
				opened := prevSpots != -1 && spots > 0 && prevSpots == 0
				if prevSpots == -1 {
					emit("watch_start", map[string]any{"spots_left": spots})
				} else if opened {
					emit("spot_open", map[string]any{
						"spots_left": spots,
						"prev_spots": prevSpots,
					})
				} else if spots != prevSpots {
					emit("watch_tick", map[string]any{
						"spots_left": spots,
						"prev_spots": prevSpots,
					})
				}
				if spots == 0 {
					retryAutoBook = false
				} else if autoBook && (opened || retryAutoBook) && attemptAutoBook() {
					return true, nil
				}
				prevSpots = spots
				return false, nil
			}

			if done, err := tick(); err != nil || done {
				return err
			}
			for {
				select {
				case <-ctx.Done():
					emit("watch_canceled", map[string]any{})
					return nil
				case <-ticker.C:
					if maxDuration > 0 && time.Now().After(deadline) {
						emit("watch_timeout", map[string]any{"max_duration": maxDuration.String()})
						return nil
					}
					if done, err := tick(); err != nil {
						return err
					} else if done {
						return nil
					}
				}
			}
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 60*time.Second, "polling interval (e.g. 30s, 5m)")
	cmd.Flags().BoolVar(&autoBook, "auto-book", false, "create reservation the moment a spot opens")
	cmd.Flags().DurationVar(&maxDuration, "max-duration", 24*time.Hour, "give up after this duration (0 = forever)")
	cmd.Flags().StringVar(&paymentOption, "payment-option", "", "payment option id to use when auto-booking (default: server picks)")
	return cmd
}

// extractSpotsLeft pulls the remaining-spots count out of a Customer API
// class response. Mariana Tek exposes this as `attributes.remaining_spots`
// or `attributes.spots_remaining` depending on the brand config; we accept
// both. Returns 0 when the field is absent (treat absent as full).
func extractSpotsLeft(raw json.RawMessage) int {
	var env struct {
		Data struct {
			Attributes map[string]any `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return 0
	}
	for _, key := range []string{"remaining_spots", "spots_remaining", "available_spots"} {
		if v, ok := env.Data.Attributes[key]; ok {
			switch n := v.(type) {
			case float64:
				return int(n)
			case int:
				return n
			}
		}
	}
	return 0
}
