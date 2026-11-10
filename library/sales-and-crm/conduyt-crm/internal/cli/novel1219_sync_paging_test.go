// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "testing"

// PATCH(novel1219-sync-activities-deals): Greptile PR #2450 round 3. GET /activities pages by number and has no
// next cursor; deals have no last-changed filter, so a created_after lower bound would leave changed deals stale.
func TestNovel1219ActivitiesSyncAndListPageByNumber(t *testing.T) {
	d := determinePaginationDefaults("activities")
	if d.cursorType != "page" || d.cursorParam != "page" || d.initialCursor != "1" || d.limitParam != "per_page" {
		t.Fatalf("activities sync paging = %+v, want page/page/1/per_page", d)
	}
	cfg := resourceReadConfigs["activities"]
	if cfg.paginationType != "page" || cfg.cursorParam != "page" || cfg.limitParam != "per_page" {
		t.Fatalf("activities read config = %+v, want page paging", cfg)
	}
}

func TestNovel1219DealsSyncHasNoCreatedAfterLowerBound(t *testing.T) {
	if got := syncResourceSinceParam("deals"); got != "" {
		t.Fatalf("deals since param = %q, want none (full paging every sync)", got)
	}
}
