package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/planner"
	"github.com/spf13/cobra"
)

type publicationHTTPTrap struct{ requests atomic.Int32 }

type publicationFailingWriter struct{ err error }

func (w publicationFailingWriter) Write([]byte) (int, error) { return 0, w.err }

func (t *publicationHTTPTrap) RoundTrip(*http.Request) (*http.Response, error) {
	t.requests.Add(1)
	return nil, errors.New("unexpected HTTP during unsupported-command validation")
}

func TestPlanningPublicationRejectsUnsupportedStoreRoutesBeforeIO(t *testing.T) {
	for _, args := range [][]string{{"import", "source"}, {"workflow", "archive"}, {"workflow", "status"}, {"sync"}, {"search", "sushi"}, {"export", "source"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			planningIsolate(t)
			trap := &publicationHTTPTrap{}
			previous := http.DefaultTransport
			http.DefaultTransport = trap
			t.Cleanup(func() { http.DefaultTransport = previous })
			factoryCalls := planningTestFactory(t, &planningTestTransport{})
			home := filepath.Join(t.TempDir(), "must-not-exist")
			cmd := RootCmd()
			cmd.SetArgs(append(append([]string{}, args...), "--home", home))
			var out, diag bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&diag)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), "unknown command") {
				t.Fatalf("unsupported command did not reject at root: err=%v stdout=%q stderr=%q", err, out.String(), diag.String())
			}
			if out.Len() != 0 || trap.requests.Load() != 0 || *factoryCalls != 0 {
				t.Fatalf("unsupported command produced output or performed HTTP: stdout=%q HTTP=%d factory=%d", out.String(), trap.requests.Load(), *factoryCalls)
			}
			if _, err := os.Stat(home); !os.IsNotExist(err) {
				t.Fatalf("unsupported command wrote a home/cache/store directory: %v", err)
			}
		})
	}
}

func TestPlanningPublicationKeepsApprovedPathsAndStandardUtilities(t *testing.T) {
	planningIsolate(t)
	root := RootCmd()
	for _, path := range [][]string{{"venues", "search"}, {"venues", "get"}, {"cuisines", "list"}, {"courses", "list"}, {"courses", "get"}, {"availability", "check"}, {"availability", "scan"}, {"booking-url"}, {"doctor"}, {"version"}, {"agent-context"}, {"profile"}, {"api"}} {
		cmd, remainder, err := root.Find(path)
		if err != nil || len(remainder) != 0 || cmd.Name() != path[len(path)-1] {
			t.Errorf("supported command lost: %v command=%v remaining=%v err=%v", path, cmd, remainder, err)
		}
	}
	for _, c := range root.Commands() {
		switch c.Name() {
		case "import", "workflow", "sync", "search", "export":
			t.Errorf("unsupported route remains registered, even if hidden: %s", c.CommandPath())
		}
	}
}

func TestPlanningPublicationRejectsCalendarAsWriteBeforeOpeningInput(t *testing.T) {
	planningIsolate(t)
	if path, err := resourceWritePath("source"); err == nil || path != "" {
		t.Fatalf("read-only calendar still registered as writable: path=%q err=%v", path, err)
	}
	trap := &publicationHTTPTrap{}
	previous := http.DefaultTransport
	http.DefaultTransport = trap
	t.Cleanup(func() { http.DefaultTransport = previous })
	// Exercise the retained private constructor too: it must reject before
	// opening an input file or creating an API client, regardless of root wiring.
	cmd := newImportCmd(&rootFlags{})
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"source", "--input", filepath.Join(t.TempDir(), "missing.jsonl")})
	var out, diag bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&diag)
	err := cmd.Execute()
	if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "unknown writable resource") {
		t.Fatalf("private import did not reject the write registry first: err=%v stdout=%q", err, out.String())
	}
	if out.Len() != 0 || trap.requests.Load() != 0 {
		t.Fatalf("rejected import produced a success result or HTTP: stdout=%q requests=%d", out.String(), trap.requests.Load())
	}
}

func TestPlanningPublicationRemovesRegeneratedStoreRegistrations(t *testing.T) {
	planningIsolate(t)
	root := &cobra.Command{Use: "tablecheck-pp-cli"}
	for _, name := range []string{"import", "workflow", "sync", "search", "export", "version"} {
		root.AddCommand(&cobra.Command{Use: name})
	}
	registerPlanningCommands(root, &rootFlags{})
	for _, c := range root.Commands() {
		switch c.Name() {
		case "import", "workflow", "sync", "search", "export":
			t.Errorf("regenerated unsupported command escaped the registration guard: %s", c.Name())
		}
	}
	if cmd, remaining, err := root.Find([]string{"version"}); err != nil || len(remaining) != 0 || cmd.Name() != "version" {
		t.Fatalf("guard removed a standard utility: command=%v remaining=%v err=%v", cmd, remaining, err)
	}
}

func publicationFormatResult() planner.Result {
	return planner.Result{
		"items": []map[string]any{
			{"id": "course-one", "name": "Example course", "price": "19800.0", "price_basis": nil, "qty_remaining": nil, "tax_type": "included"},
			{"id": "course-two", "name": "Unknown price", "price": nil, "price_basis": nil, "qty_remaining": nil, "tax_type": "included"},
		},
		"pagination": map[string]any{"limit": 2, "has_more": false, "next_offset": nil},
		"meta":       map[string]any{"source": "live", "requests": 0, "transport": "local-cache", "freshness": map[string]any{"cache_hit": true, "fetched_at": "2026-09-27T14:00:00Z", "served_at": "2026-09-27T14:00:10Z"}},
	}
}

func TestPlanningPublicationNativeFormatsAreCSVTSVAndQuietIdentities(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags rootFlags
		want  string
	}{
		{"csv", rootFlags{asJSON: true, csv: true, selectFields: "items.id,items.price"}, "id,price\ncourse-one,19800.0\ncourse-two,null\n"},
		{"plain", rootFlags{asJSON: true, plain: true, selectFields: "items.id,items.price"}, "id\tprice\ncourse-one\t19800.0\ncourse-two\tnull\n"},
		{"quiet", rootFlags{asJSON: true, quiet: true, selectFields: "items.id,items.price"}, "course-one\ncourse-two\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			var out bytes.Buffer
			cmd.SetOut(&out)
			if err := planningPrint(cmd, &tc.flags, publicationFormatResult()); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.want {
				t.Fatalf("native %s result changed: got=%q want=%q", tc.name, out.String(), tc.want)
			}
			if json.Valid(out.Bytes()) {
				t.Fatalf("native %s unexpectedly produced JSON: %s", tc.name, out.String())
			}
			if tc.name == "csv" {
				rows, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
				if err != nil || len(rows) != 3 || rows[1][1] != "19800.0" || rows[2][1] != "null" {
					t.Fatalf("CSV is invalid or changed string/null money: rows=%v err=%v", rows, err)
				}
			}
		})
	}
}

func TestPlanningPublicationDefaultAndAgentJSONPreserveFreshnessAndNulls(t *testing.T) {
	for _, agent := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "agent"}[agent], func(t *testing.T) {
			cmd := &cobra.Command{}
			var out bytes.Buffer
			cmd.SetOut(&out)
			flags := &rootFlags{agent: agent, compact: agent, asJSON: agent}
			if err := planningPrint(cmd, flags, publicationFormatResult()); err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			var want map[string]any
			encoded, _ := json.Marshal(publicationFormatResult())
			_ = json.Unmarshal(encoded, &want)
			if !reflect.DeepEqual(got, want) || strings.Count(strings.TrimSpace(out.String()), "\n") != 0 {
				t.Fatalf("normalized JSON lost fields/freshness/nulls or compact shape: %s", out.String())
			}
		})
	}
}

func TestPlanningPublicationNativeFormatWriterErrorsPropagate(t *testing.T) {
	for _, flags := range []rootFlags{{csv: true}, {plain: true}, {quiet: true}} {
		cmd := &cobra.Command{}
		failure := errors.New("output writer failed")
		cmd.SetOut(publicationFailingWriter{failure})
		if err := planningPrint(cmd, &flags, publicationFormatResult()); !errors.Is(err, failure) {
			t.Fatalf("native format swallowed output failure: flags=%+v err=%v", flags, err)
		}
	}
}
