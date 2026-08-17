// Copyright 2026 Ryan Cooper and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// fakePagingClient serves page-keyed BGG-shaped XML-normalized responses so
// paginatedGet's multi-page walk can be exercised without a network.
type fakePagingClient struct {
	pages map[string]json.RawMessage
	calls []string
}

func (f *fakePagingClient) GetWithHeaders(_ context.Context, _ string, params map[string]string, _ map[string]string) (json.RawMessage, error) {
	p := params["page"]
	if p == "" || p == "0" {
		p = "1"
	}
	f.calls = append(f.calls, p)
	if body, ok := f.pages[p]; ok {
		return body, nil
	}
	// Past the last page: an empty play list with the same total.
	return json.RawMessage(`{"plays":{"@total":"250","@page":"` + p + `"}}`), nil
}

// bggPlaysPage builds a BGG plays envelope with n play elements and the given
// total, mirroring the XML->JSON normalized shape the client produces.
func bggPlaysPage(page, total, n int) json.RawMessage {
	plays := make([]string, n)
	for i := range plays {
		plays[i] = fmt.Sprintf(`{"@id":"%d"}`, page*1000+i)
	}
	body := fmt.Sprintf(
		`{"plays":{"@total":"%d","@page":"%d","play":[%s]}}`,
		total, page, strings.Join(plays, ","),
	)
	return json.RawMessage(body)
}

// TestPaginatedGetPageTypeWalksAllPages verifies that page-number pagination
// with no limit param (BGG plays/guild) advances past page 1 and collects every
// item, stopping exactly at the declared total without a speculative extra
// fetch. Regression test for the "plays --all stops after page 1" bug.
func TestPaginatedGetPageTypeWalksAllPages(t *testing.T) {
	fake := &fakePagingClient{pages: map[string]json.RawMessage{
		"1": bggPlaysPage(1, 250, 100),
		"2": bggPlaysPage(2, 250, 100),
		"3": bggPlaysPage(3, 250, 50),
	}}

	data, err := paginatedGet(
		context.Background(), fake, "/plays",
		map[string]string{"page": "1"}, nil,
		true,       // fetchAll
		"page",     // cursorParam
		"page",     // paginationType
		"", "", "", // limitParam, nextCursorPath, hasMoreField
	)
	if err != nil {
		t.Fatalf("paginatedGet returned error: %v", err)
	}

	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("result is not an array: %v", err)
	}
	if len(items) != 250 {
		t.Errorf("collected %d items, want 250", len(items))
	}
	// Pages 1-3 fetched; the exact total stops the walk before page 4.
	wantCalls := []string{"1", "2", "3"}
	if strings.Join(fake.calls, ",") != strings.Join(wantCalls, ",") {
		t.Errorf("fetched pages %v, want %v", fake.calls, wantCalls)
	}
}

// TestPaginatedGetPageTypeSinglePage verifies that a single short page does not
// trigger a speculative second fetch when the total is satisfied.
func TestPaginatedGetPageTypeSinglePage(t *testing.T) {
	fake := &fakePagingClient{pages: map[string]json.RawMessage{
		"1": bggPlaysPage(1, 40, 40),
	}}

	var data json.RawMessage
	var err error
	stderr := captureStderr(t, func() {
		data, err = paginatedGet(
			context.Background(), fake, "/plays",
			map[string]string{"page": "1"}, nil,
			true, "page", "page", "", "", "",
		)
	})
	if err != nil {
		t.Fatalf("paginatedGet returned error: %v", err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("result is not an array: %v", err)
	}
	if len(items) != 40 {
		t.Errorf("collected %d items, want 40", len(items))
	}
	if len(fake.calls) != 1 {
		t.Errorf("made %d fetches (%v), want 1", len(fake.calls), fake.calls)
	}
	if strings.Contains(stderr, "pagination_signal_missing") {
		t.Fatalf("stderr = %q, want no false missing-pagination warning", stderr)
	}
}

func TestPaginatedGetPageTypeSinglePageWithoutTotalWarns(t *testing.T) {
	fake := &fakePagingClient{pages: map[string]json.RawMessage{
		"1": json.RawMessage(`{"plays":{"@page":"1"}}`),
	}}

	stderr := captureStderr(t, func() {
		_, _ = paginatedGet(
			context.Background(), fake, "/plays",
			map[string]string{"page": "1"}, nil,
			true, "page", "page", "", "", "",
		)
	})
	if !strings.Contains(stderr, "pagination_signal_missing") {
		t.Fatalf("stderr = %q, want missing-pagination warning when no total/cursor signal exists", stderr)
	}
}

// TestNextFullPagePageCursorGuards verifies the helper only advances page-type
// pagination with no limit param, and respects the declared total.
func TestNextFullPagePageCursorGuards(t *testing.T) {
	cases := []struct {
		name         string
		paginationT  string
		limitParam   string
		itemCount    int
		pageSize     int
		collected    int
		total        int
		wantOK       bool
		wantNextPage string
	}{
		{"page under total advances", "page", "", 100, 100, 100, 250, true, "2"},
		{"page at total stops", "page", "", 50, 100, 250, 250, false, ""},
		{"offset type ignored", "offset", "", 100, 100, 100, 250, false, ""},
		{"limit param ignored", "page", "pagesize", 100, 100, 100, 250, false, ""},
		{"empty page stops", "page", "", 0, 100, 100, 250, false, ""},
		{"no total full page advances", "page", "", 100, 100, 100, 0, true, "2"},
		{"no total short page stops", "page", "", 40, 100, 40, 0, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := map[string]string{"page": "1"}
			next, ok := nextFullPagePageCursor(params, "page", tc.paginationT, tc.limitParam, tc.itemCount, tc.pageSize, tc.collected, tc.total)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v, want %v", ok, tc.wantOK)
			}
			if ok && next != tc.wantNextPage {
				t.Errorf("next=%q, want %q", next, tc.wantNextPage)
			}
		})
	}
}

// TestPaginationTotalFromEnvelope verifies total extraction from flat and
// XML-normalized nested envelopes.
func TestPaginationTotalFromEnvelope(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"bgg nested string attr", `{"plays":{"@total":"250","play":[]}}`, 250},
		{"flat numeric total", `{"total":42,"items":[]}`, 42},
		{"flat string total", `{"total":"99"}`, 99},
		{"absent", `{"plays":{"play":[]}}`, 0},
		{"zero ignored", `{"plays":{"@total":"0","play":[]}}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var obj map[string]json.RawMessage
			if err := json.Unmarshal([]byte(tc.body), &obj); err != nil {
				t.Fatalf("bad test body: %v", err)
			}
			if got := paginationTotalFromEnvelope(obj); got != tc.want {
				t.Errorf("total=%d, want %d", got, tc.want)
			}
		})
	}
}

// fakeGuildClient serves BGG guild member pages: a two-level XML-normalized
// wrapper {"guild": {"members": {"@count": "N", "member": [...]}}}.
type fakeGuildClient struct {
	pageItems map[string]int
	total     int
	calls     []string
}

func (f *fakeGuildClient) GetWithHeaders(_ context.Context, _ string, params map[string]string, _ map[string]string) (json.RawMessage, error) {
	p := params["page"]
	if p == "" || p == "0" {
		p = "1"
	}
	f.calls = append(f.calls, p)
	n := f.pageItems[p] // 0 for pages past the end
	members := make([]string, n)
	for i := range members {
		members[i] = fmt.Sprintf(`{"@name":"u%s_%d"}`, p, i)
	}
	body := fmt.Sprintf(
		`{"guild":{"@id":"42","members":{"@count":"%d","@page":"%s","member":[%s]}}}`,
		f.total, p, strings.Join(members, ","),
	)
	return json.RawMessage(body), nil
}

// TestPaginatedGetGuildMembersWalksAllPages is the regression test for the
// guild --all --members 1 bug: the two-level {"guild":{"members":{"@count","member":[...]}}}
// wrapper must extract individual member records (not the members wrapper as a
// single item) and recognize @count as the total so the walk stops exactly
// instead of running to the 100-page cap.
func TestPaginatedGetGuildMembersWalksAllPages(t *testing.T) {
	fake := &fakeGuildClient{
		total:     55,
		pageItems: map[string]int{"1": 25, "2": 25, "3": 5},
	}

	data, err := paginatedGet(
		context.Background(), fake, "/guild",
		map[string]string{"page": "1"}, nil,
		true, "page", "page", "", "", "",
	)
	if err != nil {
		t.Fatalf("paginatedGet returned error: %v", err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("result is not an array: %v", err)
	}
	if len(items) != 55 {
		t.Errorf("collected %d members, want 55", len(items))
	}
	// Each item must be an individual member record (has @name), not the
	// members wrapper (which would carry @count/member).
	for _, it := range items {
		var m map[string]json.RawMessage
		if json.Unmarshal(it, &m) != nil {
			t.Fatalf("member is not an object: %s", it)
		}
		if _, isWrapper := m["member"]; isWrapper {
			t.Fatalf("extracted the members wrapper instead of a member record: %s", it)
		}
		if _, ok := m["@name"]; !ok {
			t.Errorf("member record missing @name: %s", it)
		}
	}
	wantCalls := []string{"1", "2", "3"}
	if strings.Join(fake.calls, ",") != strings.Join(wantCalls, ",") {
		t.Errorf("fetched pages %v, want %v (should stop at @count total, not 100-page cap)", fake.calls, wantCalls)
	}
}

// TestExtractPaginatedItemsGuildWrapper checks the two-level wrapper unwrap in
// isolation, including the single-member BadgerFish collapse.
func TestExtractPaginatedItemsGuildWrapper(t *testing.T) {
	t.Run("multiple members", func(t *testing.T) {
		var obj map[string]json.RawMessage
		_ = json.Unmarshal([]byte(`{"members":{"@count":"3","member":[{"@name":"a"},{"@name":"b"},{"@name":"c"}]}}`), &obj)
		items, ok := extractPaginatedItems(obj)
		if !ok || len(items) != 3 {
			t.Fatalf("got ok=%v len=%d, want ok=true len=3", ok, len(items))
		}
	})
	t.Run("single member collapsed to object", func(t *testing.T) {
		var obj map[string]json.RawMessage
		_ = json.Unmarshal([]byte(`{"members":{"@count":"1","member":{"@name":"solo"}}}`), &obj)
		items, ok := extractPaginatedItems(obj)
		if !ok || len(items) != 1 {
			t.Fatalf("got ok=%v len=%d, want ok=true len=1", ok, len(items))
		}
	})
}

// TestPaginationTotalNestedCount verifies @count nested inside a collection
// wrapper (guild -> members -> @count) is found.
func TestPaginationTotalNestedCount(t *testing.T) {
	var obj map[string]json.RawMessage
	_ = json.Unmarshal([]byte(`{"guild":{"@id":"42","members":{"@count":"55","member":[]}}}`), &obj)
	if got := paginationTotalFromEnvelope(obj); got != 55 {
		t.Errorf("total=%d, want 55", got)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	defer func() {
		os.Stderr = old
		_ = r.Close()
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close stderr pipe: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stderr pipe: %v", err)
	}
	return string(out)
}
