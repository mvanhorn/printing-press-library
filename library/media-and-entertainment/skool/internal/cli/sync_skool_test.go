// Copyright 2026 Zain Haseeb and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestExtractSkoolPageRecordsPosts(t *testing.T) {
	page := json.RawMessage(`{"pageProps":{"total":2,"postTrees":[
		{"post":{"id":"p1","name":"first-post"}},
		{"post":{"id":"p2","name":"second-post"}}
	]}}`)
	items, keys := extractSkoolPageRecords(page, "posts")
	if len(items) != 2 {
		t.Fatalf("want 2 posts, got %d", len(items))
	}
	if recordIdentity(items[0]) != "p1" || recordIdentity(items[1]) != "p2" {
		t.Fatalf("unexpected ids: %s %s", recordIdentity(items[0]), recordIdentity(items[1]))
	}
	if len(keys) == 0 {
		t.Fatal("want pageProps keys reported")
	}
}

func TestExtractSkoolPageRecordsMembersDirectArray(t *testing.T) {
	page := json.RawMessage(`{"pageProps":{"users":[
		{"id":"u1","firstName":"A","lastName":"B"},
		{"id":"u2","name":"C"}
	]}}`)
	items, _ := extractSkoolPageRecords(page, "members")
	if len(items) != 2 {
		t.Fatalf("want 2 members, got %d", len(items))
	}
}

func TestExtractSkoolPageRecordsMembersWrappedAndNested(t *testing.T) {
	page := json.RawMessage(`{"pageProps":{"usersData":{"users":[
		{"userId":"gm1","user":{"id":"u1","firstName":"A"}},
		{"userId":"gm2","user":{"id":"u2","firstName":"B"}}
	]}}}`)
	items, _ := extractSkoolPageRecords(page, "members")
	if len(items) != 2 {
		t.Fatalf("want 2 members, got %d", len(items))
	}
	if recordIdentity(items[0]) != "u1" {
		t.Fatalf("want the inner user object, got %s", string(items[0]))
	}
}

func TestExtractSkoolPageRecordsUnknownEnvelopeReportsKeys(t *testing.T) {
	page := json.RawMessage(`{"pageProps":{"somethingElse":[{"id":"x1"}],"other":5}}`)
	items, keys := extractSkoolPageRecords(page, "members")
	if len(items) != 0 {
		t.Fatalf("want no members from an unrecognized envelope, got %d", len(items))
	}
	if len(keys) != 2 || keys[0] != "other" || keys[1] != "somethingElse" {
		t.Fatalf("want sorted pageProps keys, got %v", keys)
	}
}

func TestIsSkoolCommunityResource(t *testing.T) {
	for _, r := range []string{"posts", "members"} {
		if !isSkoolCommunityResource(r) {
			t.Fatalf("%s should route to the community sync path", r)
		}
	}
	if isSkoolCommunityResource("notifications") {
		t.Fatal("notifications should stay on the generic flat-list sync path")
	}
}

// TestSyncSkoolCommunityResourceDoesNotEmitSyncError pins the single-emission
// contract: worker-level failure paths set Err and stay silent, and the
// aggregation loop in sync.go is the only place that prints a sync_error line.
// Two identical events per failed resource is worse than none for an agent
// consumer counting failures.
func TestSyncSkoolCommunityResourceDoesNotEmitSyncError(t *testing.T) {
	prevHuman := humanFriendly
	humanFriendly = false
	defer func() { humanFriendly = prevHuman }()

	prevStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	os.Stdout = w
	res := syncSkoolCommunityResource(nil, nil, "posts", "", 1)
	w.Close()
	os.Stdout = prevStdout

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}
	out := buf.String()

	if res.Err == nil {
		t.Fatal("want an error result when no community is resolvable")
	}
	if strings.Contains(out, `"event":"sync_error"`) {
		t.Fatalf("worker must not emit sync_error; aggregation owns it. got: %s", out)
	}
	if !strings.Contains(out, `"event":"sync_start"`) {
		t.Fatalf("want the sync_start event preserved, got: %s", out)
	}
}
