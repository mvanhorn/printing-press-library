// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestNovelQrollupHelpWires smoke-tests that the qrollup command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelQrollupHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"qrollup", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("qrollup --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "qrollup"} {
		if !strings.Contains(help, want) {
			t.Fatalf("qrollup --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestBuildQueueRowsPreservesEveryStatsSource(t *testing.T) {
	queues := []map[string]json.RawMessage{{"Number": json.RawMessage(`"800"`)}}
	stats := map[string]map[string][]json.RawMessage{
		"800": {
			"performance": {json.RawMessage(`{"QueueDn":"800","Calls":2}`)},
			"sla":         {json.RawMessage(`{"QueueDn":"800","Breaches":1}`)},
		},
	}
	rows, _ := buildQueueRows(queues, stats)
	if len(rows) != 1 || len(rows[0].Stats) != 2 {
		t.Fatalf("stats sources were overwritten: %#v", rows)
	}
}

func TestQrollupWithinWindow(t *testing.T) {
	cutoff := time.Now().UTC().Add(-time.Hour)
	recent := fmt.Sprintf(`{"CallTime":%q}`, time.Now().UTC().Format(time.RFC3339))
	old := fmt.Sprintf(`{"CallTime":%q}`, time.Now().UTC().Add(-2*time.Hour).Format(time.RFC3339))
	if !qrollupWithinWindow(json.RawMessage(recent), cutoff) || qrollupWithinWindow(json.RawMessage(old), cutoff) {
		t.Fatal("qrollup time-window filter did not distinguish recent and old rows")
	}
}
