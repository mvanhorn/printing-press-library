// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/approval"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/workflow"
	"github.com/spf13/cobra"
)

func newOperationsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "operations", Short: "Inspect and reconcile protected FedEx operations"}
	cmd.AddCommand(newOperationsStatusCmd(flags))
	cmd.AddCommand(newOperationsReconcileCmd(flags))
	return cmd
}

func newOperationsStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status <operation-id>",
		Short: "Show one durable protected-operation record",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			state, err := newApprovalStore()
			if err != nil {
				return configErr(err)
			}
			record, err := state.Get(args[0])
			if err != nil {
				return fmt.Errorf("read operation: %w", err)
			}
			data, err := json.Marshal(record)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
}

func newOperationsReconcileCmd(flags *rootFlags) *cobra.Command {
	var resolution string
	var reason string
	cmd := &cobra.Command{
		Use:   "reconcile <target-operation-id>",
		Short: "Record an operator-verified outcome after reconciliation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetID := args[0]
			resolution = strings.ToLower(strings.TrimSpace(resolution))
			if resolution != "not_executed" && resolution != "succeeded" {
				return fmt.Errorf("--resolution must be not_executed or succeeded")
			}
			if len(strings.TrimSpace(reason)) < 10 {
				return fmt.Errorf("--reason must contain at least 10 characters of operator evidence")
			}
			reasonDigest := sha256.Sum256([]byte(strings.TrimSpace(reason)))
			reasonHash := hex.EncodeToString(reasonDigest[:])
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			origin, err := approval.NormalizeOrigin(cfg.BaseURL)
			if err != nil {
				return configErr(err)
			}
			mutation := approval.Mutation{
				Action: "reconcile_operation",
				Origin: origin,
				Method: "POST",
				Path:   "/local/operations/reconcile",
				Request: map[string]any{
					"target_operation_id": targetID,
					"resolution":          resolution,
					"reason_hash":         reasonHash,
				},
			}
			state, err := newApprovalStore()
			if err != nil {
				return configErr(err)
			}
			if !flags.yes {
				record, err := state.Create(mutation, approval.ReviewSummary{
					ReconciliationTarget:     targetID,
					ReconciliationResolution: resolution,
					ReconciliationReasonHash: reasonHash,
				})
				if err != nil {
					return fmt.Errorf("create reconciliation approval: %w", err)
				}
				data, _ := json.Marshal(map[string]any{
					"status":              approval.StatusPending,
					"action":              mutation.Action,
					"origin":              origin,
					"operation_id":        record.ID,
					"confirmation_digest": record.ConfirmationDigest,
					"expires_at":          record.ExpiresAt,
					"review":              record.Review,
					"instruction":         "Review the reconciliation evidence, then repeat this exact command with --yes, --operation-id, and --confirmation-digest.",
				})
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}
			if strings.TrimSpace(flags.operationID) == "" || strings.TrimSpace(flags.confirmationDigest) == "" {
				return errors.New("reconciliation confirmation requires --operation-id and --confirmation-digest from the preview")
			}
			_, permit, err := state.Consume(flags.operationID, flags.confirmationDigest, mutation)
			if err != nil {
				return fmt.Errorf("consume reconciliation approval: %w", err)
			}
			defer permit.Release()
			if err := state.ReconcileWithHook(targetID, resolution, reasonHash, func(targetRecord *approval.Record) error {
				return workflow.ReconcileOperationalState(cmd.Context(), targetRecord.Action, targetRecord.ID, targetRecord.Review.TrackingNumber, targetRecord.Review.PickupConfirmation, resolution)
			}); err != nil {
				_ = state.Complete(flags.operationID, approval.StatusRejected, "reconciliation_ledger_failed")
				return fmt.Errorf("reconcile operation and local ledger: %w", err)
			}
			if err := state.Complete(flags.operationID, approval.StatusSucceeded, ""); err != nil {
				return fmt.Errorf("operation reconciled but approval completion failed: %w", err)
			}
			data, _ := json.Marshal(map[string]any{"operation_id": targetID, "status": "reconciled_" + resolution, "reason_hash": reasonHash, "recorded_at": time.Now().UTC()})
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&resolution, "resolution", "", "Verified outcome: not_executed or succeeded")
	cmd.Flags().StringVar(&reason, "reason", "", "Operator evidence (stored only as a SHA-256 digest)")
	_ = cmd.MarkFlagRequired("resolution")
	_ = cmd.MarkFlagRequired("reason")
	return cmd
}

func newApprovalStore() (*approval.Store, error) {
	dir, err := approval.DefaultStoreDir()
	if err != nil {
		return nil, err
	}
	return approval.NewStore(dir, cliPendingOperationTTL), nil
}
