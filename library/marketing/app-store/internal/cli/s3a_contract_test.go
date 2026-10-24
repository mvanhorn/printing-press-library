// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
)

// s3aCommands are the slice-3a commands with a valid invocation each; list
// commands print a bare JSON array, the others an object with a note.
var s3aCommands = []struct {
	args []string
	list bool
}{
	{[]string{"movers", "--genre", "36"}, true},
	{[]string{"rank-history", "324684580"}, false},
	{[]string{"keyword-history", "spotify", "--app", "324684580"}, false},
	{[]string{"watch-listing", "324684580"}, false},
	{[]string{"review-digest", "6448311069"}, false},
	{[]string{"search", "crash"}, true},
}

func TestS3AEmptyStoreContract(t *testing.T) {
	stores := map[string]string{"missing file": t.TempDir() + "/absent.db", "empty tables": s3aEmptyDB(t)}
	for name, db := range stores {
		for _, c := range s3aCommands {
			args := append(append([]string{}, c.args...), "--db", db, "--json")
			r := s3aRun(t, args...)
			if r.code != 0 {
				t.Fatalf("%s %v: exit %d (%v)", name, c.args, r.code, r.err)
			}
			if c.list {
				if strings.TrimSpace(r.stdout) != "[]" || !strings.Contains(r.stderr, "no ") {
					t.Fatalf("%s %v: want [] plus a stderr note, got %q / %q", name, c.args, r.stdout, r.stderr)
				}
				continue
			}
			var obj map[string]any
			if err := json.Unmarshal([]byte(r.stdout), &obj); err != nil {
				t.Fatalf("%s %v: not a JSON object: %v\n%s", name, c.args, err, r.stdout)
			}
			if note, _ := obj["note"].(string); note == "" {
				t.Fatalf("%s %v: empty result without a note:\n%s", name, c.args, r.stdout)
			}
			for _, key := range []string{"series", "changes", "byVersion", "complaintTerms"} {
				if v, ok := obj[key]; ok {
					if arr, isArr := v.([]any); !isArr || len(arr) != 0 {
						t.Fatalf("%s %v: %s = %v, want []", name, c.args, key, v)
					}
				}
			}
		}
	}
}

func TestS3AOutputModes(t *testing.T) {
	db := s3aSeededDB(t)
	empty := s3aEmptyDB(t)
	for _, c := range s3aCommands {
		for _, store := range []string{db, empty} {
			// --agent wraps the payload in one {meta, results} envelope.
			r := s3aRun(t, append(append([]string{}, c.args...), "--db", store, "--agent")...)
			var env struct {
				Meta    map[string]any  `json:"meta"`
				Results json.RawMessage `json:"results"`
			}
			if r.code != 0 || json.Unmarshal([]byte(r.stdout), &env) != nil || env.Meta["source"] != "local" || len(env.Results) == 0 {
				t.Fatalf("%v --agent: exit %d meta %v\n%s", c.args, r.code, env.Meta, r.stdout)
			}
			// --csv is a parseable CSV stream (or the empty-rows marker).
			r = s3aRun(t, append(append([]string{}, c.args...), "--db", store, "--csv")...)
			if r.code != 0 {
				t.Fatalf("%v --csv: exit %d (%v)", c.args, r.code, r.err)
			}
			if out := strings.TrimSpace(r.stdout); out != "" && out != emptyTabularResultMarker {
				if _, err := csv.NewReader(strings.NewReader(r.stdout)).ReadAll(); err != nil && c.list {
					t.Fatalf("%v --csv not parseable: %v\n%s", c.args, err, r.stdout)
				}
			}
			// Human output renders without error.
			r = s3aRun(t, append(append([]string{}, c.args...), "--db", store, "--human-friendly")...)
			if r.code != 0 || strings.TrimSpace(r.stdout) == "" {
				t.Fatalf("%v human: exit %d, stdout %q", c.args, r.code, r.stdout)
			}
		}
	}
}

func TestS3AAgentCompactKeepsPayload(t *testing.T) {
	db := s3aSeededDB(t)
	r := s3aRun(t, "movers", "--genre", "6014", "--db", db, "--agent")
	var env struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &env); err != nil || len(env.Results) != 6 {
		t.Fatalf("movers --agent: %v\n%s", err, r.stdout)
	}
	for _, key := range []string{"movement", "appId", "rank", "previousRank", "delta"} {
		if _, ok := env.Results[0][key]; !ok {
			t.Fatalf("--agent compaction dropped %q: %v", key, env.Results[0])
		}
	}
	r = s3aRun(t, "watch-listing", "324684580", "--db", db, "--agent")
	var wl struct {
		Results struct {
			Changes []map[string]any `json:"changes"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &wl); err != nil || len(wl.Results.Changes) != 3 {
		t.Fatalf("watch-listing --agent: %v\n%s", err, r.stdout)
	}
	for _, key := range []string{"field", "from", "to"} {
		if _, ok := wl.Results.Changes[0][key]; !ok {
			t.Fatalf("--agent compaction dropped %q: %v", key, wl.Results.Changes[0])
		}
	}
}

func TestS3ACommandsAnnotated(t *testing.T) {
	root := RootCmd()
	for _, c := range s3aCommands {
		cmd, _, err := root.Find([]string{c.args[0]})
		if err != nil || cmd.Name() != c.args[0] {
			t.Fatalf("%s not registered: %v", c.args[0], err)
		}
		if cmd.Annotations["pp:data-source"] != "local" || cmd.Annotations["mcp:read-only"] != "true" {
			t.Errorf("%s annotations = %v", c.args[0], cmd.Annotations)
		}
		if !strings.HasPrefix(cmd.Example, "  app-store-pp-cli "+c.args[0]) {
			t.Errorf("%s example must start indented with the command: %q", c.args[0], cmd.Example)
		}
		if cmd.Flags().Lookup("db") == nil {
			t.Errorf("%s has no --db flag", c.args[0])
		}
		if strings.Contains(cmd.Use, "<") && cmd.Annotations["pp:happy-args"] == "" {
			t.Errorf("%s takes a positional but has no pp:happy-args", c.args[0])
		}
	}
}
