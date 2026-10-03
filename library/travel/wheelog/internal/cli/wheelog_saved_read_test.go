// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/wheelog"
	"github.com/spf13/cobra"
)

func wheelogReadOnlyFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "saved.db")
	db, err := openWheelogStore(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	zero, two := 0, 2
	spot := wheelog.Spot{ID: 166345, Name: "Public restroom", Category: "toilet", Source: "live", DetailStatus: "checked", ObservedAt: time.Now().UTC().Format(time.RFC3339), Questions: []wheelog.Question{{ID: 102, Label: "Turning space", Positive: &two, Negative: &zero, State: "reported_affirmative"}}, Gaps: []string{}}
	if err := db.ObserveWheelog(context.Background(), spot); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0400); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
	return path
}

func runWheelogReadMode(t *testing.T, factory func(*rootFlags) *cobra.Command, mode string, args ...string) ([]byte, error) {
	t.Helper()
	f := &rootFlags{asJSON: true, agent: true, compact: true, noLearn: true, dataSource: mode, timeout: time.Second, configPath: filepath.Join(t.TempDir(), "missing.toml")}
	cmd := factory(f)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	err := cmd.Execute()
	if err == nil {
		var compact bytes.Buffer
		if compactErr := json.Compact(&compact, out.Bytes()); compactErr != nil {
			t.Fatal(compactErr)
		}
		return compact.Bytes(), nil
	}
	return out.Bytes(), err
}

func TestWheelogSavedReadsDoNotModifyReadOnlyDatabase(t *testing.T) {
	path := wheelogReadOnlyFixture(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		factory func(*rootFlags) *cobra.Command
		args    []string
	}{{newNovelShortlistListCmd, nil}, {newSpotsSearchCmd, []string{"Public"}}, {newSpotsInspectCmd, []string{"166345"}}, {newNovelSpotsCompareCmd, []string{"166345"}}, {newNovelShortlistChangesCmd, nil}} {
		raw, err := runWheelogReadMode(t, tc.factory, "local", append(tc.args, "--db", path, "--require-question", "102")...)
		if err != nil || !bytes.Contains(raw, []byte(`"positive_reports":2`)) {
			t.Fatalf("saved evidence unavailable: %s (%v)", raw, err)
		}
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0400 {
		t.Fatalf("saved read changed file permissions: %v (%v)", info, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("saved read changed database bytes: %v", err)
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Stat(path + suffix); !os.IsNotExist(err) {
			t.Fatalf("saved read created sidecar %s (%v)", suffix, err)
		}
	}
}

func TestWheelogAutoTriesLiveBeforeSavedDatabase(t *testing.T) {
	broken := filepath.Join(t.TempDir(), "not-a-database")
	if err := os.Mkdir(broken, 0700); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		spot := map[string]any{"id": 166345, "name": "Public restroom", "spotCategory": map[string]any{"category": "toilet", "questionList": []any{map[string]any{"id": 102, "question": "Turning space", "totalGood": 2, "totalBad": 0}}}}
		content := map[string]any{"spot": spot}
		if strings.Contains(r.URL.Path, "Timeline") {
			content = map[string]any{"timelineList": []any{map[string]any{"type": "spot", "timeline": spot}}, "request": map[string]any{"word": r.Form.Get("word"), "type": "spot", "categoryList": r.Form["categoryList[]"]}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": []any{map[string]any{"resultCode": 0}}, "content": content})
	}))
	defer server.Close()
	t.Setenv("WHEELOG_BASE_URL", server.URL)
	for _, tc := range []struct {
		factory func(*rootFlags) *cobra.Command
		args    []string
	}{{newSpotsInspectCmd, []string{"166345"}}, {newNovelSpotsCompareCmd, []string{"166345"}}, {newSpotsSearchCmd, []string{"Public", "--limit", "1"}}} {
		raw, err := runWheelogReadMode(t, tc.factory, "auto", append(tc.args, "--db", broken, "--require-question", "102")...)
		if err != nil || !bytes.Contains(raw, []byte(`"positive_reports":2`)) || !bytes.Contains(raw, []byte(`"data_source":"live"`)) {
			t.Fatalf("auto source blocked by cache: %s (%v)", raw, err)
		}
	}
}

func TestWheelogAutoReportsSavedFallbackFailure(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(http.StatusBadRequest) }))
	defer server.Close()
	t.Setenv("WHEELOG_BASE_URL", server.URL)
	broken := filepath.Join(t.TempDir(), "not-a-database")
	if err := os.Mkdir(broken, 0700); err != nil {
		t.Fatal(err)
	}
	_, err := runWheelogReadMode(t, newSpotsInspectCmd, "auto", "166345", "--db", broken)
	if calls == 0 || err == nil || !strings.Contains(err.Error(), "saved fallback unavailable") {
		t.Fatalf("cache failure hidden or live not tried: calls=%d err=%v", calls, err)
	}
	path := wheelogReadOnlyFixture(t)
	raw, err := runWheelogReadMode(t, newSpotsInspectCmd, "auto", "166345", "--db", path, "--require-question", "102")
	if err != nil || !bytes.Contains(raw, []byte(`"positive_reports":2`)) || !bytes.Contains(raw, []byte(`"data_source":"local"`)) {
		t.Fatalf("read-only fallback lost evidence: %s (%v)", raw, err)
	}
}

func TestWheelogSavedCommandsRejectActiveWALThenShowNewestCounts(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "saved.db")
	db, err := openWheelogStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	zero, two := 0, 2
	spot := wheelog.Spot{ID: 166345, Name: "Public restroom", Category: "toilet", DetailStatus: "checked", ObservedAt: time.Now().UTC().Format(time.RFC3339), Questions: []wheelog.Question{{ID: 102, Label: "Turning space", Positive: &two, Negative: &zero, State: "reported_affirmative"}}, Gaps: []string{}}
	if err = db.ObserveWheelog(ctx, spot); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = openWheelogStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	spot.Questions[0].Positive = &zero
	spot.Questions[0].State = "unreported"
	if err = db.ObserveWheelog(ctx, spot); err != nil {
		t.Fatal(err)
	}
	commands := []struct {
		factory func(*rootFlags) *cobra.Command
		args    []string
	}{{newNovelShortlistListCmd, nil}, {newSpotsSearchCmd, []string{"Public"}}, {newSpotsInspectCmd, []string{"166345"}}, {newNovelSpotsCompareCmd, []string{"166345"}}, {newNovelShortlistChangesCmd, nil}}
	for _, tc := range commands {
		_, err := runWheelogReadMode(t, tc.factory, "local", append(tc.args, "--db", path, "--require-question", "102")...)
		if err == nil || !strings.Contains(err.Error(), "cache_visibility_unavailable") {
			t.Fatalf("committed WAL silently returned old saved counts: %v", err)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadRequest) }))
	defer server.Close()
	t.Setenv("WHEELOG_BASE_URL", server.URL)
	for _, tc := range commands[1:4] {
		_, err := runWheelogReadMode(t, tc.factory, "auto", append(tc.args, "--db", path)...)
		if err == nil || !strings.Contains(err.Error(), "saved fallback unavailable") || !strings.Contains(err.Error(), "cache_visibility_unavailable") {
			t.Fatalf("auto fallback hid active WAL failure: %v", err)
		}
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range commands {
		raw, err := runWheelogReadMode(t, tc.factory, "local", append(tc.args, "--db", path, "--require-question", "102")...)
		if err != nil || !bytes.Contains(raw, []byte(`"positive_reports":0`)) {
			t.Fatalf("newest zero-count evidence unavailable after checkpoint: %s (%v)", raw, err)
		}
	}
}

func TestWheelogSourceSaveAndRefreshRejectHardLinkBeforeCacheWrite(t *testing.T) {
	path := wheelogReadOnlyFixture(t)
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias.db")
	if err := os.Link(path, alias); err != nil {
		t.Skipf("hard-link unavailable: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"result": []any{map[string]any{"resultCode": 0}}, "content": map[string]any{"spot": map[string]any{"id": 166345, "name": "Public restroom", "spotCategory": map[string]any{"category": "toilet", "questionList": []any{map[string]any{"id": 102, "question": "Turning space", "totalGood": 4, "totalBad": 0}}}}}})
	}))
	defer server.Close()
	t.Setenv("WHEELOG_BASE_URL", server.URL)
	for _, tc := range []struct {
		factory func(*rootFlags) *cobra.Command
		args    []string
	}{{newWheelogSaveCmd, []string{"166345"}}, {newNovelShortlistChangesCmd, []string{"--limit", "1"}}} {
		_, err := runWheelogReadMode(t, tc.factory, "live", append(tc.args, "--db", alias)...)
		if err == nil || !strings.Contains(err.Error(), "cache_visibility_unavailable") {
			t.Fatalf("source write opened hard-link cache: %v", err)
		}
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Stat(alias + suffix); !os.IsNotExist(err) {
			t.Fatalf("source write created alternate sidecar: %s (%v)", suffix, err)
		}
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	raw, err := runWheelogReadMode(t, newSpotsInspectCmd, "local", "166345", "--db", path, "--require-question", "102")
	if err != nil || !bytes.Contains(raw, []byte(`"positive_reports":2`)) {
		t.Fatalf("rejected source write changed baseline: %s (%v)", raw, err)
	}
}
