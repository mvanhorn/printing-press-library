package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/jalan"
)

// Exercise the real entrypoint because Execute owns the --deliver output buffer.
func executeStayDelivery(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	restoreHome, err := cliutil.SetHomeOverride("")
	if err != nil {
		t.Fatalf("reset home override: %v", err)
	}
	defer restoreHome()
	out, err := os.CreateTemp(t.TempDir(), "stdout-")
	if err != nil {
		t.Fatal(err)
	}
	diagnostic, err := os.CreateTemp(t.TempDir(), "stderr-")
	if err != nil {
		out.Close()
		t.Fatal(err)
	}
	defer out.Close()
	defer diagnostic.Close()
	previousArgs, previousOut, previousErr := os.Args, os.Stdout, os.Stderr
	defer func() { os.Args, os.Stdout, os.Stderr = previousArgs, previousOut, previousErr }()
	os.Args = append([]string{"jalan-pp-cli", "--no-learn", "--home", t.TempDir(), "stay"}, args...)
	os.Stdout, os.Stderr = out, diagnostic
	runErr := Execute()
	if _, err := out.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if _, err := diagnostic.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(out)
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := io.ReadAll(diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	return string(output), string(stderr), runErr
}

func partialStayDeliveryFixture() *fakeStayService {
	response := stayFixture()
	response.Meta["status"] = "partial"
	response.FetchFailures = []map[string]any{{"alternative_index": 1, "code": "parse_failure", "message": "one source alternative failed"}}
	return &fakeStayService{response: response, err: &jalan.PartialError{Failures: response.FetchFailures, Cause: &jalan.Error{Code: "parse_failure", Message: "source format changed"}}}
}

func assertStayPartialDeliveryOutput(t *testing.T, output, diagnostic string, err error) {
	t.Helper()
	if err == nil || ExitCode(err) != 8 {
		t.Fatalf("partial exit lost: %v stderr=%s", err, diagnostic)
	}
	payload := decodeStay(t, output)
	if len(payload) != 4 || len(payload["results"].([]any)) != 1 || len(payload["fetch_failures"].([]any)) != 1 {
		t.Fatalf("partial envelope incomplete: %s", output)
	}
	if !json.Valid([]byte(diagnostic)) || strings.Count(diagnostic, "\n") != 1 || decodeStay(t, diagnostic)["error"].(map[string]any)["code"] != "partial" {
		t.Fatalf("expected one partial diagnostic: %s", diagnostic)
	}
}

func TestStayPartialExecuteDeliversCompleteFileAndKeepsExit(t *testing.T) {
	installStayFake(t, partialStayDeliveryFixture())
	target := filepath.Join(t.TempDir(), "partial.json")
	output, diagnostic, err := executeStayDelivery(t, "compare", "385995", "--dates", futureStayDate(), "--deliver", "file:"+target)
	assertStayPartialDeliveryOutput(t, output, diagnostic, err)
	delivered, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("partial output never reached file sink: %v", readErr)
	}
	if !bytes.Equal(delivered, []byte(output)) {
		t.Fatalf("file does not contain the complete streamed partial envelope: %s", delivered)
	}
}

func TestStayPartialExecuteDeliversWebhookOnceAndKeepsExit(t *testing.T) {
	installStayFake(t, partialStayDeliveryFixture())
	var calls atomic.Int32
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected delivery request: %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading delivery body: %v", err)
		}
		received <- body
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	output, diagnostic, err := executeStayDelivery(t, "compare", "385995", "--dates", futureStayDate(), "--deliver", "webhook:"+server.URL)
	assertStayPartialDeliveryOutput(t, output, diagnostic, err)
	if calls.Load() != 1 {
		t.Fatalf("partial delivery missing/duplicated: calls=%d", calls.Load())
	}
	select {
	case body := <-received:
		if !bytes.Equal(body, []byte(output)) {
			t.Fatalf("webhook lost the partial envelope: %s", body)
		}
	default:
		t.Fatal("webhook body was not received")
	}
}

func TestStayPartialExecuteWithoutSinkKeepsOutputAndExit(t *testing.T) {
	installStayFake(t, partialStayDeliveryFixture())
	output, diagnostic, err := executeStayDelivery(t, "compare", "385995", "--dates", futureStayDate())
	assertStayPartialDeliveryOutput(t, output, diagnostic, err)
}

func TestStayFailedExecuteNeverDelivers(t *testing.T) {
	cases := []struct {
		name, code string
		exit       int
		response   jalan.Response
		args       []string
	}{
		{name: "access", code: "access_failure", exit: 4, args: []string{"property", "385995"}},
		{name: "parse", code: "parse_failure", exit: 9, args: []string{"property", "385995"}},
		{name: "usage", code: "usage", exit: 2, args: []string{"property"}},
		{name: "empty-partial", code: "partial", exit: 8, response: jalan.Response{Results: []any{}, FetchFailures: []map[string]any{{"code": "parse_failure"}}}, args: []string{"compare", "385995", "--dates", futureStayDate()}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installStayFake(t, &fakeStayService{response: tc.response, err: &jalan.Error{Code: tc.code, Message: "request failed", Hint: "inspect request"}})
			target := filepath.Join(t.TempDir(), "untouched.json")
			if err := os.WriteFile(target, []byte("existing payload"), 0600); err != nil {
				t.Fatal(err)
			}
			output, diagnostic, err := executeStayDelivery(t, append(append([]string{}, tc.args...), "--deliver", "file:"+target)...)
			if output != "" || err == nil || ExitCode(err) != tc.exit || !json.Valid([]byte(diagnostic)) {
				t.Fatalf("failure contract: stdout=%s stderr=%s err=%v", output, diagnostic, err)
			}
			contents, err := os.ReadFile(target)
			if err != nil || string(contents) != "existing payload" {
				t.Fatalf("failed request wrote a file sink: %s err=%v", contents, err)
			}
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(http.StatusNoContent) }))
			defer server.Close()
			output, diagnostic, err = executeStayDelivery(t, append(append([]string{}, tc.args...), "--deliver", "webhook:"+server.URL)...)
			if output != "" || err == nil || ExitCode(err) != tc.exit || !json.Valid([]byte(diagnostic)) || calls.Load() != 0 {
				t.Fatalf("failed request delivered to webhook: calls=%d stdout=%s stderr=%s err=%v", calls.Load(), output, diagnostic, err)
			}
		})
	}
}

func TestStayPartialExecuteDeliveryFailureEmitsOneActionableError(t *testing.T) {
	for _, sink := range []string{"file", "webhook"} {
		t.Run(sink, func(t *testing.T) {
			installStayFake(t, partialStayDeliveryFixture())
			var target string
			if sink == "file" {
				parent := filepath.Join(t.TempDir(), "regular-file")
				if err := os.WriteFile(parent, []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
				target = filepath.Join(parent, "partial.json")
			} else {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
				defer server.Close()
				target = server.URL
			}
			output, diagnostic, err := executeStayDelivery(t, "compare", "385995", "--dates", futureStayDate(), "--deliver", sink+":"+target)
			if err == nil || ExitCode(err) != 5 {
				t.Fatalf("delivery failure lost: %v stderr=%s", err, diagnostic)
			}
			payload := decodeStay(t, output)
			if len(payload["results"].([]any)) != 1 || len(payload["fetch_failures"].([]any)) != 1 {
				t.Fatalf("delivery failure lost stdout partial facts: %s", output)
			}
			if !json.Valid([]byte(diagnostic)) || strings.Count(diagnostic, "\n") != 1 {
				t.Fatalf("duplicate/unstructured diagnostics: %s", diagnostic)
			}
			failure := decodeStay(t, diagnostic)["error"].(map[string]any)
			if failure["code"] != "delivery_failure" || !strings.Contains(failure["hint"].(string), "--deliver") {
				t.Fatalf("delivery failure not actionable: %s", diagnostic)
			}
		})
	}
}
