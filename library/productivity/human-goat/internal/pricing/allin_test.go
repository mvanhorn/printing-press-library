package pricing

import "testing"

func TestAllIn_StandardMetro(t *testing.T) {
	got := AllIn(3333, "")

	if diff := abs(got.AllInCents - 4466); diff > 2 {
		t.Fatalf("AllInCents = %d, want within +/-2 cents of 4466", got.AllInCents)
	}
	if got.ServiceFeeCents <= 0 {
		t.Fatalf("ServiceFeeCents = %d, want > 0", got.ServiceFeeCents)
	}
	if got.TrustFeeCents <= 0 {
		t.Fatalf("TrustFeeCents = %d, want > 0", got.TrustFeeCents)
	}
	if got.AllInCents != got.BaseCents+got.ServiceFeeCents+got.TrustFeeCents {
		t.Fatalf("AllInCents = %d, want base + fees = %d", got.AllInCents, got.BaseCents+got.ServiceFeeCents+got.TrustFeeCents)
	}
}

func TestAllIn_CaliforniaServiceFeeOnly(t *testing.T) {
	got := AllIn(3333, "CA")

	if got.TrustFeeCents != 0 {
		t.Fatalf("TrustFeeCents = %d, want 0", got.TrustFeeCents)
	}
	if !got.ServiceFeeOnly {
		t.Fatal("ServiceFeeOnly = false, want true")
	}
	if got.AllInCents != 3333+got.ServiceFeeCents {
		t.Fatalf("AllInCents = %d, want base + service fee = %d", got.AllInCents, 3333+got.ServiceFeeCents)
	}
}

func TestAllIn_Massachusetts(t *testing.T) {
	got := AllIn(3333, "Massachusetts")

	if !got.ServiceFeeOnly {
		t.Fatal("ServiceFeeOnly = false, want true")
	}
}

func TestIsServiceFeeOnlyState(t *testing.T) {
	tests := map[string]bool{
		"CA":            true,
		"MA":            true,
		"California":    true,
		"Massachusetts": true,
		"ca":            true,
		"massachusetts": true,
		"WA":            false,
		"NY":            false,
		"":              false,
	}

	for state, want := range tests {
		if got := IsServiceFeeOnlyState(state); got != want {
			t.Fatalf("IsServiceFeeOnlyState(%q) = %v, want %v", state, got, want)
		}
	}
}

func TestFormatCents(t *testing.T) {
	tests := map[int]string{
		4466: "$44.66",
		3300: "$33.00",
		5:    "$0.05",
	}

	for cents, want := range tests {
		if got := FormatCents(cents); got != want {
			t.Fatalf("FormatCents(%d) = %q, want %q", cents, got, want)
		}
	}
}

func TestAllIn_NegativeClamped(t *testing.T) {
	got := AllIn(-100, "")

	if got.AllInCents != 0 {
		t.Fatalf("AllInCents = %d, want 0", got.AllInCents)
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}

	return n
}
