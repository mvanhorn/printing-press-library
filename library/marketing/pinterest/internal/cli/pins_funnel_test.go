// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/pinterest/internal/cliutil/testenv"
)

func TestComputeFunnelPercentilesAndRates(t *testing.T) {
	pins := []topPinRow{{"high", map[string]any{"IMPRESSION": 100.0, "SAVE": 80.0, "OUTBOUND_CLICK": 1.0, "PIN_CLICK": 3.0}}, {"b", map[string]any{"IMPRESSION": 100.0, "SAVE": 20.0, "OUTBOUND_CLICK": 20.0}}, {"c", map[string]any{"IMPRESSION": 100.0, "SAVE": 30.0, "OUTBOUND_CLICK": 30.0}}, {"d", map[string]any{"IMPRESSION": 100.0, "SAVE": 40.0, "OUTBOUND_CLICK": 40.0}}, {"zero", map[string]any{"IMPRESSION": 0.0, "SAVE": 2.0, "OUTBOUND_CLICK": 2.0}}}
	rows := computeFunnel(pins)
	if !rows[0].SavedNotClicked || rows[1].SavedNotClicked || rows[2].SavedNotClicked || rows[3].SavedNotClicked || rows[4].SavedNotClicked {
		t.Fatalf("flags = %#v", rows)
	}
	if rows[0].SaveRate != .8 || rows[0].OutboundClickRate != .01 || rows[4].SaveRate != 0 || rows[4].OutboundClickRate != 0 {
		t.Fatalf("rates = %#v", rows)
	}
	if computeFunnel(pins[:3])[0].SavedNotClicked {
		t.Fatal("flagged with fewer than four Pins")
	}
}

func TestPinsFunnelSendsQuery(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("PINTEREST_ACCESS_TOKEN", "test-token")
	srv := newLoopbackTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("num_of_pins") != "10" || r.URL.Query().Get("sort_by") != "SAVE" || len(r.URL.Query()["metric_types"]) != 1 || r.URL.Query().Get("metric_types") != "IMPRESSION,SAVE,OUTBOUND_CLICK,PIN_CLICK" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"pins":[{"pin_id":"p1","metrics":{"IMPRESSION":10,"SAVE":2}}]}`))
	}))
	t.Setenv("PINTEREST_BASE_URL", srv.URL)
	out, err := runPinterestCmd(t, "", "pins", "funnel", "--limit=10", "--sort-by=SAVE", "--json")
	if err != nil {
		t.Fatalf("funnel: %v %s", err, out)
	}
	var got struct {
		Pins []funnelRow `json:"pins"`
	}
	if err = json.Unmarshal([]byte(out), &got); err != nil || len(got.Pins) != 1 {
		t.Fatalf("response %s: %v", out, err)
	}
}

// TestNovelPinsFunnelHelpWires smoke-tests that the pins funnel command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPinsFunnelHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"pins", "funnel", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pins funnel --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "funnel"} {
		if !strings.Contains(help, want) {
			t.Fatalf("pins funnel --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestComputeFunnelNoFlagsWhenRatesFlat(t *testing.T) {
	pins := make([]topPinRow, 0, 5)
	for i := 0; i < 5; i++ {
		pins = append(pins, topPinRow{PinID: string(rune('a' + i)), Metrics: map[string]any{"IMPRESSION": float64(1000)}})
	}
	for _, r := range computeFunnel(pins) {
		if r.SavedNotClicked {
			t.Fatalf("pin %s flagged with zero saves: %+v", r.PinID, r)
		}
	}
}
