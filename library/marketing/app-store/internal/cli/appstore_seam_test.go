// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// The shared test seam for the hand-built App Store commands: every client
// newAppStoreClient builds is routed through a fake Apple transport with a
// private cache, fast pacing and (optionally) a pinned clock. The slice
// harnesses (s2a, s2b) and the integration tests all use it.

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
)

// fakeAppStore configures useFakeAppStore.
type fakeAppStore struct {
	// Transport answers every request (a RoundTripper or an httptest rewrite).
	Transport http.RoundTripper
	// CacheDir is the client cache directory (a test temp dir).
	CacheDir string
	// Cached keeps the response cache on; otherwise every client is NoCache.
	Cached bool
	// Now, when set, is the client clock (capture times and cache expiry).
	Now func() time.Time
	// StatePath, when set, is the governor state file every client of the
	// test shares, so consecutive commands behave like consecutive CLI runs
	// (throttle holds and pacing carry over). Empty keeps each client's
	// governor in memory and never touches the real state directory.
	StatePath string
}

// useFakeAppStore routes every App Store client the commands build through
// f for the rest of the test.
func useFakeAppStore(t *testing.T, f fakeAppStore) {
	t.Helper()
	prev := appStoreClientHook
	appStoreClientHook = func(o *appstore.Options) {
		o.HTTPClient = &http.Client{Transport: f.Transport}
		o.CacheDir = f.CacheDir
		o.NoCache = o.NoCache || !f.Cached
		rates := map[appstore.RateClass]float64{}
		for class := range appstore.DefaultRates {
			rates[class] = 1000
		}
		o.Rates = rates
		o.MaxRate = 0
		o.Jitter = -1
		o.RetryDelay = time.Millisecond
		o.StatePath = f.StatePath
		o.Notice = nil
		if f.Now != nil {
			o.Now = f.Now
		}
	}
	t.Cleanup(func() { appStoreClientHook = prev })
}

func TestNewAppStoreClientUsesTheSharedSeam(t *testing.T) {
	fake := &fakeApple{handler: func(r *http.Request) fakeResponse { return synthLookup(r, nil) }}
	useFakeAppStore(t, fakeAppStore{Transport: fake, CacheDir: t.TempDir()})
	c, err := newAppStoreClient(&rootFlags{country: "gb"})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Lookup(t.Context(), appstore.LookupQuery{IDs: []string{"324684580"}})
	if err != nil || len(resp.Results) != 1 {
		t.Fatalf("lookup through the seam = %+v %v", resp, err)
	}
	urls := fake.urls()
	if len(urls) != 1 || !strings.Contains(urls[0], "country=gb") {
		t.Fatalf("the seam did not carry the request to the fake transport with the flag storefront: %v", urls)
	}
	// The hook leaves the root flags in charge of everything it does not set.
	if c.Country() != "gb" {
		t.Fatalf("country = %s", c.Country())
	}
}
