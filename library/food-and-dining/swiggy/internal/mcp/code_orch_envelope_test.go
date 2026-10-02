package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

func TestExecuteUsesSwiggyToolEnvelope(t *testing.T) {
	for _, endpoint := range []string{"food.place_food_order", "instamart.checkout", "dineout.book_table"} {
		t.Run(endpoint, func(t *testing.T) {
			domain, tool, _ := strings.Cut(endpoint, ".")
			if domain == "instamart" {
				domain = "im"
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var request struct {
					JSONRPC string `json:"jsonrpc"`
					Method  string `json:"method"`
					ID      int64  `json:"id"`
					Params  struct {
						Name      string         `json:"name"`
						Arguments map[string]any `json:"arguments"`
					} `json:"params"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				if r.Method != "POST" || r.URL.Path != "/"+domain || request.JSONRPC != "2.0" || request.Method != "tools/call" || request.ID == 0 || request.Params.Name != tool || request.Params.Arguments["cartId"] != "test-cart" {
					t.Errorf("incorrect request: %s %s %+v", r.Method, r.URL, request)
				}
				if r.Header.Get("Accept") != "application/json, text/event-stream" {
					t.Error("missing MCP Accept header")
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"structuredContent":{"orderId":"test-order"}}}`))
			}))
			defer server.Close()
			t.Setenv("SWIGGY_BASE_URL", server.URL)
			t.Setenv("SWIGGY_CONFIG", filepath.Join(t.TempDir(), "config.json"))
			t.Setenv("SWIGGY_ACCESS_TOKEN", "test-token")
			t.Setenv("PRINTING_PRESS_VERIFY", "")
			result, err := handleCodeOrchExecute(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"endpoint_id": endpoint, "params": map[string]any{"cartId": "test-cart"}}}})
			if err != nil || result.IsError || calls != 1 {
				t.Fatalf("execute: result=%+v err=%v calls=%d", result, err, calls)
			}
			text := result.Content[0].(mcplib.TextContent).Text
			if !strings.Contains(text, "test-order") || strings.Contains(text, "structuredContent") {
				t.Fatalf("response not unwrapped: %s", text)
			}
		})
	}
}

// Verify mode must fetch genuine read results while keeping all other POSTs
// behind the mutation short circuit. This covers the complete registry.
func TestExecuteVerifyModePreservesReadIntent(t *testing.T) {
	readOnly := map[string]bool{
		"dineout.get_available_slots":        true,
		"dineout.get_booking_status":         true,
		"dineout.get_payment_options":        true,
		"dineout.get_restaurant_details":     true,
		"dineout.get_saved_locations":        true,
		"dineout.search_restaurants_dineout": true,
		"food.fetch_food_coupons":            true,
		"food.get_addresses":                 true,
		"food.get_food_cart":                 true,
		"food.get_food_delivery_status":      true,
		"food.get_food_order_details":        true,
		"food.get_food_orders":               true,
		"food.get_payment_options":           true,
		"food.get_restaurant_menu":           true,
		"food.search_menu":                   true,
		"food.search_restaurants":            true,
		"instamart.get_addresses":            true,
		"instamart.get_cart":                 true,
		"instamart.get_delivery_status":      true,
		"instamart.get_order_details":        true,
		"instamart.get_orders":               true,
		"instamart.get_payment_options":      true,
		"instamart.list_coupons":             true,
		"instamart.search_products":          true,
	}
	for _, ep := range codeOrchEndpoints {
		t.Run(ep.ID, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if !readOnly[ep.ID] {
					t.Error("mutation reached transport in verify mode")
				}
				var request struct {
					Method string `json:"method"`
					Params struct {
						Name string `json:"name"`
					} `json:"params"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				_, tool, _ := strings.Cut(ep.ID, ".")
				if request.Method != "tools/call" || request.Params.Name != tool {
					t.Errorf("wrong envelope: %+v", request)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"structuredContent":{"source":"provider-read"}}}`))
			}))
			defer server.Close()
			t.Setenv("SWIGGY_BASE_URL", server.URL)
			t.Setenv("SWIGGY_CONFIG", filepath.Join(t.TempDir(), "config.json"))
			t.Setenv("SWIGGY_ACCESS_TOKEN", "test-token")
			t.Setenv("PRINTING_PRESS_VERIFY", "1")
			t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "")
			result, err := handleCodeOrchExecute(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"endpoint_id": ep.ID}}})
			if err != nil || result == nil || result.IsError {
				t.Fatalf("execute: result=%+v err=%v", result, err)
			}
			text := result.Content[0].(mcplib.TextContent).Text
			if readOnly[ep.ID] {
				if calls != 1 || !strings.Contains(text, "provider-read") {
					t.Fatalf("read was suppressed: calls=%d result=%s", calls, text)
				}
			} else if calls != 0 || strings.Contains(text, "provider-read") {
				t.Fatalf("mutation escaped verification: calls=%d result=%s", calls, text)
			}
		})
	}
}
