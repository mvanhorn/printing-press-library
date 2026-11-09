package mcp

import "testing"

// PATCH(reprint-637-renamed-command-aliases): every legacy code-orchestration id resolves to an endpoint that exists,
// including contacts.dnc.get-contacts-id (renamed by the 11-08 reprint).
func TestLegacyEndpointIDsResolveToLiveEndpoints(t *testing.T) {
	if _, ok := codeOrchLegacyEndpointIDs["contacts.dnc.get-contacts-id"]; !ok {
		t.Fatal("contacts.dnc.get-contacts-id has no legacy mapping")
	}
	for old, next := range codeOrchLegacyEndpointIDs {
		found := false
		for _, ep := range codeOrchEndpoints {
			if ep.ID == old {
				t.Errorf("legacy id %q is still a live endpoint id", old)
			}
			if ep.ID == next {
				found = true
			}
		}
		if !found {
			t.Errorf("legacy id %q maps to %q, which is not an endpoint", old, next)
		}
	}
}
