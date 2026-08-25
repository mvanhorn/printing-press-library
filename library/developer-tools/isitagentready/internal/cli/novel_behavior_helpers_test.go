// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/store"
)

// rawRep builds a raw scan-report JSON for tests: a flat checkID->status map
// under one "discovery" category, plus optional next-level requirements
// (checkID->prompt). Tests run serially (no t.Parallel) because seedStore sets
// a process-global home override.
func rawRep(url, at string, level int, checks, reqs map[string]string) json.RawMessage {
	cat := map[string]store.Check{}
	for id, st := range checks {
		cat[id] = store.Check{Status: st, Message: id + " " + st}
	}
	rep := store.Report{
		URL: url, ScannedAt: at, Level: level, LevelName: "L",
		Checks: map[string]map[string]store.Check{"discovery": cat},
	}
	for id, p := range reqs {
		rep.NextLevel = store.NextLevel{Name: "Next", Requirements: append(rep.NextLevel.Requirements, store.Requirement{
			Check: id, Description: "fix " + id, Prompt: p,
			SkillURL: "https://isitagentready.com/.well-known/agent-skills/" + id + "/SKILL.md",
		})}
	}
	b, _ := json.Marshal(rep)
	return b
}

func sampleRec(url, at string, level int, checks, reqs map[string]string) store.ScanRecord {
	return store.ScanRecord{URL: url, ScannedAt: at, Level: level, LevelName: "L", Raw: rawRep(url, at, level, checks, reqs)}
}

// seedStore points the CLI data dir at a temp home and writes the given scan
// records there. The home override is cleaned up at test end.
func seedStore(t *testing.T, recs ...store.ScanRecord) string {
	t.Helper()
	home := t.TempDir()
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	t.Cleanup(restore)
	path, err := store.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	for _, r := range recs {
		if err := store.Append(path, r); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	return home
}

// runCLI executes the full command tree with --home set, capturing stdout and
// stderr. --agent is added unless the caller already passes an output flag so
// commands emit JSON for assertions.
func runCLI(t *testing.T, home string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := RootCmd()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	full := append([]string{"--home", home}, args...)
	root.SetArgs(full)
	err = root.Execute()
	return out.String(), errb.String(), err
}
