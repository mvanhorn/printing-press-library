// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/workflow"
)

func validateNarrowReadRequest(action string, body map[string]any) error {
	switch action {
	case "get_rates":
		if err := validateAccountObject(body, "accountNumber"); err != nil {
			return err
		}
		shipment, err := requiredObject(body, "requestedShipment")
		if err != nil {
			return err
		}
		for _, partyName := range []string{"shipper", "recipient"} {
			party, err := requiredObject(shipment, partyName)
			if err != nil {
				return err
			}
			address, err := requiredObject(party, "address")
			if err != nil {
				return err
			}
			if err := requireNonblankFields(address, "postalCode", "countryCode"); err != nil {
				return fmt.Errorf("requestedShipment.%s.address: %w", partyName, err)
			}
		}
		if err := requireEnum(shipment, "pickupType", map[string]bool{"CONTACT_FEDEX_TO_SCHEDULE": true, "DROPOFF_AT_FEDEX_LOCATION": true, "USE_SCHEDULED_PICKUP": true}); err != nil {
			return fmt.Errorf("requestedShipment: %w", err)
		}
		for _, field := range []string{"serviceType", "packagingType"} {
			if err := validateOptionalNonblankField(shipment, field); err != nil {
				return fmt.Errorf("requestedShipment: %w", err)
			}
		}
		if _, present := shipment["rateRequestType"]; present {
			if _, err := requiredStringArray(shipment, "rateRequestType", map[string]bool{"LIST": true, "INCENTIVE": true, "ACCOUNT": true, "PREFERRED": true}); err != nil {
				return fmt.Errorf("requestedShipment: %w", err)
			}
		}
		if _, present := body["carrierCodes"]; present {
			if _, err := requiredStringArray(body, "carrierCodes", map[string]bool{"FDXE": true, "FDXG": true, "FXSP": true, "FXCC": true}); err != nil {
				return err
			}
		}
		pickupDetailPresent := false
		if rawPickupDetail, present := shipment["pickupDetail"]; present {
			pickupDetail, ok := rawPickupDetail.(map[string]any)
			if !ok {
				return fmt.Errorf("requestedShipment.pickupDetail must be an object")
			}
			pickupDetailPresent = true
			if err := validateOptionalEnumField(pickupDetail, "requestType", map[string]bool{"FUTURE_DAY": true, "SAME_DAY": true}); err != nil {
				return fmt.Errorf("requestedShipment.pickupDetail: %w", err)
			}
			if err := validateOptionalEnumField(pickupDetail, "requestSource", map[string]bool{"AUTOMATION": true, "CUSTOMER_SERVICE": true}); err != nil {
				return fmt.Errorf("requestedShipment.pickupDetail: %w", err)
			}
			for _, field := range []string{"readyDateTime", "latestPickupDateTime"} {
				if err := validateOptionalDateField(pickupDetail, field); err != nil {
					return fmt.Errorf("requestedShipment.pickupDetail: %w", err)
				}
			}
			if err := validateOptionalNonblankField(pickupDetail, "courierInstructions"); err != nil {
				return fmt.Errorf("requestedShipment.pickupDetail: %w", err)
			}
			if pickupDetail["requestType"] == "FUTURE_DAY" {
				if err := requireNonblankFields(pickupDetail, "readyDateTime", "latestPickupDateTime"); err != nil {
					return fmt.Errorf("requestedShipment.pickupDetail: %w", err)
				}
			}
		}
		if _, present := body["processingOptions"]; present {
			if _, err := requiredStringArray(body, "processingOptions", map[string]bool{"INCLUDE_PICKUPRATES": true}); err != nil {
				return err
			}
			if !pickupDetailPresent {
				return fmt.Errorf("requestedShipment.pickupDetail is required with INCLUDE_PICKUPRATES")
			}
		}
		if rawControls, present := body["rateRequestControlParameters"]; present {
			controls, ok := rawControls.(map[string]any)
			if !ok {
				return fmt.Errorf("rateRequestControlParameters must be an object")
			}
			for _, field := range []string{"returnTransitTimes", "servicesNeededOnRateFailure"} {
				if value, present := controls[field]; present {
					if _, ok := value.(bool); !ok {
						return fmt.Errorf("rateRequestControlParameters.%s must be boolean", field)
					}
				}
			}
			if err := validateOptionalEnumField(controls, "variableOptions", map[string]bool{"SATURDAY_DELIVERY": true, "FREIGHT_GUARANTEE": true, "SMART_POST_ALLOWED_INDICIA": true, "SMARTPOST_HUB_ID": true}); err != nil {
				return fmt.Errorf("rateRequestControlParameters: %w", err)
			}
			if err := validateOptionalEnumField(controls, "rateSortOrder", map[string]bool{"COMMITASCENDING": true, "SERVICENAMETRADITIONAL": true, "COMMITDESCENDING": true}); err != nil {
				return fmt.Errorf("rateRequestControlParameters: %w", err)
			}
		}
		if count, present := shipment["totalPackageCount"]; present && !positiveIntegerAtMost(count, 100) {
			return fmt.Errorf("requestedShipment.totalPackageCount must be an integer from 1 through 100 when provided")
		}
		return validatePackages(shipment, false)
	case "validate_address":
		addresses, ok := anySlice(body["addressesToValidate"])
		if !ok || len(addresses) == 0 || len(addresses) > 100 {
			return fmt.Errorf("addressesToValidate must contain between 1 and 100 addresses")
		}
		for i, value := range addresses {
			entry, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("addressesToValidate[%d] must be an object", i)
			}
			address, ok := entry["address"].(map[string]any)
			if !ok || len(address) == 0 {
				return fmt.Errorf("addressesToValidate[%d].address must be a nonempty object", i)
			}
			if err := validateAddressResolutionAddress(address); err != nil {
				return fmt.Errorf("addressesToValidate[%d].address: %w", i, err)
			}
			if err := validateOptionalNonblankField(entry, "clientReferenceId"); err != nil {
				return fmt.Errorf("addressesToValidate[%d]: %w", i, err)
			}
		}
		if err := validateOptionalNonblankField(body, "inEffectAsOfTimestamp"); err != nil {
			return err
		}
		if rawControls, present := body["validateAddressControlParameters"]; present {
			controls, ok := rawControls.(map[string]any)
			if !ok {
				return fmt.Errorf("validateAddressControlParameters must be an object")
			}
			if value, present := controls["includeResolutionTokens"]; present {
				if _, ok := value.(bool); !ok {
					return fmt.Errorf("validateAddressControlParameters.includeResolutionTokens must be boolean")
				}
			}
		}
		return nil
	case "validate_shipment":
		if err := validateAccountObject(body, "accountNumber"); err != nil {
			return err
		}
		shipment, err := requiredObject(body, "requestedShipment")
		if err != nil {
			return err
		}
		shipper, err := requiredObject(shipment, "shipper")
		if err != nil {
			return err
		}
		if err := validateParty(shipper); err != nil {
			return fmt.Errorf("requestedShipment.shipper: %w", err)
		}
		recipients, ok := anySlice(shipment["recipients"])
		if !ok || len(recipients) != 1 {
			return fmt.Errorf("requestedShipment.recipients must contain exactly one recipient")
		}
		recipient, ok := recipients[0].(map[string]any)
		if !ok {
			return fmt.Errorf("requestedShipment.recipients[0] must be an object")
		}
		if err := validateParty(recipient); err != nil {
			return fmt.Errorf("requestedShipment.recipients[0]: %w", err)
		}
		if err := requireNonblankFields(shipment, "serviceType", "packagingType"); err != nil {
			return fmt.Errorf("requestedShipment: %w", err)
		}
		totalWeight, ok := numericValue(shipment["totalWeight"])
		if !ok || math.IsNaN(totalWeight) || math.IsInf(totalWeight, 0) || totalWeight < 1 || totalWeight != math.Trunc(totalWeight) {
			return fmt.Errorf("requestedShipment.totalWeight must be a positive integer")
		}
		if err := requireEnum(shipment, "pickupType", map[string]bool{"CONTACT_FEDEX_TO_SCHEDULE": true, "DROPOFF_AT_FEDEX_LOCATION": true, "USE_SCHEDULED_PICKUP": true}); err != nil {
			return fmt.Errorf("requestedShipment: %w", err)
		}
		payment, err := requiredObject(shipment, "shippingChargesPayment")
		if err != nil {
			return fmt.Errorf("requestedShipment: %w", err)
		}
		paymentTypes := map[string]bool{"SENDER": true, "RECIPIENT": true, "THIRD_PARTY": true, "COLLECT": true}
		if err := requireEnum(payment, "paymentType", paymentTypes); err != nil {
			return fmt.Errorf("requestedShipment.shippingChargesPayment: %w", err)
		}
		if payment["paymentType"] == "RECIPIENT" || payment["paymentType"] == "THIRD_PARTY" {
			payor, err := requiredObject(payment, "payor")
			if err != nil {
				return fmt.Errorf("requestedShipment.shippingChargesPayment: %w", err)
			}
			responsibleParty, err := requiredObject(payor, "responsibleParty")
			if err != nil {
				return fmt.Errorf("requestedShipment.shippingChargesPayment.payor: %w", err)
			}
			if err := validateAccountObject(responsibleParty, "accountNumber"); err != nil {
				return fmt.Errorf("requestedShipment.shippingChargesPayment.payor.responsibleParty: %w", err)
			}
		}
		label, err := requiredObject(shipment, "labelSpecification")
		if err != nil {
			return fmt.Errorf("requestedShipment: %w", err)
		}
		if err := requireEnum(label, "imageType", map[string]bool{"ZPLII": true, "EPL2": true, "PDF": true, "PNG": true}); err != nil {
			return fmt.Errorf("requestedShipment.labelSpecification: %w", err)
		}
		labelStockTypes := map[string]bool{
			"PAPER_4X6": true, "STOCK_4X675": true, "PAPER_4X675": true, "PAPER_4X8": true, "PAPER_4X9": true,
			"PAPER_7X475": true, "PAPER_85X11_BOTTOM_HALF_LABEL": true, "PAPER_85X11_TOP_HALF_LABEL": true, "PAPER_LETTER": true,
			"STOCK_4X675_LEADING_DOC_TAB": true, "STOCK_4X8": true, "STOCK_4X9_LEADING_DOC_TAB": true, "STOCK_4X6": true,
			"STOCK_4X675_TRAILING_DOC_TAB": true, "STOCK_4X9_TRAILING_DOC_TAB": true, "STOCK_4X9": true,
			"STOCK_4X85_TRAILING_DOC_TAB": true, "STOCK_4X105_TRAILING_DOC_TAB": true,
		}
		if err := requireEnum(label, "labelStockType", labelStockTypes); err != nil {
			return fmt.Errorf("requestedShipment.labelSpecification: %w", err)
		}
		if format, present := label["labelFormatType"]; present {
			if err := requireEnum(map[string]any{"labelFormatType": format}, "labelFormatType", map[string]bool{"COMMON2D": true, "LABEL_DATA_ONLY": true}); err != nil {
				return fmt.Errorf("requestedShipment.labelSpecification: %w", err)
			}
		}
		return validatePackages(shipment, true)
	case "pickup_availability":
		return workflow.ValidatePickupAvailabilityRequest(body)
	default:
		return fmt.Errorf("unsupported read operation %q", action)
	}
}

func requiredObject(value map[string]any, name string) (map[string]any, error) {
	object, ok := value[name].(map[string]any)
	if !ok || len(object) == 0 {
		return nil, fmt.Errorf("%s must be a nonempty object", name)
	}
	return object, nil
}

func validateAccountObject(value map[string]any, name string) error {
	account, err := requiredObject(value, name)
	if err != nil {
		return err
	}
	return requireNonblankFields(account, "value")
}

func requireNonblankFields(value map[string]any, names ...string) error {
	for _, name := range names {
		text, ok := value[name].(string)
		if !ok || strings.TrimSpace(text) == "" {
			return fmt.Errorf("%s must be a nonempty string", name)
		}
	}
	return nil
}

func validateAddressResolutionAddress(address map[string]any) error {
	if err := validateStreetLines(address); err != nil {
		return err
	}
	if err := requireNonblankFields(address, "countryCode"); err != nil {
		return err
	}
	for _, field := range []string{"city", "stateOrProvinceCode", "postalCode"} {
		if err := validateOptionalStringField(address, field); err != nil {
			return err
		}
	}
	if strings.EqualFold(strings.TrimSpace(address["countryCode"].(string)), "US") {
		postalCode, _ := address["postalCode"].(string)
		city, _ := address["city"].(string)
		state, _ := address["stateOrProvinceCode"].(string)
		if strings.TrimSpace(postalCode) == "" && (strings.TrimSpace(city) == "" || strings.TrimSpace(state) == "") {
			return fmt.Errorf("US addresses require postalCode or both city and stateOrProvinceCode")
		}
	}
	return nil
}

func validateShipmentAddress(address map[string]any) error {
	if err := validateStreetLines(address); err != nil {
		return err
	}
	if err := requireNonblankFields(address, "city", "countryCode"); err != nil {
		return err
	}
	for _, field := range []string{"stateOrProvinceCode", "postalCode"} {
		if err := validateOptionalStringField(address, field); err != nil {
			return err
		}
	}
	country := strings.ToUpper(strings.TrimSpace(address["countryCode"].(string)))
	if country == "US" || country == "CA" || country == "PR" {
		if err := requireNonblankFields(address, "stateOrProvinceCode", "postalCode"); err != nil {
			return err
		}
	}
	return nil
}

func validateStreetLines(address map[string]any) error {
	lines, ok := anySlice(address["streetLines"])
	if !ok || len(lines) == 0 {
		return fmt.Errorf("streetLines must contain at least one line")
	}
	for _, line := range lines {
		if text, ok := line.(string); !ok || strings.TrimSpace(text) == "" {
			return fmt.Errorf("streetLines must contain only nonempty strings")
		}
	}
	return nil
}

func validateOptionalNonblankField(value map[string]any, field string) error {
	if candidate, present := value[field]; present {
		text, ok := candidate.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return fmt.Errorf("%s must be a nonempty string when provided", field)
		}
	}
	return nil
}

func validateOptionalStringField(value map[string]any, field string) error {
	if candidate, present := value[field]; present {
		if _, ok := candidate.(string); !ok {
			return fmt.Errorf("%s must be a string when provided", field)
		}
	}
	return nil
}

func validateOptionalEnumField(value map[string]any, field string, allowed map[string]bool) error {
	if _, present := value[field]; !present {
		return nil
	}
	return requireEnum(value, field, allowed)
}

func validateOptionalDateField(value map[string]any, field string) error {
	raw, present := value[field]
	if !present {
		return nil
	}
	text, ok := raw.(string)
	if !ok || text != strings.TrimSpace(text) {
		return fmt.Errorf("%s must be a YYYY-MM-DD date when provided", field)
	}
	if _, err := time.Parse("2006-01-02", text); err != nil {
		return fmt.Errorf("%s must be a YYYY-MM-DD date when provided", field)
	}
	return nil
}

func validateParty(party map[string]any) error {
	contact, err := requiredObject(party, "contact")
	if err != nil {
		return err
	}
	if err := requireNonblankFields(contact, "phoneNumber"); err != nil {
		return fmt.Errorf("contact: %w", err)
	}
	personName, _ := contact["personName"].(string)
	companyName, _ := contact["companyName"].(string)
	if strings.TrimSpace(personName) == "" && strings.TrimSpace(companyName) == "" {
		return fmt.Errorf("contact requires a nonempty personName or companyName")
	}
	for _, field := range []string{"personName", "companyName"} {
		if err := validateOptionalNonblankField(contact, field); err != nil {
			return fmt.Errorf("contact: %w", err)
		}
	}
	address, err := requiredObject(party, "address")
	if err != nil {
		return err
	}
	return validateShipmentAddress(address)
}

func validatePackages(shipment map[string]any, exactlyOne bool) error {
	packages, ok := anySlice(shipment["requestedPackageLineItems"])
	if !ok || len(packages) == 0 || (!exactlyOne && len(packages) > 99) || (exactlyOne && len(packages) != 1) {
		if exactlyOne {
			return fmt.Errorf("requestedShipment.requestedPackageLineItems must contain exactly one package")
		}
		return fmt.Errorf("requestedShipment.requestedPackageLineItems must contain at least one package")
	}
	totalGroupedPackages := 0
	for index, value := range packages {
		pkg, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("requestedShipment.requestedPackageLineItems[%d] must be an object", index)
		}
		weight, err := requiredObject(pkg, "weight")
		if err != nil {
			return fmt.Errorf("requestedShipment.requestedPackageLineItems[%d]: %w", index, err)
		}
		if err := requireEnum(weight, "units", map[string]bool{"LB": true, "KG": true}); err != nil {
			return fmt.Errorf("requestedShipment.requestedPackageLineItems[%d].weight: %w", index, err)
		}
		number, ok := numericValue(weight["value"])
		if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number <= 0 {
			return fmt.Errorf("requestedShipment.requestedPackageLineItems[%d].weight.value must be greater than zero", index)
		}
		groupedPackages := 1
		if count, present := pkg["groupPackageCount"]; present {
			if exactlyOne && !numericOne(count) {
				return fmt.Errorf("requestedShipment.requestedPackageLineItems[%d].groupPackageCount must be the integer 1 when provided", index)
			}
			if !exactlyOne && !positiveIntegerAtMost(count, 100) {
				return fmt.Errorf("requestedShipment.requestedPackageLineItems[%d].groupPackageCount must be an integer from 1 through 100 when provided", index)
			}
			if !exactlyOne {
				groupCount, _ := numericValue(count)
				groupedPackages = int(groupCount)
			}
		}
		if !exactlyOne {
			totalGroupedPackages += groupedPackages
			if totalGroupedPackages > 100 {
				return fmt.Errorf("requestedShipment package groups exceed the 100-package rate limit")
			}
		}
		if sequence, present := pkg["sequenceNumber"]; exactlyOne && present && !numericOne(sequence) {
			return fmt.Errorf("requestedShipment.requestedPackageLineItems[%d].sequenceNumber must be the integer 1 when provided", index)
		}
	}
	if exactlyOne {
		if count, present := shipment["totalPackageCount"]; present && !numericOne(count) {
			return fmt.Errorf("requestedShipment.totalPackageCount must be the integer 1 when provided")
		}
	} else if count, present := shipment["totalPackageCount"]; present {
		totalCount, _ := numericValue(count)
		if int(totalCount) != totalGroupedPackages {
			return fmt.Errorf("requestedShipment.totalPackageCount must equal the sum of grouped package counts")
		}
	}
	return nil
}

func requiredStringArray(value map[string]any, name string, allowed map[string]bool) ([]string, error) {
	values, ok := anySlice(value[name])
	if !ok || len(values) == 0 {
		return nil, fmt.Errorf("%s must contain at least one string", name)
	}
	result := make([]string, 0, len(values))
	for _, item := range values {
		text, ok := item.(string)
		trimmed := strings.TrimSpace(text)
		if !ok || text != trimmed || trimmed == "" || (allowed != nil && !allowed[text]) {
			return nil, fmt.Errorf("%s contains an unsupported value", name)
		}
		result = append(result, text)
	}
	return result, nil
}

func requireEnum(value map[string]any, name string, allowed map[string]bool) error {
	text, ok := value[name].(string)
	if !ok || text != strings.TrimSpace(text) || !allowed[text] {
		return fmt.Errorf("%s contains an unsupported value", name)
	}
	return nil
}

func anySlice(value any) ([]any, bool) {
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

func numericValue(value any) (float64, bool) {
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

func numericOne(value any) bool {
	number, ok := numericValue(value)
	return ok && !math.IsNaN(number) && !math.IsInf(number, 0) && number == 1
}

func positiveIntegerAtMost(value any, maximum float64) bool {
	number, ok := numericValue(value)
	return ok && !math.IsNaN(number) && !math.IsInf(number, 0) && number >= 1 && number <= maximum && number == math.Trunc(number)
}

func minimizeNarrowReadResponse(action string, data []byte) (any, error) {
	switch action {
	case "get_rates":
		return minimizeRateResponse(data)
	case "validate_address":
		return minimizeAddressResponse(data)
	case "validate_shipment":
		return minimizeShipmentValidationResponse(data)
	case "pickup_availability":
		return minimizePickupAvailabilityResponse(data)
	default:
		return nil, fmt.Errorf("unsupported read operation %q", action)
	}
}

func minimizeRateResponse(data []byte) (any, error) {
	var response struct {
		Output struct {
			RateReplyDetails []struct {
				ServiceType       string `json:"serviceType"`
				OperationalDetail struct {
					TransitTime string `json:"transitTime"`
					DeliveryDay string `json:"deliveryDay"`
				} `json:"operationalDetail"`
				RatedShipmentDetails []struct {
					RateType            string  `json:"rateType"`
					TotalNetCharge      float64 `json:"totalNetCharge"`
					TotalBaseCharge     float64 `json:"totalBaseCharge"`
					TotalNetFedExCharge float64 `json:"totalNetFedExCharge"`
					Currency            string  `json:"currency"`
					ShipmentRateDetail  struct {
						RateType       string  `json:"rateType"`
						TotalNetCharge float64 `json:"totalNetCharge"`
						Currency       string  `json:"currency"`
					} `json:"shipmentRateDetail"`
				} `json:"ratedShipmentDetails"`
			} `json:"rateReplyDetails"`
		} `json:"output"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("FedEx rate response was not valid JSON: %w", err)
	}
	rates := make([]map[string]any, 0)
	for _, detail := range response.Output.RateReplyDetails {
		for _, rated := range detail.RatedShipmentDetails {
			rateType := rated.RateType
			if rateType == "" {
				rateType = rated.ShipmentRateDetail.RateType
			}
			netCharge := rated.TotalNetCharge
			if netCharge == 0 {
				netCharge = rated.ShipmentRateDetail.TotalNetCharge
			}
			currency := rated.Currency
			if currency == "" {
				currency = rated.ShipmentRateDetail.Currency
			}
			rates = append(rates, map[string]any{
				"service_type":           detail.ServiceType,
				"rate_type":              rateType,
				"total_net_charge":       netCharge,
				"total_base_charge":      rated.TotalBaseCharge,
				"total_net_fedex_charge": rated.TotalNetFedExCharge,
				"currency":               currency,
				"transit_time":           detail.OperationalDetail.TransitTime,
				"delivery_day":           detail.OperationalDetail.DeliveryDay,
			})
		}
	}
	return map[string]any{"rates": rates}, nil
}

func minimizeAddressResponse(data []byte) (any, error) {
	var response struct {
		Output struct {
			ResolvedAddresses []struct {
				Classification      string   `json:"classification"`
				StreetLines         []string `json:"streetLines"`
				StreetLinesToken    []string `json:"streetLinesToken"`
				City                string   `json:"city"`
				StateOrProvinceCode string   `json:"stateOrProvinceCode"`
				PostalCode          string   `json:"postalCode"`
				CountryCode         string   `json:"countryCode"`
			} `json:"resolvedAddresses"`
		} `json:"output"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("FedEx address response was not valid JSON: %w", err)
	}
	addresses := make([]map[string]any, 0, len(response.Output.ResolvedAddresses))
	for _, resolved := range response.Output.ResolvedAddresses {
		streetLines := resolved.StreetLines
		if len(streetLines) == 0 {
			streetLines = resolved.StreetLinesToken
		}
		addresses = append(addresses, map[string]any{
			"classification":         resolved.Classification,
			"street_lines":           streetLines,
			"city":                   resolved.City,
			"state_or_province_code": resolved.StateOrProvinceCode,
			"postal_code":            resolved.PostalCode,
			"country_code":           resolved.CountryCode,
		})
	}
	return map[string]any{"resolved_addresses": addresses}, nil
}

func minimizeShipmentValidationResponse(data []byte) (any, error) {
	var response struct {
		Output struct {
			Alerts []struct {
				Code      string `json:"code"`
				AlertType string `json:"alertType"`
			} `json:"alerts"`
		} `json:"output"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("FedEx shipment validation response was not valid JSON: %w", err)
	}
	if len(response.Output.Alerts) == 0 {
		return nil, fmt.Errorf("FedEx shipment validation response contained no validation evidence")
	}
	valid := false
	hasFailure := false
	alerts := make([]map[string]any, 0, len(response.Output.Alerts))
	for _, alert := range response.Output.Alerts {
		code := strings.TrimSpace(alert.Code)
		alertType := strings.ToUpper(strings.TrimSpace(alert.AlertType))
		if code == "" || alertType == "" {
			return nil, fmt.Errorf("FedEx shipment validation response contained an incomplete alert")
		}
		if strings.EqualFold(code, "SHIPMENT.VALIDATION.SUCCESS") {
			valid = true
		}
		if alertType == "ERROR" || alertType == "FAILURE" {
			hasFailure = true
		}
		alerts = append(alerts, map[string]any{"code": code, "alert_type": alertType})
	}
	if !valid && !hasFailure {
		return nil, fmt.Errorf("FedEx shipment validation response contained no explicit success or failure evidence")
	}
	return map[string]any{"valid": valid && !hasFailure, "alerts": alerts}, nil
}

func minimizePickupAvailabilityResponse(data []byte) (any, error) {
	type duration struct {
		Hours   int `json:"hours"`
		Minutes int `json:"minutes"`
	}
	var response struct {
		Output struct {
			Options []struct {
				Carrier                  string    `json:"carrier"`
				Available                *bool     `json:"available"`
				PickupDate               string    `json:"pickupDate"`
				CutoffTime               string    `json:"cutOffTime"`
				AccessTime               *duration `json:"accessTime"`
				ResidentialAvailable     *bool     `json:"residentialAvailable"`
				CountryRelationship      string    `json:"countryRelationship"`
				ScheduleDay              string    `json:"scheduleDay"`
				DefaultReadyTime         string    `json:"defaultReadyTime"`
				DefaultLatestTimeOptions string    `json:"defaultLatestTimeOptions"`
			} `json:"options"`
		} `json:"output"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("FedEx pickup availability response was not valid JSON: %w", err)
	}
	if len(response.Output.Options) == 0 {
		return nil, fmt.Errorf("FedEx pickup availability response contained no options")
	}
	options := make([]map[string]any, 0, len(response.Output.Options))
	for index, option := range response.Output.Options {
		carrier := strings.TrimSpace(option.Carrier)
		if carrier != "FDXE" && carrier != "FDXG" {
			return nil, fmt.Errorf("FedEx pickup availability option %d contained an unsupported carrier", index)
		}
		if option.Available == nil {
			return nil, fmt.Errorf("FedEx pickup availability option %d omitted availability", index)
		}
		if strings.TrimSpace(option.PickupDate) == "" {
			return nil, fmt.Errorf("FedEx pickup availability option %d omitted pickupDate", index)
		}
		result := map[string]any{
			"carrier":     carrier,
			"available":   *option.Available,
			"pickup_date": option.PickupDate,
		}
		if option.CutoffTime != "" {
			result["cutoff_time"] = option.CutoffTime
		}
		if option.AccessTime != nil {
			if option.AccessTime.Hours < 0 || option.AccessTime.Minutes < 0 || option.AccessTime.Minutes > 59 {
				return nil, fmt.Errorf("FedEx pickup availability option %d contained an invalid accessTime", index)
			}
			result["access_time"] = map[string]any{"hours": option.AccessTime.Hours, "minutes": option.AccessTime.Minutes}
		}
		if option.ResidentialAvailable != nil {
			result["residential_available"] = *option.ResidentialAvailable
		}
		for key, value := range map[string]string{
			"country_relationship":        option.CountryRelationship,
			"schedule_day":                option.ScheduleDay,
			"default_ready_time":          option.DefaultReadyTime,
			"default_latest_time_options": option.DefaultLatestTimeOptions,
		} {
			if strings.TrimSpace(value) != "" {
				result[key] = value
			}
		}
		options = append(options, result)
	}
	return map[string]any{"options": options}, nil
}
