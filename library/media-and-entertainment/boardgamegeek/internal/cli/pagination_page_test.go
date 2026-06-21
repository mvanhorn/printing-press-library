// Copyright 2026 Ryan Cooper and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
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
		true,    // fetchAll
		"page",  // cursorParam
		"page",  // paginationType
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

	data, err := paginatedGet(
		context.Background(), fake, "/plays",
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
	if len(items) != 40 {
		t.Errorf("collected %d items, want 40", len(items))
	}
	if len(fake.calls) != 1 {
		t.Errorf("made %d fetches (%v), want 1", len(fake.calls), fake.calls)
	}
}

// TestNextFullPagePageCursorGuards verifies the helper only advances page-type
// pagination with no limit param, and respects the declared total.
func TestNextFullPagePageCursorGuards(t *testing.T) {
	cases := []struct {
		name          string
		paginationT   string
		limitParam    string
		itemCount     int
		pageSize      int
		collected     int
		total         int
		wantOK        bool
		wantNextPage  string
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
