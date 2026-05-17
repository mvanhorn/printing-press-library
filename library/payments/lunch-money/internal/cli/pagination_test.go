package cli

import (
	"encoding/json"
	"testing"
)

type fakePagingClient struct {
	responses []json.RawMessage
	calls     []map[string]string
}

func (f *fakePagingClient) GetWithHeaders(_ string, params map[string]string, _ map[string]string) (json.RawMessage, error) {
	copied := map[string]string{}
	for k, v := range params {
		copied[k] = v
	}
	f.calls = append(f.calls, copied)
	if len(f.calls) > len(f.responses) {
		return json.RawMessage(`{"transactions":[],"has_more":false}`), nil
	}
	return f.responses[len(f.calls)-1], nil
}

// PATCH: has_more-only offset pagination must advance offset between pages.
func TestPaginatedGetAdvancesOffsetWhenHasMore(t *testing.T) {
	fake := &fakePagingClient{responses: []json.RawMessage{
		json.RawMessage(`{"transactions":[{"id":1}],"has_more":true}`),
		json.RawMessage(`{"transactions":[{"id":2}],"has_more":false}`),
	}}

	raw, err := paginatedGet(fake, "/transactions", map[string]string{
		"limit":  "1",
		"offset": "0",
	}, nil, true, "offset", "", "has_more")
	if err != nil {
		t.Fatalf("paginatedGet: %v", err)
	}
	if len(fake.calls) != 2 {
		t.Fatalf("calls = %#v, want two pages", fake.calls)
	}
	if fake.calls[0]["offset"] != "0" || fake.calls[1]["offset"] != "1" {
		t.Fatalf("offset calls = %#v, want 0 then 1", fake.calls)
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	if len(rows) != 2 || rows[0]["id"] != float64(1) || rows[1]["id"] != float64(2) {
		t.Fatalf("rows = %#v", rows)
	}
}
