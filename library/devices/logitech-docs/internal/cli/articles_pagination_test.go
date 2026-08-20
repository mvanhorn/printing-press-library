// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored regression tests for Zendesk cursor pagination on articles.
//
// Two upstream behaviours make this worth pinning down, because both fail
// SILENTLY rather than erroring:
//
//  1. The flat /articles.json list ignores a `section_id` query param and
//     returns the entire 33k-article corpus. Section scoping only works
//     through /sections/{id}/articles.json.
//  2. Zendesk cursor pagination requires the bracketed keys `page[after]`
//     and `page[size]`. A plain `?cursor=` is accepted and ignored, handing
//     back page 1 forever.
//
// A generated sync that picks the sanitized flag name (`cursor`) over the
// wire key, or leaves cursorParam empty, therefore looks healthy while
// capturing only the first page. These tests assert the wire keys on the
// request itself, which is the only thing that actually proves the walk.
//
// This file is hand-authored so it survives `generate --force` regen-merge as
// a whole unit.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/logitech-docs/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// recordingClient captures the params of every Get so a test can assert on
// what actually went over the wire, page by page.
type recordingClient struct {
	calls  []map[string]string
	paths  []string
	pages  []json.RawMessage
	served int
}

func (r *recordingClient) Get(_ context.Context, path string, params map[string]string) (json.RawMessage, error) {
	copied := make(map[string]string, len(params))
	for k, v := range params {
		copied[k] = v
	}
	r.calls = append(r.calls, copied)
	r.paths = append(r.paths, path)
	if len(r.pages) == 0 {
		return nil, fmt.Errorf("recordingClient has no canned pages")
	}
	page := r.pages[min(r.served, len(r.pages)-1)]
	r.served++
	return page, nil
}

func (r *recordingClient) RateLimit() float64 { return 0 }

// TestArticlesPaginationDefaultsUseZendeskWireKeys pins the five values the
// generated sync loop feeds into every articles request. Four of the five were
// already correct in a regen that still fetched exactly one page, because
// cursorParam came back empty and the loop then never sent a cursor at all —
// so asserting cursorType alone is not enough to catch the regression.
func TestArticlesPaginationDefaultsUseZendeskWireKeys(t *testing.T) {
	t.Parallel()

	// "sections/articles" is the key the PRODUCTION path uses: articles are
	// synced as a dependent, and syncDependentResource looks up
	// determinePaginationDefaults(dep.ParentTable + "/" + dep.Name). Asserting
	// only the flat "articles" key would guard a branch sync never reaches.
	for _, resource := range []string{"sections/articles", "articles"} {
		got := determinePaginationDefaults(resource)
		for _, tc := range []struct {
			field, got, want string
		}{
			{"cursorParam", got.cursorParam, "page[after]"},
			{"cursorType", got.cursorType, "cursor"},
			{"nextCursorPath", got.nextCursorPath, "meta.after_cursor"},
			{"limitParam", got.limitParam, "page[size]"},
		} {
			if tc.got != tc.want {
				t.Errorf("determinePaginationDefaults(%q).%s = %q, want %q", resource, tc.field, tc.got, tc.want)
			}
		}
		if got.limit != 100 {
			t.Errorf("determinePaginationDefaults(%q).limit = %d, want 100", resource, got.limit)
		}
	}
}

// TestDependentSectionWalkSendsCursorAndScopedPath is the production-path
// counterpart: it seeds one section, runs the dependent fan-out, and asserts
// the requests go to the section-scoped URL and that request 2 carries
// page[after]. syncResource (the flat path) is not what sync uses for articles.
func TestDependentSectionWalkSendsCursorAndScopedPath(t *testing.T) {
	t.Parallel()

	deps := dependentResourceDefs()
	if len(deps) == 0 {
		t.Fatal("no dependent resources declared — sync has no path to articles")
	}
	dep := deps[0]

	const cursor1 = "SECTION_CURSOR_1"
	c := &recordingClient{pages: []json.RawMessage{
		json.RawMessage(`{"articles":[{"id":11,"title":"a","section_id":777}],"meta":{"has_more":true,"after_cursor":"` + cursor1 + `"}}`),
		json.RawMessage(`{"articles":[{"id":12,"title":"b","section_id":777}],"meta":{"has_more":false,"after_cursor":""}}`),
	}}

	db := openTestStore(t)
	if _, _, err := db.UpsertBatch(dep.ParentTable, []json.RawMessage{
		json.RawMessage(`{"id":777,"name":"Test Section"}`),
	}); err != nil {
		t.Fatalf("seed parent table %s: %v", dep.ParentTable, err)
	}

	var buf bytes.Buffer
	res := syncDependentResource(context.Background(), c, db, dep, "", true, 0, false, false, &syncUserParams{}, &buf, 1)
	if res.Err != nil {
		t.Fatalf("dependent sync errored: %v", res.Err)
	}
	if len(c.calls) < 2 {
		t.Fatalf("expected >=2 requests for the seeded section (cursor did not advance), got %d", len(c.calls))
	}
	if got := c.paths[0]; !strings.Contains(got, "/sections/777/articles") {
		t.Errorf("request path = %q, want the section-scoped /sections/777/articles path", got)
	}
	if got := c.calls[1]["page[after]"]; got != cursor1 {
		t.Errorf("request 2 page[after] = %q, want %q", got, cursor1)
	}
	for _, banned := range []string{"section_id", "per_page", "page", "cursor"} {
		for i, call := range c.calls {
			if _, ok := call[banned]; ok {
				t.Errorf("request %d sent banned param %q (params: %v)", i+1, banned, call)
			}
		}
	}
}

// TestArticlesCommandSurface pins the public CLI contract the reviewer asked
// for. Without this, restoring --section-id/--page/--per-page would pass every
// other test in this file.
func TestArticlesCommandSurface(t *testing.T) {
	t.Parallel()

	root := newRootCmd(&rootFlags{})
	find := func(path ...string) *cobra.Command {
		cur := root
		for _, name := range path {
			var next *cobra.Command
			for _, c := range cur.Commands() {
				if c.Name() == name {
					next = c
					break
				}
			}
			if next == nil {
				t.Fatalf("command %v not found", path)
			}
			cur = next
		}
		return cur
	}

	list := find("articles", "list")
	for _, gone := range []string{"section-id", "page", "per-page"} {
		if list.Flags().Lookup(gone) != nil {
			t.Errorf("articles list still exposes --%s; the upstream API ignores it or caps pagination", gone)
		}
	}
	for _, want := range []string{"page-size", "cursor"} {
		if list.Flags().Lookup(want) == nil {
			t.Errorf("articles list is missing --%s", want)
		}
	}

	bySection := find("articles", "by-section")
	if got := bySection.Annotations["pp:path"]; got != "/api/v2/help_center/en-us/sections/{section_id}/articles.json" {
		t.Errorf("articles by-section pp:path = %q, want the section-scoped path", got)
	}
	if !strings.Contains(bySection.Use, "<section_id>") {
		t.Errorf("articles by-section Use = %q, want a required <section_id> positional", bySection.Use)
	}
	if got := bySection.Annotations["mcp:read-only"]; got != "true" {
		t.Errorf("articles by-section mcp:read-only = %q, want \"true\" (it only reads)", got)
	}
}

// TestArticlesSyncSendsCursorOnSecondRequest is the real proof: request 2 must
// carry page[after] set to the cursor request 1 returned. Inspecting generated
// code cannot show this — an existing library CLI (trigger-dev) sends
// page[after] from its command while its sync sends a plain `after`.
func TestArticlesSyncSendsCursorOnSecondRequest(t *testing.T) {
	t.Parallel()

	const cursor1 = "CURSOR_FROM_PAGE_1"
	c := &recordingClient{pages: []json.RawMessage{
		json.RawMessage(`{"articles":[{"id":1,"title":"a"}],"meta":{"has_more":true,"after_cursor":"` + cursor1 + `"}}`),
		json.RawMessage(`{"articles":[{"id":2,"title":"b"}],"meta":{"has_more":false,"after_cursor":""}}`),
	}}

	db := openTestStore(t)
	res := syncResource(context.Background(), c, db, "articles", "", true, 0, false, false, &syncUserParams{}, nil)
	if res.Err != nil {
		t.Fatalf("syncResource returned error: %v", res.Err)
	}

	if len(c.calls) < 2 {
		t.Fatalf("expected at least 2 requests (cursor did not advance), got %d", len(c.calls))
	}

	first, second := c.calls[0], c.calls[1]
	if got := second["page[after]"]; got != cursor1 {
		t.Errorf("request 2 page[after] = %q, want %q — the cursor from response 1 was not forwarded", got, cursor1)
	}
	if got := first["page[size]"]; got != "100" {
		t.Errorf("request 1 page[size] = %q, want \"100\"", got)
	}

	// The silent-no-op params must never appear. section_id is ignored by the
	// flat endpoint; per_page/page are the capped offset scheme that stops at
	// page 100.
	for _, banned := range []string{"section_id", "per_page", "page", "cursor"} {
		for i, call := range c.calls {
			if _, ok := call[banned]; ok {
				t.Errorf("request %d sent banned param %q (params: %v)", i+1, banned, call)
			}
		}
	}
}

// TestArticlesSyncStopsWhenHasMoreFalse guards the terminal condition: a
// response with has_more false ends the walk instead of looping on a repeated
// cursor.
func TestArticlesSyncStopsWhenHasMoreFalse(t *testing.T) {
	t.Parallel()

	c := &recordingClient{pages: []json.RawMessage{
		json.RawMessage(`{"articles":[{"id":1,"title":"a"}],"meta":{"has_more":false,"after_cursor":""}}`),
	}}

	db := openTestStore(t)
	res := syncResource(context.Background(), c, db, "articles", "", true, 0, false, false, &syncUserParams{}, nil)
	if res.Err != nil {
		t.Fatalf("syncResource returned error: %v", res.Err)
	}
	if len(c.calls) != 1 {
		t.Fatalf("has_more:false should end the walk after 1 request, got %d", len(c.calls))
	}
}

// TestDependentWithEmptyParentWarnsInsteadOfReportingSuccess covers the
// regression that `sync --resources articles` creates: articles are reachable
// only by walking sections, so an unsynced parent table means zero coverage.
// The generated path returned a bare syncResult there, which the summary
// counted as a successful zero-record sync — a silent no-op in exactly the
// class of bug this CLI was patched to remove.
func TestDependentWithEmptyParentWarnsInsteadOfReportingSuccess(t *testing.T) {
	t.Parallel()

	deps := dependentResourceDefs()
	if len(deps) == 0 {
		t.Skip("no dependent resources declared")
	}
	c := &recordingClient{pages: []json.RawMessage{json.RawMessage(`{"articles":[]}`)}}
	db := openTestStore(t) // parent table is empty

	var buf bytes.Buffer
	res := syncDependentResource(context.Background(), c, db, deps[0], "", true, 0, false, false, &syncUserParams{}, &buf, 1)

	if res.Warn == nil {
		t.Errorf("empty parent table produced no Warn — the run would be counted as a success")
	}
	if res.Err != nil {
		t.Errorf("empty parent table should warn, not error: %v", res.Err)
	}
	if len(c.calls) != 0 {
		t.Errorf("expected no upstream requests with an empty parent table, got %d", len(c.calls))
	}
	if got := buf.String(); !strings.Contains(got, "dependent_parent_table_empty") {
		t.Errorf("expected a structured sync_warning event, got %q", got)
	}
}

// TestArticlesBySectionUsesScopedPath proves section scoping goes through the
// path, not a query param — the fix for the reported bug.
func TestArticlesBySectionUsesScopedPath(t *testing.T) {
	t.Parallel()

	const wantPath = "/api/v2/help_center/en-us/sections/{section_id}/articles.json"
	found := false
	for _, dep := range dependentResourceDefs() {
		if dep.PathTemplate == wantPath {
			found = true
			if dep.ParentTable != "sections" || dep.ParentIDParam != "section_id" {
				t.Errorf("dependent def = parent %q/%q, want sections/section_id", dep.ParentTable, dep.ParentIDParam)
			}
		}
	}
	if !found {
		t.Errorf("no dependent resource walks %s — sync would not cover sections", wantPath)
	}
}

// TestDependentParentFetchFailureIsNotSuccess covers the other half of the
// silent-truncation risk: articles are reachable only by walking every section,
// so one failing section must not be checkpointed as a successful sync. The
// generated path printed a sync_error but left rep.failure nil, so the
// aggregator counted the run as a success.
func TestDependentParentFetchFailureIsNotSuccess(t *testing.T) {
	t.Parallel()

	deps := dependentResourceDefs()
	if len(deps) == 0 {
		t.Fatal("no dependent resources declared")
	}
	dep := deps[0]

	c := &failingClient{}
	db := openTestStore(t)
	if _, _, err := db.UpsertBatch(dep.ParentTable, []json.RawMessage{
		json.RawMessage(`{"id":777,"name":"Test Section"}`),
	}); err != nil {
		t.Fatalf("seed parent table: %v", err)
	}

	var buf bytes.Buffer
	res := syncDependentResource(context.Background(), c, db, dep, "", true, 0, false, false, &syncUserParams{}, &buf, 1)
	if res.Err == nil && res.Warn == nil {
		t.Error("a failing section fetch was reported as a clean success — partial corpus would be checkpointed")
	}
}

// failingClient fails every request, standing in for a transient upstream
// error on one section.
type failingClient struct{}

func (failingClient) Get(_ context.Context, _ string, _ map[string]string) (json.RawMessage, error) {
	return nil, fmt.Errorf("simulated upstream 500")
}
func (failingClient) RateLimit() float64 { return 0 }
