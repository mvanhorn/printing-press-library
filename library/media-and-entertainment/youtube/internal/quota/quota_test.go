package quota

import (
	"strings"
	"testing"
)

func TestCost_KnownReads(t *testing.T) {
	tests := []struct {
		resource, method string
		want             int
	}{
		{"search", "list", 100},
		{"videos", "list", 1},
		{"channels", "list", 1},
		{"captions", "download", 200},
		{"members", "list", 2},
	}
	for _, tt := range tests {
		got := Cost(tt.resource, tt.method)
		if got != tt.want {
			t.Errorf("Cost(%q,%q) = %d; want %d", tt.resource, tt.method, got, tt.want)
		}
	}
}

func TestCost_KnownWrites(t *testing.T) {
	tests := []struct {
		resource, method string
		want             int
	}{
		{"videos", "insert", 1600},
		{"videos", "update", 50},
		{"videos", "rate", 50},
		{"playlists", "insert", 50},
		{"captions", "insert", 400},
	}
	for _, tt := range tests {
		got := Cost(tt.resource, tt.method)
		if got != tt.want {
			t.Errorf("Cost(%q,%q) = %d; want %d", tt.resource, tt.method, got, tt.want)
		}
	}
}

func TestCost_CaseInsensitive(t *testing.T) {
	if Cost("SEARCH", "LIST") != 100 {
		t.Errorf("Cost should be case-insensitive on resource and method")
	}
	if Cost("Videos", "Insert") != 1600 {
		t.Errorf("Cost should be case-insensitive on mixed case")
	}
}

func TestCost_UnknownReadDefaultsToOne(t *testing.T) {
	if got := Cost("madeup", "list"); got != 1 {
		t.Errorf("unknown read endpoint = %d; want 1 (documented minimum)", got)
	}
}

func TestCost_UnknownWriteDefaultsToFifty(t *testing.T) {
	for _, method := range []string{"insert", "update", "delete", "set", "unset", "rate"} {
		if got := Cost("madeup", method); got != 50 {
			t.Errorf("unknown write endpoint with method %q = %d; want 50", method, got)
		}
	}
}

func TestAll_ReturnsCopy(t *testing.T) {
	a := All()
	if len(a) == 0 {
		t.Fatal("All() returned empty map")
	}
	a["fake.endpoint"] = 999
	if _, ok := table["fake.endpoint"]; ok {
		t.Error("All() must return a copy; mutation leaked into the package table")
	}
}

func TestEndpoints_NonEmpty(t *testing.T) {
	got := Endpoints()
	if len(got) == 0 {
		t.Fatal("Endpoints() returned empty slice")
	}
	if len(got) != len(table) {
		t.Errorf("Endpoints() returned %d entries; want %d (table size)", len(got), len(table))
	}
	// Spot-check that a known key is present.
	found := false
	for _, k := range got {
		if k == "search.list" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Endpoints() missing search.list")
	}
}

func TestHashKey_EmptyIsAnon(t *testing.T) {
	if got := HashKey(""); got != "anon" {
		t.Errorf("HashKey(\"\") = %q; want \"anon\"", got)
	}
}

func TestHashKey_StableAndOpaque(t *testing.T) {
	a := HashKey("AIzaSyExampleKey")
	b := HashKey("AIzaSyExampleKey")
	if a != b {
		t.Errorf("HashKey is not stable: %q vs %q", a, b)
	}
	if len(a) != 12 {
		t.Errorf("HashKey returned %d chars; want 12", len(a))
	}
	// Different keys must hash differently.
	if HashKey("AIzaSyOtherKey") == a {
		t.Error("HashKey collided on distinct inputs")
	}
	// The hash must not contain the raw key.
	if strings.Contains(a, "AIzaSy") {
		t.Errorf("HashKey leaked raw key prefix in output: %q", a)
	}
}

func TestIsWriteMethod(t *testing.T) {
	writes := []string{"insert", "update", "delete", "set", "unset", "rate", "reportabuse", "setmoderationstatus"}
	for _, m := range writes {
		if !isWriteMethod(m) {
			t.Errorf("isWriteMethod(%q) = false; want true", m)
		}
	}
	reads := []string{"list", "get", "download", ""}
	for _, m := range reads {
		if isWriteMethod(m) {
			t.Errorf("isWriteMethod(%q) = true; want false", m)
		}
	}
}

func TestDailyDefault(t *testing.T) {
	if DailyDefault != 10000 {
		t.Errorf("DailyDefault = %d; want 10000 (documented YouTube Data API v3 default)", DailyDefault)
	}
}
