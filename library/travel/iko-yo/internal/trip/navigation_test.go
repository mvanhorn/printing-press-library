// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package trip

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Synthetic cards retain the observed source's link and icon structure.
func navigationPage(id int, paging bool) string {
	nav := `<a href="/events/regions/6/prefectures/11">埼玉県</a>`
	if paging {
		nav += `<a href="?page=2">2</a><a href="?page=3">3</a><a href="https://example.com/events/regions/6/prefectures/11?page=2">foreign</a>`
	}
	return fmt.Sprintf(`<h2 class="c-heading--h2_event">イベント</h2><div class="c-list"><a class="c-list__link" href="/events/%d"><img alt="Synthetic event %d"><div><i class="icon-spot"></i>埼玉県草加市</div><div><i class="icon-date"></i>2026年11月15日</div></a></div>%s`, id, id, nav)
}
func TestObservedNavigationLinks(t *testing.T) {
	for _, tc := range []struct {
		page     string
		wantNext int
	}{{BaseURL + "/events/regions/6/prefectures/11", 2}, {BaseURL + "/events/regions/6/prefectures/11?page=2", 3}} {
		p, e := ParseListing([]byte(navigationPage(1, true)), "events", tc.page, observed)
		if e != nil {
			t.Fatal(e)
		}
		want := BaseURL + "/events/regions/6/prefectures/11?page=" + fmt.Sprint(tc.wantNext)
		if p.NextURL != want || p.LastPage != 3 || p.Areas[11] != BaseURL+"/events/regions/6/prefectures/11" {
			t.Fatalf("navigation %+v", p)
		}
	}
}
func TestDiscoverFollowsNextAndHonorsIndependentPageCap(t *testing.T) {
	for _, tc := range []struct {
		cap, wantRecords, wantNext int
		complete                   bool
	}{{1, 1, 2, false}, {2, 2, 3, false}} {
		c := New(2)
		requests := []string{}
		c.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests = append(requests, req.URL.String())
			page := 1
			if req.URL.Query().Get("page") == "2" {
				page = 2
			}
			body := navigationPage(page, true)
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
		})
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		view, records, e := c.Discover(ctx, "events", 6, 11, tc.cap, Query{Kind: "events", AgeMonths: -1, Limit: 1})
		cancel()
		if e != nil {
			t.Fatal(e)
		}
		if len(requests) != tc.cap || len(records) != tc.wantRecords || view.ScannedRecords != tc.wantRecords || len(view.Records) != 1 || view.Coverage[0].PagesScanned != tc.cap || view.Coverage[0].NextPage != tc.wantNext || view.Coverage[0].CompleteForListing != tc.complete || view.Coverage[0].SourceComplete {
			t.Fatalf("cap %d: requests=%v view=%+v", tc.cap, requests, view)
		}
		if tc.cap == 2 && !strings.HasSuffix(requests[1], "?page=2") {
			t.Fatalf("skipped next page: %v", requests)
		}
	}
}
func TestDiscoverRejectsUnobservedPrefecture(t *testing.T) {
	for _, tc := range []struct {
		name       string
		prefecture int
		body       string
	}{
		{"missing ID", 13, navigationPage(1, false)},
		{"same ID under wrong region", 11, strings.ReplaceAll(navigationPage(1, false), "/regions/6/", "/regions/5/")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := New(2)
			c.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader(tc.body)), Request: req}, nil
			})
			_, _, err := c.Discover(context.Background(), "events", 6, tc.prefecture, 1, Query{Kind: "events", AgeMonths: -1, Limit: 5})
			if err == nil || !strings.Contains(err.Error(), "not an observed source option") {
				t.Fatalf("invalid prefecture accepted: %v", err)
			}
		})
	}
}
