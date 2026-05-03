// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"

	"github.com/spf13/cobra"
)

func newTrackWatchCmd(flags *rootFlags) *cobra.Command {
	var (
		nums       []string
		interval   time.Duration
		maxIters   int
		webhook    string
		outputFile string
	)
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Long-poll a set of tracking numbers and emit new scan events as they appear",
		Example: strings.Trim(`
  # Long-running daemon: poll every 5m, emit new events as they appear
  fedex-pp-cli track watch --tracking 794633071234 --tracking 794633071235 --interval 5m

  # Forward to a webhook
  fedex-pp-cli track watch --tracking 794633071234 --webhook https://hooks.example.com/track

  # One-shot diff (ideal for cron / CI): poll once then exit
  fedex-pp-cli track watch --tracking 794633071234 --max-iterations 1
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(nums) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would watch:", strings.Join(nums, ","))
				return nil
			}
			if interval <= 0 {
				interval = 10 * time.Minute
			}
			if maxIters == 0 && cliutil.IsVerifyEnv() {
				maxIters = 1
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			st, _ := store.Open("")
			if st != nil {
				defer st.Close()
			}

			var outFh *os.File
			if outputFile != "" {
				outFh, err = os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
				if err != nil {
					return err
				}
				defer outFh.Close()
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			iter := 0
			for {
				iter++
				for _, n := range nums {
					res := pollTrackingDiff(ctx, c, st, n)
					for _, ev := range res.NewEvents {
						line, _ := json.Marshal(map[string]any{
							"tracking_number": n,
							"event":           ev,
							"observed_at":     time.Now().UTC().Format(time.RFC3339),
						})
						fmt.Fprintln(cmd.OutOrStdout(), string(line))
						if outFh != nil {
							fmt.Fprintln(outFh, string(line))
						}
						if webhook != "" {
							_ = postWebhook(webhook, line)
						}
					}
				}
				if maxIters > 0 && iter >= maxIters {
					return nil
				}
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(interval):
				}
			}
		},
	}
	cmd.Flags().StringSliceVar(&nums, "tracking", nil, "Tracking numbers to watch (repeatable)")
	cmd.Flags().DurationVar(&interval, "interval", 10*time.Minute, "Poll interval")
	cmd.Flags().IntVar(&maxIters, "max-iterations", 0, "Stop after N polls (0 = unlimited)")
	cmd.Flags().StringVar(&webhook, "webhook", "", "POST new events as JSON to this URL")
	cmd.Flags().StringVar(&outputFile, "output", "", "Append new events as JSONL to this file")
	return cmd
}

func postWebhook(url string, body []byte) error {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
