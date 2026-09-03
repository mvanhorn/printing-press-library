// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/approval"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/workflow"
)

const pendingOperationTTL = 10 * time.Minute

type narrowOperation struct {
	name        string
	action      string
	method      string
	path        string
	readOnly    bool
	destructive bool
	idempotent  bool
	description string
}

var narrowOperations = []narrowOperation{
	{name: "get_rates", action: "get_rates", method: "POST", path: "/rate/v1/rates/quotes", readOnly: true, idempotent: true, description: "Get FedEx rate quotes for a shipment request. This calls FedEx but does not create a shipment."},
	{name: "validate_address", action: "validate_address", method: "POST", path: "/address/v1/addresses/resolve", readOnly: true, description: "Validate and standardize an address with FedEx. This may be billable, is never retried automatically, and does not create a shipment."},
	{name: "validate_shipment", action: "validate_shipment", method: "POST", path: "/ship/v1/shipments/packages/validate", readOnly: true, idempotent: true, description: "Validate a single-package shipment request without creating a shipment or label."},
	{name: "pickup_availability", action: "pickup_availability", method: "POST", path: "/pickup/v1/pickups/availabilities", readOnly: true, idempotent: true, description: "Check FedEx Express or Ground pickup availability without scheduling a pickup."},
	{name: "create_label", action: "create_label", method: "POST", path: "/ship/v1/shipments", description: "Preview or, after bound confirmation, create one FedEx shipment, persist its tracking ledger entry, and write one private PDF label."},
	{name: "cancel_shipment", action: "cancel_shipment", method: "PUT", path: "/ship/v1/shipments/cancel", destructive: true, description: "Preview or, after bound confirmation, cancel one FedEx shipment. First call returns a pending operation; confirmation must repeat the exact request."},
	{name: "schedule_pickup", action: "schedule_pickup", method: "POST", path: "/pickup/v1/pickups", description: "Check availability, then preview or, after bound confirmation, schedule and persist one FedEx pickup."},
	{name: "cancel_pickup", action: "cancel_pickup", method: "PUT", path: "/pickup/v1/pickups/cancel", destructive: true, description: "Preview or, after bound confirmation, cancel one FedEx pickup. First call returns a pending operation; confirmation must repeat the exact request."},
}

func registerNarrowTools(s *server.MCPServer) {
	for _, operation := range narrowOperations {
		op := operation
		options := []mcplib.ToolOption{
			mcplib.WithDescription(op.description),
			mcplib.WithObject("request", mcplib.Required(), mcplib.Description("Exact FedEx REST request object for this operation."), mcplib.Properties(requestSchemaProperties(op.action))),
			mcplib.WithReadOnlyHintAnnotation(op.readOnly),
			mcplib.WithDestructiveHintAnnotation(op.destructive),
			mcplib.WithIdempotentHintAnnotation(op.idempotent),
			mcplib.WithOpenWorldHintAnnotation(true),
		}
		if !op.readOnly {
			options = append(options,
				mcplib.WithBoolean("confirm", mcplib.Description("Set true only after reviewing the pending operation summary.")),
				mcplib.WithString("operation_id", mcplib.Description("Pending operation ID returned by the preview call.")),
				mcplib.WithString("confirmation_digest", mcplib.Description("SHA-256 confirmation digest returned by the preview call.")),
			)
		}
		if op.action == workflow.ActionSchedulePickup {
			options = append(options,
				mcplib.WithObject("availability_request", mcplib.Description("FedEx pickup availability request verified during preview; repeat it unchanged for confirmation.")),
				mcplib.WithString("availability_override_reason", mcplib.Description("Documented reason to bypass availability preflight when an availability request cannot be used.")),
			)
		}
		if op.action == workflow.ActionCancelPickup {
			options = append(options, mcplib.WithString("legacy_reason", mcplib.Description("Required reason when cancelling a pickup not found in the local ledger.")))
		}
		handler := makeNarrowReadHandler(op)
		if !op.readOnly {
			handler = makeNarrowMutationHandler(op)
		}
		tool := mcplib.NewTool(op.name, options...)
		if requestSchema, ok := tool.InputSchema.Properties["request"].(map[string]any); ok {
			requestSchema["required"] = requestSchemaRequired(op.action)
		}
		if op.action == workflow.ActionSchedulePickup {
			if availabilitySchema, ok := tool.InputSchema.Properties["availability_request"].(map[string]any); ok {
				availabilitySchema["properties"] = requestSchemaProperties("pickup_availability")
				availabilitySchema["required"] = requestSchemaRequired("pickup_availability")
			}
		}
		s.AddTool(tool, handler)
	}
}

func requestSchemaRequired(action string) []string {
	switch action {
	case "get_rates":
		return []string{"accountNumber", "requestedShipment"}
	case "validate_address":
		return []string{"addressesToValidate"}
	case "validate_shipment":
		return []string{"accountNumber", "requestedShipment"}
	case "pickup_availability":
		return []string{"pickupAddress", "pickupRequestType", "carriers", "countryRelationship"}
	case workflow.ActionCreateLabel:
		return []string{"labelResponseOptions", "accountNumber", "requestedShipment"}
	case workflow.ActionCancelShipment:
		return []string{"accountNumber", "senderCountryCode", "trackingNumber", "deletionControl"}
	case workflow.ActionSchedulePickup:
		return []string{"associatedAccountNumber", "originDetail", "totalWeight", "packageCount", "carrierCode"}
	case workflow.ActionCancelPickup:
		return []string{"pickupConfirmationCode"}
	default:
		return nil
	}
}

func requestSchemaProperties(action string) map[string]any {
	account := map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"type": "string"}}, "required": []string{"value"}}
	addressProperties := map[string]any{"streetLines": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string"}}, "city": map[string]any{"type": "string"}, "stateOrProvinceCode": map[string]any{"type": "string"}, "postalCode": map[string]any{"type": "string"}, "countryCode": map[string]any{"type": "string"}}
	addressValidationAddress := map[string]any{"type": "object", "properties": addressProperties, "required": []string{"streetLines", "countryCode"}, "allOf": []any{map[string]any{"if": map[string]any{"properties": map[string]any{"countryCode": map[string]any{"enum": []string{"US"}}}}, "then": map[string]any{"anyOf": []any{map[string]any{"properties": map[string]any{"postalCode": map[string]any{"type": "string", "minLength": 1, "pattern": "\\S"}}, "required": []string{"postalCode"}}, map[string]any{"properties": map[string]any{"city": map[string]any{"type": "string", "minLength": 1, "pattern": "\\S"}, "stateOrProvinceCode": map[string]any{"type": "string", "minLength": 1, "pattern": "\\S"}}, "required": []string{"city", "stateOrProvinceCode"}}}}}}}
	shipmentAddress := map[string]any{"type": "object", "properties": addressProperties, "required": []string{"streetLines", "city", "countryCode"}, "allOf": []any{map[string]any{"if": map[string]any{"properties": map[string]any{"countryCode": map[string]any{"enum": []string{"US", "CA", "PR"}}}}, "then": map[string]any{"properties": map[string]any{"stateOrProvinceCode": map[string]any{"type": "string", "minLength": 1, "pattern": "\\S"}, "postalCode": map[string]any{"type": "string", "minLength": 1, "pattern": "\\S"}}, "required": []string{"stateOrProvinceCode", "postalCode"}}}}}
	operationalAddress := map[string]any{"type": "object", "properties": addressProperties, "required": []string{"streetLines", "city", "stateOrProvinceCode", "postalCode", "countryCode"}}
	pickupAvailabilityAddressProperties := map[string]any{
		"streetLines":           map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string", "minLength": 3, "maxLength": 35}},
		"urbanizationCode":      map[string]any{"type": "string"},
		"city":                  map[string]any{"type": "string"},
		"stateOrProvinceCode":   map[string]any{"type": "string", "maxLength": 2},
		"postalCode":            map[string]any{"type": "string"},
		"countryCode":           map[string]any{"type": "string", "minLength": 2, "maxLength": 2},
		"residential":           map[string]any{"type": "boolean"},
		"addressClassification": map[string]any{"type": "string", "enum": []string{"MIXED", "UNKNOWN", "BUSINESS", "RESIDENTIAL"}},
	}
	pickupAvailabilityAddress := map[string]any{"type": "object", "properties": pickupAvailabilityAddressProperties, "required": []string{"postalCode", "countryCode"}}
	contact := map[string]any{"type": "object", "properties": map[string]any{"personName": map[string]any{"type": "string"}, "companyName": map[string]any{"type": "string"}, "phoneNumber": map[string]any{"type": "string"}}, "required": []string{"phoneNumber"}, "anyOf": []any{map[string]any{"required": []string{"personName"}}, map[string]any{"required": []string{"companyName"}}}}
	party := map[string]any{"type": "object", "properties": map[string]any{"contact": contact, "address": operationalAddress}, "required": []string{"contact", "address"}}
	shipmentValidationParty := map[string]any{"type": "object", "properties": map[string]any{"contact": contact, "address": shipmentAddress}, "required": []string{"contact", "address"}}
	rateAddress := map[string]any{"type": "object", "properties": map[string]any{"postalCode": map[string]any{"type": "string"}, "countryCode": map[string]any{"type": "string"}}, "required": []string{"postalCode", "countryCode"}}
	rateControls := map[string]any{"type": "object", "properties": map[string]any{
		"returnTransitTimes":          map[string]any{"type": "boolean"},
		"servicesNeededOnRateFailure": map[string]any{"type": "boolean"},
		"variableOptions":             map[string]any{"type": "string", "enum": []string{"SATURDAY_DELIVERY", "FREIGHT_GUARANTEE", "SMART_POST_ALLOWED_INDICIA", "SMARTPOST_HUB_ID"}},
		"rateSortOrder":               map[string]any{"type": "string", "enum": []string{"COMMITASCENDING", "SERVICENAMETRADITIONAL", "COMMITDESCENDING"}},
	}}
	ratePickupDetail := map[string]any{"type": "object", "properties": map[string]any{
		"readyDateTime":        map[string]any{"type": "string"},
		"latestPickupDateTime": map[string]any{"type": "string"},
		"courierInstructions":  map[string]any{"type": "string"},
		"requestType":          map[string]any{"type": "string", "enum": []string{"FUTURE_DAY", "SAME_DAY"}},
		"requestSource":        map[string]any{"type": "string", "enum": []string{"AUTOMATION", "CUSTOMER_SERVICE"}},
	}, "allOf": []any{map[string]any{"if": map[string]any{"properties": map[string]any{"requestType": map[string]any{"enum": []string{"FUTURE_DAY"}}}, "required": []string{"requestType"}}, "then": map[string]any{"required": []string{"readyDateTime", "latestPickupDateTime"}}}}}
	weight := map[string]any{"type": "object", "properties": map[string]any{"units": map[string]any{"type": "string", "enum": []string{"LB", "KG"}}, "value": map[string]any{"type": "number", "exclusiveMinimum": 0}}, "required": []string{"units", "value"}}
	paymentPayor := map[string]any{"type": "object", "properties": map[string]any{"responsibleParty": map[string]any{"type": "object", "properties": map[string]any{"accountNumber": account}, "required": []string{"accountNumber"}}}, "required": []string{"responsibleParty"}}
	shippingPayment := map[string]any{"type": "object", "properties": map[string]any{"paymentType": map[string]any{"type": "string", "enum": []string{"SENDER", "RECIPIENT", "THIRD_PARTY", "COLLECT"}}, "payor": paymentPayor}, "required": []string{"paymentType"}, "allOf": []any{map[string]any{"if": map[string]any{"properties": map[string]any{"paymentType": map[string]any{"enum": []string{"RECIPIENT", "THIRD_PARTY"}}}}, "then": map[string]any{"required": []string{"payor"}}}}}
	dimensions := map[string]any{"type": "object", "properties": map[string]any{"length": map[string]any{"type": "integer", "minimum": 1, "maximum": 999}, "width": map[string]any{"type": "integer", "minimum": 1, "maximum": 999}, "height": map[string]any{"type": "integer", "minimum": 1, "maximum": 999}, "units": map[string]any{"enum": []any{"CM", "IN", "", nil}}}, "required": []string{"length", "width", "height"}}
	pickupShipmentAttributes := map[string]any{"type": "object", "properties": map[string]any{"serviceType": map[string]any{"type": "string"}, "weight": weight, "packagingType": map[string]any{"type": "string"}, "dimensions": dimensions}, "required": []string{"serviceType"}, "allOf": []any{map[string]any{"if": map[string]any{"properties": map[string]any{"packagingType": map[string]any{"enum": []string{"YOUR_PACKAGING"}}}, "required": []string{"packagingType"}}, "then": map[string]any{"required": []string{"dimensions"}}}}}
	pickupPackageDetails := map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "object", "properties": map[string]any{"packageSpecialServices": map[string]any{"type": "object", "properties": map[string]any{"specialServiceTypes": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string"}}}, "required": []string{"specialServiceTypes"}}}, "required": []string{"packageSpecialServices"}}}
	switch action {
	case "get_rates":
		return map[string]any{
			"accountNumber": account,
			"requestedShipment": map[string]any{"type": "object", "properties": map[string]any{
				"shipper":    map[string]any{"type": "object", "properties": map[string]any{"address": rateAddress}, "required": []string{"address"}},
				"recipient":  map[string]any{"type": "object", "properties": map[string]any{"address": rateAddress}, "required": []string{"address"}},
				"pickupType": map[string]any{"type": "string", "enum": []string{"CONTACT_FEDEX_TO_SCHEDULE", "DROPOFF_AT_FEDEX_LOCATION", "USE_SCHEDULED_PICKUP"}}, "serviceType": map[string]any{"type": "string"}, "packagingType": map[string]any{"type": "string"}, "pickupDetail": ratePickupDetail,
				"rateRequestType":           map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string", "enum": []string{"LIST", "INCENTIVE", "ACCOUNT", "PREFERRED"}}},
				"totalPackageCount":         map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
				"requestedPackageLineItems": map[string]any{"type": "array", "minItems": 1, "maxItems": 99, "items": map[string]any{"type": "object", "properties": map[string]any{"weight": weight, "groupPackageCount": map[string]any{"type": "integer", "minimum": 1, "maximum": 100}}, "required": []string{"weight"}}},
			}, "required": []string{"shipper", "recipient", "pickupType", "requestedPackageLineItems"}},
			"rateRequestControlParameters": rateControls,
			"processingOptions":            map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string", "enum": []string{"INCLUDE_PICKUPRATES"}}},
			"carrierCodes":                 map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string", "enum": []string{"FDXE", "FDXG", "FXSP", "FXCC"}}},
		}
	case "validate_address":
		return map[string]any{
			"addressesToValidate":              map[string]any{"type": "array", "minItems": 1, "maxItems": 100, "items": map[string]any{"type": "object", "properties": map[string]any{"address": addressValidationAddress, "clientReferenceId": map[string]any{"type": "string"}}, "required": []string{"address"}}},
			"inEffectAsOfTimestamp":            map[string]any{"type": "string"},
			"validateAddressControlParameters": map[string]any{"type": "object", "properties": map[string]any{"includeResolutionTokens": map[string]any{"type": "boolean"}}},
		}
	case "validate_shipment":
		return map[string]any{
			"accountNumber": account,
			"requestedShipment": map[string]any{"type": "object", "properties": map[string]any{
				"shipper": shipmentValidationParty, "recipients": map[string]any{"type": "array", "minItems": 1, "maxItems": 1, "items": shipmentValidationParty},
				"pickupType":  map[string]any{"type": "string", "enum": []string{"CONTACT_FEDEX_TO_SCHEDULE", "DROPOFF_AT_FEDEX_LOCATION", "USE_SCHEDULED_PICKUP"}},
				"serviceType": map[string]any{"type": "string"}, "packagingType": map[string]any{"type": "string"}, "totalWeight": map[string]any{"type": "integer", "minimum": 1},
				"shippingChargesPayment":    shippingPayment,
				"labelSpecification":        map[string]any{"type": "object", "properties": map[string]any{"imageType": map[string]any{"type": "string", "enum": []string{"ZPLII", "EPL2", "PDF", "PNG"}}, "labelStockType": map[string]any{"type": "string", "enum": []string{"PAPER_4X6", "STOCK_4X675", "PAPER_4X675", "PAPER_4X8", "PAPER_4X9", "PAPER_7X475", "PAPER_85X11_BOTTOM_HALF_LABEL", "PAPER_85X11_TOP_HALF_LABEL", "PAPER_LETTER", "STOCK_4X675_LEADING_DOC_TAB", "STOCK_4X8", "STOCK_4X9_LEADING_DOC_TAB", "STOCK_4X6", "STOCK_4X675_TRAILING_DOC_TAB", "STOCK_4X9_TRAILING_DOC_TAB", "STOCK_4X9", "STOCK_4X85_TRAILING_DOC_TAB", "STOCK_4X105_TRAILING_DOC_TAB"}}, "labelFormatType": map[string]any{"type": "string", "enum": []string{"COMMON2D", "LABEL_DATA_ONLY"}}}, "required": []string{"imageType", "labelStockType"}},
				"totalPackageCount":         map[string]any{"type": "integer", "enum": []int{1}},
				"requestedPackageLineItems": map[string]any{"type": "array", "minItems": 1, "maxItems": 1, "items": map[string]any{"type": "object", "properties": map[string]any{"weight": weight, "groupPackageCount": map[string]any{"type": "integer", "enum": []int{1}}, "sequenceNumber": map[string]any{"type": "integer", "enum": []int{1}}}, "required": []string{"weight"}}},
			}, "required": []string{"shipper", "recipients", "pickupType", "serviceType", "packagingType", "totalWeight", "shippingChargesPayment", "labelSpecification", "requestedPackageLineItems"}},
		}
	case "pickup_availability":
		return map[string]any{
			"pickupAddress":               pickupAvailabilityAddress,
			"dispatchDate":                map[string]any{"type": "string"},
			"packageReadyTime":            map[string]any{"type": "string"},
			"customerCloseTime":           map[string]any{"type": "string"},
			"pickupType":                  map[string]any{"type": "string", "enum": []string{"ON_CALL", "TAG"}},
			"pickupRequestType":           map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string", "enum": []string{"SAME_DAY", "FUTURE_DAY"}}},
			"shipmentAttributes":          pickupShipmentAttributes,
			"numberOfBusinessDays":        map[string]any{"type": "integer", "minimum": 0},
			"packageDetails":              pickupPackageDetails,
			"associatedAccountNumber":     map[string]any{"type": "string"},
			"associatedAccountNumberType": map[string]any{"type": "string", "enum": []string{"FEDEX_EXPRESS", "FEDEX_GROUND"}},
			"carriers":                    map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string", "enum": []string{"FDXE", "FDXG"}}},
			"countryRelationship":         map[string]any{"type": "string", "enum": []string{"DOMESTIC", "INTERNATIONAL"}},
		}
	case workflow.ActionCreateLabel:
		return map[string]any{
			"labelResponseOptions": map[string]any{"type": "string", "enum": []string{"LABEL"}},
			"accountNumber":        account,
			"requestedShipment": map[string]any{"type": "object", "properties": map[string]any{
				"shipper": party, "recipients": map[string]any{"type": "array", "minItems": 1, "maxItems": 1, "items": party},
				"serviceType": map[string]any{"type": "string"}, "packagingType": map[string]any{"type": "string"},
				"requestedPackageLineItems": map[string]any{"type": "array", "minItems": 1, "maxItems": 1, "items": map[string]any{"type": "object", "properties": map[string]any{"weight": weight, "groupPackageCount": map[string]any{"type": "integer", "enum": []int{1}}}, "required": []string{"weight"}}},
				"labelSpecification":        map[string]any{"type": "object", "properties": map[string]any{"imageType": map[string]any{"type": "string", "enum": []string{"PDF"}}, "labelStockType": map[string]any{"type": "string"}}, "required": []string{"imageType"}},
			}, "required": []string{"shipper", "recipients", "serviceType", "packagingType", "requestedPackageLineItems", "labelSpecification"}},
		}
	case workflow.ActionCancelShipment:
		return map[string]any{"accountNumber": account, "trackingNumber": map[string]any{"type": "string"}, "senderCountryCode": map[string]any{"type": "string"}, "deletionControl": map[string]any{"type": "string", "enum": []string{"DELETE_ALL_PACKAGES", "DELETE_ONE_PACKAGE"}}}
	case workflow.ActionSchedulePickup:
		return map[string]any{"associatedAccountNumber": account, "carrierCode": map[string]any{"type": "string", "enum": []string{"FDXE", "FDXG"}}, "packageCount": map[string]any{"type": "integer", "minimum": 1}, "totalWeight": weight, "originDetail": map[string]any{"type": "object", "properties": map[string]any{"pickupLocation": party, "readyDateTimestamp": map[string]any{"type": "string"}, "customerCloseTime": map[string]any{"type": "string"}}, "required": []string{"pickupLocation", "readyDateTimestamp", "customerCloseTime"}}}
	case workflow.ActionCancelPickup:
		return map[string]any{"pickupConfirmationCode": map[string]any{"type": "string"}, "associatedAccountNumber": account, "carrierCode": map[string]any{"type": "string", "enum": []string{"FDXE", "FDXG"}}, "scheduledDate": map[string]any{"type": "string"}, "location": map[string]any{"type": "string"}}
	default:
		return map[string]any{}
	}
}

func makeNarrowReadHandler(operation narrowOperation) server.ToolHandlerFunc {
	return func(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		body, err := requestObject(request)
		if err != nil {
			return toolJSONError("invalid_request", err.Error()), nil
		}
		if err := validateNarrowReadRequest(operation.action, body); err != nil {
			return toolJSONError("invalid_request", err.Error()), nil
		}
		fedexClient, err := newMCPClient()
		if err != nil {
			return toolJSONError("client_setup_error", err.Error()), nil
		}
		data, status, err := executeNarrowOperation(fedexClient, operation, body)
		if err != nil {
			var apiErr *client.APIError
			if errors.As(err, &apiErr) {
				return toolJSONError("fedex_error", fmt.Sprintf("FedEx rejected %s with HTTP %d", operation.action, apiErr.StatusCode)), nil
			}
			return toolJSONError("fedex_error", fmt.Sprintf("FedEx %s request failed", operation.action)), nil
		}
		result, err := minimizeNarrowReadResponse(operation.action, data)
		if err != nil {
			return toolJSONError("invalid_fedex_response", err.Error()), nil
		}
		return toolJSON(map[string]any{
			"status":      "succeeded",
			"http_status": status,
			"data":        result,
		}), nil
	}
}

func makeNarrowMutationHandler(operation narrowOperation) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		body, err := requestObject(request)
		if err != nil {
			return toolJSONError("invalid_request", err.Error()), nil
		}
		arguments := request.GetArguments()
		var resolvedContext any
		if operation.action == workflow.ActionCancelPickup {
			resolved, err := workflow.ResolvePickupCancellation(ctx, body, stringArgument(arguments, "legacy_reason"))
			if errors.Is(err, workflow.ErrAlreadyCancelled) {
				return toolJSON(map[string]any{"status": "already_cancelled", "action": operation.action}), nil
			}
			if err != nil {
				return toolJSONError("pickup_resolution_failed", err.Error()), nil
			}
			body = resolved.Body
			resolvedContext = resolved.Context
		}
		if err := workflow.ValidateRequest(operation.action, body); err != nil {
			return toolJSONError("invalid_request", err.Error()), nil
		}
		fedexClient, err := newMCPClient()
		if err != nil {
			return toolJSONError("client_setup_error", err.Error()), nil
		}
		confirm, _ := arguments["confirm"].(bool)
		persistOptions := workflow.PersistOptions{}
		approvalContext := resolvedContext
		review := approval.Summarize(operation.action, body)
		if operation.action == workflow.ActionSchedulePickup {
			availabilityRequest, err := optionalObjectArgument(arguments, "availability_request")
			if err != nil {
				return toolJSONError("invalid_request", err.Error()), nil
			}
			if len(availabilityRequest) > 0 {
				if err := workflow.ValidatePickupAvailabilityBinding(body, availabilityRequest); err != nil {
					return toolJSONError("pickup_preflight_mismatch", err.Error()), nil
				}
			}
			preflight, err := workflow.PreparePickupPreflight(fedexClient, confirm, availabilityRequest, stringArgument(arguments, "availability_override_reason"))
			if err != nil {
				return toolJSONError("pickup_preflight_failed", err.Error()), nil
			}
			approvalContext = preflight.Context
			persistOptions.PickupPreflight = preflight.Status
			persistOptions.PickupOverrideReason = preflight.OverrideReason
			persistOptions.PickupPreflightCutoff = preflight.CutoffTime
			persistOptions.PickupPreflightAccessStart = preflight.AccessStart
			review.PickupPreflight = preflight.Status
			review.PreflightOverride = preflight.OverrideReason
			review.PickupWindow = preflight.Window
			review.PickupCutoffTime = preflight.CutoffTime
			review.PickupAccessStart = preflight.AccessStart
		}
		origin, err := approval.NormalizeOrigin(fedexClient.BaseURL)
		if err != nil {
			return toolJSONError("client_setup_error", err.Error()), nil
		}
		mutation := approval.Mutation{Action: operation.action, Origin: origin, Method: operation.method, Path: operation.path, Request: body, Context: approvalContext}
		pendingDir, err := approval.DefaultStoreDir()
		if err != nil {
			return toolJSONError("local_state_error", err.Error()), nil
		}
		approvalStore := approval.NewStore(pendingDir, pendingOperationTTL)
		if !confirm {
			if stringArgument(arguments, "operation_id") != "" || stringArgument(arguments, "confirmation_digest") != "" {
				return toolJSONError("confirmation_invalid", "operation_id and confirmation_digest require confirm=true"), nil
			}
			record, err := approvalStore.Create(mutation, review)
			if err != nil {
				return toolJSONError("preview_error", err.Error()), nil
			}
			return toolJSON(map[string]any{
				"status":              "pending_confirmation",
				"action":              operation.action,
				"environment":         origin,
				"operation_id":        record.ID,
				"confirmation_digest": record.ConfirmationDigest,
				"expires_at":          record.ExpiresAt,
				"review":              record.Review,
				"next_step":           "After explicit human approval, call the same tool with confirm=true, this operation_id and confirmation_digest, and the exact same request and preflight context.",
			}), nil
		}

		operationID := stringArgument(arguments, "operation_id")
		digest := stringArgument(arguments, "confirmation_digest")
		if operationID == "" || digest == "" {
			return toolJSONError("confirmation_required", "confirm=true requires operation_id and confirmation_digest from a preview call"), nil
		}
		record, permit, err := approvalStore.Consume(operationID, digest, mutation)
		if err != nil {
			return toolJSONError("confirmation_rejected", err.Error()), nil
		}
		defer permit.Release()
		persistOptions.OperationID = operationID
		if persistOptions.PickupPreflight == "verified" {
			persistOptions.PickupPreflightCutoff = record.Review.PickupCutoffTime
			persistOptions.PickupPreflightAccessStart = record.Review.PickupAccessStart
		}
		alreadyComplete, beginErr := workflow.BeginMutation(ctx, operation.action, body, persistOptions)
		if beginErr != nil {
			_ = approvalStore.Complete(operationID, approval.StatusRejected, "local_state")
			return toolJSONErrorFields("local_state", beginErr.Error(), map[string]any{"operation_id": operationID}), nil
		}
		if alreadyComplete != nil {
			if completeErr := approvalStore.Complete(operationID, approval.StatusSucceeded, ""); completeErr != nil {
				return toolJSONErrorFields("local_state_error", completeErr.Error(), map[string]any{"operation_id": operationID}), nil
			}
			return toolJSON(map[string]any{"status": "succeeded", "action": operation.action, "operation_id": operationID, "http_status": 200, "data": alreadyComplete}), nil
		}

		operationClient := *fedexClient
		operationClient.MutationPermit = permit
		data, status, executeErr := executeNarrowOperation(&operationClient, operation, body)
		if executeErr != nil {
			completionStatus := approval.StatusRejected
			errorClass := "fedex_rejected"
			var unknown *client.OutcomeUnknownError
			if errors.As(executeErr, &unknown) {
				completionStatus = approval.StatusOutcomeUnknown
				errorClass = "outcome_unknown"
				workflow.PersistOutcomeUnknown(ctx, operation.action, body, persistOptions)
			} else {
				workflow.PersistRejected(ctx, operation.action, body, persistOptions)
			}
			if completeErr := approvalStore.Complete(operationID, completionStatus, errorClass); completeErr != nil {
				return toolJSONErrorFields("local_state_error", fmt.Sprintf("remote operation failed and local completion record could not be updated: %v", completeErr), map[string]any{"operation_id": operationID, "http_status": status}), nil
			}
			return toolJSONErrorFields(errorClass, mutationErrorMessage(errorClass, status), map[string]any{"operation_id": operationID, "operation_status": completionStatus, "http_status": status}), nil
		}
		result, persistErr := workflow.PersistSuccess(ctx, operation.action, body, data, persistOptions)
		if persistErr != nil {
			completionStatus := approval.StatusOutcomeUnknown
			errorClass := "local_persistence"
			if errors.Is(persistErr, workflow.ErrRemoteRejected) {
				completionStatus = approval.StatusRejected
				errorClass = "remote_rejected"
				workflow.PersistRejected(ctx, operation.action, body, persistOptions)
			} else {
				workflow.PersistOutcomeUnknown(ctx, operation.action, body, persistOptions)
			}
			_ = approvalStore.Complete(operationID, completionStatus, errorClass)
			return toolJSONErrorFields(errorClass, persistErr.Error(), map[string]any{"operation_id": operationID, "operation_status": completionStatus, "http_status": status}), nil
		}
		if err := approvalStore.Complete(operationID, approval.StatusSucceeded, ""); err != nil {
			return toolJSONErrorFields("local_state_error", "FedEx accepted the operation but its local completion record could not be updated; do not retry", map[string]any{"operation_id": operationID, "http_status": status}), nil
		}
		return toolJSON(map[string]any{"status": "succeeded", "action": operation.action, "operation_id": operationID, "http_status": status, "data": result}), nil
	}
}

func executeNarrowOperation(fedexClient *client.Client, operation narrowOperation, body map[string]any) (json.RawMessage, int, error) {
	switch operation.method {
	case "POST":
		return fedexClient.Post(operation.path, body)
	case "PUT":
		return fedexClient.Put(operation.path, body)
	default:
		return nil, 0, fmt.Errorf("unsupported narrow operation method %q", operation.method)
	}
}

func requestObject(request mcplib.CallToolRequest) (map[string]any, error) {
	arguments := request.GetArguments()
	value, ok := arguments["request"]
	if !ok {
		return nil, fmt.Errorf("request is required")
	}
	body, ok := value.(map[string]any)
	if !ok || body == nil {
		return nil, fmt.Errorf("request must be a JSON object")
	}
	return body, nil
}

func optionalObjectArgument(arguments map[string]any, name string) (map[string]any, error) {
	value, ok := arguments[name]
	if !ok || value == nil {
		return nil, nil
	}
	body, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a JSON object", name)
	}
	return body, nil
}

func toolJSON(value any) *mcplib.CallToolResult {
	data, err := json.Marshal(value)
	if err != nil {
		return mcplib.NewToolResultError(`{"status":"error","error_class":"encoding_error"}`)
	}
	return mcplib.NewToolResultText(string(data))
}

func toolJSONError(errorClass, message string) *mcplib.CallToolResult {
	return toolJSONErrorFields(errorClass, message, nil)
}

func toolJSONErrorFields(errorClass, message string, fields map[string]any) *mcplib.CallToolResult {
	envelope := map[string]any{
		"status":      "error",
		"error_class": errorClass,
		"message":     message,
	}
	for key, value := range fields {
		envelope[key] = value
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return mcplib.NewToolResultError(`{"status":"error","error_class":"encoding_error"}`)
	}
	return mcplib.NewToolResultError(string(data))
}

func stringArgument(arguments map[string]any, name string) string {
	value, _ := arguments[name].(string)
	return strings.TrimSpace(value)
}

func decodeJSON(data json.RawMessage) any {
	if len(data) == 0 {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return map[string]any{"unparsed_response": true}
	}
	return decoded
}

func summarizeMutationResponse(action string, value any) map[string]any {
	allowed := map[string]struct{}{}
	switch action {
	case "create_label":
		for _, key := range []string{"mastertrackingnumber", "trackingnumber", "shipmentid", "servicetype", "totalnetcharge", "currency"} {
			allowed[key] = struct{}{}
		}
	case "schedule_pickup":
		for _, key := range []string{"pickupconfirmationcode", "confirmationnumber", "scheduleddate"} {
			allowed[key] = struct{}{}
		}
	case "cancel_shipment", "cancel_pickup":
		for _, key := range []string{"cancelled", "canceled", "successful", "success"} {
			allowed[key] = struct{}{}
		}
	}

	result := map[string]any{}
	collectAllowedFields(value, allowed, result)
	if len(result) == 0 {
		result["response_received"] = true
	}
	return result
}

func collectAllowedFields(value any, allowed map[string]struct{}, result map[string]any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalized := strings.ToLower(key)
			if _, ok := allowed[normalized]; ok && isSafeScalar(child) {
				appendResult(result, key, child)
				continue
			}
			collectAllowedFields(child, allowed, result)
		}
	case []any:
		for _, child := range typed {
			collectAllowedFields(child, allowed, result)
		}
	}
}

func isSafeScalar(value any) bool {
	switch value.(type) {
	case string, bool, float64, json.Number:
		return true
	default:
		return false
	}
}

func appendResult(result map[string]any, key string, value any) {
	if existing, ok := result[key]; ok {
		if values, isSlice := existing.([]any); isSlice {
			result[key] = append(values, value)
		} else {
			result[key] = []any{existing, value}
		}
		return
	}
	result[key] = value
}

func mutationErrorMessage(errorClass string, status int) string {
	if errorClass == "outcome_unknown" {
		return "FedEx mutation outcome is unknown; reconcile remote and local state before creating a new preview"
	}
	if status > 0 {
		return fmt.Sprintf("FedEx rejected the mutation with HTTP %d", status)
	}
	return "FedEx rejected the mutation before a confirmed remote outcome was recorded"
}
