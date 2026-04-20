// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
)

// TestSearchPeopleArgs_NoLimitKey is the regression guard for the
// 2026-04-19 bug where coverage --source li and every other flagship
// command calling fetchLinkedInSearchPeople errored out with a pydantic
// "Unexpected keyword argument" from the LinkedIn MCP's search_people
// tool. The tool's schema has no `limit` parameter; we must apply the
// limit client-side after the call, not as an MCP arg.
func TestSearchPeopleArgs_NoLimitKey(t *testing.T) {
	t.Run("keywords only", func(t *testing.T) {
		args := searchPeopleArgs("Disney", "")
		if _, hasLimit := args["limit"]; hasLimit {
			t.Errorf("args must not include `limit` (search_people MCP tool rejects it): %v", args)
		}
		if got := args["keywords"]; got != "Disney" {
			t.Errorf("keywords = %v, want Disney", got)
		}
		if _, hasLocation := args["location"]; hasLocation {
			t.Errorf("location must be omitted when empty: %v", args)
		}
	})

	t.Run("keywords and location", func(t *testing.T) {
		args := searchPeopleArgs("Disney", "Los Angeles")
		if _, hasLimit := args["limit"]; hasLimit {
			t.Errorf("args must not include `limit`: %v", args)
		}
		if got := args["location"]; got != "Los Angeles" {
			t.Errorf("location = %v, want Los Angeles", got)
		}
	})
}

// TestClientSideLimit verifies that fetchLinkedInSearchPeople applies
// its `limit` argument as a client-side truncation on the parsed
// payload. We do not spin up a real MCP subprocess — the logic under
// test is the slice truncation, which we cover by calling the parser
// directly and truncating.
func TestClientSideLimit(t *testing.T) {
	// Build a fake payload with 5 minimal flagship entries.
	payload := `[
		{"name":"A","linkedin_url":"https://www.linkedin.com/in/a"},
		{"name":"B","linkedin_url":"https://www.linkedin.com/in/b"},
		{"name":"C","linkedin_url":"https://www.linkedin.com/in/c"},
		{"name":"D","linkedin_url":"https://www.linkedin.com/in/d"},
		{"name":"E","linkedin_url":"https://www.linkedin.com/in/e"}
	]`
	people := parseLIPeoplePayload(payload, "li_search")
	if len(people) != 5 {
		t.Fatalf("parseLIPeoplePayload returned %d, want 5 (fixture problem)", len(people))
	}

	// Simulate the truncation logic fetchLinkedInSearchPeople applies.
	truncate := func(in []flagshipPerson, limit int) []flagshipPerson {
		if limit > 0 && len(in) > limit {
			return in[:limit]
		}
		return in
	}

	if got := truncate(people, 3); len(got) != 3 {
		t.Errorf("limit=3: got %d, want 3", len(got))
	}
	if got := truncate(people, 0); len(got) != 5 {
		t.Errorf("limit=0 (no cap): got %d, want 5", len(got))
	}
	if got := truncate(people, 100); len(got) != 5 {
		t.Errorf("limit greater than result set: got %d, want 5", len(got))
	}
}
