// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

// Package workflow provides typed validation, preflight, parsing, and private
// persistence for FedEx label and pickup workflows. It deliberately returns
// only operational fields; callers should not persist complete FedEx responses.
package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	ActionCreateLabel    = "create_label"
	ActionCancelShipment = "cancel_shipment"
	ActionSchedulePickup = "schedule_pickup"
	ActionCancelPickup   = "cancel_pickup"
)

var ErrUnsupportedAction = errors.New("unsupported workflow action")

// AccountNumber is the account object used by FedEx shipment and pickup APIs.
type AccountNumber struct {
	Value string `json:"value"`
}

// Contact contains the FedEx contact fields needed for a shipment party or
// pickup location. Either PersonName or CompanyName may identify the party.
type Contact struct {
	PersonName     string `json:"personName,omitempty"`
	CompanyName    string `json:"companyName,omitempty"`
	PhoneNumber    string `json:"phoneNumber,omitempty"`
	PhoneExtension string `json:"phoneExtension,omitempty"`
	EmailAddress   string `json:"emailAddress,omitempty"`
}

// Address contains the common domestic and international address fields.
type Address struct {
	StreetLines         []string `json:"streetLines,omitempty"`
	City                string   `json:"city,omitempty"`
	StateOrProvinceCode string   `json:"stateOrProvinceCode,omitempty"`
	PostalCode          string   `json:"postalCode,omitempty"`
	CountryCode         string   `json:"countryCode,omitempty"`
	Residential         bool     `json:"residential,omitempty"`
}

// Party is a shipment shipper or recipient.
type Party struct {
	Contact       Contact       `json:"contact"`
	Address       Address       `json:"address"`
	AccountNumber AccountNumber `json:"accountNumber,omitempty"`
}

// Weight is a FedEx quantity. Value must be finite and greater than zero.
type Weight struct {
	Units string  `json:"units"`
	Value float64 `json:"value"`
}

// CustomerReference identifies a caller-supplied package reference.
type CustomerReference struct {
	CustomerReferenceType string `json:"customerReferenceType,omitempty"`
	Value                 string `json:"value"`
}

// PackageLineItem is the minimum package representation needed by Phase 4.
type PackageLineItem struct {
	SequenceNumber     int                 `json:"sequenceNumber,omitempty"`
	Weight             Weight              `json:"weight"`
	GroupPackageCount  int                 `json:"groupPackageCount,omitempty"`
	CustomerReferences []CustomerReference `json:"customerReferences,omitempty"`
}

// LabelSpecification describes the requested label artifact.
type LabelSpecification struct {
	ImageType      string `json:"imageType"`
	LabelStockType string `json:"labelStockType,omitempty"`
}

// RequestedShipment is the typed subset needed to safely create one shipment.
// Additional FedEx options may remain in the original request passed to the
// client; ValidateRequest validates this safety-critical subset without
// rewriting or narrowing that original body.
type RequestedShipment struct {
	Shipper                   Party              `json:"shipper"`
	Recipients                []Party            `json:"recipients"`
	ServiceType               string             `json:"serviceType"`
	PackagingType             string             `json:"packagingType"`
	TotalPackageCount         int                `json:"totalPackageCount,omitempty"`
	RequestedPackageLineItems []PackageLineItem  `json:"requestedPackageLineItems"`
	LabelSpecification        LabelSpecification `json:"labelSpecification"`
	ShipDatestamp             string             `json:"shipDatestamp,omitempty"`
}

// CreateLabelRequest is a single-shipment label request.
type CreateLabelRequest struct {
	LabelResponseOptions string            `json:"labelResponseOptions"`
	RequestedShipment    RequestedShipment `json:"requestedShipment"`
	AccountNumber        AccountNumber     `json:"accountNumber"`
	ShipAction           string            `json:"shipAction,omitempty"`
	OneLabelAtATime      bool              `json:"oneLabelAtATime,omitempty"`
}

// CancelShipmentRequest contains all identifiers bound into a cancellation.
type CancelShipmentRequest struct {
	AccountNumber     AccountNumber `json:"accountNumber"`
	TrackingNumber    string        `json:"trackingNumber"`
	SenderCountryCode string        `json:"senderCountryCode"`
	DeletionControl   string        `json:"deletionControl"`
	EmailShipment     bool          `json:"emailShipment,omitempty"`
}

// PickupLocation identifies the location at which FedEx should collect the
// packages.
type PickupLocation struct {
	Contact              Contact       `json:"contact"`
	Address              Address       `json:"address"`
	AccountNumber        AccountNumber `json:"accountNumber,omitempty"`
	DeliveryInstructions string        `json:"deliveryInstructions,omitempty"`
}

// PickupOriginDetail contains the collection location and pickup window.
type PickupOriginDetail struct {
	PickupLocation          PickupLocation `json:"pickupLocation"`
	PackageLocation         string         `json:"packageLocation,omitempty"`
	BuildingPart            string         `json:"buildingPart,omitempty"`
	BuildingPartDescription string         `json:"buildingPartDescription,omitempty"`
	ReadyDateTimestamp      string         `json:"readyDateTimestamp"`
	CustomerCloseTime       string         `json:"customerCloseTime"`
}

// SchedulePickupRequest creates either an Express (FDXE) or Ground (FDXG)
// pickup. Availability/preflight state is intentionally managed by the caller.
type SchedulePickupRequest struct {
	AssociatedAccountNumber AccountNumber      `json:"associatedAccountNumber"`
	OriginDetail            PickupOriginDetail `json:"originDetail"`
	TotalWeight             Weight             `json:"totalWeight"`
	PackageCount            int                `json:"packageCount"`
	CarrierCode             string             `json:"carrierCode"`
	Remarks                 string             `json:"remarks,omitempty"`
	CountryRelationships    string             `json:"countryRelationships,omitempty"`
}

// CancelPickupRequest carries the identifiers needed to cancel a pickup.
// Location is required for Express and optional for Ground.
type CancelPickupRequest struct {
	AssociatedAccountNumber AccountNumber `json:"associatedAccountNumber"`
	PickupConfirmationCode  string        `json:"pickupConfirmationCode"`
	CarrierCode             string        `json:"carrierCode"`
	ScheduledDate           string        `json:"scheduledDate"`
	Location                string        `json:"location,omitempty"`
	Remarks                 string        `json:"remarks,omitempty"`
}

// Validate validates a create-label request's safety-critical fields.
func (r CreateLabelRequest) Validate() error {
	var problems []string
	require(&problems, "accountNumber.value", r.AccountNumber.Value)
	if strings.TrimSpace(r.LabelResponseOptions) != "LABEL" {
		problems = append(problems, "labelResponseOptions must be LABEL")
	}
	s := r.RequestedShipment
	validateParty(&problems, "requestedShipment.shipper", s.Shipper)
	if len(s.Recipients) != 1 {
		problems = append(problems, "requestedShipment.recipients must contain exactly one recipient")
	} else {
		for i, recipient := range s.Recipients {
			validateParty(&problems, fmt.Sprintf("requestedShipment.recipients[%d]", i), recipient)
		}
	}
	require(&problems, "requestedShipment.serviceType", s.ServiceType)
	require(&problems, "requestedShipment.packagingType", s.PackagingType)
	if len(s.RequestedPackageLineItems) != 1 {
		problems = append(problems, "requestedShipment.requestedPackageLineItems must contain exactly one package")
	}
	for i, item := range s.RequestedPackageLineItems {
		validateWeight(&problems, fmt.Sprintf("requestedShipment.requestedPackageLineItems[%d].weight", i), item.Weight)
		if item.GroupPackageCount != 0 && item.GroupPackageCount != 1 {
			problems = append(problems, fmt.Sprintf("requestedShipment.requestedPackageLineItems[%d].groupPackageCount must be omitted or 1", i))
		}
	}
	if s.TotalPackageCount < 0 || s.TotalPackageCount > 0 && s.TotalPackageCount != len(s.RequestedPackageLineItems) {
		problems = append(problems, "requestedShipment.totalPackageCount must match requestedPackageLineItems")
	}
	if strings.TrimSpace(s.LabelSpecification.ImageType) != "PDF" {
		problems = append(problems, "requestedShipment.labelSpecification.imageType must be PDF")
	}
	return validationError(ActionCreateLabel, problems)
}

// Validate validates a shipment cancellation request.
func (r CancelShipmentRequest) Validate() error {
	var problems []string
	require(&problems, "accountNumber.value", r.AccountNumber.Value)
	require(&problems, "trackingNumber", r.TrackingNumber)
	require(&problems, "senderCountryCode", r.SenderCountryCode)
	switch strings.TrimSpace(r.DeletionControl) {
	case "DELETE_ALL_PACKAGES", "DELETE_ONE_PACKAGE":
	default:
		problems = append(problems, "deletionControl must be DELETE_ALL_PACKAGES or DELETE_ONE_PACKAGE")
	}
	return validationError(ActionCancelShipment, problems)
}

// Validate validates a pickup scheduling request.
func (r SchedulePickupRequest) Validate() error {
	var problems []string
	require(&problems, "associatedAccountNumber.value", r.AssociatedAccountNumber.Value)
	validatePickupLocation(&problems, "originDetail.pickupLocation", r.OriginDetail.PickupLocation)
	require(&problems, "originDetail.readyDateTimestamp", r.OriginDetail.ReadyDateTimestamp)
	require(&problems, "originDetail.customerCloseTime", r.OriginDetail.CustomerCloseTime)
	if value := strings.TrimSpace(r.OriginDetail.ReadyDateTimestamp); value != "" {
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			problems = append(problems, "originDetail.readyDateTimestamp must use RFC3339")
		}
	}
	if value := strings.TrimSpace(r.OriginDetail.CustomerCloseTime); value != "" {
		if _, err := time.Parse("15:04", value); err != nil {
			if _, withSecondsErr := time.Parse("15:04:05", value); withSecondsErr != nil {
				problems = append(problems, "originDetail.customerCloseTime must use HH:MM or HH:MM:SS")
			}
		}
	}
	validateWeight(&problems, "totalWeight", r.TotalWeight)
	if r.PackageCount <= 0 {
		problems = append(problems, "packageCount must be greater than zero")
	}
	validateCarrier(&problems, r.CarrierCode)
	return validationError(ActionSchedulePickup, problems)
}

// Validate validates a pickup cancellation request.
func (r CancelPickupRequest) Validate() error {
	var problems []string
	require(&problems, "associatedAccountNumber.value", r.AssociatedAccountNumber.Value)
	require(&problems, "pickupConfirmationCode", r.PickupConfirmationCode)
	validateCarrier(&problems, r.CarrierCode)
	date := strings.TrimSpace(r.ScheduledDate)
	if date == "" {
		problems = append(problems, "scheduledDate is required")
	} else if parsed, err := time.Parse("2006-01-02", date); err != nil || parsed.Format("2006-01-02") != date {
		problems = append(problems, "scheduledDate must use YYYY-MM-DD")
	}
	if strings.TrimSpace(r.CarrierCode) == "FDXE" {
		require(&problems, "location", r.Location)
	}
	return validationError(ActionCancelPickup, problems)
}

// ValidateRequest decodes and validates a supported request without replacing
// the caller's original body. This permits optional FedEx fields outside this
// package's safety-critical typed subset.
func ValidateRequest(action string, body any) error {
	switch typed := body.(type) {
	case CreateLabelRequest:
		if action != ActionCreateLabel {
			return actionTypeError(action, typed)
		}
		return typed.Validate()
	case *CreateLabelRequest:
		if typed == nil {
			return fmt.Errorf("%s request is nil", action)
		}
		if action != ActionCreateLabel {
			return actionTypeError(action, typed)
		}
		return typed.Validate()
	case CancelShipmentRequest:
		if action != ActionCancelShipment {
			return actionTypeError(action, typed)
		}
		return typed.Validate()
	case *CancelShipmentRequest:
		if typed == nil {
			return fmt.Errorf("%s request is nil", action)
		}
		if action != ActionCancelShipment {
			return actionTypeError(action, typed)
		}
		return typed.Validate()
	case SchedulePickupRequest:
		if action != ActionSchedulePickup {
			return actionTypeError(action, typed)
		}
		return typed.Validate()
	case *SchedulePickupRequest:
		if typed == nil {
			return fmt.Errorf("%s request is nil", action)
		}
		if action != ActionSchedulePickup {
			return actionTypeError(action, typed)
		}
		return typed.Validate()
	case CancelPickupRequest:
		if action != ActionCancelPickup {
			return actionTypeError(action, typed)
		}
		return typed.Validate()
	case *CancelPickupRequest:
		if typed == nil {
			return fmt.Errorf("%s request is nil", action)
		}
		if action != ActionCancelPickup {
			return actionTypeError(action, typed)
		}
		return typed.Validate()
	}

	data, err := requestJSON(body)
	if err != nil {
		return fmt.Errorf("encoding %s request: %w", action, err)
	}
	switch action {
	case ActionCreateLabel:
		var request CreateLabelRequest
		if err := json.Unmarshal(data, &request); err != nil {
			return requestDecodeError(action, err)
		}
		return request.Validate()
	case ActionCancelShipment:
		var request CancelShipmentRequest
		if err := json.Unmarshal(data, &request); err != nil {
			return requestDecodeError(action, err)
		}
		return request.Validate()
	case ActionSchedulePickup:
		var request SchedulePickupRequest
		if err := json.Unmarshal(data, &request); err != nil {
			return requestDecodeError(action, err)
		}
		return request.Validate()
	case ActionCancelPickup:
		var request CancelPickupRequest
		if err := json.Unmarshal(data, &request); err != nil {
			return requestDecodeError(action, err)
		}
		return request.Validate()
	default:
		return fmt.Errorf("%w: %q", ErrUnsupportedAction, action)
	}
}

func actionTypeError(action string, body any) error {
	return fmt.Errorf("%s does not accept request type %T", action, body)
}

func requestDecodeError(action string, err error) error {
	return fmt.Errorf("decoding %s request: %w", action, err)
}

func validateParty(problems *[]string, field string, party Party) {
	if strings.TrimSpace(party.Contact.PersonName) == "" && strings.TrimSpace(party.Contact.CompanyName) == "" {
		*problems = append(*problems, field+".contact requires personName or companyName")
	}
	require(problems, field+".contact.phoneNumber", party.Contact.PhoneNumber)
	validateAddress(problems, field+".address", party.Address)
}

func validatePickupLocation(problems *[]string, field string, location PickupLocation) {
	if strings.TrimSpace(location.Contact.PersonName) == "" && strings.TrimSpace(location.Contact.CompanyName) == "" {
		*problems = append(*problems, field+".contact requires personName or companyName")
	}
	require(problems, field+".contact.phoneNumber", location.Contact.PhoneNumber)
	validateAddress(problems, field+".address", location.Address)
}

func validateAddress(problems *[]string, field string, address Address) {
	hasStreet := false
	for _, line := range address.StreetLines {
		if strings.TrimSpace(line) != "" {
			hasStreet = true
			break
		}
	}
	if !hasStreet {
		*problems = append(*problems, field+".streetLines is required")
	}
	require(problems, field+".city", address.City)
	require(problems, field+".postalCode", address.PostalCode)
	require(problems, field+".countryCode", address.CountryCode)
}

func validateWeight(problems *[]string, field string, weight Weight) {
	require(problems, field+".units", weight.Units)
	if weight.Value <= 0 || math.IsNaN(weight.Value) || math.IsInf(weight.Value, 0) {
		*problems = append(*problems, field+".value must be finite and greater than zero")
	}
}

func validateCarrier(problems *[]string, carrier string) {
	switch strings.TrimSpace(carrier) {
	case "FDXE", "FDXG":
	default:
		*problems = append(*problems, "carrierCode must be FDXE or FDXG")
	}
}

func require(problems *[]string, field, value string) {
	if strings.TrimSpace(value) == "" {
		*problems = append(*problems, field+" is required")
	}
}

func validationError(action string, problems []string) error {
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("invalid %s request: %s", action, strings.Join(problems, "; "))
}
