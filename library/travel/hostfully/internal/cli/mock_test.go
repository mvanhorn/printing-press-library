package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func runMockHostfully(t *testing.T, args ...string) []byte {
	t.Helper()
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute %v: %v\n%s", args, err, output.String())
	}
	return output.Bytes()
}

func TestMockWhoisReturnsPaidCurrentReservation(t *testing.T) {
	var stays []whoisStay
	if err := json.Unmarshal(runMockHostfully(
		t, "whois", "+15550100404", "--mock", "--json",
	), &stays); err != nil {
		t.Fatal(err)
	}
	if len(stays) != 1 || stays[0].Unit != 4 || stays[0].Paid == nil || !*stays[0].Paid {
		t.Fatalf("unexpected mock whois result: %#v", stays)
	}
	if stays[0].DoorPIN != "4404" {
		t.Fatalf("mock whois PIN = %q, want 4404", stays[0].DoorPIN)
	}
}

func TestMockAvailabilityNeverCallsLoader(t *testing.T) {
	var flags rootFlags
	cmd := newAvailabilityCmdWithLoader(&flags, func(*rootFlags, []int) ([]reservation, error) {
		t.Fatal("live loader called in mock mode")
		return nil, nil
	})
	enableMockForTest(t)
	flags.asJSON = true
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--unit", "4", "--days", "2"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result availResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Units) != 1 || result.Units[0].Unit != 4 {
		t.Fatalf("unexpected mock availability: %#v", result)
	}
}
