// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"reflect"
	"testing"
)

func TestDefaultSyncResourcesUseValidMovieFilters(t *testing.T) {
	t.Parallel()

	want := []string{"comingsoon", "intheaters"}
	if got := defaultSyncResources(); !reflect.DeepEqual(got, want) {
		t.Fatalf("defaultSyncResources() = %v, want %v", got, want)
	}
	for _, resource := range want {
		path, err := syncResourcePath(resource)
		if err != nil {
			t.Fatalf("syncResourcePath(%q): %v", resource, err)
		}
		if path == "/Fandango/Movies" {
			t.Fatalf("sync path for %q omits the API's required Type query", resource)
		}
	}
}
