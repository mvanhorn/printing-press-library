// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var (
	ErrInvalidResponse     = errors.New("invalid FedEx workflow response")
	ErrTrackingMissing     = errors.New("FedEx response has no tracking number")
	ErrConfirmationMissing = errors.New("FedEx response has no pickup confirmation number")
)

// LabelDocument is the only label artifact retained from a create response.
type LabelDocument struct {
	TrackingNumber string
	ContentType    string
	DocumentType   string
	EncodedLabel   string
}

// LabelResult contains allowlisted operational fields from a successful create
// response. It never retains the raw response, addresses, contacts, or URLs.
type LabelResult struct {
	TransactionID         string
	CustomerTransactionID string
	MasterTrackingNumber  string
	TrackingNumber        string
	ServiceType           string
	ServiceName           string
	PackagingType         string
	CarrierCode           string
	NetChargeAmount       float64
	ListChargeAmount      float64
	Currency              string
	EncodedLabel          string
	Labels                []LabelDocument
}

// CancellationResult contains a shipment cancellation outcome.
type CancellationResult struct {
	TransactionID         string
	CustomerTransactionID string
	SuccessKnown          bool
	Successful            bool
}

// PickupResult contains the Ground/Express identifiers that must be persisted
// immediately after scheduling.
type PickupResult struct {
	TransactionID         string
	CustomerTransactionID string
	ConfirmationNumber    string
	CarrierCode           string
	ScheduledDate         string
	LocationCode          string
	Message               string
}

// PickupCancellationResult contains a pickup cancellation outcome and any
// identifiers echoed by FedEx.
type PickupCancellationResult struct {
	TransactionID         string
	CustomerTransactionID string
	ConfirmationNumber    string
	LocationCode          string
	SuccessKnown          bool
	Successful            bool
}

type labelEnvelope struct {
	TransactionID         string `json:"transactionId"`
	CustomerTransactionID string `json:"customerTransactionId"`
	Output                struct {
		TransactionShipments []transactionShipment `json:"transactionShipments"`
	} `json:"output"`
}

type transactionShipment struct {
	MasterTrackingNumber string          `json:"masterTrackingNumber"`
	ServiceType          string          `json:"serviceType"`
	ServiceName          string          `json:"serviceName"`
	PieceResponses       []pieceResponse `json:"pieceResponses"`
	ShipmentDocuments    []labelDocument `json:"shipmentDocuments"`
	CompletedDetail      struct {
		CarrierCode          string `json:"carrierCode"`
		PackagingDescription string `json:"packagingDescription"`
		MasterTrackingID     struct {
			TrackingNumber string `json:"trackingNumber"`
		} `json:"masterTrackingId"`
		ShipmentRating struct {
			ShipmentRateDetails []rateDetail `json:"shipmentRateDetails"`
		} `json:"shipmentRating"`
	} `json:"completedShipmentDetail"`
}

type pieceResponse struct {
	MasterTrackingNumber string          `json:"masterTrackingNumber"`
	TrackingNumber       string          `json:"trackingNumber"`
	NetCharge            flexibleNumber  `json:"netCharge"`
	NetChargeAmount      flexibleNumber  `json:"netChargeAmount"`
	NetRateAmount        flexibleNumber  `json:"netRateAmount"`
	BaseCharge           flexibleNumber  `json:"baseCharge"`
	Currency             string          `json:"currency"`
	PackageDocuments     []labelDocument `json:"packageDocuments"`
}

type labelDocument struct {
	ContentType  string `json:"contentType"`
	DocumentType string `json:"docType"`
	EncodedLabel string `json:"encodedLabel"`
}

type shipmentCancellationEnvelope struct {
	TransactionID         string `json:"transactionId"`
	CustomerTransactionID string `json:"customerTransactionId"`
	CancelledShipment     *bool  `json:"cancelledShipment"`
	Output                struct {
		CancelledShipment *bool `json:"cancelledShipment"`
	} `json:"output"`
}

type pickupCancellationEnvelope struct {
	TransactionID             string `json:"transactionId"`
	CustomerTransactionID     string `json:"customerTransactionId"`
	CancelledPickup           *bool  `json:"cancelledPickup"`
	PickupConfirmationCode    string `json:"pickupConfirmationCode"`
	CancelConfirmationMessage string `json:"cancelConfirmationMessage"`
	Location                  string `json:"location"`
	Output                    struct {
		CancelledPickup           *bool  `json:"cancelledPickup"`
		PickupConfirmationCode    string `json:"pickupConfirmationCode"`
		CancelConfirmationMessage string `json:"cancelConfirmationMessage"`
		Location                  string `json:"location"`
	} `json:"output"`
}

type rateDetail struct {
	RateType            string         `json:"rateType"`
	TotalNetCharge      flexibleNumber `json:"totalNetCharge"`
	TotalNetFedExCharge flexibleNumber `json:"totalNetFedExCharge"`
	TotalBaseCharge     flexibleNumber `json:"totalBaseCharge"`
	Currency            string         `json:"currency"`
}

type flexibleNumber float64

func (n *flexibleNumber) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) || len(data) == 0 {
		return nil
	}
	var number json.Number
	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		number = json.Number(value)
	} else {
		number = json.Number(string(data))
	}
	value, err := strconv.ParseFloat(number.String(), 64)
	if err != nil {
		return err
	}
	*n = flexibleNumber(value)
	return nil
}

// ParseLabelResponse parses the common Ground and Express shipment response
// shapes and requires a tracking number and embedded label.
func ParseLabelResponse(data []byte) (LabelResult, error) {
	var envelope labelEnvelope
	if err := decodeOne(data, &envelope); err != nil {
		return LabelResult{}, err
	}
	if len(envelope.Output.TransactionShipments) != 1 {
		return LabelResult{}, fmt.Errorf("%w: output.transactionShipments must contain exactly one shipment", ErrInvalidResponse)
	}
	shipment := envelope.Output.TransactionShipments[0]
	if len(shipment.PieceResponses) != 1 {
		return LabelResult{}, fmt.Errorf("%w: transaction shipment must contain exactly one piece response", ErrInvalidResponse)
	}
	result := LabelResult{
		TransactionID:         envelope.TransactionID,
		CustomerTransactionID: envelope.CustomerTransactionID,
		MasterTrackingNumber:  strings.TrimSpace(shipment.MasterTrackingNumber),
		ServiceType:           strings.TrimSpace(shipment.ServiceType),
		ServiceName:           strings.TrimSpace(shipment.ServiceName),
		PackagingType:         strings.TrimSpace(shipment.CompletedDetail.PackagingDescription),
		CarrierCode:           strings.TrimSpace(shipment.CompletedDetail.CarrierCode),
	}
	if result.MasterTrackingNumber == "" {
		result.MasterTrackingNumber = strings.TrimSpace(shipment.CompletedDetail.MasterTrackingID.TrackingNumber)
	}

	for _, piece := range shipment.PieceResponses {
		tracking := strings.TrimSpace(piece.TrackingNumber)
		if result.TrackingNumber == "" {
			result.TrackingNumber = tracking
		}
		if result.MasterTrackingNumber == "" {
			result.MasterTrackingNumber = strings.TrimSpace(piece.MasterTrackingNumber)
		}
		for _, document := range piece.PackageDocuments {
			if strings.TrimSpace(document.EncodedLabel) == "" || !isPDFLabelDocument(document) {
				continue
			}
			result.Labels = append(result.Labels, LabelDocument{
				TrackingNumber: tracking,
				ContentType:    strings.TrimSpace(document.ContentType),
				DocumentType:   strings.TrimSpace(document.DocumentType),
				EncodedLabel:   document.EncodedLabel,
			})
		}
		if result.NetChargeAmount == 0 {
			result.NetChargeAmount = firstNonZero(float64(piece.NetChargeAmount), float64(piece.NetCharge), float64(piece.NetRateAmount))
		}
		if result.ListChargeAmount == 0 {
			result.ListChargeAmount = float64(piece.BaseCharge)
		}
		if result.Currency == "" {
			result.Currency = strings.TrimSpace(piece.Currency)
		}
	}
	for _, document := range shipment.ShipmentDocuments {
		if strings.TrimSpace(document.EncodedLabel) == "" || !isPDFLabelDocument(document) {
			continue
		}
		result.Labels = append(result.Labels, LabelDocument{
			TrackingNumber: result.TrackingNumber,
			ContentType:    strings.TrimSpace(document.ContentType),
			DocumentType:   strings.TrimSpace(document.DocumentType),
			EncodedLabel:   document.EncodedLabel,
		})
	}
	if len(shipment.CompletedDetail.ShipmentRating.ShipmentRateDetails) > 0 {
		rate := preferredRate(shipment.CompletedDetail.ShipmentRating.ShipmentRateDetails)
		if result.NetChargeAmount == 0 {
			result.NetChargeAmount = firstNonZero(float64(rate.TotalNetCharge), float64(rate.TotalNetFedExCharge))
		}
		if result.ListChargeAmount == 0 {
			result.ListChargeAmount = float64(rate.TotalBaseCharge)
		}
		if result.Currency == "" {
			result.Currency = strings.TrimSpace(rate.Currency)
		}
	}
	if result.TrackingNumber == "" {
		result.TrackingNumber = result.MasterTrackingNumber
	}
	if result.MasterTrackingNumber == "" {
		result.MasterTrackingNumber = result.TrackingNumber
	}
	if result.TrackingNumber == "" {
		return LabelResult{}, ErrTrackingMissing
	}
	if len(result.Labels) > 1 {
		return LabelResult{}, fmt.Errorf("%w: response must contain exactly one embedded label, got %d", ErrInvalidResponse, len(result.Labels))
	}
	if len(result.Labels) == 0 {
		return LabelResult{}, ErrLabelMissing
	}
	result.EncodedLabel = result.Labels[0].EncodedLabel
	return result, nil
}

// ParseAndDecodeLabelResponse combines response parsing with bounded PDF
// validation without retaining the raw response.
func ParseAndDecodeLabelResponse(data []byte, maxBytes int64) (LabelResult, []byte, error) {
	result, err := ParseLabelResponse(data)
	if err != nil {
		return LabelResult{}, nil, err
	}
	pdf, err := DecodePDFLabel(result.EncodedLabel, maxBytes)
	if err != nil {
		return LabelResult{}, nil, err
	}
	return result, pdf, nil
}

// ParseShipmentCancellationResponse parses common shipment-cancel variants.
func ParseShipmentCancellationResponse(data []byte) (CancellationResult, error) {
	var envelope shipmentCancellationEnvelope
	if err := decodeOne(data, &envelope); err != nil {
		return CancellationResult{}, err
	}
	result := CancellationResult{TransactionID: envelope.TransactionID, CustomerTransactionID: envelope.CustomerTransactionID}
	status := envelope.Output.CancelledShipment
	if status == nil {
		status = envelope.CancelledShipment
	}
	if status != nil {
		result.SuccessKnown = true
		result.Successful = *status
	}
	return result, nil
}

// ParseCancelShipmentResponse is a shorter alias used by workflow callers.
func ParseCancelShipmentResponse(data []byte) (CancellationResult, error) {
	return ParseShipmentCancellationResponse(data)
}

// ParsePickupResponse parses common Ground and Express pickup-create variants.
func ParsePickupResponse(data []byte) (PickupResult, error) {
	root, err := decodeObject(data)
	if err != nil {
		return PickupResult{}, err
	}
	result := PickupResult{
		TransactionID:         firstString(root, "transactionId"),
		CustomerTransactionID: firstString(root, "customerTransactionId"),
		ConfirmationNumber:    firstString(root, "pickupConfirmationCode", "confirmationNumber", "pickupConfirmationNumber"),
		CarrierCode:           firstString(root, "carrierCode"),
		ScheduledDate:         firstString(root, "scheduledDate", "pickupDate"),
		LocationCode:          firstString(root, "location", "locationCode"),
		Message:               firstString(root, "message"),
	}
	if result.ConfirmationNumber == "" {
		return PickupResult{}, ErrConfirmationMissing
	}
	return result, nil
}

// ParsePickupCancellationResponse parses common Ground and Express
// pickup-cancel variants.
func ParsePickupCancellationResponse(data []byte) (PickupCancellationResult, error) {
	var envelope pickupCancellationEnvelope
	if err := decodeOne(data, &envelope); err != nil {
		return PickupCancellationResult{}, err
	}
	result := PickupCancellationResult{
		TransactionID:         envelope.TransactionID,
		CustomerTransactionID: envelope.CustomerTransactionID,
		ConfirmationNumber:    strings.TrimSpace(firstNonEmpty(envelope.Output.PickupConfirmationCode, envelope.PickupConfirmationCode)),
		LocationCode:          strings.TrimSpace(firstNonEmpty(envelope.Output.Location, envelope.Location)),
	}
	status := envelope.Output.CancelledPickup
	if status == nil {
		status = envelope.CancelledPickup
	}
	if status != nil {
		result.SuccessKnown = true
		result.Successful = *status
		return result, nil
	}
	confirmationMessage := strings.TrimSpace(firstNonEmpty(envelope.Output.CancelConfirmationMessage, envelope.CancelConfirmationMessage))
	if result.ConfirmationNumber != "" && confirmationMessage != "" {
		result.SuccessKnown = true
		result.Successful = true
	}
	return result, nil
}

// ParseCancelPickupResponse is a shorter alias used by workflow callers.
func ParseCancelPickupResponse(data []byte) (PickupCancellationResult, error) {
	return ParsePickupCancellationResponse(data)
}

// ParseResponse dispatches to the typed parser for a workflow action.
func ParseResponse(action string, data []byte) (any, error) {
	switch action {
	case ActionCreateLabel:
		return ParseLabelResponse(data)
	case ActionCancelShipment:
		return ParseShipmentCancellationResponse(data)
	case ActionSchedulePickup:
		return ParsePickupResponse(data)
	case ActionCancelPickup:
		return ParsePickupCancellationResponse(data)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedAction, action)
	}
}

func decodeOne(data []byte, destination any) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return fmt.Errorf("%w: empty body", ErrInvalidResponse)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("%w: multiple JSON values", ErrInvalidResponse)
		}
		return fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}
	return nil
}

func decodeObject(data []byte) (map[string]any, error) {
	var root map[string]any
	if err := decodeOne(data, &root); err != nil {
		return nil, err
	}
	if root == nil {
		return nil, fmt.Errorf("%w: expected JSON object", ErrInvalidResponse)
	}
	return root, nil
}

func preferredRate(rates []rateDetail) rateDetail {
	for _, rate := range rates {
		if strings.Contains(strings.ToUpper(rate.RateType), "ACCOUNT") {
			return rate
		}
	}
	return rates[0]
}

func isPDFLabelDocument(document labelDocument) bool {
	contentType := strings.ToUpper(strings.TrimSpace(document.ContentType))
	documentType := strings.ToUpper(strings.TrimSpace(document.DocumentType))
	hasLabelMarker := strings.Contains(contentType, "LABEL") || strings.Contains(documentType, "LABEL")
	hasPDFMarker := strings.Contains(contentType, "PDF") || strings.Contains(documentType, "PDF")
	return hasLabelMarker && hasPDFMarker
}

func firstNonZero(values ...float64) float64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func firstString(root any, keys ...string) string {
	for _, key := range keys {
		if value, ok := findScalar(root, key); ok {
			if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func findScalar(value any, target string) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if child, ok := typed[target]; ok {
			switch child.(type) {
			case string, bool, json.Number, float64:
				return child, true
			}
		}
		for _, child := range typed {
			if found, ok := findScalar(child, target); ok {
				return found, true
			}
		}
	case []any:
		for _, child := range typed {
			if found, ok := findScalar(child, target); ok {
				return found, true
			}
		}
	}
	return nil, false
}
