package cli

import (
	"strings"
	"testing"
)

// Placeholder property UIDs; tests pin the unit registry to these so no test
// depends on a real agency's properties.
var testUnitUIDs = map[int]string{
	1: "00000000-0000-4000-8000-000000000001",
	2: "00000000-0000-4000-8000-000000000002",
	3: "00000000-0000-4000-8000-000000000003",
	4: "00000000-0000-4000-8000-000000000004",
	5: "00000000-0000-4000-8000-000000000005",
	6: "00000000-0000-4000-8000-000000000006",
	7: "00000000-0000-4000-8000-000000000007",
}

func init() {
	setUnitRegistry(testUnitUIDs, "test")
}

func enableMockForTest(t *testing.T) {
	t.Helper()
	mockModeEnabled = true
	resetUnitRegistry()
	t.Cleanup(func() {
		mockModeEnabled = false
		setUnitRegistry(testUnitUIDs, "test")
	})
}

func TestParseUnitsSpec(t *testing.T) {
	got, err := parseUnitsSpec("1=aaa, 12=bbb")
	if err != nil {
		t.Fatal(err)
	}
	if got[1] != "aaa" || got[12] != "bbb" || len(got) != 2 {
		t.Fatalf("unexpected map %v", got)
	}
	for _, bad := range []string{"x=aaa", "1", "0=aaa", "2="} {
		if _, err := parseUnitsSpec(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestNumberPropertiesUsesDistinctTrailingNumbers(t *testing.T) {
	inactive := false
	byUnit, names := numberProperties([]propertySummary{
		{UID: "p10", Name: "Cottage #10"},
		{UID: "p2", Name: "Cottage #2"},
		{UID: "px", Name: "Cottage #3", IsActive: &inactive},
		{UID: "p1", Name: "Cottage #1"},
	})
	if byUnit[1] != "p1" || byUnit[2] != "p2" || byUnit[10] != "p10" || len(byUnit) != 3 {
		t.Fatalf("unexpected units %v", byUnit)
	}
	if !strings.Contains(names[10], "10") {
		t.Fatalf("names not carried: %v", names)
	}
}

func TestNumberPropertiesFallsBackToNameOrder(t *testing.T) {
	byUnit, _ := numberProperties([]propertySummary{
		{UID: "b", Name: "Bayview"},
		{UID: "a", Name: "Anchor"},
		{UID: "c", Name: "Cove 2"},
	})
	if byUnit[1] != "a" || byUnit[2] != "b" || byUnit[3] != "c" {
		t.Fatalf("unexpected fallback numbering %v", byUnit)
	}
}

func TestUnitsEnvOverridesDiscovery(t *testing.T) {
	t.Setenv(unitsEnvVar, "3=prop-three,5=prop-five")
	resetUnitRegistry()
	t.Cleanup(func() { setUnitRegistry(testUnitUIDs, "test") })
	if uid, ok := unitUID(5); !ok || uid != "prop-five" {
		t.Fatalf("unit 5 = %q, %v", uid, ok)
	}
	if got := unitForProperty("prop-three"); got != 3 {
		t.Fatalf("reverse lookup = %d", got)
	}
	if got := unitNumbers(); len(got) != 2 || got[0] != 3 || got[1] != 5 {
		t.Fatalf("unit numbers = %v", got)
	}
}
