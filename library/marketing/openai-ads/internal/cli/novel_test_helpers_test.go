// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared test helpers for the Phase 3 novel command tests.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/openai-ads/internal/store"
)

// novelSeedOnce guards the XDG_DATA_HOME redirection across tests in this
// package: many tests run sequentially, and each t.Setenv restores its own
// value, but the seed itself is per-test (a fresh temp dir each time).
var (
	novelSeedMu sync.Mutex
)

// novelSeedStore creates a throwaway store under a temp XDG_DATA_HOME (so
// defaultDBPath resolves inside the temp dir) and seeds a rich set of
// campaigns, ad groups, ads and custom audiences. Items mirror the dependent
// sync injection keys (campaigns_id / ad-groups_id) so the campaign_id and
// ad_group_id columns populate exactly as a TASK 0 sync would.
func novelSeedStore(t *testing.T) *store.Store {
	t.Helper()
	novelSeedMu.Lock()
	defer novelSeedMu.Unlock()

	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	dbPath := defaultDBPath("openai-ads-pp-cli")
	_ = os.MkdirAll(filepath.Dir(dbPath), 0o755)
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open seeded store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	seed := []struct {
		resource string
		items    []json.RawMessage
	}{
		{"campaigns", []json.RawMessage{
			raw(`{"id":"cmpn_1","name":"Campaign One","status":"active","bidding_type":"AUCTION","budget":{"daily_spend_limit_micros":100000000},"targeting":{"locations":{"include":[{"id":"1000156","type":"COUNTRY"}]}}}`),
			raw(`{"id":"cmpn_2","name":"Campaign Two","status":"paused","bidding_type":"AUCTION","budget":{"daily_spend_limit_micros":50000000}}`),
		}},
		{"ad-groups", []json.RawMessage{
			raw(`{"id":"adgrp_1","name":"Ad Group One","status":"active","campaigns_id":"cmpn_1","bidding_config":{"billing_event_type":"IMPRESSION","strategy":"lowest_cost","max_bid_micros":20000000,"custom_audience_bid_multipliers":[]}}`),
			raw(`{"id":"adgrp_2","name":"Ad Group Two","status":"active","campaigns_id":"cmpn_1","bidding_config":{"billing_event_type":"IMPRESSION","strategy":"lowest_cost","max_bid_micros":1000000,"custom_audience_bid_multipliers":[]}}`),
			raw(`{"id":"adgrp_3","name":"Ad Group Three","status":"active","campaigns_id":"cmpn_1","bidding_config":{"billing_event_type":"IMPRESSION","strategy":"lowest_cost","max_bid_micros":5000000,"custom_audience_bid_multipliers":[{"custom_audience_id":"aud_2","multiplier":1.5}]}}`),
		}},
		{"ads", []json.RawMessage{
			raw(`{"id":"ad_1","name":"Ad One","status":"active","ad-groups_id":"adgrp_1","review_status":"approved","creative":{"type":"image","title":"T","body":"B","target_url":"https://example.com"}}`),
			raw(`{"id":"ad_2","name":"Ad Two","status":"active","ad-groups_id":"adgrp_2","review_status":"pending","creative":{"type":"image","title":"T2","body":"B2","target_url":"https://example.com/2"}}`),
		}},
		{"custom-audiences", []json.RawMessage{
			raw(`{"id":"aud_1","name":"Audience One"}`),
			raw(`{"id":"aud_2","name":"Audience Two"}`),
		}},
	}

	for _, group := range seed {
		if _, _, err := s.UpsertBatch(group.resource, group.items); err != nil {
			t.Fatalf("seed %s: %v", group.resource, err)
		}
	}

	// Seed entity snapshot history for drift / review-watch: insert an OLD
	// snapshot for campaign cmpn_1 and ad ad_1 with different values so the
	// two-most-recent diff sees a change. UpsertBatch already wrote current
	// snapshots with time.Now(); insert the historical row directly.
	old := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	for _, q := range []string{
		`INSERT INTO entity_snapshots (entity_type, entity_id, captured_at, data) VALUES ('campaigns','cmpn_1',?, '{"id":"cmpn_1","name":"Campaign One","status":"paused","budget":{"daily_spend_limit_micros":80000000}}')`,
		`INSERT INTO entity_snapshots (entity_type, entity_id, captured_at, data) VALUES ('ads','ad_1',?, '{"id":"ad_1","name":"Ad One","status":"active","review_status":"rejected","creative":{"type":"image","title":"T","target_url":"https://example.com"}}')`,
	} {
		if _, err := s.DB().Exec(q, old); err != nil {
			t.Fatalf("seed snapshot: %v", err)
		}
	}
	return s
}

// novelEmptyStore redirects the data dir to an empty temp dir so defaultDBPath
// points nowhere (DB absent) for the empty-store command tests.
func novelEmptyStore(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
}

func raw(s string) json.RawMessage { return json.RawMessage(s) }

// runNovelCmd executes a root command with the given args and returns stdout.
// Learn hooks are disabled so the deterministic local store is the only side
// effect under test.
func runNovelCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	out, _, err := runNovelCmdOutErr(t, args...)
	return out, err
}

// runNovelCmdOutErr runs a novel command and returns stdout and stderr
// separately, so tests can assert that stdout stays a clean machine-readable
// payload while human guidance (empty-state notes, warnings) lands on stderr.
func runNovelCmdOutErr(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	t.Setenv("OPENAI_ADS_NO_LEARN", "true")
	cmd := RootCmd()
	var out, errb bytes.Buffer
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()
	return out.String(), errb.String(), err
}
