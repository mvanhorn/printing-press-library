// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/store"
)

func TestCarstayMCPStateContractsUseActualCLI(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "carstay-contract-cli")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", bin, "./cmd/carstay-pp-cli")
	build.Dir = filepath.Join("..", "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v %s", err, output)
	}
	run := func(home string, readOnly bool, args ...string) []byte {
		t.Helper()
		cmd := exec.Command(bin, append([]string{"--home", home, "--agent"}, args...)...)
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			if !strings.EqualFold(name, "CARSTAY_MCP_READ_ONLY") && !strings.EqualFold(name, "CARSTAY_NO_LEARN") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		if readOnly {
			cmd.Env = append(cmd.Env, "CARSTAY_MCP_READ_ONLY=true")
		}
		stdout, err := cmd.Output()
		if err != nil {
			t.Fatalf("CLI %q: %v", args, err)
		}
		return stdout
	}
	t.Run("native-read-context-is-file-free", func(t *testing.T) {
		for _, args := range [][]string{{"spots", "coverage", "--dry-run"}, {"directory", "--dry-run"}, {"spots", "compare", "first", "second", "--dry-run"}} {
			home := t.TempDir()
			run(home, true, args...)
			count := 0
			if err := filepath.WalkDir(home, func(_ string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !entry.IsDir() {
					count++
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("readonly native created %d files for %q", count, args)
			}
		}
		// Normal CLI optional learning stays enabled; this is not --no-learn.
		home := t.TempDir()
		run(home, false, "spots", "coverage", "--dry-run")
		count := 0
		filepath.WalkDir(home, func(_ string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				count++
			}
			return err
		})
		if count == 0 {
			t.Fatal("normal CLI journaling was disabled")
		}
	})
	t.Run("learning-read-sees-active-committed-wal", func(t *testing.T) {
		home := t.TempDir()
		dbPath := filepath.Join(home, "active.db")
		s, err := store.OpenWithContext(context.Background(), dbPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		// The profile normally uses TRUNCATE. Seed an actual external WAL
		// writer without asking the ordinary profile to reset its journal mode.
		writer, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(1000)")
		if err != nil {
			t.Fatal(err)
		}
		defer writer.Close()
		writer.SetMaxOpenConns(1)
		held, err := writer.Conn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer held.Close()
		if _, err := held.ExecContext(context.Background(), "PRAGMA journal_mode=WAL"); err != nil {
			t.Fatal(err)
		}
		const id = "000000000000000000000abc"
		if _, err := held.ExecContext(context.Background(), `INSERT INTO resources(resource_type,id,data) VALUES('directory',?,'{}')`, id); err != nil {
			t.Fatal(err)
		}
		if _, err := held.ExecContext(context.Background(), `INSERT INTO search_learnings(query_pattern,query_entities,resource_type,resource_id,action,source,confidence,last_observed_at) VALUES('synthetic station','[]','directory',?,'boost','taught',2,CURRENT_TIMESTAMP)`, id); err != nil {
			t.Fatal(err)
		}
		if info, err := os.Stat(dbPath + "-wal"); err != nil || info.Size() == 0 {
			t.Fatalf("active WAL fixture missing: %v", err)
		}
		reader := exec.Command(bin, "--home", home, "--agent", "recall", "synthetic station", "--db", dbPath)
		stdout, readErr := reader.CombinedOutput()
		if readErr != nil {
			// An active WAL mode can prevent this TRUNCATE profile's RW open.
			// Explicit unavailability is valid; stale successful recall is not.
			if !strings.Contains(strings.ToLower(string(stdout)), "locked") && !strings.Contains(strings.ToLower(string(stdout)), "busy") {
				t.Fatalf("unexpected active-writer failure: %v %s", readErr, stdout)
			}
			if err := held.Close(); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			stdout = run(home, false, "recall", "synthetic station", "--db", dbPath)
		}
		var envelope struct {
			Results struct {
				Found   bool `json:"found"`
				Results []struct {
					ID string `json:"resource_id"`
				} `json:"results"`
			} `json:"results"`
		}
		if err := json.Unmarshal(stdout, &envelope); err != nil {
			t.Fatal(err)
		}
		matched := false
		for _, hit := range envelope.Results.Results {
			if hit.ID == id {
				matched = true
			}
		}
		if !envelope.Results.Found || !matched {
			t.Fatalf("current committed WAL rule was missed: %s", stdout)
		}
	})
}

func carstayStateSnapshot(t *testing.T, home string) map[string][32]byte {
	t.Helper()
	out := map[string][32]byte{}
	if err := filepath.WalkDir(home, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(home, path)
		if err != nil {
			return err
		}
		out[rel] = sha256.Sum256(data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func carstaySeedStaleState(t *testing.T, home string) {
	t.Helper()
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	s, err := store.OpenWithContext(context.Background(), defaultDBPath("carstay-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`INSERT INTO sync_state(resource_type,last_synced_at,total_count) VALUES('directory','2000-01-01T00:00:00Z',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data,synced_at,updated_at) VALUES('directory','stale-fixture','{}','2000-01-01T00:00:00Z','2000-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCarstayMCPNonDryNativeReadsPreserveStaleState(t *testing.T) {
	var requests atomic.Int64
	const fixture = `[{"_id":"632c59b82b614b99a252d1b2","name":"架空の確認用スポット","prefecture":"三重県","activityOnly":false,"approvedEn":true,"price":2200}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/ja/api/data/no-route/stations" {
			http.NotFound(w, r)
			return
		}
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, fixture)
	}))
	defer srv.Close()
	// Remap only the observed public base URL in a disposable test build;
	// shipping code keeps its fixed public source and gains no fixture flag.
	original, err := filepath.Abs(filepath.Join("..", "carstay", "client.go"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), `base = "https://carstay.jp"`) != 1 {
		t.Fatal("public source seam changed")
	}
	buildDir := t.TempDir()
	replacement := filepath.Join(buildDir, "client.go")
	modified := strings.Replace(string(data), `base = "https://carstay.jp"`, "base = "+fmt.Sprintf("%q", srv.URL), 1)
	if err := os.WriteFile(replacement, []byte(modified), 0600); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(buildDir, "overlay.json")
	mapping, _ := json.Marshal(map[string]any{"Replace": map[string]string{original: replacement}})
	if err := os.WriteFile(overlay, mapping, 0600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(buildDir, "carstay-live-contract-cli")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-overlay", overlay, "-trimpath", "-buildvcs=false", "-o", bin, "./cmd/carstay-pp-cli")
	build.Dir = filepath.Join("..", "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("fixture CLI build: %v %s", err, output)
	}
	for _, args := range [][]string{{"spots", "coverage"}, {"directory", "--limit", "1"}} {
		home := t.TempDir()
		carstaySeedStaleState(t, home)
		before := carstayStateSnapshot(t, home)
		cmd := exec.Command(bin, append([]string{"--home", home, "--agent"}, args...)...)
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			if !strings.EqualFold(name, "CARSTAY_MCP_READ_ONLY") && !strings.EqualFold(name, "CARSTAY_NO_LEARN") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		cmd.Env = append(cmd.Env, "CARSTAY_MCP_READ_ONLY=true")
		stdout, err := cmd.Output()
		if err != nil {
			t.Fatalf("non-dry CLI %q: %v", args, err)
		}
		var result struct {
			Meta struct {
				Source string `json:"source"`
			} `json:"meta"`
			Results json.RawMessage `json:"results"`
		}
		if err := json.Unmarshal(stdout, &result); err != nil || result.Meta.Source != "live" || len(result.Results) == 0 {
			t.Fatalf("real fixture response missing: %s %v", stdout, err)
		}
		if !strings.Contains(string(result.Results), "japanese_overnight") && !strings.Contains(string(result.Results), "632c59b82b614b99a252d1b2") {
			t.Fatalf("fixture rows not observed: %s", stdout)
		}
		if after := carstayStateSnapshot(t, home); !reflect.DeepEqual(before, after) {
			t.Fatalf("non-dry native mutated stale store/journal/cache: %q", args)
		}
	}
	if requests.Load() != 2 {
		t.Fatalf("non-dry fixture requests=%d, want2", requests.Load())
	}
}

func TestCarstayMCPContextSuppressesNonDryReadCachingAndRefresh(t *testing.T) {
	home := t.TempDir()
	carstaySeedStaleState(t, home)
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	t.Setenv("CARSTAY_MCP_READ_ONLY", "true")
	t.Setenv("CARSTAY_NO_LEARN", "")
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"id":"632c59b82b614b99a252d1b2","name":"fixture"}]`)
	}))
	defer srv.Close()
	configPath := filepath.Join(home, "fixture-config.json")
	configuration, _ := json.Marshal(map[string]string{"base_url": srv.URL})
	if err := os.WriteFile(configPath, configuration, 0600); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{configPath: configPath, dataSource: "auto", timeout: 2 * time.Second, rateLimit: 2}
	c, err := flags.newClient()
	if err != nil {
		t.Fatal(err)
	}
	before := carstayStateSnapshot(t, home)
	for i := 0; i < 2; i++ {
		data, provenance, err := resolveRead(context.Background(), c, flags, "directory", true, "/fixture", nil, nil, io.Discard)
		if err != nil || provenance.Source != "live" || !strings.Contains(string(data), "632c59b82b614b99a252d1b2") {
			t.Fatalf("live resolver: %s %+v %v", data, provenance, err)
		}
	}
	meta := autoRefreshIfStale(context.Background(), flags, []string{"directory"})
	if meta.Ran || meta.Reason != "mcp_read_only" {
		t.Fatalf("readonly context refreshed stale state: %+v", meta)
	}
	if requests.Load() != 2 {
		t.Fatalf("HTTP response was served from implicit cache: %d requests", requests.Load())
	}
	if after := carstayStateSnapshot(t, home); !reflect.DeepEqual(before, after) {
		t.Fatal("non-dry response cache/write-through/refresh mutated local state")
	}
}
