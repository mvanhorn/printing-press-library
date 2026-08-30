// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "testing"

func TestBatchUpdateBookmarksUsesAllCollectionsPath(t *testing.T) {
	t.Parallel()
	cmd := newRaindropsBatchUpdateBookmarksCmd(&rootFlags{})
	if got := cmd.Annotations["pp:path"]; got != "/raindrops/0" {
		t.Fatalf("batch update path = %q, want /raindrops/0", got)
	}
}
