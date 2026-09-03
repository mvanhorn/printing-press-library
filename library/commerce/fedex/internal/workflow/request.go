// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package workflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func requestJSON(body any) ([]byte, error) {
	switch value := body.(type) {
	case json.RawMessage:
		return value, nil
	case []byte:
		return value, nil
	default:
		return json.Marshal(body)
	}
}

// CanonicalBody converts a request into deterministic JSON. Object keys are
// sorted by encoding/json, insignificant whitespace is removed, and trailing
// JSON values are rejected.
func CanonicalBody(body any) ([]byte, error) {
	var source []byte
	switch value := body.(type) {
	case json.RawMessage:
		source = value
	case []byte:
		source = value
	default:
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
		source = encoded
	}

	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decoding request body: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decoding request body: multiple JSON values")
		}
		return nil, fmt.Errorf("decoding request body: %w", err)
	}
	canonical, err := json.Marshal(decoded)
	if err != nil {
		return nil, fmt.Errorf("canonicalizing request body: %w", err)
	}
	return canonical, nil
}

// CanonicalBodySHA256 returns a lowercase hexadecimal SHA-256 of CanonicalBody.
func CanonicalBodySHA256(body any) (string, error) {
	canonical, err := CanonicalBody(body)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

// RequestHash is an explicit workflow-oriented alias for CanonicalBodySHA256.
func RequestHash(body any) (string, error) { return CanonicalBodySHA256(body) }

// OperationalFields is the allowlisted request information needed to persist
// or reconcile a workflow. It contains no complete request or response body.
type OperationalFields struct {
	Action             string
	RequestHash        string
	AccountNumber      string
	TrackingNumber     string
	DeletionControl    string
	SenderCountryCode  string
	ServiceType        string
	PackagingType      string
	ShipperName        string
	ShipperPostal      string
	ShipperCountry     string
	RecipientName      string
	RecipientAddress   string
	RecipientCity      string
	RecipientState     string
	RecipientPostal    string
	RecipientCountry   string
	WeightValue        float64
	WeightUnits        string
	Reference          string
	PackageCount       int
	CarrierCode        string
	ScheduledDate      string
	ReadyDateTimestamp string
	CustomerCloseTime  string
	ConfirmationNumber string
	LocationCode       string
}

// ExtractOperationalFields validates body and extracts only fields needed by
// the shipment/pickup ledgers. The hash is calculated from the original body,
// including optional FedEx fields not represented by this package's typed
// safety subset.
func ExtractOperationalFields(action string, body any) (OperationalFields, error) {
	if err := ValidateRequest(action, body); err != nil {
		return OperationalFields{}, err
	}
	hash, err := RequestHash(body)
	if err != nil {
		return OperationalFields{}, err
	}
	fields := OperationalFields{Action: action, RequestHash: hash}

	data, err := requestJSON(body)
	if err != nil {
		return OperationalFields{}, fmt.Errorf("encoding %s request: %w", action, err)
	}
	switch action {
	case ActionCreateLabel:
		var request CreateLabelRequest
		if err := json.Unmarshal(data, &request); err != nil {
			return OperationalFields{}, requestDecodeError(action, err)
		}
		s := request.RequestedShipment
		fields.AccountNumber = strings.TrimSpace(request.AccountNumber.Value)
		fields.ServiceType = strings.TrimSpace(s.ServiceType)
		fields.PackagingType = strings.TrimSpace(s.PackagingType)
		fields.ShipperName = partyName(s.Shipper)
		fields.ShipperPostal = strings.TrimSpace(s.Shipper.Address.PostalCode)
		fields.ShipperCountry = strings.TrimSpace(s.Shipper.Address.CountryCode)
		fields.PackageCount = len(s.RequestedPackageLineItems)
		if len(s.Recipients) > 0 {
			recipient := s.Recipients[0]
			fields.RecipientName = partyName(recipient)
			fields.RecipientAddress = strings.Join(nonEmptyStrings(recipient.Address.StreetLines), ", ")
			fields.RecipientCity = strings.TrimSpace(recipient.Address.City)
			fields.RecipientState = strings.TrimSpace(recipient.Address.StateOrProvinceCode)
			fields.RecipientPostal = strings.TrimSpace(recipient.Address.PostalCode)
			fields.RecipientCountry = strings.TrimSpace(recipient.Address.CountryCode)
		}
		for _, item := range s.RequestedPackageLineItems {
			if fields.WeightUnits == "" {
				fields.WeightUnits = strings.TrimSpace(item.Weight.Units)
			}
			if strings.EqualFold(fields.WeightUnits, strings.TrimSpace(item.Weight.Units)) {
				fields.WeightValue += item.Weight.Value
			}
			if fields.Reference == "" {
				fields.Reference = preferredReference(item.CustomerReferences)
			}
		}
	case ActionCancelShipment:
		var request CancelShipmentRequest
		if err := json.Unmarshal(data, &request); err != nil {
			return OperationalFields{}, requestDecodeError(action, err)
		}
		fields.AccountNumber = strings.TrimSpace(request.AccountNumber.Value)
		fields.TrackingNumber = strings.TrimSpace(request.TrackingNumber)
		fields.DeletionControl = strings.TrimSpace(request.DeletionControl)
		fields.SenderCountryCode = strings.TrimSpace(request.SenderCountryCode)
	case ActionSchedulePickup:
		var request SchedulePickupRequest
		if err := json.Unmarshal(data, &request); err != nil {
			return OperationalFields{}, requestDecodeError(action, err)
		}
		fields.AccountNumber = strings.TrimSpace(request.AssociatedAccountNumber.Value)
		fields.CarrierCode = strings.TrimSpace(request.CarrierCode)
		fields.PackageCount = request.PackageCount
		fields.WeightValue = request.TotalWeight.Value
		fields.WeightUnits = strings.TrimSpace(request.TotalWeight.Units)
		fields.ReadyDateTimestamp = strings.TrimSpace(request.OriginDetail.ReadyDateTimestamp)
		fields.CustomerCloseTime = strings.TrimSpace(request.OriginDetail.CustomerCloseTime)
		fields.ScheduledDate = datePart(fields.ReadyDateTimestamp)
	case ActionCancelPickup:
		var request CancelPickupRequest
		if err := json.Unmarshal(data, &request); err != nil {
			return OperationalFields{}, requestDecodeError(action, err)
		}
		fields.AccountNumber = strings.TrimSpace(request.AssociatedAccountNumber.Value)
		fields.ConfirmationNumber = strings.TrimSpace(request.PickupConfirmationCode)
		fields.CarrierCode = strings.TrimSpace(request.CarrierCode)
		fields.ScheduledDate = strings.TrimSpace(request.ScheduledDate)
		fields.LocationCode = strings.TrimSpace(request.Location)
	default:
		return OperationalFields{}, fmt.Errorf("%w: %q", ErrUnsupportedAction, action)
	}
	return fields, nil
}

func partyName(party Party) string {
	if value := strings.TrimSpace(party.Contact.CompanyName); value != "" {
		return value
	}
	return strings.TrimSpace(party.Contact.PersonName)
}

func nonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func preferredReference(references []CustomerReference) string {
	for _, reference := range references {
		if strings.TrimSpace(reference.CustomerReferenceType) == "CUSTOMER_REFERENCE" && strings.TrimSpace(reference.Value) != "" {
			return strings.TrimSpace(reference.Value)
		}
	}
	for _, reference := range references {
		if value := strings.TrimSpace(reference.Value); value != "" {
			return value
		}
	}
	return ""
}

func datePart(timestamp string) string {
	if len(timestamp) >= len("2006-01-02") {
		return timestamp[:len("2006-01-02")]
	}
	return timestamp
}
