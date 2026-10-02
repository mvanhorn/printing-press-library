package cli

import (
	"bytes"
	"encoding/json"
	"github.com/spf13/cobra"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/driveplaza/internal/cliutil/testenv"
)

func TestDrivePlazaDryRunAndReadOnly(t *testing.T) {
	for _, path := range [][]string{{"route"}, {"interchanges"}, {"roads"}, {"conditions"}, {"sapa", "list"}, {"sapa", "detail"}, {"sapa", "facilities"}, {"notices"}, {"handoff"}} {
		t.Run(strings.Join(path, "-"), func(t *testing.T) {
			testenv.Isolate(t)
			root := RootCmd()
			cmd, _, e := root.Find(path)
			if e != nil || cmd.Annotations["mcp:read-only"] != "true" {
				t.Fatalf("command not read-only: %v", e)
			}
			root.SetArgs(append(path, "--dry-run", "--agent", "--no-learn"))
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			if e = root.Execute(); e != nil {
				t.Fatal(e)
			}
			if !json.Valid(out.Bytes()) || !bytes.Contains(out.Bytes(), []byte(`"dry_run":true`)) {
				t.Fatalf("dry-run not structured: %s", out.String())
			}
		})
	}
}

func TestDrivePlazaCollectionProjectionKeepsShape(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{Use: "example"}
	cmd.SetOut(&out)
	flags := &rootFlags{agent: true, asJSON: true, compact: true, selectFields: "items.id"}
	v := map[string]any{"meta": map[string]any{"source": "live", "upstream_requests": 2}, "results": map[string]any{"items": []map[string]any{{"id": "1040/1040021/1", "name": "example"}}, "total": 1}}
	if e := dpEmit(cmd, flags, v); e != nil {
		t.Fatal(e)
	}
	var decoded map[string]json.RawMessage
	if e := json.Unmarshal(out.Bytes(), &decoded); e != nil {
		t.Fatal(e)
	}
	var rows struct {
		Items []map[string]any `json:"items"`
	}
	if e := json.Unmarshal(decoded["results"], &rows); e != nil || len(rows.Items) != 1 || len(rows.Items[0]) != 1 {
		t.Fatalf("collection shape changed: %s %v", out.String(), e)
	}
	if !flags.agent || !flags.compact || flags.selectFields != "items.id" {
		t.Fatal("projection did not restore caller flags")
	}
}

func TestDrivePlazaEmptyTabularOutput(t *testing.T) {
	for _, mode := range []string{"csv", "plain", "quiet"} {
		t.Run(mode, func(t *testing.T) {
			var out bytes.Buffer
			cmd := &cobra.Command{Use: "example"}
			cmd.SetOut(&out)
			flags := &rootFlags{csv: mode == "csv", plain: mode == "plain", quiet: mode == "quiet"}
			v := map[string]any{"meta": map[string]any{"source": "live", "upstream_requests": 2}, "results": map[string]any{"items": []any{}, "total": 0, "note": "No matches"}}
			if e := dpEmit(cmd, flags, v); e != nil {
				t.Fatal(e)
			}
			if out.Len() != 0 {
				t.Fatalf("empty set became a tabular row: %s", out.String())
			}
		})
	}
}
func TestDrivePlazaUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"route", "--agent"}, {"route", "--from", "nerima", "--to", "sendai-minami", "--at", "2026-02-30T08:00"}, {"sapa", "detail", "--id", "../../etc"}, {"sapa", "list", "--road", "ALL"}, {"interchanges", "--query", "a", "--limit", "1000"}, {"notices", "--since", "2026-02-30"}, {"handoff", "--purpose", "invalid"}, {"route", "--data-source", "local", "--agent"}, {"route", "--from", strings.Repeat("x", 101), "--to", "sendai-minami", "--at", "2026-10-10T08:00"}, {"route", "--from", "nerima", "--to", "sendai-minami", "--at", "2026-10-10T08:00", "--via", "a,b,c,d,e,f"}} {
		testenv.Isolate(t)
		root := RootCmd()
		root.SetArgs(append(args, "--no-learn"))
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		e := root.Execute()
		if e == nil || ExitCode(e) != 2 {
			t.Fatalf("%v error=%v code=%d", args, e, ExitCode(e))
		}
	}
}
func TestDrivePlazaProjectionPreservesProvenance(t *testing.T) {
	for _, selectFields := range []string{"purpose,url", "results.purpose,results.url"} {
		testenv.Isolate(t)
		root := RootCmd()
		root.SetArgs([]string{"handoff", "--agent", "--no-learn", "--select", selectFields})
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		if e := root.Execute(); e != nil {
			t.Fatal(e)
		}
		var value struct {
			Meta    map[string]any   `json:"meta"`
			Results []map[string]any `json:"results"`
		}
		if e := json.Unmarshal(out.Bytes(), &value); e != nil {
			t.Fatal(e, out.String())
		}
		if value.Meta["status"] != "handoff_only" || len(value.Results) != 8 || len(value.Results[0]) != 2 {
			t.Fatalf("projection lost provenance or nested results: %s", out.String())
		}
		if bytes.Count(bytes.TrimSpace(out.Bytes()), []byte("\n")) != 0 {
			t.Fatal("agent JSON is not compact")
		}
	}
}
