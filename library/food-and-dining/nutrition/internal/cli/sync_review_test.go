// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestFoodsSyncUsesPagePagination(t *testing.T) {
	defaults := determinePaginationDefaults("foods")
	if !resourceSupportsPagination("foods") {
		t.Fatal("foods must be marked paginated")
	}
	if defaults.cursorType != "page" || defaults.cursorParam != "pageNumber" || defaults.limitParam != "pageSize" {
		t.Fatalf("foods pagination defaults = %#v", defaults)
	}
}
