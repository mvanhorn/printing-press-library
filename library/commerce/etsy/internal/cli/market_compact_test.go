// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"github.com/spf13/cobra"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMarketCompactJSONTags(t *testing.T) {
	types := map[string]any{"demand": mktDemandTerm{}, "shops": mktShopRow{}, "price": mktPriceGroup{}, "gaps": mktGapTarget{}, "tags": mktTermStat{}, "listings": mktDemandListing{}}
	for name, value := range types {
		t.Run(name, func(t *testing.T) {
			keys := mktCompactKeys[name]
			if name == "listings" {
				keys = mktCompactListingKeys
			}
			typ := reflect.TypeOf(value)
			tags := map[string]bool{}
			for i := 0; i < typ.NumField(); i++ {
				tags[strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]] = true
			}
			for _, key := range keys {
				if !tags[key] {
					t.Fatalf("missing json tag %s", key)
				}
			}
		})
	}
}

func compactAssertKeys(t *testing.T, rows any, keys []string) {
	t.Helper()
	list, ok := rows.([]any)
	if !ok || len(list) == 0 {
		t.Fatalf("missing rows: %v", rows)
	}
	for _, row := range list {
		obj := row.(map[string]any)
		allowed := map[string]bool{}
		for _, key := range keys {
			allowed[key] = true
			if key != "note" && key != "currency" && key != "matches_all" {
				if _, ok := obj[key]; !ok {
					t.Errorf("missing %s in %v", key, obj)
				}
			}
		}
		for key := range obj {
			if !allowed[key] {
				t.Errorf("unexpected %s", key)
			}
		}
	}
}

func TestMarketCompact(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	defer db.Close()
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	snap := mktFxSnapshot(t, db.DB(), at, []string{"linen apron"}, 1, "complete", mktFxProg{})
	mktFxShop(t, db.DB(), snap, 9000001, 500, 20, 1600000000)
	for i := 0; i < 20; i++ {
		mktFxResult(t, db.DB(), snap, mktFxListing{term: "linen apron", pos: i + 1, id: 1000000001 + int64(i), shop: 9000001, title: "linen apron", price: 2400 + int64(i), tags: []string{"linen apron"}, taxonomy: 1234})
	}
	run := func(args ...string) map[string]any {
		t.Helper()
		out, err := mktRunCLI(t, args...)
		if err != nil {
			t.Fatal(err, out)
		}
		return mktDecode(t, out)
	}
	rules := `{"targets":[{"name":"linen apron","include":["linen apron"]}]}`
	for _, name := range []string{"demand", "shops", "price", "gaps", "tags"} {
		t.Run(name, func(t *testing.T) {
			args := []string{"market", name}
			if name == "gaps" {
				args = append(args, "--rules-json", rules)
			}
			full := run(args...)
			projected := run(append(append([]string{}, args...), "--compact")...)
			check := func(m map[string]any) {
				if name == "tags" {
					obj := m["results"].(map[string]any)
					compactAssertKeys(t, obj["tags"], mktCompactKeys[name])
					compactAssertKeys(t, obj["title_words"], mktCompactKeys[name])
					if !reflect.DeepEqual(obj["basis"], full["results"].(map[string]any)["basis"]) {
						t.Fatal("basis changed")
					}
				} else {
					compactAssertKeys(t, m["results"], mktCompactKeys[name])
				}
			}
			check(projected)
			// Explicit compact still receives the normal agent wrapper.
			agent := run(append(append([]string{}, args...), "--agent", "--compact")...)
			if agent["meta"] == nil {
				t.Fatal("agent wrapper missing")
			}
			check(agent["results"].(map[string]any))
			stripped := map[string]string{"demand": "order_basis", "shops": "age_years", "price": "min", "gaps": "distinct_shops", "tags": "top_rate"}[name]
			path := "results." + stripped
			if name == "tags" {
				path = "results.tags." + stripped
			}
			selectedPath := run(append(append([]string{}, args...), "--compact", "--select", path)...)
			selectedRows := selectedPath["results"]
			fullRows := full["results"]
			projectedRows := projected["results"]
			if name == "tags" {
				selectedRows = selectedRows.(map[string]any)["tags"]
				fullRows = fullRows.(map[string]any)["tags"]
				projectedRows = projectedRows.(map[string]any)["tags"]
			}
			compactAssertKeys(t, selectedRows, []string{stripped})
			for i, row := range fullRows.([]any) {
				original := row.(map[string]any)
				reduced := projectedRows.([]any)[i].(map[string]any)
				if _, exists := original[stripped]; !exists {
					t.Fatalf("fixture lacks %s", stripped)
				}
				for key, value := range reduced {
					if key != "listings" && !reflect.DeepEqual(value, original[key]) {
						t.Fatalf("projected value %s changed", key)
					}
				}
				if note, exists := original["note"]; exists && reduced["note"] != note {
					t.Fatal("row note changed")
				}
			}

			if name == "demand" {
				for _, row := range projected["results"].([]any) {
					compactAssertKeys(t, row.(map[string]any)["listings"], mktCompactListingKeys)
				}
			}
			for key, value := range full {
				if key != "results" && !reflect.DeepEqual(value, projected[key]) {
					t.Errorf("metadata %s changed", key)
				}
			}
			for _, flag := range []string{"--compact=false", "--agent"} {
				m := run(append(append([]string{}, args...), flag)...)
				if flag == "--agent" {
					m = m["results"].(map[string]any)
				}
				if !reflect.DeepEqual(m["results"], full["results"]) {
					t.Fatalf("%s altered full rows", flag)
				}
			}
			selected := run(append(append([]string{}, args...), "--compact", "--select", "results")...)
			if !reflect.DeepEqual(selected["results"], full["results"]) {
				t.Fatal("select lost fields")
			}
		})
	}
	mktFxSnapshot(t, db.DB(), at.Add(24*time.Hour), []string{"linen apron"}, 1, "complete", mktFxProg{})
	for _, name := range []string{"trend"} {
		t.Run(name, func(t *testing.T) {
			full := run("market", name)
			compact := run("market", name, "--compact")
			if !reflect.DeepEqual(full, compact) {
				t.Fatal("unchanged command differs")
			}
		})
	}
}

// A terminal override exercises structured-output gating without a real TTY.
type mktCompactTerminal struct{ bytes.Buffer }

func (*mktCompactTerminal) TerminalOverride() bool { return true }

func TestMarketCompactOutputModes(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		terminal, asJSON, projected bool
	}{
		{"pipe", false, false, true}, {"terminal", true, false, false}, {"terminal_json", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flags := &rootFlags{compact: true, asJSON: tc.asJSON}
			parent := &cobra.Command{Use: "market"}
			cmd := &cobra.Command{Use: "demand"}
			parent.AddCommand(cmd)
			cmd.Flags().Bool("compact", false, "")
			if err := cmd.Flags().Set("compact", "true"); err != nil {
				t.Fatal(err)
			}
			var pipe bytes.Buffer
			tty := &mktCompactTerminal{}
			if tc.terminal {
				cmd.SetOut(tty)
			} else {
				cmd.SetOut(&pipe)
			}
			if err := mktEmit(cmd, flags, []mktDemandTerm{{OrderBasis: "position", Term: "linen apron", Listings: []mktDemandListing{}}}, nil, "", nil...); err != nil {
				t.Fatal(err)
			}
			text := pipe.String()
			if tc.terminal {
				text = tty.String()
			}
			// Even terminal rendering retains this full-row column when no projection runs.
			if strings.Contains(text, "order_basis") == tc.projected {
				t.Fatal(text)
			}
			if !flags.compact {
				t.Fatal("caller flags mutated")
			}
		})
	}
	for _, name := range []string{"snapshot", "trend"} {
		t.Run(name, func(t *testing.T) {
			emit := func(compact bool) map[string]any {
				flags := &rootFlags{compact: compact, asJSON: true}
				parent := &cobra.Command{Use: "market"}
				cmd := &cobra.Command{Use: name}
				parent.AddCommand(cmd)
				cmd.Flags().Bool("compact", false, "")
				if compact {
					if err := cmd.Flags().Set("compact", "true"); err != nil {
						t.Fatal(err)
					}
				}
				var out bytes.Buffer
				cmd.SetOut(&out)
				if err := mktEmit(cmd, flags, map[string]any{"id": 1000000001, "state": "complete", "sections": []any{}}, nil, "", nil...); err != nil {
					t.Fatal(err)
				}
				return mktDecode(t, out.String())
			}
			if !reflect.DeepEqual(emit(false), emit(true)) {
				t.Fatal("unprojected results changed")
			}
		})
	}
}
