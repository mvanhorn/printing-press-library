package cli

import (
	"encoding/json"
	"testing"
)

func TestSendfoxPageMetadata(t *testing.T) {
	for _, tc := range []struct {
		body, next string
		more       bool
	}{
		{`{"current_page":1,"last_page":2,"per_page":20}`, "2", true},
		{`{"current_page":2,"last_page":2,"per_page":20}`, "", false},
		{`{"current_page":1,"last_page":1,"data":[]}`, "", false},
		{`{"current_page":1,"total":45,"per_page":20}`, "2", true},
		{`{"current_page":2,"total":45,"per_page":20}`, "3", true},
		{`{"current_page":3,"total":45,"per_page":20}`, "", false},
		{`{"current_page":1,"total":0,"per_page":20}`, "", false},
	} {
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(tc.body), &m); err != nil {
			t.Fatal(err)
		}
		next, more := extractPaginationFromEnvelope(m, "page", "")
		if next != tc.next || more != tc.more {
			t.Fatalf("%s => %s %v", tc.body, next, more)
		}
	}
}
