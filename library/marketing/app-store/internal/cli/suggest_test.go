// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

func hintsHandler(t *testing.T) func(r *http.Request) fakeResponse {
	insta := fixtureBytes(t, "hints_insta_us.plist")
	return func(r *http.Request) fakeResponse {
		if r.URL.Host != "search.itunes.apple.com" {
			t.Errorf("unexpected request %s", r.URL)
			return notFoundResp()
		}
		if r.URL.Query().Get("term") == "insta" {
			return okBody(insta)
		}
		return okBody([]byte(`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>hints</key><array></array></dict></plist>`))
	}
}

func TestSuggestReturnsPrefixHints(t *testing.T) {
	env := newS2bEnv(t, false, hintsHandler(t))
	out, stderr, err := env.run("suggest", "insta", "--json")
	if err != nil {
		t.Fatalf("suggest: %v\n%s", err, stderr)
	}
	var got suggestResult
	decodeJSON(t, out, &got)
	if got.Term != "insta" || got.Storefront != "us" || len(got.Suggestions) < 5 {
		t.Fatalf("got %+v", got)
	}
	for _, s := range got.Suggestions {
		if !strings.HasPrefix(s, "insta") {
			t.Fatalf("suggestion %q does not start with insta", s)
		}
	}
	env.fake.mu.Lock()
	sf := env.fake.requests[0].Header.Get("X-Apple-Store-Front")
	env.fake.mu.Unlock()
	if sf != "143441,12" {
		t.Fatalf("X-Apple-Store-Front = %q, want 143441,12", sf)
	}
	caps, _ := store.Captures(context.Background(), env.store(), store.CaptureFilter{})
	if len(caps) != 1 || caps[0].Host != "search.itunes.apple.com" {
		t.Fatalf("capture log = %+v", caps)
	}
	out, _, err = env.run("suggest", "insta", "--limit", "3", "--json")
	if err != nil {
		t.Fatal(err)
	}
	decodeJSON(t, out, &got)
	if len(got.Suggestions) != 3 {
		t.Fatalf("--limit 3 returned %d", len(got.Suggestions))
	}
}

func TestSuggestEmptyTermExits2WithoutNetwork(t *testing.T) {
	env := newS2bEnv(t, false, hintsHandler(t))
	for _, args := range [][]string{{"suggest", ""}, {"suggest", "   "}, {"suggest", "insta", "--limit", "-1"}} {
		if _, _, err := env.run(append(args, "--json")...); ExitCode(err) != 2 {
			t.Fatalf("%v: exit %d (%v), want 2", args, ExitCode(err), err)
		}
	}
	if n := len(env.fake.urls()); n != 0 {
		t.Fatalf("sent %d requests for an empty term", n)
	}
}

func TestSuggestNoHintsIsHonest(t *testing.T) {
	env := newS2bEnv(t, false, hintsHandler(t))
	out, _, err := env.run("suggest", "zzqxj", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got suggestResult
	decodeJSON(t, out, &got)
	if got.Suggestions == nil || len(got.Suggestions) != 0 || !strings.Contains(got.Note, "verified empty") {
		t.Fatalf("got %+v", got)
	}
	if !strings.Contains(out, `"suggestions": []`) {
		t.Fatalf("empty suggestions must print as []:\n%s", out)
	}
}

func TestSuggestThrottleExits7(t *testing.T) {
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse { return throttled() })
	if _, _, err := env.run("suggest", "insta", "--json"); ExitCode(err) != 7 {
		t.Fatalf("exit %d (%v), want 7", ExitCode(err), err)
	}
}
