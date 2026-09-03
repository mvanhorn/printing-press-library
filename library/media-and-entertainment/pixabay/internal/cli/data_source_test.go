// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored tests for Pixabay write-through list-wrapper recognition.

package cli

import (
	"encoding/json"
	"testing"
)

// TestExtractWriteThroughListItems_PixabayHitsEnvelope pins the auto-mode
// write-through extractor against the live Pixabay search envelope
// ({total,totalHits,hits:[...]}). Without "hits" in writeThroughListWrapperKeys
// the envelope is treated as one ID-less object and skipped, so live results
// never land in the local store for later local/offline search.
func TestExtractWriteThroughListItems_PixabayHitsEnvelope(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  string
		want int
	}{
		{
			name: "lowercase hits",
			raw:  `{"total":2,"totalHits":2,"hits":[{"id":195893,"tags":"blossom"},{"id":3063284,"tags":"tree"}]}`,
			want: 2,
		},
		{
			name: "PascalCase Hits",
			raw:  `{"Total":1,"TotalHits":1,"Hits":[{"id":1850181,"tags":"cat"}]}`,
			want: 1,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var envelope map[string]json.RawMessage
			if err := json.Unmarshal([]byte(tc.raw), &envelope); err != nil {
				t.Fatalf("unmarshal envelope: %v", err)
			}
			items, ok := extractWriteThroughListItems(envelope)
			if !ok {
				t.Fatal("Pixabay hits envelope was not recognized as a list wrapper")
			}
			if len(items) != tc.want {
				t.Fatalf("extracted %d items, want %d", len(items), tc.want)
			}
			var first map[string]any
			if err := json.Unmarshal(items[0], &first); err != nil {
				t.Fatalf("unmarshal first hit: %v", err)
			}
			if _, ok := first["id"]; !ok {
				t.Fatalf("first extracted item missing id: %s", items[0])
			}
		})
	}
}
