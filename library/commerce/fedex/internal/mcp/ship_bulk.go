// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"errors"
	"fmt"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/client"
)

const (
	shipBulkMaxShipments = 50
	shipBulkPath         = "/ship/v1/shipments"
)

func registerShipBulkTool(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("ship_bulk",
			mcplib.WithDescription(fmt.Sprintf("Create up to %d FedEx shipments and labels in one approved call. Each item in `shipments` is a full FedEx create-shipment request body (the same shape shipments_create takes). Shipments are sent one at a time (paced below FedEx's per-IP limit), never retried, saved to the local archive, and each one is checked against the 24h duplicate ledger. The batch stops at the first rate limit or unknown outcome and reports the rest as not_attempted. Changes FedEx state.", shipBulkMaxShipments)),
			mcplib.WithArray("shipments", mcplib.Required(), mcplib.Description("FedEx create-shipment request bodies, one per shipment."), mcplib.Items(map[string]any{"type": "object"})),
			mcplib.WithBoolean("allow_duplicate", mcplib.Description(allowDuplicateDescription)),
			mcplib.WithReadOnlyHintAnnotation(false),
			mcplib.WithDestructiveHintAnnotation(true),
			mcplib.WithIdempotentHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleShipBulk,
	)
}

func handleShipBulk(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	arguments := request.GetArguments()
	raw, ok := arguments["shipments"].([]any)
	if !ok || len(raw) == 0 {
		return toolJSONError("invalid_request", "shipments must be a non-empty array of FedEx create-shipment request objects"), nil
	}
	if len(raw) > shipBulkMaxShipments {
		return toolJSONError("invalid_request", fmt.Sprintf("shipments has %d items; the limit is %d per call", len(raw), shipBulkMaxShipments)), nil
	}
	shipments := make([]map[string]any, len(raw))
	for i, item := range raw {
		body, ok := item.(map[string]any)
		if !ok || body == nil {
			return toolJSONError("invalid_request", fmt.Sprintf("shipments[%d] must be a JSON object", i)), nil
		}
		shipments[i] = body
	}

	fedexClient, err := newMCPClient()
	if err != nil {
		return toolJSONError("client_setup_error", err.Error()), nil
	}
	fedexClient.WriteApproved = true
	fedexClient.AllowDuplicate, _ = arguments["allow_duplicate"].(bool)
	fedexClient.Context = ctx

	results := make([]map[string]any, 0, len(shipments))
	counts := map[string]int{}
	stopped := ""
	for i, body := range shipments {
		if stopped != "" {
			results = append(results, map[string]any{"index": i, "status": "not_attempted"})
			counts["not_attempted"]++
			continue
		}
		// Pacing is shared with every other MCP write and never ramps up.
		if err := writePacer.wait(ctx); err != nil {
			stopped = "canceled by the caller; the remaining shipments were not sent"
			results = append(results, map[string]any{"index": i, "status": "not_attempted"})
			counts["not_attempted"]++
			continue
		}
		data, status, err := fedexClient.Post(shipBulkPath, body)
		if err == nil {
			entry := map[string]any{"index": i, "status": "succeeded", "http_status": status, "data": summarizeWriteResponse("ship_bulk", decodeJSON(data))}
			if archiveErr := archiveShipment(context.WithoutCancel(ctx), body, data); archiveErr != nil {
				entry["archive_warning"] = "shipment created but not saved to the local archive: " + archiveErr.Error()
			}
			results = append(results, entry)
			counts["succeeded"]++
			continue
		}
		class := errorClass(err)
		entry := map[string]any{"index": i, "status": class, "message": err.Error()}
		if status > 0 {
			entry["http_status"] = status
		}
		results = append(results, entry)
		counts[class]++
		var apiErr *client.APIError
		if ctx.Err() != nil {
			stopped = "canceled by the caller; the remaining shipments were not sent"
		} else if class == "outcome_unknown" {
			stopped = "unknown outcome; check FedEx before re-sending the remaining shipments"
		} else if errors.As(err, &apiErr) && (apiErr.StatusCode == 429 || apiErr.StatusCode == 403) {
			stopped = fmt.Sprintf("FedEx returned HTTP %d; wait before re-sending the remaining shipments", apiErr.StatusCode)
		}
	}
	summary := map[string]any{"status": "completed", "counts": counts, "results": results}
	if stopped != "" {
		summary["status"] = "stopped"
		summary["stop_reason"] = stopped
	}
	if counts["succeeded"] == 0 {
		data := toolJSON(summary)
		data.IsError = true
		return data, nil
	}
	return toolJSON(summary), nil
}
