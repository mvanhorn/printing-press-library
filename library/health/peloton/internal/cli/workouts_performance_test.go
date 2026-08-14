// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/store"
)

// TestWorkoutsPerformanceLiveFetchPopulatesOfflineRead guards the fix for
// the generic write-through cache's structural blind spot on this one
// command: performance_graph responses carry no id field (the workout id
// lives only in the request path, which write-through never sees), so a
// live "auto"-mode fetch always warned "not cached locally" and left
// `offline performance <id>` empty for data the live call had just
// returned successfully. This drives the real `workouts performance`
// command end-to-end against a fake server (not a live account) and
// asserts the local store picks it up.
func TestWorkoutsPerformanceLiveFetchPopulatesOfflineRead(t *testing.T) {
	home := t.TempDir()
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"metrics":[{"display_name":"Output","values":[1,2,3]}]}`))
	}))
	defer server.Close()
	t.Setenv("PELOTON_BASE_URL", server.URL)
	seedValidOAuthBundleForLiveFetchTests(t)

	root := newRootCmd(&rootFlags{})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"workouts", "performance", "w1", "--home", home, "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("workouts performance: %v\noutput: %s", err, out.String())
	}

	db, err := store.OpenReadOnly(defaultDBPath("peloton-pp-cli"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer db.Close()

	fact, err := db.GetProviderFact("performance", "w1")
	if err != nil {
		t.Fatalf("live fetch did not populate offline performance data for w1: %v", err)
	}
	if len(fact.Payload) == 0 {
		t.Fatal("stored performance fact has empty payload")
	}
}
