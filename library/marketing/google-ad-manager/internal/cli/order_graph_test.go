// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestTailSegment(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"networks/123/orders/456", "456"},
		{"networks/123/lineItems/789", "789"},
		{"456", "456"},
		{"", ""},
		{"  networks/1/adUnits/2  ", "2"},
		{"trailing/", ""},
	}
	for _, c := range cases {
		if got := tailSegment(c.in); got != c.want {
			t.Errorf("tailSegment(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "  ", "x", "y"); got != "x" {
		t.Errorf("firstNonEmpty = %q, want x", got)
	}
	if got := firstNonEmpty("", "   "); got != "" {
		t.Errorf("firstNonEmpty(all-empty) = %q, want empty", got)
	}
}

func TestParseOrderLineItem(t *testing.T) {
	raw := []byte(`{
		"name": "networks/123/lineItems/6789121812",
		"displayName": "Holiday Promo",
		"lineItemType": "STANDARD",
		"goal": {"goalType": "LIFETIME", "units": "100000"},
		"startTime": "2026-01-01T00:00:00Z",
		"endTime": {"date": {"year": 2026, "month": 2, "day": 1}}
	}`)
	node := parseOrderLineItem(raw)
	if node.ID != "6789121812" {
		t.Errorf("ID = %q, want 6789121812", node.ID)
	}
	if node.Name != "Holiday Promo" {
		t.Errorf("Name = %q, want Holiday Promo", node.Name)
	}
	if node.LineItemType != "STANDARD" {
		t.Errorf("LineItemType = %q, want STANDARD", node.LineItemType)
	}
	if len(node.Goal) == 0 {
		t.Error("Goal should be populated")
	}
	// startTime is a JSON string; endTime is a JSON object — both pass through raw.
	if string(node.StartTime) != `"2026-01-01T00:00:00Z"` {
		t.Errorf("StartTime = %s, want the raw string literal", node.StartTime)
	}
	if len(node.EndTime) == 0 {
		t.Error("EndTime (object shape) should pass through")
	}
}

func TestParseOrderLineItemEmpty(t *testing.T) {
	node := parseOrderLineItem([]byte(`{"name":"networks/1/lineItems/9"}`))
	if node.ID != "9" {
		t.Errorf("ID = %q, want 9", node.ID)
	}
	if node.Goal != nil || node.StartTime != nil || node.EndTime != nil {
		t.Error("absent optional fields should stay nil (omitted from output)")
	}
}
