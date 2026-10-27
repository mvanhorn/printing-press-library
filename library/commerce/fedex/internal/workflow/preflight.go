// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const PickupAvailabilityPath = "/pickup/v1/pickups/availabilities"

type pickupAvailabilityClient interface {
	Post(path string, body any) (json.RawMessage, int, error)
}

// PickupPreflight is approval-bound context for a pickup creation. Context may
// be hashed with the mutation but must not be sent to FedEx's creation endpoint.
type PickupPreflight struct {
	Context        any
	Status         string
	OverrideReason string
	Window         string
	CutoffTime     string
	AccessStart    string
}

// ValidatePickupAvailabilityBinding ensures that a successful availability
// response applies to the exact pickup about to be created.
func ValidatePickupAvailabilityBinding(scheduleRequest, availabilityRequest map[string]any) error {
	if err := ValidateRequest(ActionSchedulePickup, scheduleRequest); err != nil {
		return err
	}
	if err := ValidatePickupAvailabilityRequest(availabilityRequest); err != nil {
		return fmt.Errorf("invalid pickup availability request: %w", err)
	}
	account := strings.TrimSpace(stringField(availabilityRequest, "associatedAccountNumber"))
	wantAccount := strings.TrimSpace(nestedString(scheduleRequest, "associatedAccountNumber", "value"))
	if account == "" || account != wantAccount {
		return fmt.Errorf("availability associatedAccountNumber must match associatedAccountNumber.value in the pickup request")
	}
	carrier := strings.TrimSpace(stringField(scheduleRequest, "carrierCode"))
	carriers := stringList(availabilityRequest["carriers"])
	if len(carriers) != 1 || carriers[0] != carrier {
		return fmt.Errorf("availability carriers must contain only pickup carrierCode %s", carrier)
	}
	requestTypes := stringList(availabilityRequest["pickupRequestType"])
	if len(requestTypes) != 1 || (requestTypes[0] != "SAME_DAY" && requestTypes[0] != "FUTURE_DAY") {
		return fmt.Errorf("availability pickupRequestType must contain exactly one supported request type")
	}
	countryRelationship := strings.TrimSpace(stringField(availabilityRequest, "countryRelationship"))
	if countryRelationship != "DOMESTIC" && countryRelationship != "INTERNATIONAL" {
		return fmt.Errorf("availability countryRelationship must be DOMESTIC or INTERNATIONAL")
	}
	ready := strings.TrimSpace(nestedString(scheduleRequest, "originDetail", "readyDateTimestamp"))
	readyTime, err := time.Parse(time.RFC3339, ready)
	if err != nil {
		return fmt.Errorf("originDetail.readyDateTimestamp must be RFC3339 for availability binding: %w", err)
	}
	if strings.TrimSpace(stringField(availabilityRequest, "dispatchDate")) != readyTime.Format("2006-01-02") {
		return fmt.Errorf("availability dispatchDate must match the pickup ready date")
	}
	packageReady := strings.TrimSpace(stringField(availabilityRequest, "packageReadyTime"))
	if packageReady == "" || !strings.HasPrefix(readyTime.Format("15:04:05"), packageReady) {
		return fmt.Errorf("availability packageReadyTime must match the pickup ready time")
	}
	wantClose := strings.TrimSpace(nestedString(scheduleRequest, "originDetail", "customerCloseTime"))
	if strings.TrimSpace(stringField(availabilityRequest, "customerCloseTime")) != wantClose {
		return fmt.Errorf("availability customerCloseTime must match the pickup request")
	}
	availabilityAddress, ok := objectAt(availabilityRequest, "pickupAddress")
	if !ok {
		return fmt.Errorf("availability pickupAddress is required")
	}
	pickupAddress, ok := objectAt(scheduleRequest, "originDetail", "pickupLocation", "address")
	if !ok {
		return fmt.Errorf("originDetail.pickupLocation.address is required")
	}
	for _, field := range []string{"postalCode", "countryCode", "streetLines", "urbanizationCode", "city", "stateOrProvinceCode", "residential", "addressClassification"} {
		availabilityValue, supplied := availabilityAddress[field]
		if !supplied {
			continue
		}
		pickupValue, present := pickupAddress[field]
		if !present {
			return fmt.Errorf("availability pickupAddress.%s must match originDetail.pickupLocation.address.%s", field, field)
		}
		left, _ := json.Marshal(availabilityValue)
		right, _ := json.Marshal(pickupValue)
		if !bytes.Equal(left, right) {
			return fmt.Errorf("availability pickupAddress.%s must match originDetail.pickupLocation.address.%s", field, field)
		}
	}
	return nil
}

// PreparePickupPreflight requires either a successful availability request or
// a documented override. Confirmation calls re-bind the same context but do not
// repeat the preflight network request that was completed during preview.
func PreparePickupPreflight(client pickupAvailabilityClient, confirming bool, availabilityRequest map[string]any, overrideReason string) (PickupPreflight, error) {
	overrideReason = strings.TrimSpace(overrideReason)
	if len(availabilityRequest) > 0 && overrideReason != "" {
		return PickupPreflight{}, fmt.Errorf("use either an availability request or an override reason, not both")
	}
	if len(availabilityRequest) == 0 && overrideReason == "" {
		return PickupPreflight{}, fmt.Errorf("pickup scheduling requires an availability request or a documented override reason")
	}
	if overrideReason != "" {
		if len(overrideReason) < 10 {
			return PickupPreflight{}, fmt.Errorf("availability override reason must contain at least 10 characters")
		}
		return PickupPreflight{Context: map[string]any{"pickup_preflight": "overridden", "reason": overrideReason}, Status: "overridden", OverrideReason: overrideReason}, nil
	}
	result := PickupPreflight{Context: map[string]any{"pickup_preflight": "verified", "request": availabilityRequest}, Status: "verified", Window: "verified during preview"}
	if confirming {
		return result, nil
	}
	data, _, err := client.Post(PickupAvailabilityPath, availabilityRequest)
	if err != nil {
		return PickupPreflight{}, fmt.Errorf("pickup availability preflight: %w", err)
	}
	carriers := stringList(availabilityRequest["carriers"])
	if len(carriers) != 1 {
		return PickupPreflight{}, fmt.Errorf("pickup availability preflight requires exactly one carrier")
	}
	dispatchDate := strings.TrimSpace(stringField(availabilityRequest, "dispatchDate"))
	if dispatchDate == "" {
		return PickupPreflight{}, fmt.Errorf("pickup availability preflight requires dispatchDate")
	}
	option, evidenceErr := matchingPickupAvailability(data, carriers[0], dispatchDate)
	if evidenceErr != nil {
		return PickupPreflight{}, evidenceErr
	}
	if !option.Available {
		return PickupPreflight{}, fmt.Errorf("FedEx reported that pickup is unavailable for the requested window")
	}
	result.CutoffTime = option.CutoffTime
	result.AccessStart = option.AccessTime
	parts := make([]string, 0, 2)
	if result.CutoffTime != "" {
		parts = append(parts, "cutoff="+result.CutoffTime)
	}
	if result.AccessStart != "" {
		parts = append(parts, "access="+result.AccessStart)
	}
	if len(parts) > 0 {
		result.Window = strings.Join(parts, ", ")
	}
	return result, nil
}

type pickupAvailabilityOption struct {
	Available  bool
	CutoffTime string
	AccessTime string
}

func matchingPickupAvailability(data []byte, carrier, dispatchDate string) (pickupAvailabilityOption, error) {
	type duration struct {
		Hours   int `json:"hours"`
		Minutes int `json:"minutes"`
	}
	var response struct {
		Output struct {
			Options []struct {
				Carrier    string `json:"carrier"`
				Available  *bool  `json:"available"`
				PickupDate string `json:"pickupDate"`

				CutoffTime string    `json:"cutOffTime"`
				AccessTime *duration `json:"accessTime"`
			} `json:"options"`
		} `json:"output"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return pickupAvailabilityOption{}, fmt.Errorf("pickup availability response is not valid JSON")
	}
	matches := make([]pickupAvailabilityOption, 0, 1)
	for _, option := range response.Output.Options {
		if strings.TrimSpace(option.Carrier) != carrier || strings.TrimSpace(option.PickupDate) != dispatchDate {
			continue
		}
		if option.Available == nil {
			return pickupAvailabilityOption{}, fmt.Errorf("matching pickup availability option omitted availability")
		}
		accessTime := ""
		if option.AccessTime != nil {
			if option.AccessTime.Hours < 0 || option.AccessTime.Minutes < 0 || option.AccessTime.Minutes > 59 {
				return pickupAvailabilityOption{}, fmt.Errorf("matching pickup availability option contained an invalid accessTime")
			}
			accessTime = fmt.Sprintf("%dh%02dm", option.AccessTime.Hours, option.AccessTime.Minutes)
		}
		matches = append(matches, pickupAvailabilityOption{Available: *option.Available, CutoffTime: strings.TrimSpace(option.CutoffTime), AccessTime: accessTime})
	}
	if len(matches) != 1 {
		return pickupAvailabilityOption{}, fmt.Errorf("FedEx availability response must contain exactly one option matching carrier %s and date %s", carrier, dispatchDate)
	}
	return matches[0], nil
}

func nestedString(value map[string]any, path ...string) string {
	current := any(value)
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = object[key]
	}
	text, _ := current.(string)
	return text
}

func objectAt(value map[string]any, path ...string) (map[string]any, bool) {
	current := any(value)
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current = object[key]
	}
	object, ok := current.(map[string]any)
	return object, ok
}

func stringList(value any) []string {
	var result []string
	switch values := value.(type) {
	case []any:
		for _, value := range values {
			if text, ok := value.(string); ok {
				result = append(result, strings.TrimSpace(text))
			}
		}
	case []string:
		for _, value := range values {
			result = append(result, strings.TrimSpace(value))
		}
	}
	return result
}
