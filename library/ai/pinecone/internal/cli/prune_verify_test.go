// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

type fakePruneVerifier struct {
	responses []json.RawMessage
	paths     []string
	params    []map[string]string
}

func (f *fakePruneVerifier) GetWithHeadersNoCache(_ context.Context, path string, params, _ map[string]string) (json.RawMessage, error) {
	f.paths = append(f.paths, path)
	f.params = append(f.params, params)
	if len(f.paths) > len(f.responses) {
		return nil, fmt.Errorf("unexpected fetch")
	}
	return f.responses[len(f.paths)-1], nil
}

func TestVerifyPruneCandidatesRechecksLiveTimestampAndScope(t *testing.T) {
	f := &fakePruneVerifier{responses: []json.RawMessage{json.RawMessage(`{"vectors":{
		"stale":{"id":"stale","metadata":{"timestamp":"2020-01-01T00:00:00Z"}},
		"fresh":{"id":"fresh","metadata":{"timestamp":"2030-01-01T00:00:00Z"}}
	}}`)}}
	cutoff := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	got, missing, err := verifyPruneCandidates(context.Background(), f, "https://index-a.svc.pinecone.io/vectors/fetch", "index-a", "production", []string{"stale", "fresh"}, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "stale" || len(missing) != 0 {
		t.Fatalf("verified deletions=%v, want stale only", got)
	}
	if len(f.paths) != 1 || !strings.HasPrefix(f.paths[0], "https://index-a.svc.pinecone.io/vectors/fetch?") || f.params[0]["namespace"] != "production" {
		t.Fatalf("fetch target=%v params=%v", f.paths, f.params)
	}
}

func TestVerifyPruneCandidatesFailsClosedOnIncompleteOrWrongScope(t *testing.T) {
	cutoff := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		body string
	}{
		{"null vector", `{"vectors":{"stale":null}}`},
		{"missing metadata", `{"vectors":{"stale":{"id":"stale"}}}`},
		{"wrong index", `{"vectors":{"stale":{"id":"stale","index_name":"index-b","metadata":{"timestamp":"2020-01-01T00:00:00Z"}}}}`},
		{"wrong namespace", `{"vectors":{"stale":{"id":"stale","namespace":"staging","metadata":{"timestamp":"2020-01-01T00:00:00Z"}}}}`},
		{"malformed response", `{"vectors":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakePruneVerifier{responses: []json.RawMessage{json.RawMessage(tc.body)}}
			got, missing, err := verifyPruneCandidates(context.Background(), f, "https://index-a.svc.pinecone.io/vectors/fetch", "index-a", "production", []string{"stale"}, cutoff)
			if err == nil || len(got) != 0 || len(missing) != 0 {
				t.Fatalf("unsafe fetch accepted: verified=%v error=%v", got, err)
			}
		})
	}
}

func TestVerifyPruneCandidatesSkipsAbsentVectorAcrossBatches(t *testing.T) {
	ids := make([]string, 101)
	first := make(map[string]any, 100)
	for i := range ids {
		ids[i] = fmt.Sprintf("id-%03d", i)
		if i < 100 {
			first[ids[i]] = map[string]any{"id": ids[i], "metadata": map[string]any{"timestamp": "2020-01-01T00:00:00Z"}}
		}
	}
	page, err := json.Marshal(map[string]any{"vectors": first})
	if err != nil {
		t.Fatal(err)
	}
	f := &fakePruneVerifier{responses: []json.RawMessage{page, json.RawMessage(`{"vectors":{}}`)}}
	got, missing, err := verifyPruneCandidates(context.Background(), f, "https://index-a.svc.pinecone.io/vectors/fetch", "index-a", "", ids, time.Now())
	if err != nil || len(got) != 100 || len(missing) != 1 || missing[0] != ids[100] || len(f.paths) != 2 {
		t.Fatalf("missing vector handling: verified=%d missing=%v calls=%d error=%v", len(got), missing, len(f.paths), err)
	}
}

func TestVerifyPruneCandidatesFailsClosedOnMalformedLaterBatch(t *testing.T) {
	ids := make([]string, 101)
	first := make(map[string]any, 100)
	for i := range ids {
		ids[i] = fmt.Sprintf("id-%03d", i)
		if i < 100 {
			first[ids[i]] = map[string]any{"metadata": map[string]any{"timestamp": "2020-01-01T00:00:00Z"}}
		}
	}
	page, err := json.Marshal(map[string]any{"vectors": first})
	if err != nil {
		t.Fatal(err)
	}
	f := &fakePruneVerifier{responses: []json.RawMessage{page, json.RawMessage(`{"vectors":null}`)}}
	got, missing, err := verifyPruneCandidates(context.Background(), f, "https://index-a.svc.pinecone.io/vectors/fetch", "index-a", "", ids, time.Now())
	if err == nil || got != nil || missing != nil || len(f.paths) != 2 {
		t.Fatalf("malformed later batch accepted: verified=%v missing=%v calls=%d error=%v", got, missing, len(f.paths), err)
	}
}
