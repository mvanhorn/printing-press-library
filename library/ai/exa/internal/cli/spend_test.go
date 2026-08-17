// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavior tests for the cost journal + spend aggregation.

package cli

import (
	"bytes"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/ai/exa/internal/client"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCostJournalRecordsBilledResponses verifies recordCostJournal writes one
// JSONL line per response carrying costDollars.total and skips unbilled ones.
func TestCostJournalRecordsBilledResponses(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	journal := client.CostJournalPath()
	client.RecordCostJournal("POST", "/search", []byte(`{"requestId":"abc123","costDollars":{"total":0.007}}`))
	client.RecordCostJournal("POST", "/contents", []byte(`{"costDollars":{"total":0.003}}`))
	// No costDollars -> no line
	client.RecordCostJournal("GET", "/monitors", []byte(`{"results":[]}`))
	// Malformed JSON -> no line
	client.RecordCostJournal("POST", "/answer", []byte(`not json`))

	data, err := os.ReadFile(journal)
	if err != nil {
		t.Fatalf("reading journal: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 journal lines, got %d:\n%s", len(lines), data)
	}
	var e costEntry
	if err := json.Unmarshal([]byte(lines[0]), &e); err != nil {
		t.Fatalf("parsing journal line: %v", err)
	}
	if e.Path != "/search" || e.Cost != 0.007 || e.RequestID != "abc123" {
		t.Fatalf("unexpected entry: %+v", e)
	}
}

// TestCostJournalSkipsVerifyEnv verifies the verify environment never pollutes
// the journal.
func TestCostJournalSkipsVerifyEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")

	client.RecordCostJournal("POST", "/search", []byte(`{"costDollars":{"total":0.007}}`))
	if _, err := os.Stat(client.CostJournalPath()); !os.IsNotExist(err) {
		t.Fatalf("journal should not exist under verify env, got %v", err)
	}
}

// TestSpendAggregatesJournal drives the spend command against a seeded
// journal and checks the JSON envelope.
func TestSpendAggregatesJournal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	// Seed the journal with two entries, one older than the window.
	now := time.Now().UTC()
	entry := map[string]any{
		"ts":     now.Add(-48 * time.Hour).Format(time.RFC3339),
		"method": "POST", "path": "/search", "cost": 0.007,
	}
	b, _ := json.Marshal(entry)
	if err := os.MkdirAll(filepath.Dir(client.CostJournalPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(client.CostJournalPath(), append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	entry["ts"] = now.Format(time.RFC3339)
	entry["path"] = "/contents"
	entry["cost"] = 0.003
	b, _ = json.Marshal(entry)
	f, err := os.OpenFile(client.CostJournalPath(), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f.Write(append(b, '\n'))
	f.Close()

	cmd := RootCmd()
	cmd.SetArgs([]string{"spend", "--days", "30", "--json"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("spend error: %v (stderr: %s)", err, errb.String())
	}
	var view struct {
		Days    int `json:"days"`
		Entries []struct {
			Day      string  `json:"day"`
			Resource string  `json:"resource"`
			Cost     float64 `json:"cost"`
		} `json:"entries"`
		Total    float64 `json:"totalCost"`
		TotalUSD string  `json:"totalCostUsd"`
	}
	if err := json.Unmarshal(out.Bytes(), &view); err != nil {
		t.Fatalf("spend output not JSON: %v\n%s", err, out.String())
	}
	if view.Total != 0.01 {
		t.Fatalf("expected total 0.01, got %v", view.Total)
	}
	if view.TotalUSD != "$0.0100" {
		t.Fatalf("expected $0.0100, got %s", view.TotalUSD)
	}
	if len(view.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(view.Entries), view.Entries)
	}
	if view.Entries[0].Resource != "searches" || view.Entries[1].Resource != "contents" {
		t.Fatalf("unexpected resource mapping: %+v", view.Entries)
	}
}

// TestSpendResourceFilter verifies the --resource filter narrows the report.
func TestSpendResourceFilter(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	now := time.Now().UTC()
	entry := map[string]any{
		"ts": now.Format(time.RFC3339), "method": "POST", "path": "/search", "cost": 0.007,
	}
	b, _ := json.Marshal(entry)
	os.MkdirAll(filepath.Dir(client.CostJournalPath()), 0o700)
	os.WriteFile(client.CostJournalPath(), append(b, '\n'), 0o600)
	entry["path"] = "/answer"
	entry["cost"] = 0.004
	b, _ = json.Marshal(entry)
	f, _ := os.OpenFile(client.CostJournalPath(), os.O_APPEND|os.O_WRONLY, 0o600)
	f.Write(append(b, '\n'))
	f.Close()

	cmd := RootCmd()
	cmd.SetArgs([]string{"spend", "--resource", "answers", "--json"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("spend error: %v", err)
	}
	var view struct {
		Total float64 `json:"totalCost"`
	}
	json.Unmarshal(out.Bytes(), &view)
	if view.Total != 0.004 {
		t.Fatalf("expected filtered total 0.004, got %v", view.Total)
	}
}

// TestSpendEmptyJournal reports a clean empty envelope (exit 0) when no cost
// data exists, so agents and the scorecard probe get a graceful answer.
func TestSpendEmptyJournal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	cmd := RootCmd()
	cmd.SetArgs([]string{"spend", "--json"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("spend error: %v", err)
	}
	var view struct {
		Total float64 `json:"totalCost"`
	}
	if err := json.Unmarshal(out.Bytes(), &view); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out.String())
	}
	if view.Total != 0 {
		t.Fatalf("expected zero total, got %v", view.Total)
	}
	if !strings.Contains(errb.String(), "no cost journal found") {
		t.Fatalf("unexpected stderr: %s", errb.String())
	}
}

// TestResourceFromPath covers the path -> resource mapping used by spend.
func TestResourceFromPath(t *testing.T) {
	cases := map[string]string{
		"/search":                    "searches",
		"/contents":                  "contents",
		"/answer":                    "answers",
		"/findSimilar":               "find-similar",
		"/monitors":                  "monitors",
		"/monitors/{id}/runs":        "monitors",
		"/agent/runs":                "agents",
		"/v0/websets/{id}/items":     "websets",
		"/v0/webhooks/{id}/attempts": "webhooks",
		"/v0/imports":                "imports",
		"/v0/events":                 "events",
		"/v0/teams/me":               "v0",
	}
	for path, want := range cases {
		if got := resourceFromPath(path); got != want {
			t.Errorf("resourceFromPath(%q) = %q, want %q", path, got, want)
		}
	}
}

var _ = filepath.Join
