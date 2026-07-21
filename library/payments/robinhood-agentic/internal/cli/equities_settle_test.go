// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavior tests for equities settle: terminal-state classification, poll
// stop conditions, and defensive report building. Hermetic — no network, no
// store; pure functions with in-memory fixtures. Keeps the wiring smoke test.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestIsTerminalOrderState(t *testing.T) {
	tests := []struct {
		state string
		want  bool
	}{
		{"filled", true},
		{"cancelled", true},
		{"rejected", true},
		{"failed", true},
		{"voided", true},
		{"Filled", true},        // case-insensitive
		{"  cancelled  ", true}, // whitespace-tolerant
		{"new", false},
		{"queued", false},
		{"confirmed", false},
		{"unconfirmed", false},
		{"partially_filled", false},
		{"", false},
		{"accepted", false}, // cancel {accepted} echo is not terminal truth
	}
	for _, tt := range tests {
		t.Run("state="+tt.state, func(t *testing.T) {
			if got := isTerminalOrderState(tt.state); got != tt.want {
				t.Errorf("isTerminalOrderState(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestSettlePollDone(t *testing.T) {
	tests := []struct {
		name      string
		state     string
		fillPrice string
		want      bool
	}{
		{"non-terminal keeps polling", "queued", "", false},
		{"partially filled keeps polling", "partially_filled", "12.30", false},
		{"filled with null price keeps polling for backfill", "filled", "", false},
		{"filled with backfilled price stops", "filled", "12.34", true},
		{"cancelled stops without price", "cancelled", "", true},
		{"rejected stops without price", "rejected", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := settlePollDone(tt.state, tt.fillPrice); got != tt.want {
				t.Errorf("settlePollDone(%q, %q) = %v, want %v", tt.state, tt.fillPrice, got, tt.want)
			}
		})
	}
}

func TestBuildSettleReport(t *testing.T) {
	tests := []struct {
		name  string
		order map[string]any
		want  equitySettleReport
	}{
		{
			name: "filled order with average_price and executions",
			order: map[string]any{
				"state":         "filled",
				"symbol":        "AAPL",
				"average_price": "190.12",
				"price":         "191.00",
				"quantity":      "10",
				"executions": []any{
					map[string]any{"price": "190.10"},
					map[string]any{"price": "190.14"},
				},
			},
			want: equitySettleReport{
				OrderID: "ord-1", Symbol: "AAPL", State: "filled", Terminal: true,
				FillPrice: "190.12", Quantity: "10", ExecutionsCount: 2,
			},
		},
		{
			name: "null price falls back to first execution",
			order: map[string]any{
				"state":    "filled",
				"symbol":   "MSFT",
				"price":    nil,
				"quantity": float64(5),
				"executions": []any{
					map[string]any{"price": float64(410.5)},
				},
			},
			want: equitySettleReport{
				OrderID: "ord-2", Symbol: "MSFT", State: "filled", Terminal: true,
				FillPrice: "410.5", Quantity: "5", ExecutionsCount: 1,
			},
		},
		{
			name: "non-terminal order with missing fields",
			order: map[string]any{
				"state": "queued",
			},
			want: equitySettleReport{
				OrderID: "ord-3", Symbol: "", State: "queued", Terminal: false,
				FillPrice: "", Quantity: "", ExecutionsCount: 0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSettleReport(tt.want.OrderID, tt.order)
			if got != tt.want {
				t.Errorf("buildSettleReport() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestExtractSettleOrder(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantState string
		wantErr   bool
	}{
		{
			name:      "envelope with data.orders array takes first",
			raw:       `{"data":{"orders":[{"state":"filled"},{"state":"queued"}]},"guide":"g"}`,
			wantState: "filled",
		},
		{
			name:      "envelope with single order object under data",
			raw:       `{"data":{"state":"cancelled","symbol":"AAPL"}}`,
			wantState: "cancelled",
		},
		{
			name:      "envelope with bare data array",
			raw:       `{"data":[{"state":"rejected"}]}`,
			wantState: "rejected",
		},
		{
			name:    "empty orders array is not found",
			raw:     `{"data":{"orders":[]}}`,
			wantErr: true,
		},
		{
			name:    "null data is not found",
			raw:     `{"data":null}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, err := extractSettleOrder(json.RawMessage(tt.raw))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("extractSettleOrder(%s) expected error, got order %+v", tt.raw, order)
				}
				return
			}
			if err != nil {
				t.Fatalf("extractSettleOrder(%s) unexpected error: %v", tt.raw, err)
			}
			if got := settleFieldString(order, "state"); got != tt.wantState {
				t.Errorf("extractSettleOrder(%s) state = %q, want %q", tt.raw, got, tt.wantState)
			}
		})
	}
}

// TestNovelEquitiesSettleHelpWires smoke-tests that the equities settle command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelEquitiesSettleHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"equities", "settle", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("equities settle --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "settle"} {
		if !strings.Contains(help, want) {
			t.Fatalf("equities settle --help missing %q in output:\n%s", want, help)
		}
	}
}
