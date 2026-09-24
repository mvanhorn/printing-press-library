package store

import (
	"encoding/json"
	"testing"
)

// A search result must not erase what enrich stored (Greptile, PR #2048).
func TestMergeListingJSONKeepsEnrichedFields(t *testing.T) {
	prev := []byte(`{"zimmo_code":"LRZA4","price":600000,"description":"long text","documents":["a.pdf"],"price_history":[{"p":620000}],"rented":true,"epc":"G"}`)
	next := []byte(`{"zimmo_code":"LRZA4","price":575000,"description":"","documents":[],"rented":false}`)
	var got map[string]any
	if err := json.Unmarshal(mergeListingJSON(prev, next), &got); err != nil {
		t.Fatal(err)
	}
	if got["price"].(float64) != 575000 || got["rented"].(bool) {
		t.Errorf("fresh search values must win: %v", got)
	}
	if got["description"] != "long text" || got["epc"] != "G" || len(got["documents"].([]any)) != 1 || got["price_history"] == nil {
		t.Errorf("enriched fields must survive: %v", got)
	}
	if string(mergeListingJSON([]byte("not json"), next)) != string(next) {
		t.Error("invalid prev must return next")
	}
}
