// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/japan47go/internal/cliutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBoundedDiscoveryAndMatchingCount(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("tag") != "62" || r.URL.Query().Get("keyword") != "妻籠" {
			t.Error(r.URL)
		}
		page := r.URL.Query().Get("page")
		fmt.Fprintf(w, `<script id="__NEXT_DATA__">{"props":{"pageProps":{"totalCount":107769,"items":[{"slug":"`+tsumago+`","name":"妻籠宿案内人の会","categoryTag":{"id2":617}}],"pageInfo":{"totalCnt":27,"perPage":20,"totalPageCnt":2,"pageNo":%s}}}}</script>`, page)
	}))
	defer srv.Close()
	c := NewClient(0)
	c.Base = srv.URL
	c.HTTP = srv.Client()
	d, e := c.Discover(context.Background(), DiscoverOptions{Query: "妻籠", Kind: "guides", MaxPages: 1, Limit: 3})
	if e != nil || calls != 1 || len(d.Candidates) != 1 || d.Coverage.MatchingTotal != 27 || d.Coverage.NextPage == nil || d.Coverage.Complete {
		t.Fatal(d, e, calls)
	}
	d, e = c.Discover(context.Background(), DiscoverOptions{Query: "妻籠", Kind: "guides", MaxPages: 2, Limit: 3})
	if e != nil || len(d.Candidates) != 1 || d.Coverage.Records != 2 || !d.Coverage.Complete {
		t.Fatal(d, e)
	}
}
func TestTypedThrottleAndUnavailable(t *testing.T) {
	for _, code := range []int{429, 503} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }))
		c := NewClient(0)
		c.Base = srv.URL
		c.HTTP = srv.Client()
		_, e := c.Get(context.Background(), tsumago)
		srv.Close()
		if code == 429 {
			var rate *cliutil.RateLimitError
			if !errors.As(e, &rate) {
				t.Fatal(e)
			}
		} else {
			var h *HTTPError
			if !errors.As(e, &h) || h.Status != code {
				t.Fatal(e)
			}
		}
	}
}
