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

// TestClassesFiltersLiveFetchPopulatesOfflineRead guards NEW ISSUE D from a
// third live post-fix verification sweep: `offline classes filters` reads a
// single stored fact at family="filters", id="v1", but nothing wrote it --
// classes_filters.go had no write-through cache call at all (unlike every
// other single-fetch command in this CLI), and the endpoint returns one
// global filter-vocabulary object, not a list the generic write-through
// cache's item-extraction logic could populate it from. This drives the
// real `classes filters` command end-to-end against a fake server and
// asserts a live call now populates the store `offline classes filters`
// reads from.
func TestClassesFiltersLiveFetchPopulatesOfflineRead(t *testing.T) {
	home := t.TempDir()
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"instructors":[{"name":"Ada"}],"disciplines":["cycling"]}`))
	}))
	defer server.Close()
	t.Setenv("PELOTON_BASE_URL", server.URL)
	seedValidOAuthBundleForLiveFetchTests(t)

	root := newRootCmd(&rootFlags{})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"classes", "filters", "--browse-category", "cycling", "--home", home, "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("classes filters: %v\noutput: %s", err, out.String())
	}

	db, err := store.OpenReadOnly(defaultDBPath("peloton-pp-cli"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer db.Close()

	fact, err := db.GetProviderFact("filters", "v1")
	if err != nil {
		t.Fatalf("live fetch did not populate offline filters data: %v", err)
	}
	if len(fact.Payload) == 0 {
		t.Fatal("stored filters fact has empty payload")
	}
}
