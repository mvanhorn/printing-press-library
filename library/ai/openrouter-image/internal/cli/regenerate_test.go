// Copyright 2026 neal-kyle and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/openrouter-image/internal/store"
)

// TestNovelRegenerateHelpWires smoke-tests that the regenerate command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelRegenerateHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"regenerate", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("regenerate --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "regenerate"} {
		if !strings.Contains(help, want) {
			t.Fatalf("regenerate --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestNovelRegenerateLedgerFailureFailsRun is the regression test for the
// review finding "ledger write failures are hidden": regenerate repeats the
// same unchecked ledger write as generate, so it must also fail (non-zero
// exit) when the billed re-run cannot be recorded.
func TestNovelRegenerateLedgerFailureFailsRun(t *testing.T) {
	var hits atomic.Int32
	srv := fakeImageAPI(t, &hits)
	dir := cliTestEnv(t, srv)
	dbPath := filepath.Join(dir, "store.db")

	// Seed a healthy ledger with one generation, then break the write path.
	ctx := context.Background()
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := db.EnsureOpenRouterImageTables(ctx); err != nil {
		t.Fatalf("ensure tables: %v", err)
	}
	seedID := newLedgerID("openai/gpt-image-1")
	if err := db.LedgerGeneration(ctx, store.GenerationEntry{
		ID:     seedID,
		Model:  "openai/gpt-image-1",
		Prompt: "a cat",
		Params: `{"model":"openai/gpt-image-1","prompt":"a cat"}`,
	}); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}
	db.Close()
	breakLedgerWrite(t, dbPath)

	cmd := RootCmd()
	cmd.SetArgs([]string{"regenerate", seedID, "--db", dbPath, "--home", dir, "--agent"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err = cmd.Execute()
	if err == nil {
		t.Fatalf("regenerate with failing ledger write returned nil error; output:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "ledger") {
		t.Fatalf("error %q does not mention the ledger", err)
	}
	if hits.Load() == 0 {
		t.Fatalf("fake API server never hit; output:\n%s", out.String())
	}
	if code := ExitCode(err); code == 0 {
		t.Fatalf("ExitCode(%v) = 0, want non-zero", err)
	}
}
