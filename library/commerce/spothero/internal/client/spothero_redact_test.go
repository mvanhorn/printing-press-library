package client

import (
	"strings"
	"testing"
)

func TestStripAccessKey(t *testing.T) {
	in := `{"results":[{"rental_id":12345678901234567,"access_key":"secret","nested":{"access_key":"secret2","keep":1}}]}`
	got := string(stripAccessKey([]byte(in)))
	if strings.Contains(got, "access_key") || strings.Contains(got, "secret") {
		t.Fatalf("secret survived: %s", got)
	}
	// Reservation IDs are large, so they must not pass through a float.
	if !strings.Contains(got, "12345678901234567") || !strings.Contains(got, `"keep":1`) {
		t.Fatalf("other fields changed: %s", got)
	}
}

func TestStripAccessKeyLeavesOtherBodiesAlone(t *testing.T) {
	for _, in := range []string{`{"a":1}`, `not json "access_key"`} {
		if got := string(stripAccessKey([]byte(in))); got != in {
			t.Fatalf("changed %q to %q", in, got)
		}
	}
}
