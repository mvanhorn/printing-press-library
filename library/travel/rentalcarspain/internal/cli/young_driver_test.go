// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

// The suppliers do not price driver age into their online zero-excess quote
// (verified: ages 23 and 35 return identical prices). Young-driver surcharges
// are collected at the counter, so the tool must disclose that for under-25
// drivers and stay silent for standard-age ones.
func TestYoungDriverNotice(t *testing.T) {
	// Standard-age and unspecified (0) drivers get no caveat.
	for _, age := range []int{0, 25, 30, 35, 70} {
		if note := youngDriverNotice(age); note != "" {
			t.Errorf("youngDriverNotice(%d) = %q, want empty", age, note)
		}
	}
	// Under-25 drivers get the counter-surcharge caveat.
	for _, age := range []int{18, 21, 23, 24} {
		note := youngDriverNotice(age)
		if note == "" {
			t.Errorf("youngDriverNotice(%d) = empty, want a caveat", age)
			continue
		}
		if !strings.Contains(note, "counter") {
			t.Errorf("youngDriverNotice(%d) should mention the counter surcharge; got %q", age, note)
		}
	}
}
