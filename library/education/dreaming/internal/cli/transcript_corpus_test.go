// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/education/dreaming/internal/store"
)

func TestMergeCorpusCountsFailedVideoUpserts(t *testing.T) {
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "corpus.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	res := mergeCorpus(db, &corpusFile{Videos: []json.RawMessage{
		json.RawMessage(`{"id":"valid-video","title":"Valid"}`),
		json.RawMessage(`{"title":"Missing ID"}`),
	}})

	if res.err != nil {
		t.Fatalf("mergeCorpus: %v", res.err)
	}
	if res.videosUpserted != 1 || res.videosFailed != 1 {
		t.Fatalf("got %d upserted and %d failed, want 1 and 1", res.videosUpserted, res.videosFailed)
	}
}
