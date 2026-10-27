// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"
)

// Prepare validates the request and records in-flight ledger state. A non-nil
// receipt means the local ledger already completed the operation and FedEx
// must not be called again.
func Prepare(ctx context.Context, action string, body map[string]any, options PersistOptions) ([]byte, error) {
	if err := ValidateRequest(action, body); err != nil {
		return nil, err
	}
	if strings.TrimSpace(options.OperationID) == "" {
		return nil, fmt.Errorf("workflow operation id is required")
	}
	already, err := BeginMutation(ctx, action, body, options)
	if err != nil || already == nil {
		return nil, err
	}
	return MarshalSafeResult(already)
}

// PickupOperationError is a schedule-pickup failure that may have left an
// outcome_unknown ledger row. OperationID is the value for
// `operations reconcile --operation-id`.
type PickupOperationError struct {
	OperationID string
	Err         error
}

func (e *PickupOperationError) Error() string {
	if e == nil {
		return "pickup outcome unknown"
	}
	if e.Err == nil {
		return "pickup outcome unknown (workflow operation " + e.OperationID + ")"
	}
	return fmt.Sprintf("%s (workflow operation %s)", e.Err.Error(), e.OperationID)
}

func (e *PickupOperationError) Unwrap() error { return e.Err }

func withPickupOperation(action, operationID string, err error) error {
	if err == nil || action != ActionSchedulePickup || strings.TrimSpace(operationID) == "" {
		return err
	}
	return &PickupOperationError{OperationID: operationID, Err: err}
}

// Finish records the FedEx result. unknown is true when the write was sent
// and the outcome cannot be determined.
func Finish(ctx context.Context, action string, body map[string]any, response []byte, callErr error, unknown bool, options PersistOptions) ([]byte, error) {
	if callErr != nil {
		if unknown {
			PersistOutcomeUnknown(ctx, action, body, options)
			return nil, withPickupOperation(action, options.OperationID, callErr)
		}
		PersistRejected(ctx, action, body, options)
		return nil, callErr
	}
	result, err := PersistSuccess(ctx, action, body, response, options)
	if err != nil {
		if errors.Is(err, ErrRemoteRejected) {
			PersistRejected(ctx, action, body, options)
			return nil, err
		}
		PersistOutcomeUnknown(ctx, action, body, options)
		return nil, withPickupOperation(action, options.OperationID, err)
	}
	return MarshalSafeResult(result)
}

// AbortInFlight drops ledger state recorded by Prepare when FedEx was never
// contacted (approval refusal, duplicate guard, or dry-run).
func AbortInFlight(ctx context.Context, action string, body map[string]any, options PersistOptions) {
	fields, err := ExtractOperationalFields(action, body)
	if err != nil {
		return
	}
	ledger, err := store.Open(store.DefaultPath())
	if err != nil {
		return
	}
	defer ledger.Close()
	switch action {
	case ActionCancelShipment:
		_ = ledger.ReleaseShipmentCanceling(ctx, fields.TrackingNumber)
	case ActionCancelPickup:
		_ = ledger.ReleasePickupCanceling(ctx, fields.ConfirmationNumber, options.OperationID)
	case ActionSchedulePickup:
		_ = ledger.DeleteExecutingPickup(ctx, options.OperationID)
	}
}
