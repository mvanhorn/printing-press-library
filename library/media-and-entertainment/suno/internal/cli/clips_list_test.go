// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"testing"
)

func TestDrainAllClips_CollectsEveryPage(t *testing.T) {
	pages := map[string]feedPage{
		"":   {Clips: raws("a", "b"), NextCursor: "c1", HasMore: true},
		"c1": {Clips: raws("c", "d"), NextCursor: "c2", HasMore: true},
		"c2": {Clips: raws("e"), NextCursor: "", HasMore: false},
	}
	all, err := drainAllClips(context.Background(), fakeFetcher(pages), 2, "")
	if err != nil {
		t.Fatalf("drainAllClips: %v", err)
	}
	if len(all) != 5 {
		t.Errorf("drained %d clips, want 5 (regression: --all must walk all pages)", len(all))
	}
}
