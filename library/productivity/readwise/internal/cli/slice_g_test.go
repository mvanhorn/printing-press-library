package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"io"
	"strconv"
	"testing"
)

// Decode twice: valid JSON followed by a hint or second JSON value is a failure.
func sliceGJSON(t *testing.T, args ...string) json.RawMessage {
	t.Helper()
	cmd := RootCmd()
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v: %v; %s", args, err, stderr.String())
	}
	dec := json.NewDecoder(&out)
	var value json.RawMessage
	if err := dec.Decode(&value); err != nil {
		t.Fatalf("%v: %v: %s", args, err, out.String())
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		t.Fatalf("%v: extra stdout: %v", args, err)
	}
	return value
}

func TestSliceGDryRunEnvelopes(t *testing.T) {
	testenv.Isolate(t)
	commands := [][]string{
		{"ask", "attention"}, {"backlog"}, {"connections"}, {"dedupe"}, {"digest"},
		{"mirror"}, {"triage", "stale"}, {"read-next"},
		{"watch", "add", "--name", "inbox", "--kind", "docs"},
		{"watch", "list"}, {"watch", "run"}, {"watch", "remove", "1"}, {"watch", "diff", "inbox"},
	}
	for _, args := range commands {
		for _, mode := range []string{"--json", "--agent"} {
			t.Run(args[0]+"/"+mode+"/"+strconv.Itoa(len(args)), func(t *testing.T) {
				raw := sliceGJSON(t, append(append([]string{}, args...), "--dry-run", mode)...)
				var envelope struct {
					Action  string `json:"action"`
					Success bool   `json:"success"`
					DryRun  bool   `json:"dry_run"`
					Status  any    `json:"status"`
				}
				if err := json.Unmarshal(raw, &envelope); err != nil {
					t.Fatal(err)
				}
				if envelope.Action == "" || !envelope.Success || !envelope.DryRun || envelope.Status != nil {
					t.Fatalf("bad envelope: %s", raw)
				}
			})
		}
	}
}

func TestSliceGWatchJSON(t *testing.T) {
	testenv.Isolate(t)
	db, path := e3DB(t)
	for _, mode := range []string{"--json", "--agent"} {
		if raw := sliceGJSON(t, "watch", "list", "--db", path, mode); string(raw) != "[]" {
			t.Fatalf("empty list: %s", raw)
		}
	}
	if _, _, err := db.UpsertBatch("documents", []json.RawMessage{json.RawMessage(`{"id":"doc","title":"Example","location":"new"}`)}); err != nil {
		t.Fatal(err)
	}
	var added struct {
		ID             int64  `json:"id"`
		Name           string `json:"name"`
		BaselineSeeded int    `json:"baseline_seeded"`
	}
	if err := json.Unmarshal(sliceGJSON(t, "watch", "add", "--db", path, "--name", "inbox", "--kind", "docs", "--json"), &added); err != nil {
		t.Fatal(err)
	}
	if added.ID == 0 || added.Name != "inbox" || added.BaselineSeeded != 1 {
		t.Fatalf("created: %+v", added)
	}
	if _, _, err := db.UpsertBatch("documents", []json.RawMessage{json.RawMessage(`{"id":"new","title":"New","location":"new"}`)}); err != nil {
		t.Fatal(err)
	}
	var run struct {
		Watches []struct {
			ID          int64   `json:"id"`
			NewCount    int     `json:"new_count"`
			Notified    bool    `json:"notified"`
			FailedHooks []int64 `json:"failed_hooks"`
		} `json:"watches"`
		TotalNew int `json:"total_new"`
	}
	if err := json.Unmarshal(sliceGJSON(t, "watch", "run", "--db", path, "--agent"), &run); err != nil {
		t.Fatal(err)
	}
	if len(run.Watches) != 1 || run.TotalNew != 1 || run.Watches[0].NewCount != 1 || run.Watches[0].Notified || run.Watches[0].FailedHooks == nil {
		t.Fatalf("run: %+v", run)
	}
	var removed map[string]int64
	if err := json.Unmarshal(sliceGJSON(t, "watch", "remove", strconv.FormatInt(added.ID, 10), "--db", path, "--json"), &removed); err != nil {
		t.Fatal(err)
	}
	if removed["removed"] != added.ID {
		t.Fatal(removed)
	}
	watches, err := db.ListWatches(context.Background())
	if err != nil || len(watches) != 0 {
		t.Fatalf("remove failed: %v %v", watches, err)
	}
}
