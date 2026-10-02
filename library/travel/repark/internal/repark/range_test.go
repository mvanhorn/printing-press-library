package repark

import "testing"

func TestMarkerRangeBoundedJapanWindow(t *testing.T) {
	for _, tc := range []struct {
		window string
		valid  bool
	}{
		{"C34.663534,135.516310N34.664W135.515S34.663E135.517", true},
		{"", false},
		{"bad", false},
		{"C34,135N40W120S20E150", false},
		{"C34,135N34.01W134.99S33.99E135.03", false},
		{"C34,135N35W134.99S33.99E135.01", false},
		{"C34,135N34.01W134.99S33E135.01", false},
		{"C34,135N34.01W134S33.99E135.01", false},
		{"C34,135N35W136S33E137", false},
		{"C19,135N19.001W134.999S18.999E135.001", false},
		{"C34,121N34.001W120.999S33.999E121.001", false},
		{"C34,135N34W134.99S33.99E135.01", false},
		{"C34,135N34.01W134.99S33.99E135.01&extra=true", false},
	} {
		t.Run(tc.window, func(t *testing.T) {
			if err := ValidateMarkerRange(tc.window); (err == nil) != tc.valid {
				t.Fatalf("valid = %v, error = %v", tc.valid, err)
			}
		})
	}
}
