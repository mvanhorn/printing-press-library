// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

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
