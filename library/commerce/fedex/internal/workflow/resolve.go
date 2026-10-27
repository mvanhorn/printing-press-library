// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"
)

var ErrAlreadyCancelled = errors.New("pickup is already cancelled in the local ledger")

type PickupCancellationResolution struct {
	Body         map[string]any
	Context      any
	LegacyReason string
}

// ResolvePickupCancellation fills cancellation identifiers from a tool-created
// pickup ledger row. Unmatched legacy pickups require complete explicit
// identifiers and an approval-bound reason.
func ResolvePickupCancellation(ctx context.Context, body map[string]any, legacyReason string) (PickupCancellationResolution, error) {
	cloned, err := cloneObject(body)
	if err != nil {
		return PickupCancellationResolution{}, err
	}
	confirmation := stringField(cloned, "pickupConfirmationCode")
	if confirmation == "" {
		return PickupCancellationResolution{}, fmt.Errorf("pickupConfirmationCode is required")
	}
	ledger, err := store.Open(store.DefaultPath())
	if err != nil {
		return PickupCancellationResolution{}, fmt.Errorf("opening pickup ledger: %w", err)
	}
	defer ledger.Close()
	pickup, err := ledger.GetPickupByConfirmation(ctx, confirmation)
	if err != nil {
		return PickupCancellationResolution{}, fmt.Errorf("looking up pickup confirmation: %w", err)
	}
	if pickup == nil {
		legacyReason = strings.TrimSpace(legacyReason)
		if len(legacyReason) < 10 {
			return PickupCancellationResolution{}, fmt.Errorf("an unmatched legacy pickup requires --legacy-reason (or legacy_reason) with at least 10 characters and complete explicit identifiers")
		}
		if err := ValidateRequest(ActionCancelPickup, cloned); err != nil {
			return PickupCancellationResolution{}, err
		}
		return PickupCancellationResolution{Body: cloned, Context: map[string]any{"pickup_source": "legacy", "legacy_reason": legacyReason}, LegacyReason: legacyReason}, nil
	}
	if pickup.Status == "cancelled" {
		return PickupCancellationResolution{}, ErrAlreadyCancelled
	}
	if strings.Contains(pickup.Status, "outcome_unknown") || pickup.Status == "canceling" {
		return PickupCancellationResolution{}, fmt.Errorf("pickup %s has status %s; reconcile with FedEx before another cancellation attempt", confirmation, pickup.Status)
	}
	if err := mergeStringField(cloned, "carrierCode", pickup.CarrierCode); err != nil {
		return PickupCancellationResolution{}, err
	}
	if err := mergeStringField(cloned, "scheduledDate", pickup.ScheduledDate); err != nil {
		return PickupCancellationResolution{}, err
	}
	if err := mergeStringField(cloned, "location", pickup.LocationCode); err != nil {
		return PickupCancellationResolution{}, err
	}
	if err := mergeAccountField(cloned, "associatedAccountNumber", pickup.AccountNumber); err != nil {
		return PickupCancellationResolution{}, err
	}
	if err := ValidateRequest(ActionCancelPickup, cloned); err != nil {
		return PickupCancellationResolution{}, err
	}
	return PickupCancellationResolution{Body: cloned, Context: map[string]any{"pickup_source": "ledger", "pickup_operation_id": pickup.OperationID}}, nil
}

func cloneObject(value map[string]any) (map[string]any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}
	var clone map[string]any
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, fmt.Errorf("decoding request: %w", err)
	}
	return clone, nil
}

func stringField(body map[string]any, key string) string {
	value, _ := body[key].(string)
	return strings.TrimSpace(value)
}

func mergeStringField(body map[string]any, key, stored string) error {
	stored = strings.TrimSpace(stored)
	provided := stringField(body, key)
	if provided != "" && stored != "" && provided != stored {
		return fmt.Errorf("%s conflicts with the local pickup ledger", key)
	}
	if provided == "" && stored != "" {
		body[key] = stored
	}
	return nil
}

func mergeAccountField(body map[string]any, key, stored string) error {
	stored = strings.TrimSpace(stored)
	provided := ""
	if account, ok := body[key].(map[string]any); ok {
		provided, _ = account["value"].(string)
		provided = strings.TrimSpace(provided)
	}
	if provided != "" && stored != "" && provided != stored {
		return fmt.Errorf("%s.value conflicts with the local pickup ledger", key)
	}
	if provided == "" && stored != "" {
		body[key] = map[string]any{"value": stored}
	}
	return nil
}
