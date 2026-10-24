// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// The App Store governor across CLI invocations. Live on 2026-10-23,
// apps.apple.com throttled 2 of 6 uncached page fetches; the client's
// limiter and governor lived only inside one process, so the 429 was
// forgotten when the command exited and the next invocation sent right away.

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"
)

func TestAppStoreGovernorStateLivesInTheStateDir(t *testing.T) {
	home := t.TempDir()
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
	c, err := newAppStoreClient(&rootFlags{noCache: true})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "state", appstore.GovernorStateFile)
	if c.StatePath() != want {
		t.Fatalf("governor state = %q, want %q (inside --home)", c.StatePath(), want)
	}
}

func TestThrottleIsRememberedAcrossInvocations(t *testing.T) {
	f := newS2AFake(t)
	state := filepath.Join(t.TempDir(), "state", appstore.GovernorStateFile)
	useFakeAppStore(t, fakeAppStore{Transport: f.client.Transport, CacheDir: f.cacheDir, Cached: true, StatePath: state})
	f.fail = func(host, path string, q url.Values) int {
		if host == "apps.apple.com" {
			return http.StatusTooManyRequests
		}
		return 0
	}
	pageRequests := func() int {
		n := 0
		for _, r := range f.requestLog() {
			if strings.HasPrefix(r, "apps.apple.com ") {
				n++
			}
		}
		return n
	}

	// Run 1: the product page is throttled (exit 7).
	res := f.run("app", "553834731", "--json")
	if res.code != 7 || pageRequests() != 1 {
		t.Fatalf("run 1: exit %d (%v), page requests %d", res.code, res.err, pageRequests())
	}

	// Run 2, a later invocation: the Lookup comes from the cache and the
	// page request is refused by the remembered hold, so nothing is sent.
	res = f.run("events", "553834731", "--json")
	if res.code != 7 {
		t.Fatalf("run 2: exit %d (%v), want 7", res.code, res.err)
	}
	if res.requests != 0 || pageRequests() != 1 {
		t.Fatalf("run 2 sent %d request(s) (%d page requests in all) to apps.apple.com right after a 429", res.requests, pageRequests())
	}
	msg := res.err.Error()
	for _, want := range []string{"rate limited by Apple", "Apple answered HTTP 429", "holds every request to apps.apple.com until", "nothing was sent"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("run 2 error %q lacks %q", msg, want)
		}
	}
	if strings.TrimSpace(res.stdout) != "" {
		t.Fatalf("run 2 printed output despite the hold: %s", res.stdout)
	}

	// The hold is per host: a Lookup-only command still reaches itunes.apple.com.
	res = f.mustOK("ratings", "553834731", "--countries", "gb", "--json")
	if res.requests != 1 || pageRequests() != 1 {
		t.Fatalf("lookup during the page hold: requests %d, page requests %d", res.requests, pageRequests())
	}
}
