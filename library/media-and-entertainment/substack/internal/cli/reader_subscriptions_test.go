// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(reader-subscriptions-list): hand-authored; see .printing-press-patches/.

package cli

import (
	"bytes"
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
