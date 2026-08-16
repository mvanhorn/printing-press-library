// Copyright 2026 Paul Byrne and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestWeworkDogfoodContractsMaterializeOnCommandTree(t *testing.T) {
	root := RootCmd()

	search, _, err := root.Find([]string{"search"})
	if err != nil {
		t.Fatalf("find search command: %v", err)
	}
	if got := search.Annotations["pp:happy-args"]; got != "query=Austin" {
		t.Fatalf("search pp:happy-args = %q, want stable live query fixture", got)
	}

	geo, _, err := root.Find([]string{"wework-yardi", "list-locations-by-geo"})
	if err != nil {
		t.Fatalf("find geo command: %v", err)
	}
	if got := geo.Annotations["pp:happy-args"]; !strings.Contains(got, "--city=Austin") || !strings.Contains(got, "--user-longitude=-97.7431") {
		t.Fatalf("geo pp:happy-args = %q, want Austin live fixture", got)
	}

	details, _, err := root.Find([]string{"common-booking", "get-booking-details"})
	if err != nil {
		t.Fatalf("find booking details command: %v", err)
	}
	if got := details.Annotations["pp:happy-args"]; got != "--booking-uuid=example-id" {
		t.Fatalf("booking details pp:happy-args = %q, want stable not-found fixture", got)
	}
	if got := details.Annotations["pp:typed-exit-codes"]; got != "0,3" {
		t.Fatalf("booking details pp:typed-exit-codes = %q, want 0,3", got)
	}

	feedback, _, err := root.Find([]string{"feedback"})
	if err != nil {
		t.Fatalf("find feedback command: %v", err)
	}
	if !strings.Contains(feedback.Example, "wework-pp-cli feedback") {
		t.Fatalf("feedback example = %q, want runnable command example", feedback.Example)
	}
	if got := feedback.Annotations["pp:no-error-path-probe"]; got != "true" {
		t.Fatalf("feedback pp:no-error-path-probe = %q, want true", got)
	}
}
