package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPlannerAuthMetadataBindingContract(t *testing.T) {
	got := PlannerAuthMetadata()
	if got["type"] != "oauth2_refresh" || got["access_token_storage"] != "memory only" {
		t.Fatalf("wrong authentication metadata: %v", got)
	}
	auth := plannerAgentAuth()
	if len(auth.EnvVars) != 6 {
		t.Fatalf("got %d bindings", len(auth.EnvVars))
	}
	tests := []struct {
		name                string
		required, sensitive bool
	}{
		{"GOOGLE_ADS_CLIENT_SECRET", true, true},
		{"GOOGLE_ADS_REFRESH_TOKEN", true, true},
		{"GOOGLE_ADS_DEVELOPER_TOKEN", true, true},
		{"GOOGLE_ADS_CUSTOMER_ID", true, false},
		{"GOOGLE_ADS_LOGIN_CUSTOMER_ID", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := false
			for _, v := range auth.EnvVars {
				if v.Name == tt.name {
					found = true
					if v.Required != tt.required || v.Sensitive != tt.sensitive {
						t.Fatalf("wrong binding classification: %+v", v)
					}
				}
			}
			if !found {
				t.Fatal("missing binding")
			}
		})
	}
}

func TestPlannerAuthMetadataNeverReadsValues(t *testing.T) {
	t.Setenv("GOOGLE_ADS_REFRESH_TOKEN", "private-fixture-never-render")
	raw, err := json.Marshal(PlannerAuthMetadata())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "private-fixture-never-render") || strings.Contains(string(raw), "KEYWORD_PLANNER_ACCESS_TOKEN") {
		t.Fatalf("credential value or inactive binding exposed: %s", raw)
	}
}

func TestPlannerCoreCapabilitiesDiscoverable(t *testing.T) {
	for _, tt := range []struct{ query, command string }{
		{"ideas", "ideas"}, {"historical", "historical"},
		{"portfolio export", "portfolio export"}, {"doctor", "doctor"},
	} {
		t.Run(tt.query, func(t *testing.T) {
			got := rankWhich(whichIndex, tt.query, 1)
			if len(got) != 1 || got[0].Entry.Command != tt.command {
				t.Fatalf("query %q yielded %+v", tt.query, got)
			}
		})
	}
	if got := rankWhich(whichIndex, "zxqv91873", 3); len(got) != 0 {
		t.Fatalf("irrelevant query yielded %+v", got)
	}
}
