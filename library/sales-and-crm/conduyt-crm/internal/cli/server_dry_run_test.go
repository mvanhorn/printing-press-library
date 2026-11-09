package cli

// PATCH(reprint-637-server-dry-run): the global --dry-run means "print the request, send nothing" on every command. The API's
// dry_run query parameter is the local flag --server-dry-run (the request IS sent); no command registers a local --dry-run.

import (
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// email send and send-bulk keep their hand-written flag set (email-send-live-keys-cli-324.md): dry_run stays off their flags.
var serverDryRunExempt = map[string]bool{"POST /email/send": true, "POST /email/send-bulk": true}

func specDryRunOperations(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "spec.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	resolve := func(v any) map[string]any {
		m, _ := v.(map[string]any)
		for m != nil {
			ref, ok := m["$ref"].(string)
			if !ok {
				break
			}
			var node any = doc
			for _, k := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
				node = node.(map[string]any)[k]
			}
			m, _ = node.(map[string]any)
		}
		return m
	}
	out := map[string]bool{}
	for path, item := range doc["paths"].(map[string]any) {
		pi := item.(map[string]any)
		for _, method := range []string{"get", "post", "put", "patch", "delete"} {
			op, ok := pi[method].(map[string]any)
			if !ok {
				continue
			}
			params, _ := op["parameters"].([]any)
			for _, p := range params {
				if pm := resolve(p); pm != nil && pm["in"] == "query" && pm["name"] == "dry_run" {
					out[strings.ToUpper(method)+" "+path] = true
				}
			}
		}
	}
	return out
}

func TestNoLocalDryRunAndServerDryRunIsBound(t *testing.T) {
	// gocli1219: the preview sweep runs bare commands; under 4.34.2 only the verify / dogfood harnesses waive the
	// required flags under --dry-run (requiredInputWaived), so the sweep runs as the dogfood harness does.
	t.Setenv(cliutil.DogfoodEnvVar, "1")
	ops := specDryRunOperations(t)
	if len(ops) < 100 {
		t.Fatalf("spec.yaml shows only %d operations with a dry_run query parameter; the scan is blind", len(ops))
	}
	root := newRootCmd(&rootFlags{})
	if root.PersistentFlags().Lookup("dry-run") == nil {
		t.Fatal("the global --dry-run is missing")
	}
	seen := map[string]bool{}
	var walk func(c *cobra.Command, chain []string)
	walk = func(c *cobra.Command, chain []string) {
		if c != root && c.LocalNonPersistentFlags().Lookup("dry-run") != nil {
			t.Errorf("%s registers a local --dry-run, which would shadow the global preview", c.CommandPath())
		}
		key := c.Annotations["pp:method"] + " " + c.Annotations["pp:path"]
		if ops[key] && !serverDryRunExempt[key] {
			seen[key] = true
			if f := c.Flags().Lookup("server-dry-run"); f == nil || !strings.Contains(f.Usage, "the request IS sent") {
				t.Errorf("%s (%s): --server-dry-run is %v", c.CommandPath(), key, f)
			} else {
				dryRunPreviewSendsNothing(t, c, chain)
			}
		}
		for _, sub := range c.Commands() {
			walk(sub, append(append([]string{}, chain...), sub.Name()))
		}
	}
	walk(root, nil)
	for op := range ops {
		if !seen[op] && !serverDryRunExempt[op] {
			t.Errorf("%s has a dry_run query parameter but no command binds --server-dry-run", op)
		}
	}
}

// dryRunPreviewSendsNothing runs the command with the global --dry-run (and --server-dry-run) against a server that counts
// requests; the preview must print and send nothing.
func dryRunPreviewSendsNothing(t *testing.T, c *cobra.Command, chain []string) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{}}`)
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("CONDUYT_CRM_NO_LEARN", "true")
	t.Setenv("HOME", t.TempDir())
	root := newRootCmd(&rootFlags{})
	args := append(append([]string{}, chain...), "--dry-run", "--server-dry-run", "--no-cache")
	for i := 0; i < strings.Count(c.Use, "<"); i++ {
		args = append(args, "00000000-0000-4000-8000-00000000000"+string(rune('1'+i)))
	}
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	var err error
	preview := captureStderr(t, func() { err = root.Execute() })
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Errorf("%s --dry-run sent %d request(s) (err %v), want none", c.CommandPath(), n, err)
	}
	if err != nil {
		t.Errorf("%s --dry-run: %v, want a successful preview", c.CommandPath(), err)
		return
	}
	method := c.Annotations["pp:method"]
	if !strings.Contains(preview, method+" "+srv.URL+"/api/v1") || !strings.Contains(preview, "dry_run=true") || !strings.Contains(preview, "(dry run - no request sent)") {
		t.Errorf("%s --dry-run printed no %s preview carrying dry_run=true:\n%s", c.CommandPath(), method, preview)
	}
}

// captureStderr returns what fn wrote to os.Stderr (the client's dry-run preview writes there).
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	defer func() { os.Stderr = orig }()
	fn()
	_ = w.Close()
	os.Stderr = orig
	return <-done
}

// countingServer counts every request it receives.
func countingServer(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func isolateCLIEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("CONDUYT_CRM_NO_LEARN", "true")
	t.Setenv("HOME", t.TempDir())
}

// PATCH(reprint-637-server-dry-run): `feedback --send` under the global --dry-run prints the POST it would make and sends nothing.
func TestFeedbackSendUnderDryRunPrintsAndSendsNothing(t *testing.T) {
	srv, hits := countingServer(t)
	isolateCLIEnv(t)
	t.Setenv("CONDUYT_CRM_FEEDBACK_ENDPOINT", srv.URL+"/feedback")
	root := newRootCmd(&rootFlags{})
	var out, errOut strings.Builder
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"feedback", "--send", "--dry-run", "flag names are hard to guess"})
	if err := root.Execute(); err != nil {
		t.Fatalf("feedback --send --dry-run: %v", err)
	}
	if n := atomic.LoadInt32(hits); n != 0 {
		t.Errorf("feedback --send --dry-run sent %d request(s), want none", n)
	}
	for _, want := range []string{"POST " + srv.URL + "/feedback", `"text": "flag names are hard to guess"`, "(dry run - no request sent)"} {
		if !strings.Contains(errOut.String(), want) {
			t.Errorf("preview is missing %q:\n%s", want, errOut.String())
		}
	}
	if !strings.Contains(out.String(), "dry run: feedback not recorded and not sent") {
		t.Errorf("stdout = %q", out.String())
	}
	// control: without --dry-run the same invocation does reach the endpoint
	root = newRootCmd(&rootFlags{})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"feedback", "--send", "flag names are hard to guess"})
	if err := root.Execute(); err != nil || atomic.LoadInt32(hits) != 1 {
		t.Errorf("control run: err %v, %d request(s), want 1", err, atomic.LoadInt32(hits))
	}
}

// PATCH(reprint-637-server-dry-run): `--deliver webhook:<url>` under the global --dry-run prints the delivery it would make and
// sends nothing.
func TestWebhookDeliveryUnderDryRunPrintsAndSendsNothing(t *testing.T) {
	srv, hits := countingServer(t)
	isolateCLIEnv(t)
	run := func(dry bool) (string, error) {
		f := &rootFlags{}
		root := newRootCmd(f)
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		args := []string{"--deliver", "webhook:" + srv.URL + "/hook", "feedback", "hello"}
		if dry {
			args = append([]string{"--dry-run"}, args...)
		}
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			return "", err
		}
		var errOut strings.Builder
		err := finishDelivery(f, &errOut)
		return errOut.String(), err
	}
	preview, err := run(true)
	if err != nil {
		t.Fatalf("--dry-run --deliver webhook: %v", err)
	}
	if n := atomic.LoadInt32(hits); n != 0 {
		t.Errorf("--dry-run --deliver webhook sent %d request(s), want none", n)
	}
	for _, want := range []string{"POST " + srv.URL + "/hook", "Content-Type: application/json", "Body:", "dry run: feedback not recorded and not sent", "(dry run - webhook delivery not sent)"} {
		if !strings.Contains(preview, want) {
			t.Errorf("preview is missing %q:\n%s", want, preview)
		}
	}
	// control: without --dry-run the webhook is called once
	if _, err := run(false); err != nil || atomic.LoadInt32(hits) != 1 {
		t.Errorf("control run: err %v, %d request(s), want 1", err, atomic.LoadInt32(hits))
	}
}
