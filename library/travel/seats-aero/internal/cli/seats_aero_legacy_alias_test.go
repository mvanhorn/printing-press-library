package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestSeatsAeroLegacyAliasForwardsRealRequest(t *testing.T) {
	isolateNovelTest(t)
	queries := make([]url.Values, 0, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	t.Setenv("SEATS_AERO_BASE_URL", srv.URL)
	common := []string{"--origin-airport", "JFK", "--destination-airport", "NRT", "--only-direct-flights", "--take", "12", "--skip", "3", "--start-date", "2026-10-01", "--end-date", "2026-10-05", "--carriers", "DL", "--json", "--no-cache"}
	if _, _, err := executeRoot(append([]string{"seats-aero-partner-search", "--cabin", "business"}, common...)...); err != nil {
		t.Fatal(err)
	}
	if _, _, err := executeRoot(append([]string{"awards", "--cabins", "business"}, common...)...); err != nil {
		t.Fatal(err)
	}
	if len(queries) != 2 || !reflect.DeepEqual(queries[0], queries[1]) {
		t.Fatalf("queries=%v", queries)
	}
	q := queries[0]
	for key, want := range map[string]string{"cabins": "business", "only_direct_flights": "true", "take": "12", "skip": "3", "carriers": "DL"} {
		if q.Get(key) != want {
			t.Fatalf("%s=%q want %q query=%v", key, q.Get(key), want, q)
		}
	}
}

func TestSeatsAeroLegacyAliasMatchesAwardsDryRun(t *testing.T) {
	isolateNovelTest(t)
	legacyOut, legacyErr, err := executeRoot("seats-aero-partner-search", "--origin-airport", "JFK", "--destination-airport", "NRT", "--cabin", "business", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("legacy alias: %v (stderr=%q)", err, legacyErr.String())
	}
	awardsOut, awardsErr, err := executeRoot("awards", "--origin-airport", "JFK", "--destination-airport", "NRT", "--cabins", "business", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("awards: %v (stderr=%q)", err, awardsErr.String())
	}
	var legacyJSON, awardsJSON any
	legacyBytes := legacyOut.Bytes()
	if start := strings.IndexByte(string(legacyBytes), '{'); start >= 0 {
		legacyBytes = legacyBytes[start:]
	}
	if err := json.Unmarshal(legacyBytes, &legacyJSON); err != nil {
		t.Fatalf("decode legacy JSON %q: %v", legacyOut.String(), err)
	}
	if err := json.Unmarshal(awardsOut.Bytes(), &awardsJSON); err != nil {
		t.Fatalf("decode awards JSON %q: %v", awardsOut.String(), err)
	}
	if !reflect.DeepEqual(legacyJSON, awardsJSON) {
		t.Fatalf("legacy JSON = %#v, awards JSON = %#v", legacyJSON, awardsJSON)
	}
}

func TestSeatsAeroLegacyAliasAcceptsDepartureDate(t *testing.T) {
	isolateNovelTest(t)
	_, stderr, err := executeRoot("seats-aero-partner-search", "--origin-airport", "JFK", "--destination-airport", "NRT", "--departure-date", "2026-10-01", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("legacy alias: %v (stderr=%q)", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--departure-date is ignored") {
		t.Fatalf("stderr=%q, want ignored warning", stderr.String())
	}
}

func TestSeatsAeroLegacyAliasHiddenFromRootHelp(t *testing.T) {
	isolateNovelTest(t)
	stdout, stderr, err := executeRoot("--help")
	if err != nil {
		t.Fatalf("root help: %v (stderr=%q)", err, stderr.String())
	}
	if strings.Contains(stdout.String(), "seats-aero-partner-search") {
		t.Fatalf("root help exposes legacy alias: %q", stdout.String())
	}
}
