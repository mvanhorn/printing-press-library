// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
)

func TestHomeOverrideIsScopedToTheInvocation(t *testing.T) {
	testenv.Isolate(t)
	home := t.TempDir()
	run := func(dir string) {
		t.Helper()
		root := RootCmd()
		var out, errb bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&errb)
		root.SetArgs([]string{"movers", "--dry-run", "--json", "--home", dir})
		if err := root.Execute(); err != nil {
			t.Fatalf("movers --dry-run: %v\n%s", err, errb.String())
		}
	}
	under := func(dir string) bool {
		t.Helper()
		data, err := cliutil.DataDir()
		if err != nil {
			t.Fatal(err)
		}
		rel, err := filepath.Rel(dir, data)
		return err == nil && !strings.HasPrefix(rel, "..")
	}

	// A directly executed command (tests, embedders) restores the override
	// as soon as it returns: the next run in the process starts clean.
	run(home)
	if cliutil.HomeOverrideActive() {
		t.Fatal("--home leaked past the command that set it")
	}

	// Under Execute() the override lasts through Execute's own post-run work
	// (journal entry, receipt) and is restored when Execute returns.
	end := executeHomeScope()
	run(home)
	if !cliutil.HomeOverrideActive() || !under(home) {
		t.Fatal("under Execute the --home override must last until Execute returns")
	}
	end()
	if cliutil.HomeOverrideActive() {
		t.Fatal("Execute's scope did not restore the override")
	}

	// An override the caller set before the run is put back, not cleared.
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	run(t.TempDir())
	if !cliutil.HomeOverrideActive() || !under(home) {
		t.Fatal("the caller's own override was not restored after the run")
	}
}
