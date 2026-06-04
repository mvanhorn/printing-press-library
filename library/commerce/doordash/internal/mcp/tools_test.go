package mcp

import "testing"

func TestNormalizeMCPBodyValueParsesGraphQLVariablesString(t *testing.T) {
	got, err := normalizeMCPBodyValue("variables", `{"cartId":"cart_123","quantity":2}`)
	if err != nil {
		t.Fatalf("normalizeMCPBodyValue returned error: %v", err)
	}
	vars, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("variables should be parsed into map[string]any, got %T", got)
	}
	if vars["cartId"] != "cart_123" {
		t.Fatalf("cartId = %v, want cart_123", vars["cartId"])
	}
	if vars["quantity"] != float64(2) {
		t.Fatalf("quantity = %v, want 2", vars["quantity"])
	}
}

func TestNormalizeMCPBodyValueLeavesNonVariablesStringAlone(t *testing.T) {
	got, err := normalizeMCPBodyValue("query", "query text")
	if err != nil {
		t.Fatalf("normalizeMCPBodyValue returned error: %v", err)
	}
	if got != "query text" {
		t.Fatalf("query = %v, want query text", got)
	}
}

func TestNormalizeMCPBodyValueRejectsInvalidVariablesJSON(t *testing.T) {
	if _, err := normalizeMCPBodyValue("variables", `{not-json}`); err == nil {
		t.Fatal("expected invalid variables JSON to return an error")
	}
}
