package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mvanhorn/printing-press-library/library/commerce/ucp/internal/ucp"
)

const DefaultProfileURL = "https://www.igvita.com/ucp/profile.json"

// McpSearch calls a UCP merchant's MCP endpoint via JSON-RPC tools/call
// for the catalog.search capability. Use this when the manifest declares
// only mcp/embedded transports (no REST) or when the /products.json path
// is theme-overridden (returns HTTP 500 on coffeecircle, etc.).
//
// profileURL must be a publicly-reachable URL serving a UCP agent profile
// JSON with Content-Type: application/json and a Cache-Control max-age
// directive. Empty string means use DefaultProfileURL.
//
// query is the user search string. limit caps the number of returned hits.
func McpSearch(ctx context.Context, manifest *ucp.Manifest, query string, limit int, profileURL string) ([]ucp.SearchHit, error) {
	endpoint := findMCPEndpoint(manifest)
	if endpoint == "" {
		return nil, fmt.Errorf("manifest declares no mcp transport endpoint")
	}
	if profileURL == "" {
		profileURL = DefaultProfileURL
	}
	merchantHost := extractMerchantHost(endpoint)

	// Build JSON-RPC envelope. NOTE: meta.ucp-agent.profile goes in the BODY,
	// not in any HTTP header. Header-based profile is a dead end (returns 422).
	envelope := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "search_catalog",
			"arguments": map[string]any{
				"meta": map[string]any{
					"ucp-agent":       map[string]any{"profile": profileURL},
					"idempotency-key": uuid.NewString(),
				},
				"catalog": map[string]any{
					"query":   query,
					"context": map[string]any{"address_country": "US"},
				},
			},
		},
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal envelope: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ucp-pp-cli/1.2")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("merchant MCP returned HTTP %d: %s", resp.StatusCode, truncateMCP(string(respBody), 300))
	}

	// Unwrap: { result: { content: [{ type: "text", text: "<json>" }], structuredContent?: {...} } }
	var rpc struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			StructuredContent json.RawMessage `json:"structuredContent"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    any    `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &rpc); err != nil {
		return nil, fmt.Errorf("parse JSON-RPC response: %w (body: %s)", err, truncateMCP(string(respBody), 300))
	}
	if rpc.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", rpc.Error.Code, rpc.Error.Message)
	}

	// Pick inner JSON: prefer structuredContent, fall back to first text content.
	var innerJSON []byte
	if len(rpc.Result.StructuredContent) > 0 && string(rpc.Result.StructuredContent) != "null" {
		innerJSON = rpc.Result.StructuredContent
	} else if len(rpc.Result.Content) > 0 && rpc.Result.Content[0].Type == "text" {
		innerJSON = []byte(rpc.Result.Content[0].Text)
	} else {
		return []ucp.SearchHit{}, nil
	}

	// Parse inner: handles both standard {items:[...]} and Shopify extension {products:[...]}
	var inner struct {
		Items    []map[string]any `json:"items"`
		Products []map[string]any `json:"products"`
	}
	if err := json.Unmarshal(innerJSON, &inner); err != nil {
		return nil, fmt.Errorf("parse inner catalog response: %w", err)
	}
	rows := inner.Items
	if len(rows) == 0 {
		rows = inner.Products
	}
	hits := make([]ucp.SearchHit, 0, len(rows))
	for i, row := range rows {
		if i >= limit {
			break
		}
		hits = append(hits, normalizeMCPHit(merchantHost, row))
	}
	return hits, nil
}

// findMCPEndpoint walks the manifest services and returns the first mcp-transport endpoint.
func findMCPEndpoint(m *ucp.Manifest) string {
	for _, svcs := range m.UCP.Services {
		for _, s := range svcs {
			if s.Transport == "mcp" && s.Endpoint != "" {
				return s.Endpoint
			}
		}
	}
	return ""
}

func extractMerchantHost(endpoint string) string {
	// "https://bark-food.myshopify.com/api/ucp/mcp" -> "bark-food.myshopify.com"
	s := strings.TrimPrefix(endpoint, "https://")
	s = strings.TrimPrefix(s, "http://")
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	return s
}

func normalizeMCPHit(merchant string, row map[string]any) ucp.SearchHit {
	hit := ucp.SearchHit{Merchant: merchant, Currency: "USD"}
	// Title
	for _, k := range []string{"title", "name"} {
		if v, ok := row[k].(string); ok && v != "" {
			hit.Title = v
			break
		}
	}
	// Price — Shopify uses {amount: 999, currency: "USD"} nested under "price" or "priceRange"
	if priceObj, ok := row["price"].(map[string]any); ok {
		if amt, ok := priceObj["amount"].(float64); ok {
			hit.Price = int(amt)
		}
		if c, ok := priceObj["currency"].(string); ok && c != "" {
			hit.Currency = c
		}
	} else if s, ok := row["price"].(string); ok {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			hit.Price = int(f * 100)
		}
	}
	// SKU
	if v, ok := row["sku"].(string); ok {
		hit.SKU = v
	}
	// URL — prefer checkout_url from first variant if present (Shopify pattern)
	if variants, ok := row["variants"].([]any); ok && len(variants) > 0 {
		if v0, ok := variants[0].(map[string]any); ok {
			if u, ok := v0["checkout_url"].(string); ok && u != "" {
				hit.URL = u
			}
			if hit.SKU == "" {
				if s, ok := v0["sku"].(string); ok {
					hit.SKU = s
				}
			}
			// Populate VariantID from Shopify's numeric variant id (top-level "id" on the variant).
			// This enables checkout finalize to pass the right ID to /cart/add.js.
			if idF, ok := v0["id"].(float64); ok {
				hit.VariantID = int64(idF)
			}
		}
	}
	if hit.URL == "" {
		if u, ok := row["url"].(string); ok {
			hit.URL = u
		} else if h, ok := row["handle"].(string); ok && h != "" {
			hit.URL = fmt.Sprintf("https://%s/products/%s", merchant, h)
		}
	}
	return hit
}

func truncateMCP(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
