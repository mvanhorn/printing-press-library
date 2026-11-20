package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCommentAddFormsShareOneHandler(t *testing.T) {
	forms := [][]string{
		{"comments", "add", "--issue", "MOB-1686"},
		{"comments", "add", "MOB-1686"},
		{"comments", "create", "MOB-1686"},
		{"issues", "comment", "MOB-1686"},
		{"issues", "comment", "MOB-1686", "--issue", "MOB-1686"},
	}
	var canonical map[string]any
	for _, form := range forms {
		args := append(append([]string{}, form...), "--body", "x", "--dry-run", "--agent")
		out, err := executeRootForTest(args...)
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", form, err, out)
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(out), &payload); err != nil {
			t.Fatalf("%v output is not JSON: %v\n%s", form, err, out)
		}
		if canonical == nil {
			canonical = payload
			continue
		}
		if !reflect.DeepEqual(payload, canonical) {
			t.Fatalf("%v diverged from comments add --issue:\ngot=%#v\nwant=%#v", form, payload, canonical)
		}
	}
	if canonical["input"].(map[string]any)["issueId"] != "MOB-1686" {
		t.Fatalf("positional issue did not become the comment target: %#v", canonical)
	}
}

func TestCommentAddRejectsConflictingTargets(t *testing.T) {
	for _, args := range [][]string{
		{"comments", "add", "MOB-1", "--issue", "MOB-2"},
		{"issues", "comment", "MOB-1", "--project", "00000000-0000-4000-8000-000000000001"},
	} {
		out, err := executeRootForTest(append(args, "--body", "x", "--dry-run", "--agent")...)
		if err == nil {
			t.Fatalf("%v unexpectedly succeeded: %s", args, out)
		}
		if ExitCode(err) != 2 {
			t.Fatalf("%v exit code = %d, want 2: %v", args, ExitCode(err), err)
		}
	}
}

func TestIssuesCommentStaysHiddenAndLeavesIssuesCreateAlone(t *testing.T) {
	var flags rootFlags
	root := newRootCmd(&flags)
	comment, _, err := root.Find([]string{"issues", "comment"})
	if err != nil || comment.Name() != "comment" || !comment.Hidden {
		t.Fatalf("issues comment = %v (hidden=%v), err=%v", comment.CommandPath(), comment.Hidden, err)
	}
	create, _, err := root.Find([]string{"issues", "create"})
	if err != nil || create.Short != "Create a new Linear issue and record it in the pp_created ledger" {
		t.Fatalf("issues create resolved to %q (%s), err=%v", create.CommandPath(), create.Short, err)
	}
}

func TestIssuesUpdateAliasesEdit(t *testing.T) {
	run := func(verb string) string {
		t.Helper()
		out, err := executeRootForTest("issues", verb, "MOB-1", "--title", "x", "--dry-run", "--agent")
		if err != nil {
			t.Fatalf("issues %s failed: %v\n%s", verb, err, out)
		}
		return out
	}
	if update, edit := run("update"), run("edit"); update != edit {
		t.Fatalf("issues update diverged from issues edit:\nupdate=%s\nedit=%s", update, edit)
	}
}

func TestLabelsFlagAccumulatesIntoLabelName(t *testing.T) {
	for _, path := range [][]string{{"issues", "create"}, {"issues", "edit"}} {
		var flags rootFlags
		root := newRootCmd(&flags)
		cmd, _, err := root.Find(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.ParseFlags([]string{"--labels", "kind:bug", "--label-name", "area:cli", "--labels=risk:low"}); err != nil {
			t.Fatalf("%v rejected --labels: %v", path, err)
		}
		got, err := cmd.Flags().GetStringSlice("label-name")
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"kind:bug", "area:cli", "risk:low"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("%v label-name = %v, want %v", path, got, want)
		}
		usage := cmd.UsageString()
		if !strings.Contains(usage, "--label-name") || strings.Contains(usage, "--labels") {
			t.Fatalf("%v help must keep the canonical flag name only:\n%s", path, usage)
		}
	}
}

func TestAgentFailuresPrintOneJSONErrorFromTheProcess(t *testing.T) {
	var linearRequests int
	linear := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		linearRequests++
		_, _ = io.WriteString(w, `{"data":{"issueLabels":{"nodes":[],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`)
	}))
	t.Cleanup(linear.Close)
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	t.Cleanup(webhook.Close)
	t.Setenv("LINEAR_BASE_URL", linear.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")
	db := filepath.Join(t.TempDir(), "linear.db")

	cases := []struct {
		name         string
		args         []string
		wantCode     int
		wantError    string
		wantRequests bool
	}{
		{
			name:         "missing label name",
			args:         []string{"issues", "create", "--team", "MOB", "--title", "x", "--label-name", "nope", "--db", db, "--agent"},
			wantCode:     2,
			wantError:    `label "nope" not found`,
			wantRequests: true,
		},
		{
			name:      "label name passed to --label",
			args:      []string{"issues", "create", "--team", "MOB", "--title", "x", "--label", "kind:bug", "--db", db, "--agent"},
			wantCode:  2,
			wantError: `--label-name "kind:bug"`,
		},
		{
			name:      "label name passed to --label in a dry run",
			args:      []string{"issues", "edit", "MOB-1", "--label", "kind:bug", "--dry-run", "--agent"},
			wantCode:  2,
			wantError: `--label-name "kind:bug"`,
		},
		{
			name:      "unknown subcommand",
			args:      []string{"comments", "frobnicate", "--agent"},
			wantCode:  2,
			wantError: `unknown subcommand "frobnicate"`,
		},
		{
			name:      "unknown command",
			args:      []string{"frobnicate", "--agent"},
			wantCode:  2,
			wantError: `unknown command "frobnicate"`,
		},
		{
			name:      "failed delivery",
			args:      []string{"issues", "create", "--team", "MOB", "--title", "x", "--db", db, "--dry-run", "--agent", "--deliver", "webhook:" + webhook.URL},
			wantCode:  1,
			wantError: "deliver to webhook:",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			linearRequests = 0
			stdout, code := executeProcessForTest(t, tc.args...)
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d; stdout=%s", code, tc.wantCode, stdout)
			}
			var envelopes []map[string]any
			dec := json.NewDecoder(strings.NewReader(stdout))
			for dec.More() {
				var doc map[string]any
				if err := dec.Decode(&doc); err != nil {
					t.Fatalf("stdout is not a JSON stream: %v\n%s", err, stdout)
				}
				if _, ok := doc["error"]; ok {
					envelopes = append(envelopes, doc)
				}
			}
			if len(envelopes) != 1 {
				t.Fatalf("got %d JSON error envelopes, want 1; stdout=%s", len(envelopes), stdout)
			}
			if msg, _ := envelopes[0]["error"].(string); !strings.Contains(msg, tc.wantError) {
				t.Fatalf("error %q does not contain %q", msg, tc.wantError)
			}
			if !tc.wantRequests && linearRequests != 0 {
				t.Fatalf("made %d Linear requests, want none", linearRequests)
			}
		})
	}
}

func executeProcessForTest(t *testing.T, args ...string) (string, int) {
	t.Helper()
	oldArgs, oldStdout, oldStderr := os.Args, os.Stdout, os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&stdout, r)
		close(done)
	}()
	os.Args = append([]string{"linear-pp-cli"}, args...)
	os.Stdout, os.Stderr = w, devNull
	runErr := Execute()
	os.Args, os.Stdout, os.Stderr = oldArgs, oldStdout, oldStderr
	_ = w.Close()
	<-done
	_ = r.Close()
	_ = devNull.Close()
	if runErr == nil {
		return stdout.String(), 0
	}
	return stdout.String(), ExitCode(runErr)
}
