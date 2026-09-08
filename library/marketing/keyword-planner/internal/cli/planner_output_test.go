package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPlannerOutputPreservesInt64AndNull(t *testing.T) {
	value := int64(9007199254740993)
	rows := []map[string]any{{"keyword": "ribeye", "month": "2026-07-01", "monthly_searches": value, "low_bid_micros": nil}}
	for _, tt := range []struct {
		name  string
		flags rootFlags
		json  bool
	}{
		{"json", rootFlags{asJSON: true}, true},
		{"select", rootFlags{asJSON: true, selectFields: "keyword,month,monthly_searches,low_bid_micros"}, true},
		{"compact", rootFlags{asJSON: true, compact: true}, true},
		{"agent", rootFlags{asJSON: true, agent: true, compact: true}, true},
		{"csv", rootFlags{csv: true}, false},
		{"plain", rootFlags{plain: true}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := printJSONFiltered(&out, rows, &tt.flags); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), "9007199254740993") || strings.Contains(out.String(), "9007199254740992") {
				t.Fatalf("integer rounded or lost: %s", &out)
			}
			if tt.json {
				if !json.Valid(out.Bytes()) || !strings.Contains(out.String(), "null") {
					t.Fatalf("invalid JSON or null lost: %s", &out)
				}
			}
		})
	}
}

func TestPlannerOutputDecodeRejectsTrailingValue(t *testing.T) {
	for _, raw := range []string{"{} {}", "[] true", "{invalid"} {
		var dst any
		if err := decodeOutputJSON([]byte(raw), &dst); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
