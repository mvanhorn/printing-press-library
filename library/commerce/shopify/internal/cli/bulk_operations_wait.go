// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/shopify/internal/cliutil"
	"github.com/spf13/cobra"
)

// minBulkOperationsPollInterval floors --poll-interval so a mistaken (or
// hostile) --poll-interval 0s/negative value cannot turn this command into a
// tight busy-loop against the GraphQL endpoint. The throttle still gates
// individual requests, but there is no reason to attempt more than 1req/s
// for a job-status poll.
const minBulkOperationsPollInterval = 1 * time.Second

// pp:data-source live

// Reuses bulkOperationCurrentQuery (declared in bulk_operations.go, the
// sibling "bulk-operations current" command) so both commands stay in sync
// against the same GraphQL shape instead of drifting apart.

type bulkOperationView struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	ErrorCode      string `json:"errorCode"`
	CreatedAt      string `json:"createdAt"`
	CompletedAt    string `json:"completedAt"`
	ObjectCount    string `json:"objectCount"`
	URL            string `json:"url"`
	PartialDataURL string `json:"partialDataUrl"`
}

var bulkOperationTerminalStates = map[string]bool{
	"COMPLETED": true,
	"FAILED":    true,
	"CANCELED":  true,
	"EXPIRED":   true,
}

func newNovelBulkOperationsWaitCmd(flags *rootFlags) *cobra.Command {
	var flagMaxWait string
	var flagPollInterval string

	cmd := &cobra.Command{
		Use:         "wait",
		Short:       "Block until the current Shopify bulk export finishes instead of hand-polling in a loop.",
		Long:        "Use this command to block until the current bulk operation finishes and return its terminal result. Do NOT use this command for a non-blocking snapshot of job state; use 'bulk-operations current' instead.",
		Example:     "  shopify-pp-cli bulk-operations wait --max-wait 10m --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would poll currentBulkOperation until a terminal state")
				return nil
			}

			maxWait, err := cliutil.ParseDurationLoose(flagMaxWait)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --max-wait %q: %w", flagMaxWait, err))
			}
			pollInterval, err := cliutil.ParseDurationLoose(flagPollInterval)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --poll-interval %q: %w", flagPollInterval, err))
			}
			if cliutil.IsDogfoodEnv() && maxWait > 20*time.Second {
				maxWait = 20 * time.Second
			}
			if pollInterval < minBulkOperationsPollInterval {
				pollInterval = minBulkOperationsPollInterval
			}

			// The loop's own deadline is --max-wait, not the generic
			// --timeout request budget (flags.timeout, default 60s) that
			// boundCtx applies elsewhere: this command's whole purpose is to
			// block for up to --max-wait, so binding the loop context to
			// flags.timeout would silently truncate a 10m wait to 60s. Each
			// individual query call is still bounded by flags.timeout (via
			// boundCtx below) so one hung request can't outlive it.
			ctx, cancel := context.WithTimeout(cmd.Context(), maxWait)
			defer cancel()
			deadline := time.Now().Add(maxWait)

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var view bulkOperationView
			for {
				queryCtx, queryCancel := boundCtx(ctx, flags)
				data, err := c.Query(queryCtx, bulkOperationCurrentQuery, nil)
				queryCancel()
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var envelope struct {
					CurrentBulkOperation *bulkOperationView `json:"currentBulkOperation"`
				}
				if err := json.Unmarshal(data, &envelope); err != nil {
					return fmt.Errorf("parsing currentBulkOperation response: %w", err)
				}
				if envelope.CurrentBulkOperation == nil {
					if flags.asJSON || wantsMachineOutput(flags) {
						return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
							"status": "NONE",
							"note":   "no bulk operation has been run yet",
						}, flags)
					}
					fmt.Fprintln(cmd.OutOrStdout(), "no bulk operation has been run yet")
					return nil
				}
				view = *envelope.CurrentBulkOperation
				if bulkOperationTerminalStates[view.Status] {
					break
				}
				if time.Now().After(deadline) {
					return rateLimitErr(fmt.Errorf("bulk operation %s still %s after waiting %s; raise --max-wait to keep waiting", view.ID, view.Status, maxWait))
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(pollInterval):
				}
			}

			if flags.asJSON || wantsMachineOutput(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\tobjects=%s\turl=%s\n", view.ID, view.Status, view.ObjectCount, view.URL)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagMaxWait, "max-wait", "10m", "Maximum time to wait for the bulk operation to reach a terminal state (e.g. 10m, 30s)")
	cmd.Flags().StringVar(&flagPollInterval, "poll-interval", "5s", "How often to re-check the bulk operation status while waiting")
	return cmd
}
