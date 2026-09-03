// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/secureio"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"
)

var (
	safeTrackingNumber = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	ErrRemoteRejected  = errors.New("FedEx reported that the requested cancellation did not complete")
)

// PersistOptions contains approval-bound local workflow context. It contains no
// reusable credentials or complete FedEx request/response bodies.
type PersistOptions struct {
	OperationID                string
	PickupPreflight            string
	PickupPreflightCutoff      string
	PickupPreflightAccessStart string
	PickupOverrideReason       string
}

// LabelReceipt is the safe result returned after durable label persistence.
type LabelReceipt struct {
	Status               string  `json:"status"`
	TrackingNumber       string  `json:"tracking_number"`
	MasterTrackingNumber string  `json:"master_tracking_number,omitempty"`
	ServiceType          string  `json:"service_type"`
	PackagingType        string  `json:"packaging_type,omitempty"`
	Charge               float64 `json:"charge,omitempty"`
	Currency             string  `json:"currency,omitempty"`
	LabelPath            string  `json:"label_path"`
	TransactionID        string  `json:"transaction_id,omitempty"`
}

type CancellationReceipt struct {
	Status         string `json:"status"`
	TrackingNumber string `json:"tracking_number"`
	TransactionID  string `json:"transaction_id,omitempty"`
}

type PickupReceipt struct {
	Status             string `json:"status"`
	ConfirmationNumber string `json:"confirmation_number"`
	CarrierCode        string `json:"carrier_code"`
	ScheduledDate      string `json:"scheduled_date,omitempty"`
	LocationCode       string `json:"location_code,omitempty"`
	TransactionID      string `json:"transaction_id,omitempty"`
	CutoffTime         string `json:"cutoff_time,omitempty"`
	AccessStartTime    string `json:"access_start_time,omitempty"`
}

// BeginMutation records the in-flight state before the FedEx request. The
// returned receipt is non-nil only when a cancellation already completed and
// no remote call is necessary.
func BeginMutation(ctx context.Context, action string, body map[string]any, options PersistOptions) (any, error) {
	fields, err := ExtractOperationalFields(action, body)
	if err != nil {
		return nil, err
	}
	ledger, err := store.Open(store.DefaultPath())
	if err != nil {
		return nil, fmt.Errorf("opening workflow ledger: %w", err)
	}
	defer ledger.Close()
	switch action {
	case ActionCreateLabel:
		if err := secureio.EnsurePrivateDir(filepath.Join(filepath.Dir(store.DefaultPath()), "labels")); err != nil {
			return nil, fmt.Errorf("preparing private label directory: %w", err)
		}
	case ActionSchedulePickup:
		if err := persistUnknownPickup(ctx, ledger, fields, options, "executing"); err != nil {
			return nil, fmt.Errorf("recording pickup attempt: %w", err)
		}
	case ActionCancelShipment:
		existing, err := ledger.GetShipmentByTracking(ctx, fields.TrackingNumber)
		if err != nil {
			return nil, err
		}
		if existing == nil {
			_, err = ledger.InsertShipment(ctx, store.Shipment{TrackingNumber: fields.TrackingNumber, Account: fields.AccountNumber, ServiceType: "UNKNOWN", ShipperCountry: fields.SenderCountryCode, RequestHash: fields.RequestHash, Status: "created", CancellationStatus: "canceling"})
			return nil, err
		}
		if existing.Status == "cancelled" || existing.CancellationStatus == "cancelled" {
			return CancellationReceipt{Status: "already_cancelled", TrackingNumber: fields.TrackingNumber, TransactionID: existing.CancellationTransactionID}, nil
		}
		if existing.CancellationStatus == "canceling" || strings.Contains(existing.CancellationStatus, "outcome_unknown") {
			return nil, fmt.Errorf("shipment %s has cancellation status %s; reconcile before retrying", fields.TrackingNumber, existing.CancellationStatus)
		}
		updated, err := ledger.UpdateShipmentCancellation(ctx, fields.TrackingNumber, "canceling", "")
		if err != nil || !updated {
			return nil, fmt.Errorf("could not acquire shipment cancellation state: %w", err)
		}
	case ActionCancelPickup:
		existing, err := ledger.GetPickupByConfirmation(ctx, fields.ConfirmationNumber)
		if err != nil {
			return nil, err
		}
		if existing == nil {
			if err := ledger.UpsertPickup(ctx, store.Pickup{OperationID: options.OperationID, ConfirmationNumber: fields.ConfirmationNumber, AccountNumber: fields.AccountNumber, CarrierCode: fields.CarrierCode, ScheduledDate: fields.ScheduledDate, LocationCode: fields.LocationCode, RequestHash: fields.RequestHash, Status: "canceling"}); err != nil {
				return nil, err
			}
			return nil, nil
		}
		if existing.Status == "cancelled" {
			return PickupReceipt{Status: "already_cancelled", ConfirmationNumber: fields.ConfirmationNumber, CarrierCode: existing.CarrierCode, ScheduledDate: existing.ScheduledDate, LocationCode: existing.LocationCode, TransactionID: existing.CancellationTransactionID}, nil
		}
		if existing.Status == "canceling" || strings.Contains(existing.Status, "outcome_unknown") {
			return nil, fmt.Errorf("pickup %s has status %s; reconcile before retrying", fields.ConfirmationNumber, existing.Status)
		}
		updated, err := ledger.UpdatePickupCancellation(ctx, fields.ConfirmationNumber, "canceling", "")
		if err != nil || !updated {
			return nil, fmt.Errorf("could not acquire pickup cancellation state: %w", err)
		}
	}
	return nil, nil
}

// PersistRejected records a definite FedEx rejection without converting it
// into an outcome-unknown state.
func PersistRejected(ctx context.Context, action string, body map[string]any, options PersistOptions) {
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
		_, _ = ledger.UpdateShipmentCancellation(ctx, fields.TrackingNumber, "cancel_rejected", "")
	case ActionSchedulePickup:
		_ = persistUnknownPickup(ctx, ledger, fields, options, "rejected")
	case ActionCancelPickup:
		_, _ = ledger.UpdatePickupCancellation(ctx, fields.ConfirmationNumber, "cancel_rejected", "")
	}
}

// ReconcileOperationalState applies a confirmed manual outcome to the local
// operational ledger before the approval record is released or finalized.
func ReconcileOperationalState(ctx context.Context, action, operationID, trackingNumber, pickupConfirmation, resolution string) error {
	if action == ActionCreateLabel {
		// An outcome-unknown label create has no trustworthy tracking identifier
		// until a future import workflow records one. The approval ledger remains
		// the source of truth and still blocks a repeat when resolution=succeeded.
		return nil
	}
	ledger, err := store.Open(store.DefaultPath())
	if err != nil {
		return fmt.Errorf("opening workflow ledger for reconciliation: %w", err)
	}
	defer ledger.Close()

	switch action {
	case ActionCancelShipment:
		status := "cancel_rejected"
		if resolution == "succeeded" {
			status = "cancelled"
		}
		updated, err := ledger.UpdateShipmentCancellation(ctx, trackingNumber, status, "")
		if err != nil {
			return err
		}
		if !updated {
			return fmt.Errorf("shipment %s is missing from the reconciliation ledger", trackingNumber)
		}
	case ActionSchedulePickup:
		status := "rejected"
		if resolution == "succeeded" {
			status = "succeeded_unverified_identifiers"
		}
		updated, err := ledger.UpdatePickupStatusByOperationID(ctx, operationID, status)
		if err != nil {
			return err
		}
		if !updated {
			return fmt.Errorf("pickup operation %s is missing from the reconciliation ledger", operationID)
		}
	case ActionCancelPickup:
		status := "cancel_rejected"
		if resolution == "succeeded" {
			status = "cancelled"
		}
		updated, err := ledger.UpdatePickupCancellation(ctx, pickupConfirmation, status, "")
		if err != nil {
			return err
		}
		if !updated {
			return fmt.Errorf("pickup %s is missing from the reconciliation ledger", pickupConfirmation)
		}
	}
	return nil
}

// PersistSuccess parses a successful FedEx mutation response, writes any label
// artifact, and records only allowlisted operational fields in the private
// SQLite ledger. It must complete before an approval record is marked succeeded.
func PersistSuccess(ctx context.Context, action string, body map[string]any, response []byte, options PersistOptions) (any, error) {
	fields, err := ExtractOperationalFields(action, body)
	if err != nil {
		return nil, err
	}
	ledger, err := store.Open(store.DefaultPath())
	if err != nil {
		return nil, fmt.Errorf("opening workflow ledger after FedEx success: %w", err)
	}
	defer ledger.Close()

	switch action {
	case ActionCreateLabel:
		result, _, err := ParseAndDecodeLabelResponse(response, MaxPDFLabelBytes)
		if err != nil {
			return nil, fmt.Errorf("FedEx accepted shipment but returned an unusable label; reconcile transaction before retrying: %w", err)
		}
		if result.ServiceType != "" && result.ServiceType != fields.ServiceType {
			return nil, fmt.Errorf("FedEx label response service %s did not match requested service %s; reconcile before retrying", result.ServiceType, fields.ServiceType)
		}
		if !safeTrackingNumber.MatchString(result.TrackingNumber) {
			return nil, fmt.Errorf("FedEx accepted shipment but returned an unsafe tracking number; reconcile transaction before retrying")
		}
		labelPath := filepath.Join(filepath.Dir(store.DefaultPath()), "labels", result.TrackingNumber+".pdf")
		if err := WritePDFLabelAtomic(labelPath, result.EncodedLabel, MaxPDFLabelBytes); err != nil {
			return nil, fmt.Errorf("FedEx accepted shipment but local label persistence failed; reconcile transaction before retrying: %w", err)
		}
		serviceType := result.ServiceType
		if serviceType == "" {
			serviceType = fields.ServiceType
		}
		packagingType := result.PackagingType
		if packagingType == "" {
			packagingType = fields.PackagingType
		}
		charge := result.NetChargeAmount
		if charge == 0 {
			charge = result.ListChargeAmount
		}
		shipment := &store.Shipment{
			TrackingNumber:       result.TrackingNumber,
			MasterTrackingNumber: result.MasterTrackingNumber,
			Account:              fields.AccountNumber,
			ServiceType:          serviceType,
			PackagingType:        packagingType,
			ShipperName:          fields.ShipperName,
			ShipperPostal:        fields.ShipperPostal,
			ShipperCountry:       fields.ShipperCountry,
			RecipientName:        fields.RecipientName,
			RecipientAddress:     fields.RecipientAddress,
			RecipientCity:        fields.RecipientCity,
			RecipientState:       fields.RecipientState,
			RecipientPostal:      fields.RecipientPostal,
			RecipientCountry:     fields.RecipientCountry,
			WeightValue:          fields.WeightValue,
			WeightUnits:          fields.WeightUnits,
			Reference:            fields.Reference,
			NetChargeAmount:      result.NetChargeAmount,
			ListChargeAmount:     result.ListChargeAmount,
			LabelPath:            labelPath,
			TransactionID:        firstNonEmpty(result.TransactionID, result.CustomerTransactionID),
			RequestHash:          fields.RequestHash,
			Status:               "created",
		}
		if _, err := ledger.InsertShipment(ctx, *shipment); err != nil {
			return nil, fmt.Errorf("FedEx accepted shipment but ledger persistence failed; label retained at %s for reconciliation: %w", labelPath, err)
		}
		return LabelReceipt{Status: "created", TrackingNumber: result.TrackingNumber, MasterTrackingNumber: result.MasterTrackingNumber, ServiceType: serviceType, PackagingType: packagingType, Charge: charge, Currency: result.Currency, LabelPath: labelPath, TransactionID: shipment.TransactionID}, nil

	case ActionCancelShipment:
		result, err := ParseCancelShipmentResponse(response)
		if err != nil {
			_, _ = ledger.UpdateShipmentCancellation(ctx, fields.TrackingNumber, "cancel_outcome_unknown", "")
			return nil, fmt.Errorf("FedEx returned an ambiguous shipment-cancellation response; reconcile before retrying: %w", err)
		}
		if !result.SuccessKnown {
			_, _ = ledger.UpdateShipmentCancellation(ctx, fields.TrackingNumber, "cancel_outcome_unknown", "")
			return nil, errors.New("FedEx did not provide an explicit shipment-cancellation result; reconcile before retrying")
		}
		if !result.Successful {
			return nil, ErrRemoteRejected
		}
		transactionID := firstNonEmpty(result.TransactionID, result.CustomerTransactionID)
		if err := ensureShipmentCancellationRow(ctx, ledger, fields, transactionID); err != nil {
			return nil, err
		}
		return CancellationReceipt{Status: "cancelled", TrackingNumber: fields.TrackingNumber, TransactionID: transactionID}, nil

	case ActionSchedulePickup:
		result, err := ParsePickupResponse(response)
		if err != nil {
			_ = persistUnknownPickup(ctx, ledger, fields, options, "outcome_unknown")
			return nil, fmt.Errorf("FedEx returned an ambiguous pickup response; reconcile before retrying: %w", err)
		}
		carrier := firstNonEmpty(result.CarrierCode, fields.CarrierCode)
		if result.CarrierCode != "" && result.CarrierCode != fields.CarrierCode {
			_ = persistUnknownPickup(ctx, ledger, fields, options, "outcome_unknown")
			return nil, fmt.Errorf("FedEx pickup response carrier %s did not match requested carrier %s; reconcile before retrying", result.CarrierCode, fields.CarrierCode)
		}
		if result.ScheduledDate != "" && result.ScheduledDate != fields.ScheduledDate {
			_ = persistUnknownPickup(ctx, ledger, fields, options, "outcome_unknown")
			return nil, fmt.Errorf("FedEx pickup response date %s did not match requested date %s; reconcile before retrying", result.ScheduledDate, fields.ScheduledDate)
		}
		if carrier == "FDXE" && strings.TrimSpace(result.LocationCode) == "" {
			_ = persistUnknownPickup(ctx, ledger, fields, options, "outcome_unknown")
			return nil, fmt.Errorf("FedEx Express pickup response omitted the location code required for cancellation; reconcile before retrying")
		}
		pickup := &store.Pickup{
			OperationID:             options.OperationID,
			ConfirmationNumber:      result.ConfirmationNumber,
			AccountNumber:           fields.AccountNumber,
			CarrierCode:             carrier,
			ScheduledDate:           firstNonEmpty(result.ScheduledDate, fields.ScheduledDate),
			LocationCode:            result.LocationCode,
			CutoffTime:              options.PickupPreflightCutoff,
			AccessStartTime:         options.PickupPreflightAccessStart,
			ReadyTime:               fields.ReadyDateTimestamp,
			RequestHash:             fields.RequestHash,
			TransactionID:           firstNonEmpty(result.TransactionID, result.CustomerTransactionID),
			Status:                  "scheduled",
			PreflightStatus:         options.PickupPreflight,
			PreflightOverrideReason: options.PickupOverrideReason,
		}
		if err := ledger.UpsertPickup(ctx, *pickup); err != nil {
			return nil, fmt.Errorf("FedEx scheduled pickup but ledger persistence failed; reconcile confirmation %s before retrying: %w", result.ConfirmationNumber, err)
		}
		return PickupReceipt{Status: "scheduled", ConfirmationNumber: pickup.ConfirmationNumber, CarrierCode: pickup.CarrierCode, ScheduledDate: pickup.ScheduledDate, LocationCode: pickup.LocationCode, TransactionID: pickup.TransactionID, CutoffTime: pickup.CutoffTime, AccessStartTime: pickup.AccessStartTime}, nil

	case ActionCancelPickup:
		result, err := ParseCancelPickupResponse(response)
		if err != nil {
			_, _ = ledger.UpdatePickupCancellation(ctx, fields.ConfirmationNumber, "cancel_outcome_unknown", "")
			return nil, fmt.Errorf("FedEx returned an ambiguous pickup-cancellation response; reconcile before retrying: %w", err)
		}
		if result.ConfirmationNumber != "" && result.ConfirmationNumber != fields.ConfirmationNumber {
			_, _ = ledger.UpdatePickupCancellation(ctx, fields.ConfirmationNumber, "cancel_outcome_unknown", "")
			return nil, fmt.Errorf("FedEx pickup-cancellation response confirmation did not match the requested pickup; reconcile before retrying")
		}
		if !result.SuccessKnown {
			_, _ = ledger.UpdatePickupCancellation(ctx, fields.ConfirmationNumber, "cancel_outcome_unknown", "")
			return nil, errors.New("FedEx did not provide an explicit pickup-cancellation result; reconcile before retrying")
		}
		if !result.Successful {
			return nil, ErrRemoteRejected
		}
		transactionID := firstNonEmpty(result.TransactionID, result.CustomerTransactionID)
		if existing, err := ledger.GetPickupByConfirmation(ctx, fields.ConfirmationNumber); err == nil && existing == nil {
			if err := ledger.UpsertPickup(ctx, store.Pickup{OperationID: options.OperationID, ConfirmationNumber: fields.ConfirmationNumber, AccountNumber: fields.AccountNumber, CarrierCode: fields.CarrierCode, ScheduledDate: fields.ScheduledDate, LocationCode: fields.LocationCode, RequestHash: fields.RequestHash, Status: "cancelled", TransactionID: transactionID}); err != nil {
				return nil, fmt.Errorf("FedEx cancelled pickup but legacy ledger persistence failed; reconcile confirmation %s: %w", fields.ConfirmationNumber, err)
			}
		} else if err != nil {
			return nil, err
		} else if _, err := ledger.UpdatePickupCancellation(ctx, fields.ConfirmationNumber, "cancelled", transactionID); err != nil {
			return nil, fmt.Errorf("FedEx cancelled pickup but ledger update failed; reconcile confirmation %s: %w", fields.ConfirmationNumber, err)
		}
		return PickupReceipt{Status: "cancelled", ConfirmationNumber: fields.ConfirmationNumber, CarrierCode: fields.CarrierCode, ScheduledDate: fields.ScheduledDate, LocationCode: fields.LocationCode, TransactionID: transactionID}, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedAction, action)
	}
}

// PersistOutcomeUnknown marks any identifiable local record as requiring
// reconciliation after an ambiguous write failure.
func PersistOutcomeUnknown(ctx context.Context, action string, body map[string]any, options PersistOptions) {
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
		_, _ = ledger.UpdateShipmentCancellation(ctx, fields.TrackingNumber, "cancel_outcome_unknown", "")
	case ActionSchedulePickup:
		_ = persistUnknownPickup(ctx, ledger, fields, options, "outcome_unknown")
	case ActionCancelPickup:
		_, _ = ledger.UpdatePickupCancellation(ctx, fields.ConfirmationNumber, "cancel_outcome_unknown", "")
	}
}

func ensureShipmentCancellationRow(ctx context.Context, ledger *store.Store, fields OperationalFields, transactionID string) error {
	if existing, err := ledger.GetShipmentByTracking(ctx, fields.TrackingNumber); err == nil && existing == nil {
		if _, err := ledger.InsertShipment(ctx, store.Shipment{TrackingNumber: fields.TrackingNumber, Account: fields.AccountNumber, ServiceType: "UNKNOWN", ShipperCountry: fields.SenderCountryCode, RequestHash: fields.RequestHash, Status: "cancelled", CancellationTransactionID: transactionID, CancellationStatus: "cancelled"}); err != nil {
			return fmt.Errorf("FedEx cancelled shipment but legacy ledger persistence failed; reconcile tracking %s: %w", fields.TrackingNumber, err)
		}
		return nil
	} else if err != nil {
		return err
	}
	if _, err := ledger.UpdateShipmentCancellation(ctx, fields.TrackingNumber, "cancelled", transactionID); err != nil {
		return fmt.Errorf("FedEx cancelled shipment but ledger update failed; reconcile tracking %s: %w", fields.TrackingNumber, err)
	}
	return nil
}

func persistUnknownPickup(ctx context.Context, ledger *store.Store, fields OperationalFields, options PersistOptions, status string) error {
	return ledger.UpsertPickup(ctx, store.Pickup{OperationID: options.OperationID, AccountNumber: fields.AccountNumber, CarrierCode: fields.CarrierCode, ScheduledDate: fields.ScheduledDate, CutoffTime: options.PickupPreflightCutoff, AccessStartTime: options.PickupPreflightAccessStart, ReadyTime: fields.ReadyDateTimestamp, RequestHash: fields.RequestHash, Status: status, PreflightStatus: options.PickupPreflight, PreflightOverrideReason: options.PickupOverrideReason})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// MarshalSafeResult returns JSON for a typed receipt and cannot include the
// complete FedEx request, response, encoded label, or recipient contact tree.
func MarshalSafeResult(result any) ([]byte, error) {
	return json.Marshal(result)
}
