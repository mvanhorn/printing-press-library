package traveloka

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSimulatedExactMoney(t *testing.T) {
	tests := []struct {
		name          string
		source, scale any
		want, minor   string
		decimals      *int
		currency      string
		unknown, fail bool
	}{
		{name: "source stay total is not rounded nightly product", source: map[string]any{"currency": "SGD", "amount": "47181"}, scale: "2", want: "471.81", minor: "47181", decimals: intPointer(2), currency: "SGD"},
		{name: "zero is known", source: map[string]any{"currency": "USD", "amount": json.Number("0")}, scale: json.Number("2"), want: "0.00", minor: "0", decimals: intPointer(2), currency: "USD"},
		{name: "integer scale zero", source: map[string]any{"currency": "IDR", "amount": "12345"}, scale: 0, want: "12345", minor: "12345", decimals: intPointer(0), currency: "IDR"},
		{name: "huge amount exceeds float precision", source: map[string]any{"currency": "SGD", "amount": "9007199254740993123"}, scale: 2, want: "90071992547409931.23", minor: "9007199254740993123", decimals: intPointer(2), currency: "SGD"},
		{name: "negative amount", source: map[string]any{"currency": "SGD", "amount": "-5"}, scale: 3, want: "-0.005", minor: "-5", decimals: intPointer(3), currency: "SGD"},
		{name: "nil source", unknown: true},
		{name: "null or empty beats source zero", source: map[string]any{"currency": "SGD", "amount": "0", "nullOrEmpty": true}, scale: 2, unknown: true},
		{name: "missing amount", source: map[string]any{"currency": "SGD"}, scale: 2, unknown: true},
		{name: "nil amount", source: map[string]any{"currency": "SGD", "amount": nil}, scale: 2, unknown: true},
		{name: "unknown scale stays unknown", source: map[string]any{"currency": "SGD", "amount": "47181"}, minor: "47181", currency: "SGD"},
		{name: "empty scale stays unknown", source: map[string]any{"currency": "SGD", "amount": "47181"}, scale: "", minor: "47181", currency: "SGD"},
		{name: "unknown currency stays unknown", source: map[string]any{"amount": "100"}, scale: 2, want: "1.00", minor: "100", decimals: intPointer(2)},
		{name: "scale mismatch", source: map[string]any{"currency": "SGD", "amount": "100", "numOfDecimalPoint": "0"}, scale: 2, fail: true},
		{name: "decimal amount cannot be rounded", source: map[string]any{"currency": "SGD", "amount": "1.25"}, scale: 2, fail: true},
		{name: "float cannot lose precision", source: map[string]any{"currency": "SGD", "amount": float64(100)}, scale: 2, fail: true},
		{name: "float scale rejected", source: map[string]any{"currency": "SGD", "amount": "100"}, scale: float64(2), fail: true},
		{name: "negative scale", source: map[string]any{"currency": "SGD", "amount": "100"}, scale: -1, fail: true},
		{name: "invalid source", source: "100", scale: 2, fail: true},
		{name: "structured amount rejected safely", source: map[string]any{"amount": map[string]any{}}, scale: 2, fail: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := ParseMoney(tt.source, tt.scale)
			if tt.fail {
				if err == nil {
					t.Fatal("wanted error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.unknown {
				if m != nil {
					t.Fatal("unknown money must be nil")
				}
				return
			}
			if m == nil || m.Amount != tt.want || m.MinorUnits != tt.minor || m.Currency != tt.currency {
				t.Fatalf("incorrect exact money: %#v", m)
			}
			if tt.decimals == nil {
				if m.Decimals != nil {
					t.Fatal("guessed scale")
				}
			} else if m.Decimals == nil || *m.Decimals != *tt.decimals {
				t.Fatal("lost source scale")
			}
		})
	}
}
func TestSimulatedFormatMinorUnits(t *testing.T) {
	for _, tt := range []struct {
		raw   string
		scale int
		want  string
		fail  bool
	}{{"0", 2, "0.00", false}, {"00012", 2, "0.12", false}, {"-001", 0, "-1", false}, {"12", -1, "", true}, {"12", 19, "", true}, {"1.2", 2, "", true}, {"", 2, "", true}, {"1e2", 2, "", true}, {"-0", 3, "0.000", false}} {
		got, err := FormatMinorUnits(tt.raw, tt.scale)
		if tt.fail {
			if err == nil {
				t.Fatal("wanted error")
			}
		} else if err != nil || got != tt.want {
			t.Fatalf("%s/%d: %s %v", tt.raw, tt.scale, got, err)
		}
	}
}
func intPointer(n int) *int { return &n }
func TestSimulatedMoneyJSONUnknownScale(t *testing.T) {
	m, _ := ParseMoney(map[string]any{"currency": "SGD", "amount": "100"}, nil)
	b, err := json.Marshal(m)
	if err != nil || !strings.Contains(string(b), `"decimals":null`) {
		t.Fatal("unknown scale must remain null")
	}
}
