// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"strings"
	"testing"
)

// 🔴 Commands that answered from the network must say so. printJSONFiltered
// stamps `"source": "local"` by default, which is right for the compiled-in
// taxonomy and wrong for every command that just queried Overpass — `near` was
// emitting meta.source "local" beside a mirror_attempts array showing a live
// HTTP 200, self-contradictory inside one response. An agent keying off
// meta.source concludes it was handed a cache.
//
// The file-level `pp:data-source live` annotation already declared the intent;
// nothing enforced it, so this does. Asserted on the SOURCE rather than by
// running a command, because the whole point is that these hit the network.
func TestLiveCommandsUsePrintJSONLive(t *testing.T) {
	for _, f := range []string{"near.go", "route.go", "mirrors.go"} {
		src := readSource(t, f)
		if strings.Contains(src, "flags.printJSON(cmd,") {
			t.Errorf("%s is annotated `pp:data-source live` but calls "+
				"flags.printJSON, which stamps meta.source=\"local\"; "+
				"use flags.printJSONLive", f)
		}
		if !strings.Contains(src, "pp:data-source live") {
			t.Errorf("%s lost its `pp:data-source live` annotation — the "+
				"declaration this test enforces", f)
		}
	}
	// types is genuinely local (compiled-in taxonomy, no network) and must
	// NOT be swept along. A test that passes for the wrong reason is worse
	// than none, so the negative case is asserted too.
	if strings.Contains(readSource(t, "types.go"), "printJSONLive") {
		t.Error("types.go reads the compiled-in taxonomy and must stay local")
	}
}

func readSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("internal/cli/" + name)
	if err != nil {
		// package tests run with CWD = the package dir
		b, err = os.ReadFile(name)
	}
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(b)
}
