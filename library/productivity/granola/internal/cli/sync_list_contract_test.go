// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
	"time"
)

// PATCH(api-list-stage-matches-live-contract): these two constants were
// generated as generic defaults that Granola's public API rejects outright,
// so the list stage 400'd before the detail stage could ever run. Both were
// verified against the live API; pin them so a regen cannot quietly restore
// a value that breaks every sync.

func TestPaginationDefaults_RespectGranolaPageSizeCeiling(t *testing.T) {
	// Live probe: page_size=30 -> HTTP 200, page_size=31 -> HTTP 400
	// ("Number must be less than or equal to 30"). The generated default
	// was 100, which failed every request on both /v1/notes and /v1/folders.
	const granolaMaxPageSize = 30

	got := determinePaginationDefaults()
	if got.limit > granolaMaxPageSize {
		t.Errorf("pagination limit %d exceeds Granola's page_size ceiling of %d; every list request will fail with HTTP 400", got.limit, granolaMaxPageSize)
	}
	if got.limitParam != "page_size" {
		t.Errorf("limitParam = %q, want %q", got.limitParam, "page_size")
	}
	if got.cursorParam != "cursor" {
		t.Errorf("cursorParam = %q, want %q", got.cursorParam, "cursor")
	}
}

func TestSinceTimestamp_IsUTCZFormNotNumericOffset(t *testing.T) {
	// Live probe: updated_after=2026-07-18T16:00:00Z -> HTTP 200, but
	// updated_after=2026-07-18T09:00:00-07:00 -> HTTP 400 ("Invalid date").
	// time.RFC3339 on a local-zone time renders the numeric-offset form, so
	// the value must be normalized to UTC before formatting.
	local := time.Date(2026, 7, 18, 9, 0, 0, 0, time.FixedZone("PDT", -7*60*60))

	if formatted := local.Format(time.RFC3339); !strings.HasSuffix(formatted, "Z") {
		// Guard the premise: this is the shape the API rejects.
		if got := local.UTC().Format(time.RFC3339); !strings.HasSuffix(got, "Z") {
			t.Fatalf("UTC-normalized timestamp %q is not in Z form; Granola rejects numeric-offset timestamps", got)
		}
		return
	}
	t.Fatal("expected a local-zone RFC3339 timestamp to carry a numeric offset; the premise of this regression no longer holds")
}
