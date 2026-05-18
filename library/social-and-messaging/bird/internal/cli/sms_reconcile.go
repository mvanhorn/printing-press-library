// Copyright 2026 cleverai-zh. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/bird/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/bird/internal/store"
	"github.com/spf13/cobra"
)

type reconcileResult struct {
	BatchID          string                `json:"batchId"`
	ChannelID        string                `json:"channelId"`
	Total            int                   `json:"total"`
	Reconciled       int                   `json:"reconciled"`
	Delivered        int                   `json:"delivered"`
	Failed           int                   `json:"failed"`
	Pending          int                   `json:"pending"`
	FailuresByReason []failureRow          `json:"failuresByReason,omitempty"`
	Rows             []reconcileRowOutcome `json:"rows"`
	Retries          []sendBatchRow        `json:"retries,omitempty"`
}

type reconcileRowOutcome struct {
	MessageID string `json:"messageId"`
	Recipient string `json:"recipient"`
	Final     string `json:"final"`
	Reason    string `json:"reason,omitempty"`
}

func newSmsReconcileCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath      string
		retryFailed bool
	)
	cmd := &cobra.Command{
		Use:   "reconcile <batch_id>",
		Short: "Re-fetch delivery interactions for every message in a batch and report outcomes.",
		Long: `Reads the batch from the local store (populated by 'sms send-batch'), calls
the audit timeline for each message, groups failures by reason, and optionally
retries failed rows with --retry-failed.

Returns a structured summary with per-row final state plus a top-N of failure
reasons — the sort of post-mortem you'd otherwise hand-write after every
campaign.`,
		Example: `  bird-pp-cli sms reconcile batch_20260510_071500 --json
  bird-pp-cli sms reconcile batch_20260510_071500 --retry-failed --json`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			batchID := args[0]
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("bird-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			raw, err := db.Get("sms_batch", batchID)
			if err != nil {
				return fmt.Errorf("loading batch %s: %w", batchID, err)
			}
			var batch sendBatchSummary
			if err := json.Unmarshal(raw, &batch); err != nil {
				return fmt.Errorf("parsing batch %s: %w", batchID, err)
			}
			result := reconcileResult{
				BatchID:   batch.BatchID,
				ChannelID: batch.ChannelID,
				Total:     batch.Total,
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			reasons := []failureMessageRow{}
			for _, row := range batch.Rows {
				if row.MessageID == "" {
					continue
				}
				view, _, err := runAudit(c, batch.ChannelID, row.MessageID)
				if err != nil {
					result.Pending++
					continue
				}
				result.Reconciled++
				outcome := reconcileRowOutcome{
					MessageID: row.MessageID,
					Recipient: row.Recipient,
					Final:     view.Final,
				}
				if view.Failed {
					result.Failed++
					if len(view.Events) > 0 {
						outcome.Reason = view.Events[len(view.Events)-1].Reason
					}
					reasons = append(reasons, failureMessageRow{ID: row.MessageID, Reason: outcome.Reason})
				} else if isTerminalSuccess(view.Final) {
					result.Delivered++
				} else {
					result.Pending++
				}
				result.Rows = append(result.Rows, outcome)
			}
			result.FailuresByReason = aggregateByReason(reasons)
			sort.SliceStable(result.Rows, func(i, j int) bool {
				return result.Rows[i].Final < result.Rows[j].Final
			})
			if retryFailed {
				// PATCH: gate the retry loop on the same population it iterates
				// over (rows whose original HTTP send failed), not on
				// result.Failed (which counts Bird-side delivery failures from
				// runAudit and is a disjoint set). The previous guard silently
				// no-op'd in two common scenarios:
				//   1. All POSTs succeeded but Bird rejected delivery for N rows
				//      -> result.Failed == N, guard passed, but no row.Status
				//      == "failed", so retries stayed empty.
				//   2. All POSTs failed (network error, 4xx) -> every row had
				//      MessageID == "" and was skipped earlier, so
				//      result.Failed == 0 and the guard was false even though
				//      every row needed retrying.
				// Surfaced by Greptile P1 in PR #417 review. The flag's
				// help text reads "Re-send rows whose original send failed",
				// so this matches the documented contract.
				retries := make([]sendBatchRow, 0)
				for _, row := range batch.Rows {
					if row.Status == "failed" || row.Error != "" {
						retry := row
						retry.Status = "retried"
						if err := sendOneRow(flags, batch.ChannelID, &retry); err != nil {
							retry.Status = "retry-failed"
							retry.Error = err.Error()
						} else {
							// PATCH: clear the original error string on success.
							// `retry := row` copies the original failed row's Error
							// field; sendOneRow only writes MessageID + Status on
							// success, leaving Error intact. Without this clear,
							// persistBatch writes back a "retried but Error != ""
							// state, and the next --retry-failed pass re-attempts
							// an already-delivered row. Surfaced by Greptile P1 in
							// the PR #417 eighth review pass.
							retry.Error = ""
						}
						retries = append(retries, retry)
					}
				}
				result.Retries = retries
				// PATCH: persist the retried rows back into the stored batch so
				// a subsequent `sms reconcile <batch-id>` call can audit the
				// new MessageIDs. Without this write-back the retries live only
				// in memory; the persisted batch keeps the original
				// MessageID == "" entries and reconcile-loop line 90 skips
				// them on the next pass, making the retried deliveries
				// untrackable. Greptile P1 (third-pass review on PR #417).
				if len(retries) > 0 {
					retryByKey := make(map[string]sendBatchRow, len(retries))
					for _, r := range retries {
						retryByKey[r.IdempotencyKey] = r
					}
					for i, row := range batch.Rows {
						if r, ok := retryByKey[row.IdempotencyKey]; ok {
							batch.Rows[i] = r
						}
					}
					if err := persistBatch(db, batch); err != nil {
						fmt.Fprintf(os.Stderr, "warning: failed to persist retries for batch %s: %v\n", batch.BatchID, err)
					}
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&retryFailed, "retry-failed", false, "Re-send rows whose original send failed")
	return cmd
}
