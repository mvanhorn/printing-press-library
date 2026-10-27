// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package workflow

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

var (
	pickupAvailabilityCarriers = map[string]bool{"FDXE": true, "FDXG": true}
	pickupRequestTypes         = map[string]bool{"SAME_DAY": true, "FUTURE_DAY": true}
	pickupCountryRelationships = map[string]bool{"DOMESTIC": true, "INTERNATIONAL": true}
)

// ValidatePickupAvailabilityRequest validates the official FedEx pickup
// availability request shape. It is shared by the read tool and pickup
// scheduling preflight so malformed nested data cannot reach FedEx.
func ValidatePickupAvailabilityRequest(request map[string]any) error {
	address, err := availabilityObject(request, "pickupAddress")
	if err != nil {
		return err
	}
	// The pickup-availability PickupAddress overlay requires both fields even
	// though the reusable Address model describes postal codes conditionally.
	if err := availabilityNonblankFields(address, "postalCode", "countryCode"); err != nil {
		return fmt.Errorf("pickupAddress: %w", err)
	}
	if countryCode := strings.TrimSpace(address["countryCode"].(string)); len(countryCode) != 2 {
		return fmt.Errorf("pickupAddress.countryCode must contain exactly two characters")
	}
	if linesValue, present := address["streetLines"]; present {
		lines, ok := availabilitySlice(linesValue)
		if !ok || len(lines) == 0 {
			return fmt.Errorf("pickupAddress.streetLines must be a nonempty array when provided")
		}
		for _, line := range lines {
			text, ok := line.(string)
			length := len(strings.TrimSpace(text))
			if !ok || length < 3 || length > 35 {
				return fmt.Errorf("pickupAddress.streetLines must contain only strings between 3 and 35 characters")
			}
		}
	}
	for _, field := range []string{"urbanizationCode", "city", "stateOrProvinceCode"} {
		if err := availabilityOptionalNonblank(address, field); err != nil {
			return fmt.Errorf("pickupAddress: %w", err)
		}
	}
	if state, present := address["stateOrProvinceCode"]; present && len(strings.TrimSpace(state.(string))) > 2 {
		return fmt.Errorf("pickupAddress.stateOrProvinceCode must contain no more than two characters")
	}
	if value, present := address["residential"]; present {
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("pickupAddress.residential must be boolean when provided")
		}
	}
	if value, present := address["addressClassification"]; present {
		if err := availabilityEnum(map[string]any{"addressClassification": value}, "addressClassification", map[string]bool{"MIXED": true, "UNKNOWN": true, "BUSINESS": true, "RESIDENTIAL": true}); err != nil {
			return fmt.Errorf("pickupAddress: %w", err)
		}
	}
	if _, err := availabilityStringArray(request, "pickupRequestType", pickupRequestTypes); err != nil {
		return err
	}
	if _, err := availabilityStringArray(request, "carriers", pickupAvailabilityCarriers); err != nil {
		return err
	}
	if err := availabilityEnum(request, "countryRelationship", pickupCountryRelationships); err != nil {
		return err
	}

	for _, field := range []string{"dispatchDate", "packageReadyTime", "customerCloseTime", "associatedAccountNumber"} {
		if err := availabilityOptionalNonblank(request, field); err != nil {
			return err
		}
	}
	if value, present := request["dispatchDate"]; present {
		if _, err := time.Parse("2006-01-02", value.(string)); err != nil {
			return fmt.Errorf("dispatchDate must use YYYY-MM-DD")
		}
	}
	for _, field := range []string{"packageReadyTime", "customerCloseTime"} {
		if value, present := request[field]; present {
			if _, err := time.Parse("15:04:05", value.(string)); err != nil {
				return fmt.Errorf("%s must use HH:MM:SS", field)
			}
		}
	}
	if value, present := request["pickupType"]; present {
		if err := availabilityEnum(map[string]any{"pickupType": value}, "pickupType", map[string]bool{"ON_CALL": true, "TAG": true}); err != nil {
			return err
		}
	}
	if value, present := request["associatedAccountNumberType"]; present {
		if err := availabilityEnum(map[string]any{"associatedAccountNumberType": value}, "associatedAccountNumberType", map[string]bool{"FEDEX_EXPRESS": true, "FEDEX_GROUND": true}); err != nil {
			return err
		}
	}
	if value, present := request["numberOfBusinessDays"]; present {
		number, ok := availabilityNumber(value)
		if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number < 0 || math.Trunc(number) != number {
			return fmt.Errorf("numberOfBusinessDays must be a nonnegative integer when provided")
		}
	}
	if value, present := request["shipmentAttributes"]; present {
		attributes, ok := value.(map[string]any)
		if !ok || len(attributes) == 0 {
			return fmt.Errorf("shipmentAttributes must be a nonempty object when provided")
		}
		if err := availabilityNonblankFields(attributes, "serviceType"); err != nil {
			return fmt.Errorf("shipmentAttributes: %w", err)
		}
		if err := availabilityOptionalNonblank(attributes, "packagingType"); err != nil {
			return fmt.Errorf("shipmentAttributes: %w", err)
		}
		if packagingType, present := attributes["packagingType"]; present && strings.TrimSpace(packagingType.(string)) == "YOUR_PACKAGING" {
			if _, present := attributes["dimensions"]; !present {
				return fmt.Errorf("shipmentAttributes.dimensions is required when packagingType is YOUR_PACKAGING")
			}
		}
		if weightValue, present := attributes["weight"]; present {
			weight, ok := weightValue.(map[string]any)
			if !ok || len(weight) == 0 {
				return fmt.Errorf("shipmentAttributes.weight must be a nonempty object when provided")
			}
			if err := availabilityEnum(weight, "units", map[string]bool{"LB": true, "KG": true}); err != nil {
				return fmt.Errorf("shipmentAttributes.weight: %w", err)
			}
			number, ok := availabilityNumber(weight["value"])
			if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number <= 0 || number > 99999 {
				return fmt.Errorf("shipmentAttributes.weight.value must be greater than zero and no more than 99999")
			}
		}
		if dimensionsValue, present := attributes["dimensions"]; present {
			dimensions, ok := dimensionsValue.(map[string]any)
			if !ok || len(dimensions) == 0 {
				return fmt.Errorf("shipmentAttributes.dimensions must be a nonempty object when provided")
			}
			if value, present := dimensions["units"]; present && value != nil {
				units, ok := value.(string)
				if !ok || (units != "" && units != "CM" && units != "IN") {
					return fmt.Errorf("shipmentAttributes.dimensions.units must be CM, IN, blank, null, or omitted")
				}
			}
			for _, field := range []string{"length", "width", "height"} {
				number, ok := availabilityNumber(dimensions[field])
				if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number < 1 || number > 999 || math.Trunc(number) != number {
					return fmt.Errorf("shipmentAttributes.dimensions.%s must be an integer between 1 and 999", field)
				}
			}
		}
	}
	if value, present := request["packageDetails"]; present {
		packages, ok := availabilitySlice(value)
		if !ok || len(packages) == 0 {
			return fmt.Errorf("packageDetails must be a nonempty array when provided")
		}
		for index, item := range packages {
			pkg, ok := item.(map[string]any)
			if !ok || len(pkg) == 0 {
				return fmt.Errorf("packageDetails[%d] must be a nonempty object", index)
			}
			services, err := availabilityObject(pkg, "packageSpecialServices")
			if err != nil {
				return fmt.Errorf("packageDetails[%d]: %w", index, err)
			}
			if _, err := availabilityStringArray(services, "specialServiceTypes", nil); err != nil {
				return fmt.Errorf("packageDetails[%d].packageSpecialServices: %w", index, err)
			}
		}
	}
	return nil
}

func availabilityObject(value map[string]any, field string) (map[string]any, error) {
	object, ok := value[field].(map[string]any)
	if !ok || len(object) == 0 {
		return nil, fmt.Errorf("%s must be a nonempty object", field)
	}
	return object, nil
}

func availabilityNonblankFields(value map[string]any, fields ...string) error {
	for _, field := range fields {
		text, ok := value[field].(string)
		if !ok || strings.TrimSpace(text) == "" {
			return fmt.Errorf("%s must be a nonempty string", field)
		}
	}
	return nil
}

func availabilityOptionalNonblank(value map[string]any, field string) error {
	if candidate, present := value[field]; present {
		text, ok := candidate.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return fmt.Errorf("%s must be a nonempty string when provided", field)
		}
	}
	return nil
}

func availabilityEnum(value map[string]any, field string, allowed map[string]bool) error {
	text, ok := value[field].(string)
	if !ok || text != strings.TrimSpace(text) || !allowed[text] {
		return fmt.Errorf("%s contains an unsupported value", field)
	}
	return nil
}

func availabilityStringArray(value map[string]any, field string, allowed map[string]bool) ([]string, error) {
	values, ok := availabilitySlice(value[field])
	if !ok || len(values) == 0 {
		return nil, fmt.Errorf("%s must contain at least one string", field)
	}
	result := make([]string, 0, len(values))
	for _, item := range values {
		text, ok := item.(string)
		trimmed := strings.TrimSpace(text)
		if !ok || text != trimmed || trimmed == "" || (allowed != nil && !allowed[text]) {
			return nil, fmt.Errorf("%s contains an unsupported value", field)
		}
		result = append(result, text)
	}
	return result, nil
}

func availabilitySlice(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case []string:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = item
		}
		return result, true
	default:
		return nil, false
	}
}

func availabilityNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}
