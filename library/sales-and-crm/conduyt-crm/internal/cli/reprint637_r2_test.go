package cli

// Codex reprint637-r2. PATCH(reprint-637-required-on-changed), PATCH(reprint-637-conditional-requirements).

import (
	"encoding/json"
	"strings"
	"testing"
)

func bodyOf(t *testing.T, r recordedRequest) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(r.Body), &m); err != nil {
		t.Fatalf("body %q: %v", r.Body, err)
	}
	return m
}

// A required numeric body field passed as an explicit 0 is sent (0 is a valid value: no folders, Sunday, midnight).
func TestRequiredZeroValuesAreSent(t *testing.T) {
	for _, tc := range []struct {
		args []string
		key  string
	}{
		{[]string{"chat", "put-agents-me-knowledge", "--configured=false", "--folders=0", "--listener-id", "L1"}, "folders"},
		{[]string{"rep-shifts", "post", "--user-id", "u1", "--day-of-week", "0", "--start-minute", "0", "--end-minute", "60"}, "dayOfWeek"},
		{[]string{"rep-shifts", "post", "--user-id", "u1", "--day-of-week", "1", "--start-minute", "0", "--end-minute", "60"}, "startMinute"},
		{[]string{"notifications", "update-digest-settings", "--enabled=true", "--frequency", "daily", "--hour", "0"}, "hour"},
	} {
		srv, reqs := recordingServer(t)
		if err := runCLI(t, srv.URL, append(tc.args, "--yes")...); err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		got := reqs()
		if len(got) != 1 {
			t.Fatalf("%v sent %d requests", tc.args, len(got))
		}
		body := bodyOf(t, got[0])
		if v, ok := body[tc.key]; !ok || v != float64(0) {
			t.Fatalf("%v: body %s, want %s: 0", tc.args, got[0].Body, tc.key)
		}
	}
}

// Under the global --dry-run a missing required input is not an error: the preview prints and nothing is sent.
// A passed-but-wrong value is still refused.
func TestConditionalRequirementsLetTheDryRunPreviewPrint(t *testing.T) {
	for _, args := range [][]string{
		{"conduyt-auth", "delete-sessions", "--dry-run"},
		{"users", "notifications", "update-user-preferences", "u1", "--dry-run"},
	} {
		srv, reqs := recordingServer(t)
		var runErr error
		errOut := captureStderr(t, func() { runErr = runCLI(t, srv.URL, args...) })
		if runErr != nil {
			t.Errorf("%v: %v, want the preview", args, runErr)
		}
		if n := len(reqs()); n != 0 {
			t.Errorf("%v sent %d request(s)", args, n)
		}
		if !strings.Contains(errOut, "(dry run - no request sent)") {
			t.Errorf("%v printed no preview:\n%s", args, errOut)
		}
	}
	srv, reqs := recordingServer(t)
	if err := runCLI(t, srv.URL, "conduyt-auth", "delete-sessions", "--dry-run", "--scope", "all"); err == nil || ExitCode(err) != 2 || len(reqs()) != 0 {
		t.Errorf("--dry-run --scope all: err %v, want a usage error and no request", err)
	}
}

// An empty --preferences is refused before any request (the serializer would drop it and send {}).
func TestEmptyPreferencesIsRefused(t *testing.T) {
	for _, extra := range [][]string{{"--preferences", ""}, {"--preferences", "  "}, {"--preferences=", "--dry-run"}} {
		srv, reqs := recordingServer(t)
		err := runCLI(t, srv.URL, append([]string{"users", "notifications", "update-user-preferences", "u1", "--yes"}, extra...)...)
		if err == nil || ExitCode(err) != 2 || len(reqs()) != 0 {
			t.Errorf("%v: err %v, %d request(s); want a usage error and none", extra, err, len(reqs()))
		}
	}
	srv, reqs := recordingServer(t)
	if err := runCLI(t, srv.URL, "users", "notifications", "update-user-preferences", "u1", "--yes", "--preferences", "", "--sounds-team-chat"); err == nil || len(reqs()) != 0 {
		t.Errorf("empty --preferences beside sounds: err %v, %d request(s); want a refusal", err, len(reqs()))
	}
}
