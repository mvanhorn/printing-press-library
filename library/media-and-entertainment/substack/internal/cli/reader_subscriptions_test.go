// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(reader-subscriptions-list): hand-authored; see .printing-press-patches/.

package cli

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestReaderIsSyncableResource(t *testing.T) {
	t.Parallel()

	path, err := syncResourcePath("reader")
	if err != nil {
		t.Fatalf("syncResourcePath(reader): %v", err)
	}
	if got, want := path, "/reader/subscriptions"; got != want {
		t.Fatalf("syncResourcePath(reader) = %q, want %q", got, want)
	}
	if !slices.Contains(defaultSyncResources(), "reader") {
		t.Fatal("defaultSyncResources must include reader so sync populates local fallback")
	}
	if !slices.Contains(knownSyncResourceNames(), "reader") {
		t.Fatal("knownSyncResourceNames must include reader")
	}
	if !resourceSupportsPagination("reader") {
		t.Fatal("resourceSupportsPagination(reader) must be true so sync pages past the first response")
	}
	got, ok := readCommandResources["substack-pp-cli reader subscriptions"]
	if !ok || !slices.Equal(got, []string{"reader"}) {
		t.Fatalf("readCommandResources[reader subscriptions] = %v, ok=%v; want [reader]", got, ok)
	}
}

func TestReaderSubscriptionsLimitRejectsNonInteger(t *testing.T) {
	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"reader", "subscriptions", "--limit", "abc"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected cobra to reject --limit abc")
	}
	if !strings.Contains(err.Error(), "invalid argument") {
		t.Fatalf("error = %q, want cobra invalid-argument usage error", err)
	}
	if !isCobraUsageError(err) {
		t.Fatalf("isCobraUsageError(%v) = false, want true", err)
	}
}

func TestReaderSyncUsesOpaqueCursorNotOffset(t *testing.T) {
	t.Parallel()

	pageSize := paginationForResource("reader")
	if pageSize.cursorParam != "cursor" {
		t.Fatalf("reader cursorParam = %q, want cursor so later pages send the opaque token", pageSize.cursorParam)
	}
	if pageSize.cursorType != "cursor" {
		t.Fatalf("reader cursorType = %q, want cursor (not offset stride)", pageSize.cursorType)
	}
	if pageSize.limitParam != "limit" {
		t.Fatalf("reader limitParam = %q, want limit", pageSize.limitParam)
	}

	posts := paginationForResource("posts")
	if posts.cursorParam != "offset" {
		t.Fatalf("posts cursorParam = %q, want offset so reader override does not leak", posts.cursorParam)
	}
}

func TestExtractPageItemsReaderSubscriptionsCursor(t *testing.T) {
	t.Parallel()

	data := json.RawMessage(`{"subscriptions":[{"subscription_id":"a"},{"subscription_id":"b"}],"cursor":"next-token"}`)
	items, next, hasMore := extractPageItems(data, "cursor")
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if next != "next-token" {
		t.Fatalf("next cursor = %q, want next-token", next)
	}
	if !hasMore {
		t.Fatal("hasMore = false, want true when a cursor is present")
	}
}
