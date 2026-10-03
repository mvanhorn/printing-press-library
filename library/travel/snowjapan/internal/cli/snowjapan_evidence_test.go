package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
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

func TestLocalGetRequiresCapturedDetailProjection(t *testing.T) {
	for _, tc := range []struct {
		resource, projection, id string
		valid                    bool
	}{
		{"resorts", "catalog-v1", "nagano-prefecture/hakuba-village/able-hakuba-goryu", false},
		{"resorts", "detail-v1", "nagano-prefecture/hakuba-village/able-hakuba-goryu", true},
		{"reports", "report-metadata-v1", "hakuba-now-1st-october-2026", false},
		{"reports", "report-observations-v1", "hakuba-now-1st-october-2026", true},
	} {
		t.Run(tc.projection, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "facts.db")
			db, err := store.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(map[string]any{"id": tc.id, "projection": tc.projection, "observed_at": time.Now().UTC().Format(time.RFC3339), "new_snow_cm": 0})
			if _, _, err := db.UpsertBatch(tc.resource, []json.RawMessage{raw}); err != nil {
				t.Fatal(err)
			}
			db.Close()
			root := &cobra.Command{Use: "test", SilenceErrors: true, SilenceUsage: true}
			root.PersistentFlags().String("db", path, "")
			root.AddCommand(newSnowGet(&rootFlags{dataSource: "local", agent: true}, tc.resource))
			var out, diagnostic bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&diagnostic)
			root.SetArgs([]string{"get", tc.id})
			err = root.Execute()
			if !tc.valid {
				if err == nil || !strings.Contains(err.Error(), "detail_not_captured") || !strings.Contains(err.Error(), "--"+tc.resource) {
					t.Fatalf("list projection accepted: output=%s error=%v", out.String(), err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result["projection"] != tc.projection || result["new_snow_cm"] != float64(0) {
				t.Fatalf("detail fields lost: %s", out.String())
			}
		})
	}
}

func TestReportCaptureSelectorsAreBoundedBeforeSourceReads(t *testing.T) {
	for _, args := range [][]string{
		{"--resources", "resorts", "--reports", "hakuba-now-1st-october-2026"},
		{"--resources", "reports", "--reports", "a,b,c,d,e"},
		{"--resources", "reports", "--reports", "a,a"},
		{"--resources", "reports", "--reports", ",a"},
	} {
		root := &cobra.Command{Use: "test", SilenceErrors: true, SilenceUsage: true}
		path := filepath.Join(t.TempDir(), "not-created.db")
		root.PersistentFlags().String("db", path, "")
		root.AddCommand(newSnowSync(&rootFlags{dataSource: "live"}))
		root.SetArgs(append([]string{"sync"}, args...))
		if err := root.Execute(); err == nil {
			t.Fatalf("unbounded/mismatched selector accepted: %v", args)
		}
	}
}

func TestReportFreshnessDoesNotUseLaterPartialCapture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facts.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	metadata, _ := json.Marshal(map[string]any{"id": "niseko-now-1st-october-2026", "projection": "report-metadata-v1", "observed_at": old})
	detail, _ := json.Marshal(map[string]any{"id": "hakuba-now-1st-october-2026", "projection": "report-observations-v1", "observed_at": time.Now().UTC().Format(time.RFC3339)})
	if _, _, err := db.UpsertBatch("reports", []json.RawMessage{metadata, detail}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSyncState("reports", "", 1); err != nil {
		t.Fatal(err)
	}
	db.Close()
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().String("db", path, "")
	var hints bytes.Buffer
	cmd.SetErr(&hints)
	rows, _, err := snowLocal(context.Background(), cmd, &rootFlags{maxAge: 30 * time.Minute}, "reports", "", false)
	if err != nil || len(rows) != 2 || !strings.Contains(hints.String(), "hint: local reports include observations from "+old) {
		t.Fatalf("fresh partial report capture hid older evidence: rows=%v error=%v hints=%q", rows, err, hints.String())
	}
}

func TestLocalDetailFreshnessUsesRequestedObservation(t *testing.T) {
	for _, resource := range []string{"resorts", "reports"} {
		for _, staleTarget := range []bool{false, true} {
			name := resource + "/fresh-target"
			if staleTarget {
				name = resource + "/stale-target"
			}
			t.Run(name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "facts.db")
				db, err := store.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				fresh := time.Now().UTC().Format(time.RFC3339)
				old := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
				targetAt, otherAt := fresh, old
				if staleTarget {
					targetAt, otherAt = old, fresh
				}
				id, other, projection := "nagano-prefecture/hakuba-village/able-hakuba-goryu", "hokkaido/niseko-town/niseko-annupuri", "detail-v1"
				if resource == "reports" {
					id, other, projection = "hakuba-now-1st-october-2026", "niseko-now-1st-october-2026", "report-observations-v1"
				}
				var raw []json.RawMessage
				for _, row := range []map[string]any{
					{"id": id, "projection": projection, "observed_at": targetAt, "new_snow_cm": 0},
					{"id": other, "projection": projection, "observed_at": otherAt, "new_snow_cm": 0},
				} {
					b, _ := json.Marshal(row)
					raw = append(raw, b)
				}
				if _, _, err := db.UpsertBatch(resource, raw); err != nil {
					t.Fatal(err)
				}
				if err := db.SaveSyncState(resource, "", 1); err != nil {
					t.Fatal(err)
				}
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
				root := &cobra.Command{Use: "test", SilenceErrors: true, SilenceUsage: true}
				root.PersistentFlags().String("db", path, "")
				root.AddCommand(newSnowGet(&rootFlags{dataSource: "local", agent: true, maxAge: 30 * time.Minute}, resource))
				var out, hints bytes.Buffer
				root.SetOut(&out)
				root.SetErr(&hints)
				root.SetArgs([]string{"get", id})
				if err := root.Execute(); err != nil {
					t.Fatal(err)
				}
				var result map[string]any
				if err := json.Unmarshal(out.Bytes(), &result); err != nil || result["id"] != id || result["observed_at"] != targetAt {
					t.Fatalf("wrong selected detail: output=%s error=%v", out.String(), err)
				}
				if staleTarget != strings.Contains(hints.String(), "hint: local "+resource+" include observations from "+old) {
					t.Fatalf("freshness used unrelated rows: staleTarget=%v hints=%q", staleTarget, hints.String())
				}
			})
		}
	}
}

func TestSnowJapanSourceWriterRejectsURIPathsBeforeMigration(t *testing.T) {
	for _, suffix := range []string{"?mode=memory", "#other", "%3Fother"} {
		t.Run(suffix, func(t *testing.T) {
			dir := t.TempDir()
			base := filepath.Join(dir, "facts.db")
			foreign := filepath.Join(dir, "foreign.db")
			for _, path := range []string{base, foreign} {
				db, err := store.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				raw, _ := json.Marshal(map[string]any{"id": "a", "projection": "detail-v1", "observed_at": "at", "peak_m": 3000})
				if _, _, err := db.UpsertBatch("resorts", []json.RawMessage{raw}); err != nil {
					t.Fatal(err)
				}
				db.Close()
			}
			contents, err := os.ReadFile(foreign)
			if err != nil {
				t.Fatal(err)
			}
			literal := base + suffix
			if err := os.WriteFile(literal, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{literal, filepath.Join(dir, "new.db") + suffix} {
				before := map[string][32]byte{}
				for _, existing := range []string{base, foreign, literal} {
					b, err := os.ReadFile(existing)
					if err != nil {
						t.Fatal(err)
					}
					before[existing] = sha256.Sum256(b)
				}
				root := &cobra.Command{Use: "test", SilenceErrors: true, SilenceUsage: true}
				root.PersistentFlags().String("db", path, "")
				root.AddCommand(newSnowSync(&rootFlags{dataSource: "live"}))
				root.SetArgs([]string{"sync", "--resources", "reports", "--reports", "hakuba-now-1st-october-2026"})
				var out bytes.Buffer
				root.SetOut(&out)
				root.SetErr(&out)
				if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "URI punctuation") {
					t.Fatalf("source writer reached migration/capture: %v output=%s", err, out.String())
				}
				if out.Len() != 0 {
					t.Fatalf("rejected capture emitted factual output: %s", out.String())
				}
				for existing, hash := range before {
					b, err := os.ReadFile(existing)
					if err != nil || sha256.Sum256(b) != hash {
						t.Fatalf("wrong cache changed: %s", existing)
					}
				}
				if _, err := os.Stat(filepath.Join(dir, "new.db")); !os.IsNotExist(err) {
					t.Fatal("URI truncation created the wrong cache")
				}
				if path != literal {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatal("rejected new literal cache was created")
					}
				}
			}
		})
	}
}
