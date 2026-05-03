// Tests for the hand-written transcendence helpers.

package cli

import "testing"

func TestExtractFromData_StringField(t *testing.T) {
	blob := []byte(`{"url":"https://example.com","clicks":42}`)
	if got := extractFromData(blob, "url"); got != "https://example.com" {
		t.Errorf("extractFromData(url) = %q, want %q", got, "https://example.com")
	}
}

func TestExtractFromData_NestedPath(t *testing.T) {
	blob := []byte(`{"meta":{"source":"live"},"data":{"id":"abc"}}`)
	if got := extractFromData(blob, "data.id"); got != "abc" {
		t.Errorf("extractFromData(data.id) = %q, want %q", got, "abc")
	}
	if got := extractFromData(blob, "meta.source"); got != "live" {
		t.Errorf("extractFromData(meta.source) = %q, want %q", got, "live")
	}
}

func TestExtractFromData_MissingPath_Empty(t *testing.T) {
	blob := []byte(`{"a":1}`)
	if got := extractFromData(blob, "b"); got != "" {
		t.Errorf("extractFromData(missing) = %q, want empty", got)
	}
	if got := extractFromData(blob, "a.b.c"); got != "" {
		t.Errorf("extractFromData(deep-missing) = %q, want empty", got)
	}
}

func TestExtractFromData_NumericFieldStringified(t *testing.T) {
	blob := []byte(`{"n":3.14}`)
	if got := extractFromData(blob, "n"); got != "3.14" {
		t.Errorf("extractFromData(numeric) = %q, want 3.14", got)
	}
}

func TestExtractFromData_BooleanFieldStringified(t *testing.T) {
	blob := []byte(`{"flag":true}`)
	if got := extractFromData(blob, "flag"); got != "true" {
		t.Errorf("extractFromData(bool true) = %q, want true", got)
	}
	blob = []byte(`{"flag":false}`)
	if got := extractFromData(blob, "flag"); got != "false" {
		t.Errorf("extractFromData(bool false) = %q, want false", got)
	}
}

func TestExtractFromData_BadJSON_Empty(t *testing.T) {
	if got := extractFromData([]byte(`not json`), "any"); got != "" {
		t.Errorf("extractFromData(bad json) = %q, want empty", got)
	}
	if got := extractFromData(nil, "any"); got != "" {
		t.Errorf("extractFromData(nil) = %q, want empty", got)
	}
}

func TestExtractNumber_Happy(t *testing.T) {
	blob := []byte(`{"clicks":100,"leads":7}`)
	if got := extractNumber(blob, "clicks"); got != 100 {
		t.Errorf("extractNumber(clicks) = %v, want 100", got)
	}
	if got := extractNumber(blob, "leads"); got != 7 {
		t.Errorf("extractNumber(leads) = %v, want 7", got)
	}
}

func TestExtractNumber_NestedAndMissing(t *testing.T) {
	blob := []byte(`{"metrics":{"clicks":42}}`)
	if got := extractNumber(blob, "metrics.clicks"); got != 42 {
		t.Errorf("extractNumber(metrics.clicks) = %v, want 42", got)
	}
	if got := extractNumber(blob, "missing"); got != 0 {
		t.Errorf("extractNumber(missing) = %v, want 0", got)
	}
}

func TestExtractNumber_NonNumeric_Zero(t *testing.T) {
	blob := []byte(`{"name":"alice"}`)
	if got := extractNumber(blob, "name"); got != 0 {
		t.Errorf("extractNumber(string field) = %v, want 0", got)
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in  string
		ok  bool
		hrs float64 // expected duration in hours
	}{
		{"24h", true, 24},
		{"7d", true, 168},
		{"1w", true, 168},
		{"30m", true, 0.5},
		{"", false, 0},
		{"abc", false, 0},
		{"12", false, 0}, // missing unit
	}
	for _, c := range cases {
		got, ok := parseDuration(c.in)
		if ok != c.ok {
			t.Errorf("parseDuration(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if !ok {
			continue
		}
		gotHrs := got.Hours()
		if gotHrs != c.hrs {
			t.Errorf("parseDuration(%q) = %v hours, want %v", c.in, gotHrs, c.hrs)
		}
	}
}
