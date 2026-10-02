// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"github.com/spf13/cobra"
	"strings"
	"testing"
)

func TestLimousineLiveCommandsRejectLocalBeforeRequests(t *testing.T) {
	constructors := []func(*rootFlags) *cobra.Command{newLimousineRoutesCmd, newLimousineStopsFindCmd, newLimousineStopGetCmd, newLimousineHandoffCmd, newNovelTimetableCmd, newNovelFareCmd, newNovelTravelTimesCmd, newNovelTransfersCmd, newNovelConditionsCmd}
	for _, ctor := range constructors {
		flags := &rootFlags{dataSource: "local"}
		cmd := ctor(flags)
		err := cmd.RunE(cmd, []string{})
		if err == nil || !strings.Contains(err.Error(), "requires live Airport Limousine data") {
			t.Fatalf("%s local mode attempted work: %v", cmd.Name(), err)
		}
	}
}

func TestLimousineDiscoveryCapabilitiesResolve(t *testing.T) {
	for _, query := range []string{"routes", "handoff", "stop search"} {
		matches := rankWhich(whichIndex, query, 3)
		if len(matches) == 0 {
			t.Fatalf("missing capability %s", query)
		}
		expected := query
		if query == "stop search" {
			expected = "stops find"
		}
		found := false
		for _, m := range matches {
			if m.Entry.Command == expected {
				found = true
			}
		}
		if !found {
			t.Fatalf("query %s did not find %s: %+v", query, expected, matches)
		}
	}
}
