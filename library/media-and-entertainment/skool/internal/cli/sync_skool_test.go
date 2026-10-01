// Copyright 2026 Zain Haseeb and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/skool/internal/store"
)

type pagedSkoolClient struct {
	requests       []int
	lastPage       int
	members        bool
	memberRequests int
	failPage       int
}

func (c *pagedSkoolClient) Get(_ string, params map[string]string) (json.RawMessage, error) {
	page := 1
	if raw := params["p"]; raw != "" {
		page, _ = strconv.Atoi(raw)
	}
	c.requests = append(c.requests, page)
	if c.failPage == page {
		return nil, fmt.Errorf("simulated page failure")
	}
	if c.members {
		if params["t"] != "members" {
			return nil, fmt.Errorf("member page missing members tab parameter")
		}
		c.memberRequests++
		if page > c.lastPage {
			return json.RawMessage(`{"pageProps":{"users":[]}}`), nil
		}
		return json.RawMessage(fmt.Sprintf(`{"pageProps":{"users":[{"id":"u%d","name":"member"}]}}`, page)), nil
	}
	if page > c.lastPage {
		return json.RawMessage(`{"pageProps":{"postTrees":[]}}`), nil
	}
	return json.RawMessage(fmt.Sprintf(`{"pageProps":{"postTrees":[{"post":{"id":"p%d"}}]}}`, page)), nil
}

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
	res := syncSkoolCommunityResource(nil, nil, "posts", "", 1, false)
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

func TestSyncSkoolCommunityResourceHonorsRequestedPageLimit(t *testing.T) {
	prevHuman := humanFriendly
	humanFriendly = true
	defer func() { humanFriendly = prevHuman }()

	db, err := store.Open(t.TempDir() + "/skool.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	client := &pagedSkoolClient{lastPage: 101}
	res := syncSkoolCommunityResource(client, db, "posts", "community", 101, false)
	if res.Err != nil {
		t.Fatalf("sync: %v", res.Err)
	}
	if len(client.requests) != 101 || client.requests[100] != 101 {
		t.Fatalf("requested %d pages, want pages 1 through 101", len(client.requests))
	}
	if res.Count != 101 {
		t.Fatalf("stored %d records, want 101", res.Count)
	}
	if res.Warn != nil || res.Notice == nil || !strings.Contains(res.Notice.Error(), "max-pages cap of 101") {
		t.Fatalf("want successful sync with a page-limit notice, got %+v", res)
	}

	unlimited := &pagedSkoolClient{lastPage: 101}
	res = syncSkoolCommunityResource(unlimited, db, "posts", "community", 0, false)
	if res.Err != nil || res.Notice != nil || res.Count != 101 || len(unlimited.requests) != 102 {
		t.Fatalf("unlimited sync = %+v with %d requests, want 101 rows and no cap notice", res, len(unlimited.requests))
	}
}

func TestSyncSkoolCommunityResourceLatestOnlyDoesNotReportTruncation(t *testing.T) {
	prevHuman := humanFriendly
	humanFriendly = true
	defer func() { humanFriendly = prevHuman }()

	db, err := store.Open(t.TempDir() + "/skool.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	client := &pagedSkoolClient{lastPage: 2}
	res := syncSkoolCommunityResource(client, db, "posts", "community", 1, true)
	if res.Err != nil || res.Notice != nil || res.Count != 1 || len(client.requests) != 1 {
		t.Fatalf("latest-only sync = %+v with %d requests, want one page and no notice", res, len(client.requests))
	}
}

func TestSyncSkoolCommunityResourceRejectsNegativePageLimit(t *testing.T) {
	client := &pagedSkoolClient{lastPage: 2}
	res := syncSkoolCommunityResource(client, nil, "posts", "community", -1, false)
	if res.Err == nil || !strings.Contains(res.Err.Error(), "--max-pages") || len(client.requests) != 0 {
		t.Fatalf("negative limit must fail before requests: %+v, requests=%v", res, client.requests)
	}
}

func TestSyncSkoolCommunityResourceMembersPagination(t *testing.T) {
	prevHuman := humanFriendly
	humanFriendly = true
	defer func() { humanFriendly = prevHuman }()

	db, err := store.Open(t.TempDir() + "/skool.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	client := &pagedSkoolClient{lastPage: 3, members: true}
	res := syncSkoolCommunityResource(client, db, "members", "community", 0, false)
	if res.Err != nil || res.Notice != nil || res.Count != 3 || client.memberRequests != 4 {
		t.Fatalf("member sync = %+v, requests=%v", res, client.requests)
	}
	if _, err := db.Get("members", "u3"); err != nil {
		t.Fatalf("last member not stored: %v", err)
	}
	capped := &pagedSkoolClient{lastPage: 3, members: true}
	res = syncSkoolCommunityResource(capped, db, "members", "community", 2, false)
	if res.Err != nil || res.Count != 2 || res.Notice == nil || len(capped.requests) != 2 {
		t.Fatalf("capped member sync = %+v, requests=%v", res, capped.requests)
	}
}

func TestSyncSkoolCommunityResourcePersistsEarlierPagesOnLaterFailure(t *testing.T) {
	prevHuman := humanFriendly
	humanFriendly = true
	defer func() { humanFriendly = prevHuman }()

	db, err := store.Open(t.TempDir() + "/skool.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	client := &pagedSkoolClient{lastPage: 3, failPage: 3}
	res := syncSkoolCommunityResource(client, db, "posts", "community", 0, false)
	if res.Err == nil || res.Count != 2 {
		t.Fatalf("want later fetch error with two durable rows, got %+v", res)
	}
	if _, err := db.Get("posts", "p2"); err != nil {
		t.Fatalf("earlier page not stored: %v", err)
	}
	if _, _, count, err := db.GetSyncState("posts"); err != nil || count != 0 {
		t.Fatalf("failed run must not record a successful checkpoint: count=%d err=%v", count, err)
	}
}

func TestSyncSkoolCommunityResourceWarnsBeforeComplete(t *testing.T) {
	prevHuman := humanFriendly
	humanFriendly = false
	defer func() { humanFriendly = prevHuman }()
	prevStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = prevStdout; r.Close() }()
	db, err := store.Open(t.TempDir() + "/skool.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	res := syncSkoolCommunityResource(&pagedSkoolClient{lastPage: 2}, db, "posts", "community", 1, false)
	w.Close()
	os.Stdout = prevStdout
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}
	out := buf.String()
	if res.Err != nil || res.Notice == nil {
		t.Fatalf("want successful capped sync: %+v", res)
	}
	warning := strings.Index(out, `"reason":"max_pages_cap_hit"`)
	complete := strings.Index(out, `"event":"sync_complete"`)
	if warning < 0 || complete <= warning {
		t.Fatalf("warning must precede completion: %s", out)
	}
}
