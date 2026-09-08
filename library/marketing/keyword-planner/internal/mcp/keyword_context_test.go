package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestKeywordPlannerContextPolicy(t *testing.T) {
	t.Setenv("KEYWORD_PLANNER_DB", "/private/fixture-portfolio.db")
	got := keywordPlannerContext()
	for _, tt := range []struct {
		key  string
		want any
	}{
		{"api_version", "v25"}, {"transport", "REST"}, {"remote_mutations", false},
		{"portfolio_db", "/private/fixture-portfolio.db"},
	} {
		if got[tt.key] != tt.want {
			t.Fatalf("%s = %v", tt.key, got[tt.key])
		}
	}
	methods := got["remote_methods"].([]string)
	if len(methods) != 2 {
		t.Fatalf("unexpected remote scope: %v", methods)
	}
}

func TestKeywordPlannerContextNoCredentialValues(t *testing.T) {
	t.Setenv("GOOGLE_ADS_CLIENT_SECRET", "never-emit-private-fixture")
	b, err := json.Marshal(keywordPlannerContext())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "never-emit-private-fixture") || strings.Contains(string(b), "KEYWORD_PLANNER_ACCESS_TOKEN") {
		t.Fatalf("inactive or sensitive metadata: %s", b)
	}
}
