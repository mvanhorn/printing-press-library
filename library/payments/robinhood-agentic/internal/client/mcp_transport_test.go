// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

func TestToolNameFromPath(t *testing.T) {
	cases := []struct {
		path string
		want string
		ok   bool
	}{
		{"/mcp/trading/tools/get_accounts", "get_accounts", true},
		{"/tools/place_equity_order", "place_equity_order", true},
		{"/mcp/trading/tools/", "", false},
		{"/mcp/trading/tools/a/b", "", false},
		{"/health", "", false},
	}
	for _, c := range cases {
		got, ok := toolNameFromPath(c.path)
		if got != c.want || ok != c.ok {
			t.Errorf("toolNameFromPath(%q) = (%q,%v), want (%q,%v)", c.path, got, ok, c.want, c.ok)
		}
	}
}

func TestCoerceArgs(t *testing.T) {
	// get_equity_quotes.symbols must become an array...
	got := coerceArgs("get_equity_quotes", map[string]any{"symbols": "AAPL,MSFT, NVDA"})
	want := []string{"AAPL", "MSFT", "NVDA"}
	if !reflect.DeepEqual(got["symbols"], want) {
		t.Errorf("symbols coercion = %#v, want %#v", got["symbols"], want)
	}
	// ...but get_indexes.symbols must stay a comma-separated string.
	got = coerceArgs("get_indexes", map[string]any{"symbols": "SPX,NDX"})
	if got["symbols"] != "SPX,NDX" {
		t.Errorf("get_indexes symbols should stay a string, got %#v", got["symbols"])
	}
	// search.limit -> int
	got = coerceArgs("search", map[string]any{"limit": "5"})
	if got["limit"] != int64(5) {
		t.Errorf("limit coercion = %#v, want int64(5)", got["limit"])
	}
	// get_option_positions.nonzero -> bool
	got = coerceArgs("get_option_positions", map[string]any{"nonzero": "true"})
	if got["nonzero"] != true {
		t.Errorf("nonzero coercion = %#v, want true", got["nonzero"])
	}
}

func TestApplyToolTransformsFoldsOptionLeg(t *testing.T) {
	got := applyToolTransforms("place_option_order", map[string]any{
		"account_number":  "RH1",
		"option_id":       "opt-123",
		"side":            "buy",
		"position_effect": "open",
		"quantity":        "1",
	})
	legs, ok := got["legs"].([]any)
	if !ok || len(legs) != 1 {
		t.Fatalf("expected one leg, got %#v", got["legs"])
	}
	leg := legs[0].(map[string]any)
	if leg["option_id"] != "opt-123" || leg["side"] != "buy" || leg["position_effect"] != "open" {
		t.Errorf("leg fields wrong: %#v", leg)
	}
	if _, present := got["option_id"]; present {
		t.Error("option_id should have been moved into the leg, not left at top level")
	}
	if got["quantity"] != "1" {
		t.Error("quantity should remain at the top level")
	}
}

func TestIsMutatingTool(t *testing.T) {
	mut := []string{"place_equity_order", "cancel_option_order", "add_to_watchlist", "update_scan_config"}
	read := []string{"get_accounts", "review_equity_order", "run_scan", "search"}
	for _, tool := range mut {
		if !isMutatingTool(tool) {
			t.Errorf("%s should be mutating", tool)
		}
	}
	for _, tool := range read {
		if isMutatingTool(tool) {
			t.Errorf("%s should NOT be mutating", tool)
		}
	}
}

func TestNormalizeToolResultShapes(t *testing.T) {
	// structuredContent already carrying {data, guide}
	sc := `{"structuredContent":{"data":{"accounts":[{"account_number":"RH1"}]},"guide":"g"},"isError":false}`
	body, isErr, err := normalizeToolResult(json.RawMessage(sc))
	if err != nil || isErr {
		t.Fatalf("structuredContent: err=%v isErr=%v", err, isErr)
	}
	if !hasDataAccounts(body) {
		t.Errorf("structuredContent normalize dropped data.accounts: %s", body)
	}

	// content[].text carrying JSON without a data key -> wrapped
	ct := `{"content":[{"type":"text","text":"{\"accounts\":[{\"account_number\":\"RH1\"}]}"}],"isError":false}`
	body, _, err = normalizeToolResult(json.RawMessage(ct))
	if err != nil {
		t.Fatalf("content text: %v", err)
	}
	if !hasDataAccounts(body) {
		t.Errorf("content-text normalize did not wrap under data: %s", body)
	}

	// isError propagates
	_, isErr, _ = normalizeToolResult(json.RawMessage(`{"content":[{"type":"text","text":"{\"message\":\"bad\"}"}],"isError":true}`))
	if !isErr {
		t.Error("isError=true should propagate")
	}
}

func hasDataAccounts(body []byte) bool {
	var env struct {
		Data struct {
			Accounts []struct {
				AccountNumber string `json:"account_number"`
			} `json:"accounts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return false
	}
	return len(env.Data.Accounts) == 1 && env.Data.Accounts[0].AccountNumber == "RH1"
}

func TestStatusForCallErr(t *testing.T) {
	if got := statusForCallErr(&rpcError{Code: 401, Message: "authentication required"}); got != 401 {
		t.Errorf("401 rpcError → %d, want 401 (auth errors must not retry as 5xx)", got)
	}
	if got := statusForCallErr(&rpcError{Code: 429}); got != 429 {
		t.Errorf("429 → %d, want 429", got)
	}
	if got := statusForCallErr(&rpcError{Code: -32001, Message: "session expired"}); got != http.StatusBadGateway {
		t.Errorf("JSON-RPC code → %d, want 502", got)
	}
	if got := statusForCallErr(fmt.Errorf("dial tcp: timeout")); got != http.StatusBadGateway {
		t.Errorf("plain error → %d, want 502", got)
	}
}

func TestExtractSSEData(t *testing.T) {
	sse := "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"ok\":true}}\n\n"
	data, err := extractSSEData([]byte(sse))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("SSE data not JSON: %v", err)
	}
	if m["id"] != float64(1) {
		t.Errorf("SSE parse wrong: %#v", m)
	}
}

func TestSynthResponseIsReadable(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://x/tools/get_accounts", nil)
	resp := synthResponse(req, http.StatusOK, []byte(`{"data":{}}`))
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("synthResponse header/status wrong: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	_ = resp.Body.Close()
}
