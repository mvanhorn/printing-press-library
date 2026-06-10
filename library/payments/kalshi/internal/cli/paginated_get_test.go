// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Regression tests for the audit-2026-06-09 pagination fixes: Kalshi envelope
// extraction, cursor-following defaults, and envelope-aware result counting.

package cli

import (
	"encoding/json"
	"fmt"
	"testing"
)

// fakePagedClient serves scripted pages keyed by the cursor param value.
type fakePagedClient struct {
	pages   map[string]string // cursor value ("" = first page) -> response body
	calls   []map[string]string
	headers []map[string]string
}

func (f *fakePagedClient) GetWithHeaders(path string, params map[string]string, headers map[string]string) (json.RawMessage, error) {
	cp := map[string]string{}
	for k, v := range params {
		cp[k] = v
	}
	f.calls = append(f.calls, cp)
	f.headers = append(f.headers, headers)
	body, ok := f.pages[params["cursor"]]
	if !ok {
		return nil, fmt.Errorf("GET %s returned HTTP 404: no page for cursor %q", path, params["cursor"])
	}
	return json.RawMessage(body), nil
}

// The headline bug: --all on a Kalshi envelope returned `null` because the
// items key ("fills") was unknown and the cursor was never followed (every
// call site passes nextCursorPath=""). Three pages must concatenate.
func TestPaginatedGet_KalshiEnvelopeFollowsCursor(t *testing.T) {
	f := &fakePagedClient{pages: map[string]string{
		"":   `{"fills":[{"fill_id":"a"},{"fill_id":"b"}],"cursor":"c2"}`,
		"c2": `{"fills":[{"fill_id":"c"}],"cursor":"c3"}`,
		"c3": `{"fills":[{"fill_id":"d"}],"cursor":""}`,
	}}
	// Call exactly like the generated call sites do: nextCursorPath and
	// hasMoreField empty.
	out, err := paginatedGet(f, "/portfolio/fills", map[string]string{"limit": "100"}, nil, true, "cursor", "", "")
	if err != nil {
		t.Fatalf("paginatedGet: %v", err)
	}
	var items []map[string]string
	if err := json.Unmarshal(out, &items); err != nil {
		t.Fatalf("output not an array: %v (raw: %s)", err, out)
	}
	if len(items) != 4 {
		t.Fatalf("got %d items, want 4 (raw: %s)", len(items), out)
	}
	if items[3]["fill_id"] != "d" {
		t.Fatalf("items out of order: %v", items)
	}
	if len(f.calls) != 3 {
		t.Fatalf("made %d requests, want 3", len(f.calls))
	}
	if f.calls[1]["cursor"] != "c2" || f.calls[2]["cursor"] != "c3" {
		t.Fatalf("cursor not threaded through requests: %v", f.calls)
	}
}

// market_positions must win over event_positions (both arrays in one envelope).
func TestPaginatedGet_PositionsEnvelopePrefersMarketPositions(t *testing.T) {
	f := &fakePagedClient{pages: map[string]string{
		"": `{"event_positions":[{"event_ticker":"E1"}],"market_positions":[{"ticker":"M1"},{"ticker":"M2"}],"cursor":""}`,
	}}
	out, err := paginatedGet(f, "/portfolio/positions", nil, nil, true, "cursor", "", "")
	if err != nil {
		t.Fatalf("paginatedGet: %v", err)
	}
	var items []map[string]string
	if err := json.Unmarshal(out, &items); err != nil {
		t.Fatalf("output not an array: %v", err)
	}
	if len(items) != 2 || items[0]["ticker"] != "M1" {
		t.Fatalf("want the 2 market_positions, got: %s", out)
	}
}

// A repeated (sticky) cursor must terminate, not loop forever.
func TestPaginatedGet_StickyCursorTerminates(t *testing.T) {
	f := &fakePagedClient{pages: map[string]string{
		"":      `{"markets":[{"ticker":"A"}],"cursor":"STUCK"}`,
		"STUCK": `{"markets":[{"ticker":"B"}],"cursor":"STUCK"}`,
	}}
	out, err := paginatedGet(f, "/markets", nil, nil, true, "cursor", "", "")
	if err != nil {
		t.Fatalf("paginatedGet: %v", err)
	}
	var items []map[string]string
	_ = json.Unmarshal(out, &items)
	if len(items) != 2 {
		t.Fatalf("sticky cursor: got %d items, want 2 (first page + one repeat)", len(items))
	}
	if len(f.calls) != 2 {
		t.Fatalf("sticky cursor: made %d requests, want 2", len(f.calls))
	}
}

// An empty result set must marshal to [], never null.
func TestPaginatedGet_EmptyIsArrayNotNull(t *testing.T) {
	f := &fakePagedClient{pages: map[string]string{
		"": `{"settlements":[],"cursor":""}`,
	}}
	out, err := paginatedGet(f, "/portfolio/settlements", nil, nil, true, "cursor", "", "")
	if err != nil {
		t.Fatalf("paginatedGet: %v", err)
	}
	if string(out) != "[]" {
		t.Fatalf("empty result = %q, want []", string(out))
	}
}

// Unknown envelope shapes pass through raw rather than fabricating emptiness.
func TestPaginatedGet_UnknownEnvelopePassesThrough(t *testing.T) {
	body := `{"balance":123456,"portfolio_value":7890}`
	f := &fakePagedClient{pages: map[string]string{"": body}}
	out, err := paginatedGet(f, "/portfolio/balance", nil, nil, true, "cursor", "", "")
	if err != nil {
		t.Fatalf("paginatedGet: %v", err)
	}
	if string(out) != body {
		t.Fatalf("unknown envelope: got %s, want raw passthrough", out)
	}
}

func TestCountResultItems(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"bare array", `[{"a":1},{"a":2}]`, 2},
		{"kalshi fills envelope", `{"fills":[{"a":1},{"a":2},{"a":3}],"cursor":"x"}`, 3},
		{"single-array-key fallback", `{"weird_things":[{"a":1}],"cursor":"x"}`, 1},
		{"plain object", `{"balance":100}`, 1},
		{"empty array", `[]`, 0},
	}
	for _, c := range cases {
		if got := countResultItems(json.RawMessage(c.in)); got != c.want {
			t.Errorf("%s: countResultItems = %d, want %d", c.name, got, c.want)
		}
	}
}

// 409 on a GET must surface as an error; on a create it stays a no-op success.
func TestClassifyAPIError_409(t *testing.T) {
	getErr := fmt.Errorf("GET /portfolio/fills returned HTTP 409: conflict")
	if err := classifyAPIError(getErr); err == nil {
		t.Fatalf("409 on GET was swallowed as success")
	}
	postErr := fmt.Errorf("POST /portfolio/orders returned HTTP 409: already exists")
	if err := classifyAPIError(postErr); err != nil {
		t.Fatalf("409 on POST should be a no-op success, got: %v", err)
	}
}

func TestSinceParamValue(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"1750000000", "1750000000"},
		{"2026-06-09T08:00:00Z", "1780992000"},
		{"2026-06-09", "1780963200"},
		{"not-a-time", ""},
	}
	for _, c := range cases {
		if got := sinceParamValue(c.in); got != c.want {
			t.Errorf("sinceParamValue(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
