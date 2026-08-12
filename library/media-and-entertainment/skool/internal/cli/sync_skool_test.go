// Copyright 2026 Zain Haseeb and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

package cli

import (
	"encoding/json"
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
