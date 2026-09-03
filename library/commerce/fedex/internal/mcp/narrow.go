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
	{name: "create_label", action: "create_label", method: "POST", path: "/ship/v1/shipments", description: "Preview or, after bound confirmation, create one FedEx shipment and label. First call returns a pending operation; confirmation must repeat the exact request."},
	{name: "cancel_shipment", action: "cancel_shipment", method: "PUT", path: "/ship/v1/shipments/cancel", destructive: true, description: "Preview or, after bound confirmation, cancel one FedEx shipment. First call returns a pending operation; confirmation must repeat the exact request."},
	{name: "schedule_pickup", action: "schedule_pickup", method: "POST", path: "/pickup/v1/pickups", description: "Preview or, after bound confirmation, schedule one FedEx pickup. First call returns a pending operation; confirmation must repeat the exact request."},
	{name: "cancel_pickup", action: "cancel_pickup", method: "PUT", path: "/pickup/v1/pickups/cancel", destructive: true, description: "Preview or, after bound confirmation, cancel one FedEx pickup. First call returns a pending operation; confirmation must repeat the exact request."},
}

func registerNarrowTools(s *server.MCPServer) {
	for _, operation := range narrowOperations {
		op := operation
		options := []mcplib.ToolOption{
			mcplib.WithDescription(op.description),
			mcplib.WithObject("request", mcplib.Required(), mcplib.Description("Exact FedEx REST request object for this operation.")),
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
		handler := makeNarrowReadHandler(op)
		if !op.readOnly {
			handler = makeNarrowMutationHandler(op)
		}
		s.AddTool(mcplib.NewTool(op.name, options...), handler)
	}
}

func makeNarrowReadHandler(operation narrowOperation) server.ToolHandlerFunc {
	return func(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		body, err := requestObject(request)
		if err != nil {
			return toolJSONError("invalid_request", err.Error()), nil
		}
		fedexClient, err := newMCPClient()
		if err != nil {
			return toolJSONError("client_setup_error", err.Error()), nil
		}
		data, status, err := executeNarrowOperation(fedexClient, operation, body)
		if err != nil {
			return toolJSONError("fedex_error", err.Error()), nil
		}
		return toolJSON(map[string]any{
			"status":      "succeeded",
			"http_status": status,
			"data":        decodeJSON(data),
		}), nil
	}
}

func makeNarrowMutationHandler(operation narrowOperation) server.ToolHandlerFunc {
	return func(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		body, err := requestObject(request)
		if err != nil {
			return toolJSONError("invalid_request", err.Error()), nil
		}
		fedexClient, err := newMCPClient()
		if err != nil {
			return toolJSONError("client_setup_error", err.Error()), nil
		}
		origin, err := approval.NormalizeOrigin(fedexClient.BaseURL)
		if err != nil {
			return toolJSONError("client_setup_error", err.Error()), nil
		}
		mutation := approval.Mutation{Action: operation.action, Origin: origin, Method: operation.method, Path: operation.path, Request: body}
		pendingDir, err := approval.DefaultStoreDir()
		if err != nil {
			return toolJSONError("local_state_error", err.Error()), nil
		}
		store := approval.NewStore(pendingDir, pendingOperationTTL)
		arguments := request.GetArguments()
		confirm, _ := arguments["confirm"].(bool)
		if !confirm {
			if stringArgument(arguments, "operation_id") != "" || stringArgument(arguments, "confirmation_digest") != "" {
				return toolJSONError("confirmation_invalid", "operation_id and confirmation_digest require confirm=true"), nil
			}
			record, err := store.Create(mutation, approval.Summarize(operation.action, body))
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
				"next_step":           "After explicit human approval, call the same tool with confirm=true, this operation_id and confirmation_digest, and the exact same request object.",
			}), nil
		}

		operationID := stringArgument(arguments, "operation_id")
		digest := stringArgument(arguments, "confirmation_digest")
		if operationID == "" || digest == "" {
			return toolJSONError("confirmation_required", "confirm=true requires operation_id and confirmation_digest from a preview call"), nil
		}
		_, permit, err := store.Consume(operationID, digest, mutation)
		if err != nil {
			return toolJSONError("confirmation_rejected", err.Error()), nil
		}

		fedexClient.MutationPermit = permit
		data, status, executeErr := executeNarrowOperation(fedexClient, operation, body)
		if executeErr != nil {
			completionStatus := approval.StatusRejected
			errorClass := "fedex_rejected"
			var unknown *client.OutcomeUnknownError
			if errors.As(executeErr, &unknown) {
				completionStatus = approval.StatusOutcomeUnknown
				errorClass = "outcome_unknown"
			}
			if completeErr := store.Complete(operationID, completionStatus, errorClass); completeErr != nil {
				return toolJSONErrorFields("local_state_error", fmt.Sprintf("remote operation failed and local completion record could not be updated: %v", completeErr), map[string]any{
					"operation_id": operationID,
					"http_status":  status,
				}), nil
			}
			return toolJSONErrorFields(errorClass, mutationErrorMessage(errorClass, status), map[string]any{
				"operation_id":     operationID,
				"operation_status": completionStatus,
				"http_status":      status,
			}), nil
		}
		if err := store.Complete(operationID, approval.StatusSucceeded, ""); err != nil {
			return toolJSONErrorFields("local_state_error", "FedEx accepted the operation but its local completion record could not be updated; do not retry", map[string]any{
				"operation_id": operationID,
				"http_status":  status,
			}), nil
		}
		return toolJSON(map[string]any{
			"status":       "succeeded",
			"action":       operation.action,
			"operation_id": operationID,
			"http_status":  status,
			"data":         summarizeMutationResponse(operation.action, decodeJSON(data)),
		}), nil
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
