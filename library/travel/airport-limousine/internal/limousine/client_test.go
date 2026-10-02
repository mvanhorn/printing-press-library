// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package limousine

import (
	"context"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/airport-limousine/internal/cliutil"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPublicSearchStateIsEphemeralAndNoThrottleSilence(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method == "POST" {
			if r.Header.Get("Origin") != Origin || r.Header.Get("Referer") != Origin+"/en/busstop/" {
				t.Error("missing same-origin search headers")
			}
			r.ParseForm()
			if r.Form.Get("keyword") != "Shinjuku" {
				t.Error("wrong keyword body")
			}
			http.SetCookie(w, &http.Cookie{Name: "BusstopSearchKeywords", Value: "Shinjuku", Path: "/"})
			fmt.Fprint(w, `{"type":"success","status":200,"data":"[]"}`)
			return
		}
		cookie, e := r.Cookie("BusstopSearchKeywords")
		if e != nil || cookie.Value != "Shinjuku" {
			t.Error("keyword cookie did not reach read")
		}
		fmt.Fprint(w, `{"type":"data","nodes":[null,{"type":"data","data":[{"busstops":1},[]]}]}`)
	}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	p := &Provider{HTTP: server.Client(), Base: server.URL, Limiter: cliutil.NewAdaptiveLimiter(100)}
	p.HTTP.Jar = jar
	d, e := p.StopSearch(context.Background(), "Shinjuku")
	if e != nil || d["busstops"] == nil || hits != 2 || p.Requests != 2 {
		t.Fatalf("search contract %+v %v hits %d", d, e, hits)
	}
	throttled := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Retry-After", "1"); w.WriteHeader(429) }))
	defer throttled.Close()
	p = &Provider{HTTP: throttled.Client(), Base: throttled.URL, Limiter: cliutil.NewAdaptiveLimiter(100)}
	_, e = p.Data(context.Background(), "/en/guide/realtime/__data.json")
	var rate *cliutil.RateLimitError
	if !errors.As(e, &rate) {
		t.Fatalf("429 was not typed: %v", e)
	}
}
func TestRequestBodyCapAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, strings.Repeat("x", MaxBodyBytes+1)) }))
	defer server.Close()
	p := &Provider{HTTP: server.Client(), Base: server.URL, Limiter: cliutil.NewAdaptiveLimiter(100)}
	if _, e := p.Fetch(context.Background(), "GET", "/en/", ""); e == nil {
		t.Fatal("oversized body accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	if _, e := p.Fetch(ctx, "GET", "/en/", ""); e == nil {
		t.Fatal("cancellation ignored")
	}
}

type providerRoundTripFunc func(*http.Request) (*http.Response, error)

func (f providerRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestProviderRefusesRedirectsWithinWireBudget(t *testing.T) {
	p := New(time.Second, 0)
	wire := 0
	p.HTTP.Transport = providerRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		wire++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{Origin + "/en/redirected/"}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	})
	_, err := p.Fetch(context.Background(), "GET", "/en/", "")
	if err == nil || wire != 1 || p.Requests != 1 {
		t.Fatalf("redirect bypassed request policy: wire=%d reported=%d err=%v", wire, p.Requests, err)
	}
}
