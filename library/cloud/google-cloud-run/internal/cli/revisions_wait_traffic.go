// Copyright 2026 never-mind-3 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type waitTrafficResult struct {
	Service    string `json:"service"`
	Revision   string `json:"revision"`
	TargetPct  int    `json:"target_pct"`
	CurrentPct int    `json:"current_pct"`
	Status     string `json:"status"`
	Elapsed    string `json:"elapsed"`
}

func newNovelRevisionsWaitTrafficCmd(flags *rootFlags) *cobra.Command {
	var flagService string
	var flagRevision string
	var flagTargetPct int
	var flagTimeout string
	var flagPollInterval string

	cmd := &cobra.Command{
		Use:   "wait-traffic",
		Short: "Block until a specific revision reaches a target traffic percentage -- a CI/CD gate primitive.",
		Long:  "Polls the service's trafficStatuses array until the specified revision reaches the target traffic percentage; exits 0 on success, 1 on timeout. Distinct from operation waiting, which completes before traffic shifts occur.",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,1",
		},
		Example: strings.Trim(`
  google-cloud-run-pp-cli revisions wait-traffic --service projects/my-proj/locations/us-central1/services/my-svc --revision my-svc-00042-abc --target-pct 100
  google-cloud-run-pp-cli revisions wait-traffic --service projects/my-proj/locations/us-central1/services/my-svc --revision my-svc-00042-abc --target-pct 100 --timeout 5m --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagService == "" && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would wait for revision %s to reach %d%% traffic on service %s\n",
					flagRevision, flagTargetPct, flagService)
				return nil
			}
			if flagService == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--service is required"))
			}
			if flagRevision == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--revision is required"))
			}
			if flagTargetPct <= 0 || flagTargetPct > 100 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--target-pct must be between 1 and 100"))
			}

			timeout := 5 * time.Minute
			if flagTimeout != "" {
				d, err := time.ParseDuration(flagTimeout)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --timeout %q: %w", flagTimeout, err))
				}
				timeout = d
			}
			pollInterval := 5 * time.Second
			if flagPollInterval != "" {
				d, err := time.ParseDuration(flagPollInterval)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --poll %q: %w", flagPollInterval, err))
				}
				if d < time.Second {
					return usageErr(fmt.Errorf("--poll must be at least 1s to avoid hammering the API"))
				}
				pollInterval = d
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			start := time.Now()
			// Normalize revision: accept short names or full resource paths
			wantRevision := flagRevision

			for {
				svcData, err := c.Get(ctx, "/v2/"+flagService, nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var svc struct {
					TrafficStatuses []struct {
						Revision string `json:"revision"`
						Percent  int    `json:"percent"`
					} `json:"trafficStatuses"`
				}
				if err := json.Unmarshal(svcData, &svc); err != nil {
					return fmt.Errorf("parsing service: %w", err)
				}

				currentPct := 0
				for _, t := range svc.TrafficStatuses {
					if t.Revision == wantRevision || shortName(t.Revision) == shortName(wantRevision) {
						currentPct = t.Percent
						break
					}
				}

				elapsed := time.Since(start).Round(time.Second).String()
				result := waitTrafficResult{
					Service:    shortName(flagService),
					Revision:   shortName(wantRevision),
					TargetPct:  flagTargetPct,
					CurrentPct: currentPct,
					Elapsed:    elapsed,
				}

				if currentPct >= flagTargetPct {
					result.Status = "reached"
					if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv) {
						return printJSONFiltered(cmd.OutOrStdout(), result, flags)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "✓ revision %s reached %d%% traffic (elapsed: %s)\n",
						shortName(wantRevision), currentPct, elapsed)
					return nil
				}

				if !flags.quiet {
					fmt.Fprintf(cmd.ErrOrStderr(), "waiting: %s is at %d%% (target %d%%), elapsed %s\n",
						shortName(wantRevision), currentPct, flagTargetPct, elapsed)
				}

				select {
				case <-ctx.Done():
					result.Status = "timeout"
					if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv) {
						_ = printJSONFiltered(cmd.OutOrStdout(), result, flags)
					}
					return apiErr(fmt.Errorf("timeout: revision %s only reached %d%% traffic after %s (target %d%%)",
						shortName(wantRevision), currentPct, elapsed, flagTargetPct))
				case <-time.After(pollInterval):
				}
			}
		},
	}

	cmd.Flags().StringVar(&flagService, "service", "", "Full resource name of the service (projects/{project}/locations/{region}/services/{service})")
	cmd.Flags().StringVar(&flagRevision, "revision", "", "Revision name to wait for (short name or full resource name)")
	cmd.Flags().IntVar(&flagTargetPct, "target-pct", 100, "Target traffic percentage to wait for (1-100)")
	cmd.Flags().StringVar(&flagTimeout, "timeout", "5m", "Timeout duration (e.g. 5m, 300s)")
	cmd.Flags().StringVar(&flagPollInterval, "poll", "5s", "Polling interval (e.g. 5s, 10s)")
	return cmd
}
