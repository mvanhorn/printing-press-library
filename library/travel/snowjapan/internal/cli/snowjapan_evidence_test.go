package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/snowjapan/internal/store"
	"github.com/spf13/cobra"
)

func TestCatalogFreshnessDoesNotUseLaterDetailSync(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facts.db")
	db, e := store.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	old := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	catalog, _ := json.Marshal(map[string]any{"id": "a", "name": "A", "projection": "catalog-v1", "observed_at": old})
	detail, _ := json.Marshal(map[string]any{"id": "a", "name": "A", "projection": "detail-v1", "observed_at": time.Now().UTC().Format(time.RFC3339)})
	ctx := context.Background()
	if e = db.CaptureSnowJapan(ctx, []json.RawMessage{catalog}, true); e != nil {
		t.Fatal(e)
	}
	if e = db.CaptureSnowJapan(ctx, []json.RawMessage{detail}, false); e != nil {
		t.Fatal(e)
	}
	if e = db.SaveSyncState("resorts", "", 1); e != nil {
		t.Fatal(e)
	}
	db.Close()
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().String("db", path, "")
	var hints bytes.Buffer
	cmd.SetErr(&hints)
	_, complete, e := snowLocal(ctx, cmd, &rootFlags{maxAge: 30 * time.Minute}, "resorts", "", true)
	if e != nil || !complete || !strings.Contains(hints.String(), "hint: complete local resort directory was captured at "+old) {
		t.Fatalf("complete=%v error=%v hint=%q", complete, e, hints.String())
	}
}

func TestCoveragePartitionsInconsistentSourceEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facts.db")
	db, e := store.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	at := time.Now().UTC().Format(time.RFC3339)
	catalog, _ := json.Marshal(map[string]any{"id": "a", "name": "A", "projection": "catalog-v1", "observed_at": at})
	season, _ := json.Marshal(map[string]any{"id": "season-a", "resort_id": "a", "season": "2024-2025", "endpoint_evidence_state": "dates_outside_requested_winter", "observed_at": at})
	if e = db.CaptureSnowJapan(ctx, []json.RawMessage{catalog}, true); e != nil {
		t.Fatal(e)
	}
	if e = db.CaptureSnowJapanSeason(ctx, "2024-2025", []json.RawMessage{season}); e != nil {
		t.Fatal(e)
	}
	db.Close()
	flags := &rootFlags{dataSource: "local", agent: true}
	root := &cobra.Command{Use: "test"}
	root.PersistentFlags().String("db", path, "")
	root.AddCommand(newSnowPlan(flags, "coverage"))
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"coverage", "--season", "2024-2025"})
	if e = root.Execute(); e != nil {
		t.Fatal(e)
	}
	var v struct {
		Meta    map[string]any   `json:"meta"`
		Results []map[string]any `json:"results"`
	}
	if e = json.Unmarshal(out.Bytes(), &v); e != nil {
		t.Fatal(e)
	}
	if v.Meta["inconsistent"] != float64(1) || v.Meta["confirmed"] != float64(0) || len(v.Results) != 1 || v.Results[0]["evidence_state"] != "inconsistent_source_evidence" {
		t.Fatalf("coverage=%s", out.String())
	}
}
