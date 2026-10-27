// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/workflow"
	"github.com/spf13/cobra"
)

func newOperationsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "operations",
		Short: "Reconcile label and pickup ledger rows after an unknown FedEx outcome",
	}
	cmd.AddCommand(newOperationsReconcileCmd(flags))
	return cmd
}

func newOperationsReconcileCmd(flags *rootFlags) *cobra.Command {
	var action, resolution, reason, operationID, trackingNumber, pickupConfirmation string
	cmd := &cobra.Command{
		Use:     "reconcile",
		Short:   "Record whether an ambiguous label or pickup write ran",
		Example: "  fedex-pp-cli operations reconcile --action cancel_shipment --tracking-number 794953535000 --resolution not_executed --reason \"FedEx tracking still shows the label\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = flags
			reason = strings.TrimSpace(reason)
			if len(reason) < 10 {
				return usageErr(fmt.Errorf("--reason must contain at least 10 characters"))
			}
			switch resolution {
			case "not_executed", "succeeded":
			default:
				return usageErr(fmt.Errorf("--resolution must be not_executed or succeeded"))
			}
			switch action {
			case workflow.ActionCreateLabel, workflow.ActionCancelShipment, workflow.ActionSchedulePickup, workflow.ActionCancelPickup:
			default:
				return usageErr(fmt.Errorf("--action must be create_label, cancel_shipment, schedule_pickup, or cancel_pickup"))
			}
			if err := workflow.ReconcileOperationalState(cmd.Context(), action, operationID, trackingNumber, pickupConfirmation, resolution); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "reconciled %s as %s\n", action, resolution)
			fmt.Fprintln(cmd.OutOrStdout(), "An identical FedEx write is still refused for 24h unless you pass --allow-duplicate.")
			return nil
		},
	}
	cmd.Flags().StringVar(&action, "action", "", "Workflow action: create_label, cancel_shipment, schedule_pickup, or cancel_pickup")
	cmd.Flags().StringVar(&resolution, "resolution", "", "not_executed or succeeded")
	cmd.Flags().StringVar(&reason, "reason", "", "Why this outcome was chosen (at least 10 characters; not stored)")
	cmd.Flags().StringVar(&operationID, "operation-id", "", "Pickup operation id printed when scheduling ended outcome_unknown")
	cmd.Flags().StringVar(&trackingNumber, "tracking-number", "", "Shipment tracking number for a cancellation reconcile")
	cmd.Flags().StringVar(&pickupConfirmation, "pickup-confirmation", "", "Pickup confirmation code for a cancellation reconcile")
	_ = cmd.MarkFlagRequired("action")
	_ = cmd.MarkFlagRequired("resolution")
	_ = cmd.MarkFlagRequired("reason")
	return cmd
}
