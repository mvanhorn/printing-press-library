package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func mktSelectionPopulated(t *testing.T) {
	t.Helper()
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	id := mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 1, "complete", mktFxProg{})
	mktFxResult(t, db.DB(), id, mktFxListing{term: "linen apron", pos: 1, id: 1000000001, shop: 9000001, title: "linen apron", price: 2400, tags: []string{"linen apron"}})
	mktFxShop(t, db.DB(), id, 9000001, 100, 20, time.Now().Add(-400*24*time.Hour).Unix())
	db.Close()
}

func TestMarketSelectTrendOneCompleteSnapshot(t *testing.T) {
	mktSelectionPopulated(t)
	out, err := mktRunCLI(t, "market", "trend", "--select", "results.new_listings")
	if err != nil || !strings.Contains(out, "need at least two complete snapshots") {
		t.Fatalf("MARKET_SELECT_TREND_HINT: %v %s", err, out)
	}
}

func TestMarketSelectTypoPopulatedValidFields(t *testing.T) {
	for _, tc := range []struct{ command, field string }{{"shops", "bogus"}, {"tags", "results.bogus"}} {
		t.Run(tc.command, func(t *testing.T) {

			mktSelectionPopulated(t)
			baseline, err := mktRunCLI(t, "market", tc.command)
			if err != nil {
				t.Fatal(err)
			}
			keys := selectFieldKeys(json.RawMessage(baseline))
			if !strings.Contains(","+strings.Join(keys, ",")+",", ",notes,") {
				t.Fatalf("MARKET_SELECT_WARNING_KEYS: notes missing from %v", keys)
			}
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("MARKET_SELECT_PIPE: %v", err)
			}
			old := os.Stderr
			os.Stderr = w
			t.Cleanup(func() { os.Stderr = old; r.Close(); w.Close() })
			out, err := mktRunCLI(t, "market", tc.command, "--select", tc.field)
			w.Close()
			captured, readErr := io.ReadAll(r)
			if readErr != nil || err == nil || ExitCode(err) != 2 {
				t.Fatalf("MARKET_SELECT_TYPO_EXIT: %v %v %s", err, readErr, out)
			}
			want := fmt.Sprintf("warning: --select %q matched no fields; valid fields: %s\n", tc.field, strings.Join(keys, ", "))
			if string(captured) != want {
				t.Fatalf("MARKET_SELECT_WARNING_KEYS: stderr %q want %q", captured, want)
			}
		})
	}
}

func TestMarketSelectCaseFold(t *testing.T) {
	for _, tc := range []struct{ name, field string }{{"Results_shop_id", "Results.shop_id"}, {"RESULTS_shop_id", "RESULTS.shop_id"}} {
		field := tc.field
		t.Run(tc.name, func(t *testing.T) {
			mktSelectionPopulated(t)
			out, err := mktRunCLI(t, "market", "shops", "--select", field)
			if err != nil {
				t.Fatalf("MARKET_SELECT_CASE: %v %s", err, out)
			}
			m := mktDecode(t, out)
			if _, ok := m["notes"].([]any); !ok {
				t.Fatalf("MARKET_SELECT_CASE_NOTES: %s", out)
			}
			rows, ok := m["results"].([]any)
			if !ok || len(rows) != 1 || rows[0].(map[string]any)["shop_id"] != float64(9000001) {
				t.Fatalf("MARKET_SELECT_CASE_ROWS: %s", out)
			}
		})
	}
}

func TestMarketSelectIndeterminate(t *testing.T) {
	for _, tc := range []struct {
		raw, path string
		want      bool
	}{
		{`{"results":[],"other":[1]}`, "term", true},
		{`{"results":{"nested":{"rows":[]}}}`, "term", true},
		{`{"results":{"a":[],"b":[1]}}`, "term", false},
		{`{"results":[{"shop_id":1}],"other":[]}`, "bogus", false},
		{`{"results":{"a":[]}}`, "results.bogus", false},
		{`{"results":{"a":null}}`, "Results.a.name", true},
		{`{"results":[{"a":[]}]}`, "results.a.name", true},
		{`{"results":[{"a":1}]}`, "results.bogus", false},
	} {
		if got := mktSelectIndeterminate([]byte(tc.raw), strings.Split(tc.path, ".")); got != tc.want {
			t.Fatalf("MARKET_SELECT_INDETERMINATE: %s %s got %v want %v", tc.raw, tc.path, got, tc.want)
		}
	}
}

func TestMarketSelectNullResults(t *testing.T) {
	for _, results := range []any{nil, []any(nil), map[string]any(nil), json.RawMessage("null")} {
		cmd := RootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		flags := &rootFlags{selectFields: "Results.shop_id"}
		if err := mktEmit(cmd, flags, results, nil, mktNoSnapshotHint); err != nil {
			t.Fatalf("MARKET_SELECT_NULL_REJECTED: %v", err)
		}
		m := mktDecode(t, out.String())
		rows, ok := m["results"].([]any)
		if !ok || len(rows) != 0 || flags.selectFields != "Results.shop_id" {
			t.Fatalf("MARKET_SELECT_NULL_SHAPE: %s flags %s", out.String(), flags.selectFields)
		}
	}
}

func TestMarketSelectNotesPartialMiss(t *testing.T) {
	mktSelectionPopulated(t)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = old; r.Close(); w.Close() })
	out, err := mktRunCLI(t, "market", "shops", "--select", "notes,bogus")
	w.Close()
	captured, readErr := io.ReadAll(r)
	if err != nil || readErr != nil {
		t.Fatalf("MARKET_SELECT_NOTES_PARTIAL_REJECTED: %v %v %s", err, readErr, out)
	}
	if !strings.Contains(string(captured), "warning: --select "+strconv.Quote("bogus")+" matched no fields; valid fields:") {
		t.Fatalf("MARKET_SELECT_NOTES_PARTIAL_WARNING: %q", captured)
	}
	if _, ok := mktDecode(t, out)["notes"].([]any); !ok {
		t.Fatalf("MARKET_SELECT_NOTES_PARTIAL_SHAPE: %s", out)
	}
}
