// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/cartesia/internal/store"
)

// seedPhase3DB builds a minimally populated SQLite store with one call,
// one metric_result, two voice snapshots, and one voice row. Returns the
// db path.
func seedPhase3DB(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	if err := s.UpsertCall("c1", []byte(`{
		"id":"c1","agent_id":"a1","deployment_id":"dep_new","start_time":"2026-05-13T10:00:00Z",
		"end_time":"2026-05-13T10:05:00Z","status":"completed","summary":"customer asked to cancel order",
		"transcript":[
			{"role":"user","text":"please cancel my order"},
			{"role":"agent","text":"sure, processing the cancellation now"}
		]
	}`)); err != nil {
		t.Fatalf("UpsertCall: %v", err)
	}
	if err := s.UpsertCall("c2", []byte(`{
		"id":"c2","agent_id":"a1","deployment_id":"dep_old","start_time":"2026-05-10T10:00:00Z",
		"transcript":"older call"
	}`)); err != nil {
		t.Fatalf("UpsertCall c2: %v", err)
	}
	if err := s.UpsertMetricResult("mr1", []byte(`{
		"id":"mr1","metric_id":"metric_x","call_id":"c1","agent_id":"a1","deployment_id":"dep_new",
		"score":0.2,"passed":false,"evaluated_at":"2026-05-13T10:06:00Z"
	}`)); err != nil {
		t.Fatalf("UpsertMetricResult: %v", err)
	}
	if err := s.UpsertMetricResult("mr2", []byte(`{
		"id":"mr2","metric_id":"metric_x","call_id":"c2","agent_id":"a1","deployment_id":"dep_old",
		"score":0.9,"passed":true,"evaluated_at":"2026-05-10T10:06:00Z"
	}`)); err != nil {
		t.Fatalf("UpsertMetricResult mr2: %v", err)
	}

	if err := s.SnapshotVoice("v1", []byte(`{"id":"v1","name":"warm","language":"en"}`)); err != nil {
		t.Fatalf("SnapshotVoice: %v", err)
	}

	// Insert a voice row directly so voices find has data.
	if _, err := s.DB().Exec(`INSERT INTO "voices" (id, data, name, description, language, gender) VALUES (?, ?, ?, ?, ?, ?)`,
		"v1", `{"id":"v1","name":"warm","description":"warm and friendly"}`, "warm", "warm and friendly", "en", "f"); err != nil {
		t.Fatalf("seed voice: %v", err)
	}

	// Insert a deployment row directly so agents diff can read them.
	if _, err := s.DB().Exec(`INSERT INTO "deployments" (id, agents_id, data) VALUES (?, ?, ?)`,
		"dep_new", "a1", `{"id":"dep_new","prompt":"new","voice":"v2"}`); err != nil {
		t.Fatalf("seed dep_new: %v", err)
	}
	if _, err := s.DB().Exec(`INSERT INTO "deployments" (id, agents_id, data) VALUES (?, ?, ?)`,
		"dep_old", "a1", `{"id":"dep_old","prompt":"old","voice":"v1"}`); err != nil {
		t.Fatalf("seed dep_old: %v", err)
	}

	return dbPath
}

func TestCallsGrep_FindsHit(t *testing.T) {
	dbPath := seedPhase3DB(t)
	flags := &rootFlags{asJSON: true}
	cmd := newCallsGrepCmd(flags)
	cmd.Flags().Set("db", dbPath)
	cmd.SetContext(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, []string{"cancel"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"c1"`) {
		t.Fatalf("expected c1 in output, got: %s", out)
	}
}

func TestCallsWorst_OrdersByScore(t *testing.T) {
	dbPath := seedPhase3DB(t)
	flags := &rootFlags{asJSON: true}
	cmd := newCallsWorstCmd(flags)
	cmd.Flags().Set("db", dbPath)
	cmd.SetContext(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	out := buf.String()
	// c1 has score 0.2 which is the worst; should appear before c2 in output.
	c1 := strings.Index(out, `"c1"`)
	c2 := strings.Index(out, `"c2"`)
	if c1 == -1 {
		t.Fatalf("expected c1 in output, got: %s", out)
	}
	if c2 != -1 && c1 > c2 {
		t.Fatalf("expected c1 before c2 (worst first), got: %s", out)
	}
}

func TestAgentsAudit_Succeeds(t *testing.T) {
	dbPath := seedPhase3DB(t)
	flags := &rootFlags{asJSON: true}
	cmd := newAgentsAuditCmd(flags)
	cmd.Flags().Set("db", dbPath)
	cmd.SetContext(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, []string{"a1"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"agent_id"`) {
		t.Fatalf("expected agent_id in output, got: %s", out)
	}
}

func TestAgentsDiff_DetectsChange(t *testing.T) {
	dbPath := seedPhase3DB(t)
	flags := &rootFlags{asJSON: true}
	cmd := newAgentsDiffCmd(flags)
	cmd.Flags().Set("db", dbPath)
	cmd.Flags().Set("from", "dep_old")
	cmd.Flags().Set("to", "dep_new")
	cmd.SetContext(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, []string{"a1"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"changed"`) {
		t.Fatalf("expected 'changed' key in output, got: %s", out)
	}
}

func TestAgentsSince_ReturnsEntries(t *testing.T) {
	dbPath := seedPhase3DB(t)
	flags := &rootFlags{asJSON: true}
	cmd := newAgentsSinceCmd(flags)
	cmd.Flags().Set("db", dbPath)
	cmd.SetContext(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, []string{"a1", "30d"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"entries"`) {
		t.Fatalf("expected entries in output, got: %s", out)
	}
}

func TestMetricsTrend_AggregatesByDay(t *testing.T) {
	dbPath := seedPhase3DB(t)
	flags := &rootFlags{asJSON: true}
	cmd := newMetricsTrendCmd(flags)
	cmd.Flags().Set("db", dbPath)
	cmd.SetContext(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, []string{"metric_x"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"buckets"`) {
		t.Fatalf("expected buckets in output, got: %s", out)
	}
}

func TestVoicesChangelog_HandlesSingleSnapshot(t *testing.T) {
	dbPath := seedPhase3DB(t)
	flags := &rootFlags{asJSON: true}
	cmd := newVoicesChangelogCmd(flags)
	cmd.Flags().Set("db", dbPath)
	cmd.SetContext(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"entries"`) {
		t.Fatalf("expected entries in output, got: %s", out)
	}
}

func TestVoicesFind_MatchesName(t *testing.T) {
	dbPath := seedPhase3DB(t)
	flags := &rootFlags{asJSON: true}
	cmd := newVoicesFindCmd(flags)
	cmd.Flags().Set("db", dbPath)
	cmd.SetContext(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, []string{"warm"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"v1"`) && !strings.Contains(out, `"id"`) {
		t.Fatalf("expected hit in output, got: %s", out)
	}
}

func TestUsageEstimate_FineTune(t *testing.T) {
	dbPath := seedPhase3DB(t)
	flags := &rootFlags{asJSON: true}
	cmd := newUsageEstimateCmd(flags)
	cmd.Flags().Set("db", dbPath)
	cmd.Flags().Set("dataset", "ds_test")
	cmd.SetContext(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, []string{"fine-tune"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"estimated_credits"`) {
		t.Fatalf("expected estimated_credits in output, got: %s", out)
	}
}

func TestParseSince_HandlesFormats(t *testing.T) {
	cases := []string{"24h", "7d", "30m", "1w", "2026-05-13T00:00:00Z", "2026-05-13"}
	for _, c := range cases {
		if _, err := parseSince(c); err != nil {
			t.Errorf("parseSince(%q): %v", c, err)
		}
	}
	if t0, err := parseSince(""); err != nil || !t0.IsZero() {
		t.Errorf("parseSince(empty) = (%v, %v), want (zero, nil)", t0, err)
	}
}
