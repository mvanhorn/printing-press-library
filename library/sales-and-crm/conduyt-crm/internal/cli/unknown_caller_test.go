package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Dialer unknown-caller prompt (board #35, app main f132ed6f0, 2026-10-15): POST /calls/{id}/link, GET /calls/unknown-callers (the
// toll-free counter), GET /dialer/history?unmatched=&line= and POST /contacts with callId. spec.yaml carries all four from the public OpenAPI.
type seenReq struct {
	method string
	path   string
	query  url.Values
	body   map[string]any
}

func runUnknownCaller(t *testing.T, build func(*rootFlags) *cobra.Command, args ...string) (reqs []seenReq, err error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		rq := seenReq{method: r.Method, path: r.URL.Path, query: r.URL.Query()}
		if len(raw) > 0 {
			rq.body = map[string]any{}
			_ = json.Unmarshal(raw, &rq.body)
		}
		reqs = append(reqs, rq)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"total":14,"unmatched":9}}`))
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	cmd := build(&rootFlags{noCache: true})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return
}

const (
	ucCall    = "9b2d7c1a-5e4f-4a3b-9c8d-7e6f5a4b3c2d"
	ucContact = "3f8a0c4e-1111-4222-8333-444455556666"
)

func TestCallsCountUnknownCallersSendsLineAndSince(t *testing.T) {
	reqs, err := runUnknownCaller(t, newCallsCountUnknownCallersCmd, "--line", "+18005550199", "--since", "2026-10-15T07:00:00.000Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].method != "GET" || reqs[0].path != "/api/v1/calls/unknown-callers" {
		t.Fatalf("requests = %+v", reqs)
	}
	if reqs[0].query.Get("line") != "+18005550199" || reqs[0].query.Get("since") != "2026-10-15T07:00:00.000Z" || len(reqs[0].query) != 2 {
		t.Errorf("query = %v", reqs[0].query)
	}
}

func TestCallsCountUnknownCallersRequiresBothFlags(t *testing.T) {
	for _, args := range [][]string{{"--line", "+18005550199"}, {"--since", "2026-10-15T07:00:00.000Z"}} {
		reqs, err := runUnknownCaller(t, newCallsCountUnknownCallersCmd, args...)
		if err == nil || !strings.Contains(err.Error(), "required flag") {
			t.Errorf("%v: err = %v, want a required-flag error", args, err)
		}
		if len(reqs) != 0 {
			t.Errorf("%v: %d requests, want none", args, len(reqs))
		}
	}
}

func TestCallsLinkCallToContactBodyShapes(t *testing.T) {
	cases := []struct {
		flags []string
		want  map[string]any
	}{
		{nil, map[string]any{"contactId": ucContact}},
		{[]string{"--attach-number"}, map[string]any{"contactId": ucContact, "attachNumber": true}},
		{[]string{"--attach-number", "--server-dry-run"}, map[string]any{"contactId": ucContact, "attachNumber": true, "dryRun": true}},
	}
	for _, c := range cases {
		args := append([]string{ucCall, "--contact-id", ucContact}, c.flags...)
		reqs, err := runUnknownCaller(t, newCallsLinkCallToContactCmd, args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if len(reqs) != 1 || reqs[0].method != "POST" || reqs[0].path != "/api/v1/calls/"+ucCall+"/link" {
			t.Fatalf("%v: requests = %+v", args, reqs)
		}
		if len(reqs[0].body) != len(c.want) {
			t.Errorf("%v: body = %v, want %v", args, reqs[0].body, c.want)
		}
		for k, v := range c.want {
			if reqs[0].body[k] != v {
				t.Errorf("%v: body[%s] = %v, want %v", args, k, reqs[0].body[k], v)
			}
		}
	}
}

func TestCallsLinkCallToContactRequiresContact(t *testing.T) {
	reqs, err := runUnknownCaller(t, newCallsLinkCallToContactCmd, ucCall, "--attach-number")
	if err == nil || !strings.Contains(err.Error(), "contact-id") {
		t.Errorf("err = %v, want a contact-id required error", err)
	}
	if len(reqs) != 0 {
		t.Errorf("%d requests, want none", len(reqs))
	}
}

func TestContactsCreateCallIdRidesInTheBody(t *testing.T) {
	reqs, err := runUnknownCaller(t, newContactsCreateCmd, "--first-name", "Carla", "--call-id", ucCall)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].method != "POST" || reqs[0].path != "/api/v1/contacts" {
		t.Fatalf("requests = %+v", reqs)
	}
	b := reqs[0].body
	if len(b) != 2 || b["firstName"] != "Carla" || b["callId"] != ucCall {
		t.Errorf("body = %v, want exactly firstName and callId (no source, no owner: the server derives both)", b)
	}
}

func TestDialerGetHistoryUnmatchedAndLine(t *testing.T) {
	reqs, err := runUnknownCaller(t, newDialerGetHistoryCmd, "--unmatched", "--line", "+18005550199")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].path != "/api/v1/dialer/history" || reqs[0].query.Get("unmatched") != "1" || reqs[0].query.Get("line") != "+18005550199" {
		t.Fatalf("requests = %+v", reqs)
	}
	reqs, err = runUnknownCaller(t, newDialerGetHistoryCmd)
	if err != nil {
		t.Fatal(err)
	}
	// gocli1219: under 4.34.2 a bare read sends only the flags passed (retainCLIQueryParams), so the bare history read
	// carries no query at all; it never sends unmatched or line
	q := reqs[0].query
	if q.Has("unmatched") || q.Has("line") || len(q) != 0 {
		t.Errorf("bare history sent %v, want no query parameters", q)
	}
}

func TestUnknownCallerCommandsAreRegistered(t *testing.T) {
	root := RootCmd()
	for _, path := range [][]string{{"calls", "count-unknown-callers"}, {"calls", "link", "call-to-contact"}} {
		if c, _, err := root.Find(path); err != nil || c == nil || c.Name() != path[len(path)-1] {
			t.Errorf("%v not registered (%v)", path, err)
		}
	}
}

func TestDialerGetHistoryUnmatchedIsABooleanSwitch(t *testing.T) {
	// the bare invocation promised in the parity description: `dialer get-history --unmatched --line <n>`, and --unmatched last
	for _, args := range [][]string{{"--unmatched", "--line", "+18005550199"}, {"--line", "+18005550199", "--unmatched"}} {
		reqs, err := runUnknownCaller(t, newDialerGetHistoryCmd, args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if len(reqs) != 1 || reqs[0].query.Get("unmatched") != "1" || reqs[0].query.Get("line") != "+18005550199" {
			t.Fatalf("%v: requests = %+v", args, reqs)
		}
	}
	reqs, err := runUnknownCaller(t, newDialerGetHistoryCmd, "--unmatched=false")
	if err != nil || reqs[0].query.Has("unmatched") {
		t.Errorf("--unmatched=false sent %v (%v), want no unmatched", reqs, err)
	}
}

func TestContactsCreateCallIdRefusesSourceAndOwner(t *testing.T) {
	owner := "5a5a5a5a-1111-4222-8333-444455556666"
	for _, args := range [][]string{
		{"--first-name", "Carla", "--call-id", ucCall, "--source", "web"},
		{"--first-name", "Carla", "--call-id", ucCall, "--assigned-to", owner},
		{"--call-id", ucCall, "--source", "web", "--assigned-to", owner},
	} {
		reqs, err := runUnknownCaller(t, newContactsCreateCmd, args...)
		if err == nil || !strings.Contains(err.Error(), "--call-id cannot be combined with source or assignedTo") {
			t.Errorf("%v: err = %v, want the refusal", args, err)
		}
		if len(reqs) != 0 {
			t.Errorf("%v: %d requests, want none", args, len(reqs))
		}
	}
	// without --call-id both are still accepted
	reqs, err := runUnknownCaller(t, newContactsCreateCmd, "--first-name", "Carla", "--source", "web", "--assigned-to", owner)
	if err != nil || len(reqs) != 1 || reqs[0].body["source"] != "web" || reqs[0].body["assignedTo"] != owner {
		t.Errorf("no --call-id: err %v, requests %+v", err, reqs)
	}
}

func TestContactsCreateCallIdRefusesSourceAndOwnerFromStdin(t *testing.T) {
	for _, in := range []string{
		`{"firstName":"Carla","callId":"` + ucCall + `","source":"web"}`,
		`{"firstName":"Carla","callId":"` + ucCall + `","assignedTo":"u1"}`,
	} {
		old := os.Stdin
		r, w, _ := os.Pipe()
		_, _ = w.WriteString(in)
		_ = w.Close()
		os.Stdin = r
		reqs, err := runUnknownCaller(t, newContactsCreateCmd, "--stdin")
		os.Stdin = old
		if err == nil || !strings.Contains(err.Error(), "--call-id cannot be combined with source or assignedTo") {
			t.Errorf("%s: err = %v, want the refusal", in, err)
		}
		if len(reqs) != 0 {
			t.Errorf("%s: %d requests, want none", in, len(reqs))
		}
	}
	// stdin without callId keeps both
	old := os.Stdin
	r, w, _ := os.Pipe()
	_, _ = w.WriteString(`{"firstName":"Carla","source":"web","assignedTo":"u1"}`)
	_ = w.Close()
	os.Stdin = r
	reqs, err := runUnknownCaller(t, newContactsCreateCmd, "--stdin")
	os.Stdin = old
	if err != nil || len(reqs) != 1 {
		t.Errorf("stdin without callId: err %v, requests %+v", err, reqs)
	}
}

func TestCallsCountUnknownCallersSinceMustBeRFC3339WithOffset(t *testing.T) {
	for _, good := range []string{"2026-10-15T07:00:00Z", "2026-10-15T07:00:00.123Z", "2026-10-15T00:00:00-07:00", "2026-10-15T00:00:00+05:30"} {
		reqs, err := runUnknownCaller(t, newCallsCountUnknownCallersCmd, "--line", "+18005550199", "--since", good)
		if err != nil || len(reqs) != 1 || reqs[0].query.Get("since") != good {
			t.Errorf("%s: err %v, requests %+v", good, err, reqs)
		}
	}
	for _, bad := range []string{"2026-10-15T07:00:00", "2026-10-15T07:00:00.000", "2026-10-15", "2026-02-31T00:00:00Z", "2026-13-01T00:00:00Z", "2026-10-15T25:00:00Z", "today", "2026-10-15 07:00:00Z"} {
		reqs, err := runUnknownCaller(t, newCallsCountUnknownCallersCmd, "--line", "+18005550199", "--since", bad)
		if err == nil || !strings.Contains(err.Error(), "RFC 3339") {
			t.Errorf("%s: err = %v, want the RFC 3339 refusal", bad, err)
		}
		if len(reqs) != 0 {
			t.Errorf("%s: %d requests, want none", bad, len(reqs))
		}
	}
}
