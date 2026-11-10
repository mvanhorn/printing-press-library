// PATCH(redirect-and-redaction-policy): tests that secret screen-share codes never reach output envelopes; see .printing-press-patches.json.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/store"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func runCodeCmd(t *testing.T, dry bool, build func(*rootFlags) *cobra.Command, code string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	flags := &rootFlags{asJSON: true, noCache: true, dryRun: dry}
	cmd := build(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{code})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	return out.String()
}

func TestScreenShareCodeEnvelopesHideTheCode(t *testing.T) {
	const code = "ENVELOPE-code-ABC123456"
	cases := map[string]func(*rootFlags) *cobra.Command{
		"public post-screen-share-code":    newPublicPostScreenShareCodeCmd,
		"screen-share delete-session-code": newScreenShareDeleteSessionCodeCmd,
	}
	for name, build := range cases {
		for _, dry := range []bool{false, true} {
			out := runCodeCmd(t, dry, build, code)
			if strings.Contains(out, "ABC123456") {
				t.Errorf("%s dry=%v: code leaked in %s", name, dry, out)
			}
			var env map[string]any
			if err := json.Unmarshal([]byte(out), &env); err != nil {
				t.Fatalf("%s: not JSON: %q", name, out)
			}
			if p, _ := env["path"].(string); !strings.Contains(p, "screen-share") {
				t.Errorf("%s dry=%v: envelope path = %q", name, dry, p)
			}
		}
	}
}

func TestDoctorMasksBaseURLCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	defer srv.Close()
	u := strings.Replace(srv.URL, "http://", "http://user:pw-DOCTOR-1234@", 1) + "/api/v1?access_token=TOK-DOCTOR-9999"
	t.Setenv("CONDUYT_CRM_BASE_URL", u)
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	flags := &rootFlags{asJSON: true, noCache: true}
	cmd := newDoctorCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})
	_ = cmd.Execute()
	for _, leak := range []string{"pw-DOCTOR-1234", "TOK-DOCTOR-9999"} {
		if strings.Contains(out.String(), leak) {
			t.Fatalf("doctor leaked %q: %s", leak, out.String())
		}
	}
	if !strings.Contains(out.String(), "base_url") {
		t.Fatalf("doctor output lost base_url: %s", out.String())
	}
}

func TestLocalLookupDoesNotPrintTheSecretCode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(home, "config.toml"))
	dbPath := defaultDBPath("conduyt-crm-pp-cli")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	flags := &rootFlags{dataSource: "local"}
	_, _, err = resolveLocal(context.Background(), flags, io.Discard, "public", false, "/public/screen-share/LOCAL-code-ABC123456", nil, "user_requested")
	if err == nil {
		t.Skip("local store returned data; nothing to check")
	}
	if strings.Contains(err.Error(), "ABC123456") {
		t.Fatalf("local lookup printed the secret code: %v", err)
	}
	if !strings.Contains(err.Error(), "not found in local store") {
		t.Fatalf("the test did not reach the not-found branch: %v", err)
	}
}

// A connection failure on a sensitive path must still look like a network
// error to the local-data fallback, even though its text is masked.
func TestNetworkClassificationSurvivesRedaction(t *testing.T) {
	// A server that resets the connection (RST) after accepting it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			if tc, ok := conn.(*net.TCPConn); ok {
				_ = tc.SetLinger(0)
			}
			time.Sleep(50 * time.Millisecond)
			_ = conn.Close()
		}
	}()
	t.Setenv("CONDUYT_CRM_BASE_URL", "http://"+ln.Addr().String()+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("PRINTING_PRESS_VERIFY", "1") // retries off: no real backoff sleeps
	flags := &rootFlags{noCache: true}
	c, err := flags.newClient()
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Get(context.Background(), "/public/screen-share/NETCLASS-code-ABC123456", nil)
	if err == nil || strings.Contains(err.Error(), "ABC123456") {
		t.Fatalf("err=%v", err)
	}
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("the structured network class (*net.OpError) must survive redaction: %v", err)
	}
	if !isNetworkError(err) {
		t.Fatalf("isNetworkError must still recognise the masked connection failure: %v", err)
	}
}

func TestDoctorNeverPrintsAnInvalidBaseURLWithSecrets(t *testing.T) {
	t.Setenv("CONDUYT_CRM_BASE_URL", "http://user:pw-BADURL-1234@127.0.0.1:badport/api?access_token=TOK-BADURL-5555")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	for _, asJSON := range []bool{true, false} {
		flags := &rootFlags{asJSON: asJSON, noCache: true}
		cmd := newDoctorCmd(flags)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{})
		_ = cmd.Execute()
		for _, leak := range []string{"pw-BADURL-1234", "TOK-BADURL-5555"} {
			if strings.Contains(out.String(), leak) {
				t.Fatalf("json=%v: doctor leaked %q: %s", asJSON, leak, out.String())
			}
		}
	}
}

func TestDoctorWithdrawsAnOverBudgetInvalidBaseURL(t *testing.T) {
	marker := strings.Repeat("ZQXJ", 20000)
	t.Setenv("CONDUYT_CRM_BASE_URL", "http://127.0.0.1:badport/api?access_token="+marker)
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	for _, asJSON := range []bool{true, false} {
		flags := &rootFlags{asJSON: asJSON, noCache: true}
		cmd := newDoctorCmd(flags)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{})
		_ = cmd.Execute()
		if strings.Contains(out.String(), "ZQXJZQXJ") {
			t.Fatalf("json=%v: doctor printed part of an over-budget token", asJSON)
		}
	}
}

func TestDeliverWebhookErrorsAreMasked(t *testing.T) {
	for _, target := range []string{
		"http://user:pw-HOOK-1234@127.0.0.1:1/hook?access_token=TOK-HOOK-5555",       // transport error
		"http://user:pw-HOOK-1234@127.0.0.1:badport/hook?access_token=TOK-HOOK-5555", // construction error
	} {
		err := Deliver(DeliverSink{Scheme: "webhook", Target: target}, []byte(`{}`), false)
		if err == nil {
			t.Fatalf("%s: want an error", target)
		}
		for _, leak := range []string{"pw-HOOK-1234", "TOK-HOOK-5555"} {
			if strings.Contains(err.Error(), leak) {
				t.Errorf("%s: %q leaked in %v", target, leak, err)
			}
		}
		line := deliverFailureWarning(DeliverSink{Scheme: "webhook", Target: target}, err)
		for _, leak := range []string{"pw-HOOK-1234", "TOK-HOOK-5555"} {
			if strings.Contains(line, leak) {
				t.Errorf("warning line leaked %q: %s", leak, line)
			}
		}
	}
	// File sinks print their path as is.
	if line := deliverFailureWarning(DeliverSink{Scheme: "file", Target: "/tmp/out.json"}, errors.New("boom")); !strings.Contains(line, "/tmp/out.json") {
		t.Errorf("file target must stay visible: %s", line)
	}
}

func TestFeedbackUpstreamIsMasked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	endpoint := strings.Replace(srv.URL, "http://", "http://user:pw-FB-1234@", 1) + "/fb?access_token=TOK-FB-5555"
	t.Setenv("CONDUYT_CRM_FEEDBACK_ENDPOINT", endpoint)
	for _, asJSON := range []bool{true, false} {
		flags := &rootFlags{asJSON: asJSON}
		cmd := newFeedbackCmd(flags)
		var out, errb bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errb)
		cmd.SetIn(strings.NewReader("hello feedback"))
		cmd.SetArgs([]string{"--send", "--stdin"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		for _, leak := range []string{"pw-FB-1234", "TOK-FB-5555"} {
			if strings.Contains(out.String(), leak) || strings.Contains(errb.String(), leak) {
				t.Fatalf("json=%v: success output leaked %q: %s", asJSON, leak, out.String())
			}
		}
	}
	// Failure path: an unreachable endpoint with credentials.
	t.Setenv("CONDUYT_CRM_FEEDBACK_ENDPOINT", "http://user:pw-FB-1234@127.0.0.1:1/fb?access_token=TOK-FB-5555")
	flags := &rootFlags{asJSON: true}
	cmd := newFeedbackCmd(flags)
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetIn(strings.NewReader("hello feedback"))
	cmd.SetArgs([]string{"--send", "--stdin"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"pw-FB-1234", "TOK-FB-5555"} {
		if strings.Contains(out.String(), leak) || strings.Contains(errb.String(), leak) {
			t.Fatalf("failure output leaked %q: out=%s err=%s", leak, out.String(), errb.String())
		}
	}
}

// reprint637: the two public paths that carry a secret path code in the full public document
// (/public/screen-share/{code}/state and /webhooks/inbound/forms/{token}) mask it in the output envelope too.
func TestReprintSecretPathCodeEnvelopesHideTheCode(t *testing.T) {
	cases := map[string]struct {
		build func(*rootFlags) *cobra.Command
		extra []string
		where string
	}{
		"public post-screen-share-code-state": {newPublicPostScreenShareCodeStateCmd, []string{"--state", "x"}, "screen-share"},
		"webhooks inbound-form-submission":    {newWebhooksInboundFormSubmissionCmd, []string{"--email", "a@b.co"}, "inbound/forms"},
	}
	for name, tc := range cases {
		for _, dry := range []bool{false, true} {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"ok":true}`)
			}))
			t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
			t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
			t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
			cmd := tc.build(&rootFlags{asJSON: true, noCache: true, dryRun: dry})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(append([]string{"SECRET-code-XYZ987654"}, tc.extra...))
			err := cmd.Execute()
			srv.Close()
			if err != nil {
				t.Fatalf("%s dry=%v: %v", name, dry, err)
			}
			if strings.Contains(out.String(), "XYZ987654") {
				t.Errorf("%s dry=%v: code leaked in %s", name, dry, out.String())
			}
			var env map[string]any
			if err := json.Unmarshal(out.Bytes(), &env); err != nil {
				t.Fatalf("%s: not JSON: %q", name, out.String())
			}
			if p, _ := env["path"].(string); !strings.Contains(p, tc.where) {
				t.Errorf("%s dry=%v: envelope path = %q", name, dry, p)
			}
		}
	}
}
