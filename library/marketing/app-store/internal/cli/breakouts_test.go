// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
)

// TestNovelBreakoutsHelpWires smoke-tests that the breakouts command
// resolves at runtime and renders useful --help output.
func TestNovelBreakoutsHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"breakouts", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("breakouts --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "breakouts [flags]", "--min-markets", "--min-gain", "--genre", "--since"} {
		if !strings.Contains(help, want) {
			t.Fatalf("breakouts --help missing %q in output:\n%s", want, help)
		}
	}
}

func breakoutRowBy(rows []breakoutRow, id string) *breakoutRow {
	for i := range rows {
		if rows[i].AppID == id {
			return &rows[i]
		}
	}
	return nil
}

func storefrontCodes[T any](list []T, code func(T) string) string {
	out := make([]string, 0, len(list))
	for _, x := range list {
		out = append(out, code(x))
	}
	return strings.Join(out, ",")
}

func TestBreakoutsExcludesSingleMarketClimbers(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	seedBreakoutsScenario(t, st, s3bTestNow)

	out, _, err := runS3B(t, db, "breakouts", "--genre", "6014", "--chart", "top-free", "--since", "7d", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v breakoutsView
	decodeS3B(t, out, &v)
	if len(v.Results) != 2 || v.Results[0].AppID != "1000000001" || v.Results[1].AppID != "1000000003" {
		t.Fatalf("want hero (3 markets) then steady-two (2 markets):\n%s", out)
	}
	hero, two := v.Results[0], v.Results[1]
	if hero.Markets != 3 || hero.MedianGain != 40 || hero.BestRank != 10 || storefrontCodes(hero.Storefronts, func(m breakoutMarket) string { return m.Storefront }) != "gb,jp,us" {
		t.Fatalf("hero = %+v", hero)
	}
	gb := hero.Storefronts[0]
	if !gb.Entered || gb.FromRank != nil || gb.ToRank != 30 || gb.Gain != 171 {
		t.Fatalf("hero gb entry = %+v, want entered at 30 counted as gain 171", gb)
	}
	if two.Markets != 2 || two.MedianGain != 15 {
		t.Fatalf("steady two = %+v", two)
	}
	if breakoutRowBy(v.Results, "1000000002") != nil {
		t.Fatal("single-market climber (us +80 only) must be excluded")
	}
	if breakoutRowBy(v.Results, "1000000004") != nil || breakoutRowBy(v.Results, "2000000001") != nil {
		t.Fatal("fallers and flat apps are not breakouts")
	}
	m := v.Meta
	if m.BelowMinMarkets != 1 || m.MinMarkets != 2 || m.MinGain != 10 || m.GenreName != "Games" {
		t.Fatalf("meta = %+v", m)
	}
	if got := storefrontCodes(m.StorefrontsCompared, func(c breakoutCompared) string { return c.Storefront }); got != "de,gb,jp,us" {
		t.Fatalf("compared = %s", got)
	}
	skipped := map[string]breakoutSkipped{}
	for _, s := range m.StorefrontsSkipped {
		skipped[s.Storefront] = s
	}
	if len(skipped) != 3 || skipped["fr"].Captures != 1 || skipped["it"].Captures != 0 || !strings.Contains(skipped["ca"].Reason, "same source and row limit") {
		t.Fatalf("skipped = %+v", m.StorefrontsSkipped)
	}

	out, _, err = runS3B(t, db, "breakouts", "--genre", "games", "--min-markets", "3", "--json")
	if err != nil {
		t.Fatal(err)
	}
	v = breakoutsView{}
	decodeS3B(t, out, &v)
	if len(v.Results) != 1 || v.Results[0].AppID != "1000000001" || v.Meta.BelowMinMarkets != 2 {
		t.Fatalf("--min-markets 3 = %s", out)
	}
}

func TestBreakoutsWindowsCountriesAndNegative(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)

	out, _, err := runS3B(t, db, "breakouts", "--genre", "6014", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v breakoutsView
	decodeS3B(t, out, &v)
	if len(v.Results) != 0 || !strings.Contains(v.Meta.Note, "No storefront has two comparable captures") || !strings.Contains(out, `"results": []`) {
		t.Fatalf("empty store = %s", out)
	}

	seedBreakoutsScenario(t, st, s3bTestNow)
	v = breakoutsView{}
	out, _, err = runS3B(t, db, "breakouts", "--genre", "6014", "--since", "all", "--json")
	if err != nil {
		t.Fatal(err)
	}
	decodeS3B(t, out, &v)
	if hero := breakoutRowBy(v.Results, "1000000001"); hero == nil || hero.Markets != 4 {
		t.Fatalf("--since all should compare it (90 -> 9) too: %s", out)
	}

	v = breakoutsView{}
	out, _, err = runS3B(t, db, "breakouts", "--genre", "6014", "--countries", "us,gb,pk", "--json")
	if err != nil {
		t.Fatal(err)
	}
	decodeS3B(t, out, &v)
	if len(v.Results) != 1 || v.Results[0].AppID != "1000000001" || v.Results[0].Markets != 2 {
		t.Fatalf("us,gb,pk = %s", out)
	}
	if len(v.Meta.StorefrontsSkipped) != 1 || v.Meta.StorefrontsSkipped[0].Storefront != "pk" || !strings.Contains(v.Meta.StorefrontsSkipped[0].Reason, "no stored capture") {
		t.Fatalf("pk must be listed as skipped: %+v", v.Meta.StorefrontsSkipped)
	}

	v = breakoutsView{}
	out, _, err = runS3B(t, db, "breakouts", "--genre", "7012", "--json")
	if err != nil {
		t.Fatal(err)
	}
	decodeS3B(t, out, &v)
	if len(v.Results) != 0 || len(v.Meta.StorefrontsCompared) != 0 {
		t.Fatalf("a chart that was never captured must return nothing: %s", out)
	}

	v = breakoutsView{}
	out, _, err = runS3B(t, db, "breakouts", "--genre", "6014", "--countries", "us", "--min-markets", "2", "--json")
	if err != nil {
		t.Fatal(err)
	}
	decodeS3B(t, out, &v)
	if len(v.Results) != 0 || !strings.Contains(v.Meta.Note, "fewer than --min-markets") {
		t.Fatalf("one compared storefront cannot meet --min-markets 2: %s", out)
	}

	for _, args := range [][]string{
		{"breakouts", "--min-markets", "0"},
		{"breakouts", "--min-gain", "0"},
		{"breakouts", "--chart", "top-weird"},
		{"breakouts", "--platform", "watch"},
		{"breakouts", "--genre", "nope"},
		{"breakouts", "--since", "-1d"},
		{"breakouts", "--limit", "-1"},
		{"breakouts", "extra", "--json"},
	} {
		if _, _, err := runS3B(t, db, args...); ExitCode(err) != 2 {
			t.Fatalf("%v exit = %d (%v), want 2", args, ExitCode(err), err)
		}
	}
	out, _, err = runS3B(t, db, "breakouts", "--dry-run", "--json")
	if err != nil || !strings.Contains(out, `"dry_run":true`) {
		t.Fatalf("dry-run = %q %v", out, err)
	}
}
