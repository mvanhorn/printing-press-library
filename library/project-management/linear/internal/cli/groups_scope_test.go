// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

// Project-level arithmetic (projects burndown, milestones at-risk) resolves
// its "delivered" group at the project's own team scope. A project belongs to
// exactly one team in the common case, and only that case may pick up a
// team_state_groups override: a project spanning teams has no single correct
// override, so it must fall back to workspace scope.
func TestProjectTeamKeyForGroups(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "single team yields its key",
			raw:  `{"id":"p1","name":"Migration","teams":{"nodes":[{"id":"t1","key":"ENG","name":"Engineering"}]}}`,
			want: "ENG",
		},
		{
			name: "two teams are ambiguous, workspace scope wins",
			raw:  `{"id":"p1","teams":{"nodes":[{"key":"ENG"},{"key":"OPS"}]}}`,
			want: "",
		},
		{
			name: "no teams recorded",
			raw:  `{"id":"p1","teams":{"nodes":[]}}`,
			want: "",
		},
		{
			name: "teams key absent entirely",
			raw:  `{"id":"p1","name":"Legacy row synced before teams were selected"}`,
			want: "",
		},
		{
			name: "blank key is not a team scope",
			raw:  `{"id":"p1","teams":{"nodes":[{"key":"   "}]}}`,
			want: "",
		},
		{
			name: "malformed payload never panics",
			raw:  `{"teams":"not-an-object"}`,
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := projectTeamKeyForGroups(json.RawMessage(tc.raw)); got != tc.want {
				t.Fatalf("projectTeamKeyForGroups() = %q, want %q", got, tc.want)
			}
		})
	}
}
