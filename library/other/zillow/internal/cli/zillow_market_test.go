package cli

import "testing"

func TestBreakEvenValuesDistinguishNoBreakEven(t *testing.T) {
	reached, month, year := breakEvenValues(0)
	if reached || month != nil || year != nil {
		t.Fatalf("no break-even = reached %v, month %#v, year %#v", reached, month, year)
	}

	reached, month, year = breakEvenValues(18)
	if !reached || month != 18 || year != 1.5 {
		t.Fatalf("18-month break-even = reached %v, month %#v, year %#v", reached, month, year)
	}
}
