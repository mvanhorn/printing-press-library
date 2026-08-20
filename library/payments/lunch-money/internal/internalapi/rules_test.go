package internalapi

import (
	"net/http"
	"testing"
)

// PATCH: Cover finding a single rule by criteria_id through the captured list endpoint.
func TestGetRuleFindsCriteriaID(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/rules" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"total_returned": 2,
			"rules": [
				{"rule_id": 1, "rule_criteria_id": 11, "criteria_priority": 1},
				{"rule_id": 2, "rule_criteria_id": 22, "criteria_priority": 2}
			]
		}`))
	})
	defer done()

	rule, err := c.GetRule(22)
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if rule.RuleID != 2 || rule.RuleCriteriaID != 22 {
		t.Fatalf("rule = %+v, want criteria 22", rule)
	}
}
