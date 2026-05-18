// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/slickdeals/internal/store"

	"github.com/spf13/cobra"
)

func analyticsExtTestStore(t *testing.T) (string, *store.Store) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "snap.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	now := time.Now().UTC()
	seeds := []store.DealSnapshot{
		{DealID: "a", CapturedAt: now.Add(-1 * time.Hour), Merchant: "costco", Thumbs: 50, Title: "t", Link: "l"},
		{DealID: "b", CapturedAt: now.Add(-2 * time.Hour), Merchant: "costco", Thumbs: 75, Title: "t", Link: "l"},
		{DealID: "x", CapturedAt: now.Add(-3 * time.Hour), Merchant: "amazon", Thumbs: 200, Title: "t", Link: "l"},
		// thumbs-velocity series for deal_id=v
		{DealID: "v", CapturedAt: now.Add(-3 * time.Hour), Merchant: "costco", Thumbs: 10, Title: "v", Link: "l"},
		{DealID: "v", CapturedAt: now.Add(-2 * time.Hour), Merchant: "costco", Thumbs: 30, Title: "v", Link: "l"},
		{DealID: "v", CapturedAt: now.Add(-1 * time.Hour), Merchant: "costco", Thumbs: 25, Title: "v", Link: "l"},
	}
	for i := range seeds {
		if err := s.InsertSnapshot(&seeds[i]); err != nil {
			t.Fatalf("insert seed %d: %v", i, err)
		}
	}
	return dbPath, s
}

func runCmd(t *testing.T, cmd *cobra.Command, args ...string) []byte {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr) // separate so JSON envelope on stdout isn't polluted by empty-result hints
	cmd.SetContext(context.Background())
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute %s: %v\nstdout=%s\nstderr=%s", cmd.Use, err, stdout.String(), stderr.String())
	}
	return stdout.Bytes()
}

func decodeEnvelope(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, string(raw))
	}
	return env
}

func TestTopStoresCmd_RanksMerchants(t *testing.T) {
	dbPath, s := analyticsExtTestStore(t)
	defer s.Close()

	flags := &rootFlags{asJSON: true}
	raw := runCmd(t, newTopStoresCmd(flags), "--db", dbPath, "--window", "24h")
	env := decodeEnvelope(t, raw)

	results, ok := env["results"].([]any)
	if !ok {
		t.Fatalf("results not slice: %+v", env)
	}
	// costco appears in 3 distinct deals (a, b, v); amazon in 1 (x).
	if len(results) != 2 {
		t.Fatalf("len=%d want 2", len(results))
	}
	first := results[0].(map[string]any)
	if first["merchant"] != "costco" {
		t.Fatalf("expected costco first (more deals), got %v", first["merchant"])
	}
	if int(first["deal_count"].(float64)) != 3 {
		t.Fatalf("costco deal_count=%v want 3", first["deal_count"])
	}
}

func TestTopStoresCmd_LimitRespected(t *testing.T) {
	dbPath, s := analyticsExtTestStore(t)
	defer s.Close()

	flags := &rootFlags{asJSON: true}
	raw := runCmd(t, newTopStoresCmd(flags), "--db", dbPath, "--window", "24h", "--limit", "1")
	env := decodeEnvelope(t, raw)
	results := env["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("limit len=%d want 1", len(results))
	}
}

func TestTopStoresCmd_WindowZeroAllTime(t *testing.T) {
	dbPath, s := analyticsExtTestStore(t)
	defer s.Close()

	// Insert a stale row outside any normal window.
	stale := store.DealSnapshot{
		DealID:     "old",
		CapturedAt: time.Now().Add(-1000 * 24 * time.Hour),
		Merchant:   "ancient",
		Thumbs:     1,
		Title:      "t",
		Link:       "l",
	}
	if err := s.InsertSnapshot(&stale); err != nil {
		t.Fatalf("insert stale: %v", err)
	}

	flags := &rootFlags{asJSON: true}
	raw := runCmd(t, newTopStoresCmd(flags), "--db", dbPath, "--window", "0")
	env := decodeEnvelope(t, raw)
	results := env["results"].([]any)
	// costco, amazon, ancient = 3.
	if len(results) != 3 {
		t.Fatalf("len=%d want 3 (window=0 all-time should include ancient)", len(results))
	}
}

func TestThumbsVelocityCmd_DeltasCorrect(t *testing.T) {
	dbPath, s := analyticsExtTestStore(t)
	defer s.Close()

	flags := &rootFlags{asJSON: true}
	raw := runCmd(t, newThumbsVelocityCmd(flags), "--db", dbPath, "v")
	env := decodeEnvelope(t, raw)
	results := env["results"].([]any)
	if len(results) != 3 {
		t.Fatalf("len=%d want 3", len(results))
	}
	p0 := results[0].(map[string]any)
	p1 := results[1].(map[string]any)
	p2 := results[2].(map[string]any)
	if int(p0["delta"].(float64)) != 0 {
		t.Fatalf("first delta=%v want 0", p0["delta"])
	}
	if int(p1["thumbs"].(float64)) != 30 || int(p1["delta"].(float64)) != 20 {
		t.Fatalf("second point: thumbs=%v delta=%v want 30/20", p1["thumbs"], p1["delta"])
	}
	if int(p2["thumbs"].(float64)) != 25 || int(p2["delta"].(float64)) != -5 {
		t.Fatalf("third point: thumbs=%v delta=%v want 25/-5", p2["thumbs"], p2["delta"])
	}
}

func TestThumbsVelocityCmd_UnknownDealNotError(t *testing.T) {
	dbPath, s := analyticsExtTestStore(t)
	defer s.Close()

	flags := &rootFlags{asJSON: true}
	raw := runCmd(t, newThumbsVelocityCmd(flags), "--db", dbPath, "ghost")
	env := decodeEnvelope(t, raw)
	results, ok := env["results"].([]any)
	if !ok {
		// Empty arrays may round-trip as nil; that's acceptable.
		if env["results"] != nil {
			t.Fatalf("results not slice or nil: %+v", env)
		}
		return
	}
	if len(results) != 0 {
		t.Fatalf("len=%d want 0", len(results))
	}
}

func TestThumbsVelocityCmd_DryRunShortCircuits(t *testing.T) {
	flags := &rootFlags{asJSON: true, dryRun: true}
	cmd := newThumbsVelocityCmd(flags)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"anything"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run should succeed: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("dry-run should produce no output, got %q", buf.String())
	}
}

func TestParseWindowDuration(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		bad  bool
	}{
		{"7d", 7 * 24 * time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"0", 0, false},
		{"0d", 0, false},
		{"5", 5 * 24 * time.Hour, false}, // bare integer => days
		{"", 0, true},
		{"abc", 0, true},
		{"7x", 0, true},
	}
	for _, c := range cases {
		got, err := parseWindowDuration(c.in)
		if (err != nil) != c.bad {
			t.Fatalf("parseWindowDuration(%q) err=%v want bad=%v", c.in, err, c.bad)
		}
		if !c.bad && got != c.want {
			t.Fatalf("parseWindowDuration(%q) = %v want %v", c.in, got, c.want)
		}
	}
}

func TestAttachAnalyticsExt_AddsSubcommands(t *testing.T) {
	// Build a minimal parent and confirm attachAnalyticsExt wires both kids.
	parent := &cobra.Command{Use: "analytics"}
	flags := &rootFlags{}
	attachAnalyticsExt(parent, flags)

	found := map[string]bool{}
	for _, c := range parent.Commands() {
		found[c.Name()] = true
	}
	if !found["top-stores"] {
		t.Fatalf("top-stores not attached: %+v", parent.Commands())
	}
	if !found["thumbs-velocity"] {
		t.Fatalf("thumbs-velocity not attached: %+v", parent.Commands())
	}
}
