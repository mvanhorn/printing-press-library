package cli

// Codex reprint637-r1 P2s: the 11-08 reprint renamed two commands and dropped two conditional requirements.
// PATCH(reprint-637-renamed-command-aliases), PATCH(reprint-637-conditional-requirements).

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type recordedRequest struct {
	Method, Path, Query, Body string
}

func recordingServer(t *testing.T) (*httptest.Server, func() []recordedRequest) {
	t.Helper()
	var mu sync.Mutex
	var got []recordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, recordedRequest{r.Method, r.URL.Path, r.URL.RawQuery, string(b)})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []recordedRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]recordedRequest(nil), got...)
	}
}

func runCLI(t *testing.T, srvURL string, args ...string) error {
	t.Helper()
	isolateCLIEnv(t)
	t.Setenv("CONDUYT_CRM_BASE_URL", srvURL)
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "cdy_test_reprint637")
	root := newRootCmd(&rootFlags{})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs(append(args, "--json", "--no-cache"))
	return root.Execute()
}

func TestBareAccountExportsStillListsThem(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		query string
	}{
		{[]string{"account-exports"}, ""},
		{[]string{"account-exports", "--cursor", "c-42"}, "cursor=c-42"},
		{[]string{"account-exports", "get", "--cursor", "c-42"}, "cursor=c-42"},
	} {
		srv, reqs := recordingServer(t)
		if err := runCLI(t, srv.URL, tc.args...); err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		got := reqs()
		if len(got) == 0 || got[0].Method != "GET" || !strings.HasSuffix(got[0].Path, "/account-exports") || !strings.Contains(got[0].Query, tc.query) {
			t.Fatalf("%v sent %+v, want GET /account-exports?%s", tc.args, got, tc.query)
		}
	}
}

func TestRenamedDncCommandKeepsItsOldName(t *testing.T) {
	root := newRootCmd(&rootFlags{})
	oldCmd, _, err := root.Find([]string{"contacts", "dnc", "get-contacts-id"})
	if err != nil {
		t.Fatal(err)
	}
	newCmd, _, _ := root.Find([]string{"contacts", "dnc", "get-contact-verdicts"})
	if oldCmd != newCmd || newCmd.Annotations["pp:path"] != "/contacts/{id}/dnc" {
		t.Fatalf("contacts dnc get-contacts-id resolves to %q, want get-contact-verdicts", oldCmd.CommandPath())
	}
}

func TestDeleteSessionsNeedsExactlyOneTarget(t *testing.T) {
	for _, bad := range [][]string{
		{"conduyt-auth", "delete-sessions", "--scope", "all"},
		{"conduyt-auth", "delete-sessions", "--id", "s1", "--scope", "others"},
		{"conduyt-auth", "delete-sessions", "--id", ""},
		{"conduyt-auth", "delete-sessions", "--yes"},
	} {
		srv, reqs := recordingServer(t)
		err := runCLI(t, srv.URL, bad...)
		if err == nil || ExitCode(err) != 2 {
			t.Errorf("%v: err %v (exit %d), want a usage error", bad, err, ExitCode(err))
		}
		if n := len(reqs()); n != 0 {
			t.Errorf("%v sent %d request(s), want none", bad, n)
		}
	}
	for _, good := range []struct {
		args  []string
		query string
	}{
		{[]string{"conduyt-auth", "delete-sessions", "--id", "s1"}, "id=s1"},
		{[]string{"conduyt-auth", "delete-sessions", "--scope", "others"}, "scope=others"},
	} {
		srv, reqs := recordingServer(t)
		if err := runCLI(t, srv.URL, append(good.args, "--yes")...); err != nil {
			t.Fatalf("%v: %v", good.args, err)
		}
		got := reqs()
		if len(got) != 1 || got[0].Method != "DELETE" || got[0].Query != good.query {
			t.Fatalf("%v sent %+v, want DELETE ?%s", good.args, got, good.query)
		}
	}
}

func TestUpdateUserPreferencesNeedsPreferencesOrSounds(t *testing.T) {
	srv, reqs := recordingServer(t)
	err := runCLI(t, srv.URL, "users", "notifications", "update-user-preferences", "u1", "--yes")
	if err == nil || ExitCode(err) != 2 || len(reqs()) != 0 {
		t.Fatalf("no body flag: err %v, %d request(s); want a usage error and none", err, len(reqs()))
	}
	for _, tc := range []struct {
		args []string
		want map[string]any
	}{
		{[]string{"--sounds-team-chat=false"}, map[string]any{"sounds": map[string]any{"teamChat": false}}},
		{[]string{"--preferences", `{"deal.won":{"email":false}}`}, map[string]any{"preferences": map[string]any{"deal.won": map[string]any{"email": false}}}},
	} {
		srv, reqs := recordingServer(t)
		if err := runCLI(t, srv.URL, append([]string{"users", "notifications", "update-user-preferences", "u1", "--yes"}, tc.args...)...); err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		got := reqs()
		if len(got) != 1 || got[0].Method != "PATCH" {
			t.Fatalf("%v sent %+v", tc.args, got)
		}
		var body map[string]any
		_ = json.Unmarshal([]byte(got[0].Body), &body)
		wb, _ := json.Marshal(tc.want)
		gb, _ := json.Marshal(body)
		if string(wb) != string(gb) {
			t.Fatalf("%v body %s, want %s", tc.args, gb, wb)
		}
	}
}
