// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"testing"
)

// seedClips writes clip rows into the generic resources table that the
// restored read-commands (burn/sessions/tree) query (resource_type='clips'),
// routed into an isolated temp HOME so tests never touch the real store.
func seedClips(t *testing.T, clips []map[string]any) context.Context {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	ctx := context.Background()
	s, err := openDefaultStore(ctx)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()
	for _, c := range clips {
		id, _ := c["id"].(string)
		if err := s.Upsert("clips", id, mustJSON(c)); err != nil {
			t.Fatalf("seed clip %s: %v", id, err)
		}
	}
	return ctx
}
