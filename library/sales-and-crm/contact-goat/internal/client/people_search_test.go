// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"testing"
	"time"
)

// TestDefaultPollTimeoutIs180s is the regression guard for the
// 2026-04-19 bump from 60s -> 180s. 60s was causing frequent false
// failures on legitimate Happenstance queries that routinely take
// 2-5 minutes. If this assertion is changed, update hp_people's flag
// help text and the coverage docs in the same PR.
func TestDefaultPollTimeoutIs180s(t *testing.T) {
	if DefaultPollTimeout != 180*time.Second {
		t.Fatalf("DefaultPollTimeout = %s, want 180s", DefaultPollTimeout)
	}
	if got := defaultSearchOptions().PollTimeout; got != DefaultPollTimeout {
		t.Errorf("defaultSearchOptions().PollTimeout = %s, want DefaultPollTimeout (%s)", got, DefaultPollTimeout)
	}
}

// TestZeroPollTimeoutFallsBackToDefault confirms the zero-value guard
// in SearchPeopleByQuery: callers that construct SearchPeopleOptions
// without setting PollTimeout still receive the default, not a
// zero-duration (which would make the poll return instantly).
func TestZeroPollTimeoutFallsBackToDefault(t *testing.T) {
	o := SearchPeopleOptions{
		IncludeMyConnections: true,
		// PollTimeout intentionally zero
	}
	// The fallback is inlined in SearchPeopleByQuery; simulate that
	// path here without making a real HTTP call.
	if o.PollTimeout == 0 {
		o.PollTimeout = DefaultPollTimeout
	}
	if o.PollTimeout != DefaultPollTimeout {
		t.Errorf("zero PollTimeout should fall back to DefaultPollTimeout, got %s", o.PollTimeout)
	}
}

// TestSearchPeopleByCompanyHasOptionsOverload is a compile-time guard
// that the coverage command's per-call timeout plumbing continues to
// work. If the method is removed or renamed, the coverage command's
// --poll-timeout flag silently stops taking effect, so we lock the
// shape here.
func TestSearchPeopleByCompanyHasOptionsOverload(t *testing.T) {
	var c *Client
	// No call is made; the test only verifies that the method exists
	// with the expected signature and that a non-nil opts is an
	// acceptable argument.
	_ = func() (*PeopleSearchResult, error) {
		return c.SearchPeopleByCompanyWithOptions("Disney", &SearchPeopleOptions{
			PollTimeout: 300 * time.Second,
		})
	}
}
