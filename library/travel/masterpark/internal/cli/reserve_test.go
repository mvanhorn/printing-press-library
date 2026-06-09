package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// captureStdout runs fn with os.Stdout redirected and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	w.Close()
	os.Stdout = orig
	return <-done
}

func runCmd(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	var err error
	out := captureStdout(t, func() { err = cmd.Execute() })
	return out, err
}

func recordingServer(t *testing.T, hits *int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hits, 1)
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, `window._wpnonce = "nonce";{"errors":[],"data":[]}`)
	}))
	return srv
}

func reserveArgs(extra ...string) []string {
	base := []string{
		"--lot", "B",
		"--dropoff", "2026-06-11 07:00",
		"--pickup", "2026-06-13 18:30",
		"--quote", "1",
		"--first-name", "Alice",
		"--last-name", "Smith",
		"--email", "alice@example.com",
		"--phone", "phone-test",
		"--vehicle-make", "Honda",
		"--vehicle-model", "Civic",
		"--plate", "ABC123",
	}
	return append(base, extra...)
}

func TestReserveDryRunDefault(t *testing.T) {
	var hits int32
	srv := recordingServer(t, &hits)
	defer srv.Close()
	t.Setenv("MASTERPARK_BASE_URL", srv.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "")

	g := &globalOpts{timeout: 5 * time.Second}
	out, err := runCmd(t, newReserveCmd(g), reserveArgs()...)
	if err != nil {
		t.Fatalf("reserve dry-run error: %v", err)
	}
	if !strings.Contains(out, "DRY-RUN") {
		t.Errorf("expected DRY-RUN in output, got: %s", out)
	}
	if atomic.LoadInt32(&hits) != 0 {
		t.Errorf("dry-run must not hit the network, got %d hits", hits)
	}
}

func TestReserveVerifyNoOp(t *testing.T) {
	var hits int32
	srv := recordingServer(t, &hits)
	defer srv.Close()
	t.Setenv("MASTERPARK_BASE_URL", srv.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")

	g := &globalOpts{timeout: 5 * time.Second, json: true}
	out, err := runCmd(t, newReserveCmd(g), reserveArgs("--submit", "--yes")...)
	if err != nil {
		t.Fatalf("reserve verify error: %v", err)
	}
	if !strings.Contains(out, "verify_noop") {
		t.Errorf("expected verify_noop in output, got: %s", out)
	}
	if atomic.LoadInt32(&hits) != 0 {
		t.Errorf("verify mode must not hit live endpoints, got %d hits", hits)
	}
}

func TestReserveSubmitRequiresYes(t *testing.T) {
	g := &globalOpts{timeout: 5 * time.Second}
	_, err := runCmd(t, newReserveCmd(g), reserveArgs("--submit")...)
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Errorf("expected --yes confirmation error, got: %v", err)
	}
}

func TestReserveSubmitMissingFields(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	g := &globalOpts{timeout: 5 * time.Second}
	// Omit customer/vehicle fields.
	args := []string{
		"--lot", "B",
		"--dropoff", "2026-06-11 07:00",
		"--pickup", "2026-06-13 18:30",
		"--submit", "--yes",
	}
	_, err := runCmd(t, newReserveCmd(g), args...)
	if err == nil || !strings.Contains(err.Error(), "missing required fields") {
		t.Errorf("expected missing fields error, got: %v", err)
	}
}
