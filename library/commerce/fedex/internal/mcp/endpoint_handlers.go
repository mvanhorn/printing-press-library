// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/mutationguard"
)

const allowDuplicateDescription = "Send even if an identical request succeeded, is in flight, or ended with an unknown outcome in the last 24h. Only set this when a second identical operation is really intended."

func toolNameForEndpoint(id string) string {
	return strings.ReplaceAll(id, ".", "_")
}

func (e endpointTool) readOnly() bool {
	return client.IsReadOnlyOperation(e.Method, e.Path)
}

func registerEndpointTools(s *server.MCPServer) {
	for _, endpoint := range endpointTools {
		ep := endpoint
		description := fmt.Sprintf("%s. Calls FedEx %s %s with `request` as the JSON body.", ep.Summary, ep.Method, ep.Path)
		options := []mcplib.ToolOption{
			mcplib.WithObject("request", mcplib.Required(), mcplib.Description("FedEx REST request body for "+ep.Method+" "+ep.Path+".")),
			mcplib.WithOpenWorldHintAnnotation(true),
		}
		if ep.readOnly() {
			// Address validation is read-only but billable per call, so it is
			// not marked idempotent and is never retried automatically.
			idempotent := ep.Path != "/address/v1/addresses/resolve"
			options = append(options,
				mcplib.WithDescription(description+" Read-only."),
				mcplib.WithReadOnlyHintAnnotation(true),
				mcplib.WithDestructiveHintAnnotation(false),
				mcplib.WithIdempotentHintAnnotation(idempotent),
			)
		} else {
			options = append(options,
				mcplib.WithDescription(description+" Changes FedEx state: runs once per approved call, is never retried automatically, and refuses an identical recent request unless allow_duplicate is true."),
				mcplib.WithReadOnlyHintAnnotation(false),
				mcplib.WithDestructiveHintAnnotation(true),
				mcplib.WithIdempotentHintAnnotation(false),
				mcplib.WithBoolean("allow_duplicate", mcplib.Description(allowDuplicateDescription)),
			)
		}
		s.AddTool(mcplib.NewTool(toolNameForEndpoint(ep.ID), options...), makeEndpointHandler(ep))
	}
}

func makeEndpointHandler(ep endpointTool) server.ToolHandlerFunc {
	readOnly := ep.readOnly()
	toolName := toolNameForEndpoint(ep.ID)
	return func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		body, err := requestObject(request.GetArguments(), "request")
		if err != nil {
			return toolJSONError("invalid_request", err.Error()), nil
		}
		fedexClient, err := newMCPClient()
		if err != nil {
			return toolJSONError("client_setup_error", err.Error()), nil
		}
		if !readOnly {
			// The MCP client's per-call approval is the human gate for this
			// tool; the duplicate ledger still guards against replays.
			fedexClient.WriteApproved = true
			fedexClient.AllowDuplicate, _ = request.GetArguments()["allow_duplicate"].(bool)
			if err := writePacer.wait(ctx); err != nil {
				return toolJSONError("canceled", "canceled before the FedEx write was sent"), nil
			}
		}
		fedexClient.Context = ctx
		data, status, err := executeEndpoint(fedexClient, ep.Method, ep.Path, body)
		if err != nil {
			return writeErrorResult(err, status, readOnly), nil
		}
		result := map[string]any{"status": "succeeded", "http_status": status}
		if readOnly {
			result["data"] = decodeJSON(data)
			return toolJSON(result), nil
		}
		result["data"] = summarizeWriteResponse(toolName, decodeJSON(data))
		if toolName == "shipments_create" {
			if err := archiveShipment(context.WithoutCancel(ctx), body, data); err != nil {
				result["archive_warning"] = "shipment created but not saved to the local archive: " + err.Error()
			}
		}
		return toolJSON(result), nil
	}
}

func executeEndpoint(fedexClient *client.Client, method, path string, body map[string]any) (json.RawMessage, int, error) {
	switch method {
	case "POST":
		return fedexClient.Post(path, body)
	case "PUT":
		return fedexClient.Put(path, body)
	case "PATCH":
		return fedexClient.Patch(path, body)
	default:
		return nil, 0, fmt.Errorf("unsupported FedEx tool method %q", method)
	}
}

// errorClass maps a client error to a stable class agents can branch on.
func errorClass(err error) string {
	var duplicate *mutationguard.DuplicateError
	var unknown *client.OutcomeUnknownError
	var apiErr *client.APIError
	switch {
	case errors.As(err, &duplicate):
		return "duplicate_refused"
	case errors.Is(err, mutationguard.ErrBusy):
		return "in_flight"
	case errors.As(err, &unknown):
		return "outcome_unknown"
	case errors.As(err, &apiErr):
		return "fedex_rejected"
	default:
		return "fedex_error"
	}
}

func writeErrorResult(err error, status int, readOnly bool) *mcplib.CallToolResult {
	class := errorClass(err)
	message := err.Error()
	if class == "outcome_unknown" && !readOnly {
		message = "FedEx write outcome is unknown and it was not retried. Check FedEx (track_number or shipments_results) before sending it again; an identical request is refused for 24h unless allow_duplicate is true."
	}
	fields := map[string]any{}
	if status > 0 {
		fields["http_status"] = status
	}
	return toolJSONErrorFields(class, message, fields)
}

func requestObject(arguments map[string]any, key string) (map[string]any, error) {
	value, ok := arguments[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	body, ok := value.(map[string]any)
	if !ok || body == nil {
		return nil, fmt.Errorf("%s must be a JSON object", key)
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
