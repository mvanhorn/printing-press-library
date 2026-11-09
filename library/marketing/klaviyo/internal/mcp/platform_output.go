// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

// finalizeMCPResult applies the common MCP envelope after the provider or
// profile-scoped store handler has completed. Error text is kept aggregate-only
// and marked as an MCP error while still carrying the same metadata contract.
func finalizeMCPResult(session *platform.Session, arguments map[string]any, result *mcplib.CallToolResult) *mcplib.CallToolResult {
	if result == nil {
		result = mcplib.NewToolResultError("MCP tool returned no result")
	}
	if err := cli.AdoptMCPOutputSemantics(session, arguments); err != nil {
		result = mcplib.NewToolResultError(err.Error())
	}

	for index, content := range result.Content {
		text, ok := mcplib.AsTextContent(content)
		if !ok {
			continue
		}
		var existing any
		if json.Unmarshal([]byte(text.Text), &existing) == nil && mcpOutputAlreadyEnveloped(existing) {
			continue
		}
		payload := mcpOutputPayload(text.Text, result.IsError)
		observeMCPOutput(session, arguments, payload)
		envelope := map[string]any{"meta": mcpOutputMetadata(session)}
		if result.IsError {
			envelope["error"] = payload
		} else {
			envelope["data"] = payload
		}
		encoded, err := json.Marshal(envelope)
		if err != nil {
			result.IsError = true
			encoded = []byte(`{"meta":{"truncated":false},"error":"encode MCP output envelope"}`)
		}
		text.Text = string(encoded)
		result.Content[index] = *text
	}
	if result.StructuredContent != nil {
		payload := result.StructuredContent
		if !mcpOutputAlreadyEnveloped(payload) {
			observeMCPOutput(session, arguments, payload)
			result.StructuredContent = map[string]any{"meta": mcpOutputMetadata(session), "data": payload}
		}
	}
	return result
}

func mcpOutputAlreadyEnveloped(value any) bool {
	object, ok := value.(map[string]any)
	if !ok {
		return false
	}
	meta, hasMeta := object["meta"]
	if !hasMeta {
		return false
	}
	switch typed := meta.(type) {
	case platform.OutputMetadata:
		// StructuredContent retains the concrete metadata value in-process.
	case map[string]any:
		if _, hasTruncationContract := typed["truncated"].(bool); !hasTruncationContract {
			return false
		}
	default:
		return false
	}
	_, hasData := object["data"]
	_, hasError := object["error"]
	return hasData || hasError
}

func mcpOutputMetadata(session *platform.Session) platform.OutputMetadata {
	if session == nil {
		return platform.OutputMetadata{}
	}
	return session.OutputMetadata()
}

func mcpOutputPayload(text string, isError bool) any {
	if isError {
		return strings.TrimSpace(text)
	}
	var payload any
	if json.Unmarshal([]byte(text), &payload) == nil {
		if object, ok := payload.(map[string]any); ok {
			if _, hasMeta := object["meta"]; hasMeta {
				if data, hasData := object["data"]; hasData {
					return data
				}
			}
		}
		return payload
	}
	return text
}

func observeMCPOutput(session *platform.Session, arguments map[string]any, payload any) {
	if session == nil {
		return
	}
	if reason, ok := mcpOutputCompactionReason(payload); ok {
		session.MarkOutputTruncated(reason)
	}
	if mcpOutputHasMore(payload) {
		session.MarkOutputTruncated(platform.TruncationReason{Kind: "provider_pagination", MoreAvailable: true})
	}
	count := mcpOutputRowCount(payload)
	for rawName, rawValue := range arguments {
		name := strings.NewReplacer("-", "", "_", "").Replace(strings.ToLower(strings.TrimSpace(rawName)))
		kind, capped := mcpOutputCapNames[name]
		if !capped {
			continue
		}
		configured, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(rawValue)))
		if err == nil && configured > 0 && count >= configured {
			session.MarkOutputTruncated(platform.TruncationReason{
				Kind: kind, Configured: configured,
				Observed: count, MoreAvailable: true,
			})
		}
	}
}

func mcpOutputCompactionReason(value any) (platform.TruncationReason, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if truncated, ok := typed["_pp_truncated"].(bool); ok && truncated {
			configured, _ := mcpOutputInteger(typed["_pp_max_bytes"])
			observed, _ := mcpOutputInteger(typed["_pp_original_bytes"])
			return platform.TruncationReason{
				Kind: "mcp_compaction", Configured: configured,
				Observed: observed, MoreAvailable: true,
			}, true
		}
		for _, nested := range typed {
			if reason, ok := mcpOutputCompactionReason(nested); ok {
				return reason, true
			}
		}
	case []any:
		for _, nested := range typed {
			if reason, ok := mcpOutputCompactionReason(nested); ok {
				return reason, true
			}
		}
	}
	return platform.TruncationReason{}, false
}

func mcpOutputInteger(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		parsed, err := strconv.Atoi(typed.String())
		return parsed, err == nil
	default:
		return 0, false
	}
}

var mcpOutputCapNames = map[string]string{
	"limit": "limit", "rowlimit": "row_limit", "maxrows": "max_rows", "maxresults": "max_results",
	"maxitems": "max_items", "maxrecords": "max_records", "top": "top", "first": "first",
	"pagesize": "page_size", "perpage": "per_page", "samplesize": "sample_size", "itemlimit": "item_limit",
	"sequencelimit": "sequence_limit", "broadcastlimit": "broadcast_limit", "resourcelimit": "resource_limit",
	"campaignlimit": "campaign_limit", "adlimit": "ad_limit", "keywordlimit": "keyword_limit",
	"customerlimit": "customer_limit", "orderlimit": "order_limit", "ticketlimit": "ticket_limit",
	"maxpages": "max_pages",
}

func mcpOutputRowCount(value any) int {
	switch typed := value.(type) {
	case []any:
		return len(typed)
	case map[string]any:
		for _, key := range []string{"rows", "results", "data", "items", "records"} {
			if rows, ok := typed[key].([]any); ok {
				return len(rows)
			}
		}
		if len(typed) > 0 {
			return 1
		}
	}
	return 0
}

func mcpOutputHasMore(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"has_more", "more_available"} {
			if more, ok := typed[key].(bool); ok && more {
				return true
			}
		}
		for _, key := range []string{"next", "next_page", "next_cursor", "cursor"} {
			if next, ok := typed[key].(string); ok && strings.TrimSpace(next) != "" {
				return true
			}
		}
		for _, nested := range typed {
			if mcpOutputHasMore(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if mcpOutputHasMore(nested) {
				return true
			}
		}
	}
	return false
}
