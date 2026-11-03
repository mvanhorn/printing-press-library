// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/url"
	"reflect"
	"testing"
)

func TestSavedProfileMutationFilters(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values map[string]string
		args   []string
		want   url.Values
	}{
		{"organization zero limit", map[string]string{"query": "is:resolved", "limit": "0"},
			[]string{"organizations", "issues", "bulk-remove-an-organization-s", "test-org"},
			url.Values{"query": {"is:resolved"}, "limit": {"0"}}},
		{"organization saved empty query", map[string]string{"query": "", "limit": "0"},
			[]string{"organizations", "issues", "bulk-remove-an-organization-s", "test-org"},
			url.Values{"query": {""}, "limit": {"0"}}},
		{"organization empty override", map[string]string{"query": "is:resolved", "limit": "7"},
			[]string{"organizations", "issues", "bulk-remove-an-organization-s", "test-org", "--query", ""},
			url.Values{"query": {""}, "limit": {"7"}}},
		{"project selectors", map[string]string{"id": `["10","11"]`, "status": "resolved"},
			[]string{"projects", "issues", "bulk-mutate-a-list-of", "test-org", "test-project"},
			url.Values{"id": {"10", "11"}, "status": {"resolved"}}},
		{"project explicit overrides", map[string]string{"id": `["10","11"]`, "status": "resolved"},
			[]string{"projects", "issues", "bulk-mutate-a-list-of", "test-org", "test-project", "--id", "12", "--status", ""},
			url.Values{"id": {"12"}, "status": {""}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := recordRequests(t, `{}`)
			if err := saveProfileStore(&profileStore{Profiles: map[string]Profile{
				"saved": {Name: "saved", Values: tc.values},
			}}); err != nil {
				t.Fatal(err)
			}
			runCLI(t, append(tc.args, "--profile", "saved", "--yes", "--no-input", "--json")...)
			got := requests()
			if len(got) != 1 {
				t.Fatalf("requests = %d, want 1", len(got))
			}
			if query := got[0].URL.Query(); !reflect.DeepEqual(query, tc.want) {
				t.Fatalf("profile filters = %v, want %v", query, tc.want)
			}
		})
	}
}
