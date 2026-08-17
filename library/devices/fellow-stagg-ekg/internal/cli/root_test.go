package cli

import "testing"

func TestNormalizeBaseURLFromHost(t *testing.T) {
	got, err := normalizeBaseURL("", "192.168.1.86", 80)
	if err != nil {
		t.Fatalf("normalizeBaseURL returned error: %v", err)
	}
	if got != "http://192.168.1.86" {
		t.Fatalf("normalizeBaseURL = %q, want %q", got, "http://192.168.1.86")
	}
}

func TestNormalizeBaseURLEnforcesExclusiveInputs(t *testing.T) {
	if _, err := normalizeBaseURL("http://192.168.1.86", "192.168.1.86", 80); err == nil {
		t.Fatalf("normalizeBaseURL should reject both base-url and host")
	}
}

func TestParseKeyValues(t *testing.T) {
	lines := parseKeyValues("mode=S_Heat, tempr=37.82 C, clock=22:21, units=1.")
	want := []string{
		"mode: S_Heat",
		"tempr: 37.82 C",
		"clock: 22:21",
		"units: 1",
	}
	if len(lines) != len(want) {
		t.Fatalf("parseKeyValues len = %d, want %d", len(lines), len(want))
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("parseKeyValues[%d] = %q, want %q", i, lines[i], want[i])
		}
	}
}

