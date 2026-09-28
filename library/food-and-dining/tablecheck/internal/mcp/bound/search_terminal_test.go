package bound

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestEndpointPageResponseStopsAtExplicitTerminalSearchPage(t *testing.T) {
	for _, tc := range []struct {
		name       string
		lastPage   any
		wantCursor bool
	}{
		{"terminal", true, false},
		{"continuing", false, true},
		{"missing-indicator", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			meta := map[string]any{"search_after": "upstream-next"}
			if tc.lastPage != nil {
				meta["last_page"] = tc.lastPage
			}
			data, _ := json.Marshal(map[string]any{"shops": []map[string]any{{"id": "a"}, {"id": "b"}}, "meta": meta})
			output := EndpointPageResponse("GET", data, PageOptions{CursorParam: "search_after", NextCursorPath: "meta.search_after", TerminalPath: "meta.last_page"})
			var got struct {
				Shops      []json.RawMessage `json:"shops"`
				NextCursor string            `json:"next_cursor"`
				Meta       map[string]any    `json:"meta"`
			}
			if err := json.Unmarshal([]byte(output), &got); err != nil {
				t.Fatalf("invalid page JSON: %v: %s", err, output)
			}
			if len(got.Shops) != 2 || (got.NextCursor != "") != tc.wantCursor {
				t.Fatalf("terminal cursor decision lost data: %s", output)
			}
			if got.Meta["search_after"] != "upstream-next" {
				t.Fatalf("source cursor evidence removed: %s", output)
			}
			if tc.lastPage != nil && got.Meta["last_page"] != tc.lastPage {
				t.Fatalf("source terminal evidence removed: %s", output)
			}
		})
	}
}

func TestEndpointPageResponseTerminalSearchRetainsLocalSlices(t *testing.T) {
	shops := make([]map[string]string, MaxItems+1)
	for i := range shops {
		shops[i] = map[string]string{"id": fmt.Sprintf("shop-%d", i)}
	}
	data, _ := json.Marshal(map[string]any{"shops": shops, "meta": map[string]any{"search_after": "stale-source-token", "last_page": true}})
	opts := PageOptions{Cursor: encodeEndpointCursor(endpointCursor{Version: 1, UpstreamCursor: "previous-page"}), CursorParam: "search_after", NextCursorPath: "meta.search_after", TerminalPath: "meta.last_page"}
	first := EndpointPageResponse("GET", data, opts)
	var page1 struct {
		Shops      []json.RawMessage `json:"shops"`
		NextCursor string            `json:"next_cursor"`
	}
	if err := json.Unmarshal([]byte(first), &page1); err != nil || len(page1.Shops) != MaxItems || page1.NextCursor == "" {
		t.Fatalf("terminal source page must locally continue: %v %s", err, first)
	}
	upstream, err := UpstreamCursor(page1.NextCursor)
	if err != nil || upstream != "previous-page" {
		t.Fatalf("local cursor unexpectedly advances upstream: %q, %v", upstream, err)
	}
	opts.Cursor = page1.NextCursor
	second := EndpointPageResponse("GET", data, opts)
	var page2 struct {
		Shops      []map[string]string `json:"shops"`
		NextCursor string              `json:"next_cursor"`
	}
	if err := json.Unmarshal([]byte(second), &page2); err != nil || len(page2.Shops) != 1 || page2.Shops[0]["id"] != fmt.Sprintf("shop-%d", MaxItems) || page2.NextCursor != "" {
		t.Fatalf("terminal source page did not finish local slice: %v %s", err, second)
	}
}
