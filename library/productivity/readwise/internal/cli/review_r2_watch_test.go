package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
	"strings"
	"testing"
)

type r2TerminalBuffer struct{ bytes.Buffer }

func (*r2TerminalBuffer) EscapesTerminalControls() bool { return true }
func TestR2WatchEscapesTerminalText(t *testing.T) {
	for _, verb := range []string{"run", "diff"} {
		t.Run(verb, func(t *testing.T) {
			testenv.Isolate(t)
			db, path := e3DB(t)
			title := "title\x1b]52;c;secret\x07\x1b[2J"
			name := "reading\x1b[2J\x07"
			raw, _ := json.Marshal(map[string]string{"id": "doc-id", "title": title, "url": "https://example.test/\x1b[2J"})
			if _, _, err := db.UpsertBatch("documents", []json.RawMessage{raw}); err != nil {
				t.Fatal(err)
			}
			if _, err := db.AddWatch(context.Background(), name, "docs", "{}"); err != nil {
				t.Fatal(err)
			}
			out := &r2TerminalBuffer{}
			cmd := RootCmd()
			cmd.SetOut(out)
			cmd.SetErr(&bytes.Buffer{})
			args := []string{"watch", verb, "--db", path, "--no-learn"}
			if verb == "diff" {
				args = append(args, name)
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out.String(), "\x1b") || !strings.Contains(out.String(), "title") {
				t.Fatalf("unsafe/missing output: %q", out.String())
			}
		})
	}
}

func TestR2WatchMarkFailureContinues(t *testing.T) {
	for _, mode := range []string{"partial", "strict", "all_failed"} {
		t.Run(mode, func(t *testing.T) {
			testenv.Isolate(t)
			db, path := e3DB(t)
			if _, _, err := db.UpsertBatch("documents", []json.RawMessage{json.RawMessage(`{"id":"doc-id","title":"Reading title"}`)}); err != nil {
				t.Fatal(err)
			}
			first, err := db.AddWatch(context.Background(), "first", "docs", "{}")
			if err != nil {
				t.Fatal(err)
			}
			second, err := db.AddWatch(context.Background(), "second", "docs", "{}")
			if err != nil {
				t.Fatal(err)
			}
			condition := "NEW.watch_id=" + fmt.Sprint(first)
			if mode == "all_failed" {
				condition = "1"
			}
			if _, err := db.DB().Exec(`CREATE TRIGGER reject_seen BEFORE INSERT ON watch_seen WHEN ` + condition + ` BEGIN SELECT RAISE(ABORT,'fixture'); END`); err != nil {
				t.Fatal(err)
			}
			cmd := RootCmd()
			var out, stderr bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			args := []string{"watch", "run", "--db", path, "--no-learn"}
			if mode == "strict" {
				args = append(args, "--strict")
			}
			cmd.SetArgs(args)
			err = cmd.Execute()
			if (err != nil) != (mode != "partial") {
				t.Fatalf("exit=%v stderr=%s", err, &stderr)
			}
			if !strings.Contains(stderr.String(), fmt.Sprintf("watch #%d mark-seen:", first)) {
				t.Fatalf("missing context: %s", &stderr)
			}
			seen, err := db.SeenItemIDs(context.Background(), second, "docs")
			if err != nil {
				t.Fatal(err)
			}
			if mode != "all_failed" && len(seen) != 1 {
				t.Fatalf("second watch not consumed: %v", seen)
			}
			if !strings.Contains(out.String(), "Reading title") {
				t.Fatal("items not emitted")
			}
		})
	}
}

type r2FailWriter struct{}

func (r2FailWriter) Write([]byte) (int, error)                { return 0, fmt.Errorf("fixture output failure") }
func TestR2WatchHumanOutputFailureRetainsUnseen(t *testing.T) { r2WatchOutputFailure(t, false) }
func r2WatchOutputFailure(t *testing.T, asJSON bool) {
	testenv.Isolate(t)
	db, path := e3DB(t)
	if _, _, err := db.UpsertBatch("documents", []json.RawMessage{json.RawMessage(`{"id":"doc-id","title":"Reading title"}`)}); err != nil {
		t.Fatal(err)
	}
	id, err := db.AddWatch(context.Background(), "reading", "docs", "{}")
	if err != nil {
		t.Fatal(err)
	}
	cmd := RootCmd()
	cmd.SetOut(r2FailWriter{})
	cmd.SetErr(&bytes.Buffer{})
	args := []string{"watch", "run", "--db", path, "--no-learn"}
	if asJSON {
		args = append(args, "--json")
	}
	cmd.SetArgs(args)
	if err := cmd.Execute(); err == nil {
		t.Fatal("output error ignored")
	}
	seen, err := db.SeenItemIDs(context.Background(), id, "docs")
	if err != nil || len(seen) != 0 {
		t.Fatalf("consumed before output: %v %v", seen, err)
	}
}

func TestR2WatchDeliveryDisclosure(t *testing.T) {
	testenv.Isolate(t)
	db, path := e3DB(t)
	if _, _, err := db.UpsertBatch("documents", []json.RawMessage{json.RawMessage(`{"id":"doc-id","title":"Reading title"}`)}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddWatch(context.Background(), "reading", "docs", "{}"); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{deliverSink: DeliverSink{Scheme: "file"}}
	cmd := newWatchRunCmd(flags)
	cmd.SetContext(context.Background())
	var stderr bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--db", path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "matched item titles/urls are included") {
		t.Fatalf("missing disclosure: %s", &stderr)
	}
	stderr.Reset()
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stderr.String(), "titles/urls") {
		t.Fatalf("empty disclosure: %s", &stderr)
	}
}

func TestR2WatchJSONOutputFailureRetainsUnseen(t *testing.T) { r2WatchOutputFailure(t, true) }

func TestR2WatchHighlightControls(t *testing.T) {
	for _, name := range []string{"items", "empty"} {
		t.Run(name, func(t *testing.T) {
			out := &r2TerminalBuffer{}
			cmd := RootCmd()
			cmd.SetOut(out)
			result := watchDiffResult{WatchName: "watch\x1b[2J\x07", Kind: "highlights"}
			if name == "items" {
				result.NewCount = 1
				result.HLItems = []store.HLItem{{ID: "id\x1b[2J", Title: "title\x07", URL: "url\x1b[2J"}}
			}
			if err := emitWatchDiffResult(cmd, &rootFlags{}, result); err != nil {
				t.Fatal(err)
			}
			if strings.ContainsAny(out.String(), "\x1b\x07") || !strings.Contains(out.String(), `\x1b`) {
				t.Fatalf("unsafe/missing escape: %q", out.String())
			}
		})
	}
}
