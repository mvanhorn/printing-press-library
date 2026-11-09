package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Go CLI #324: `email send` and `email send-bulk` send the live API's key names (to, subject, html, text, contactId ...), not
// the old body/bodyHtml/scheduledAt. The tests pin the exact JSON body a sample invocation produces, and that the --stdin
// pass-through refuses a key the endpoint does not declare before any request is made.
const (
	esContact  = "6f1c0000-0000-4000-8000-0000000000a1"
	esContact2 = "6f1c0000-0000-4000-8000-0000000000a2"
	esTemplate = "6f1c0000-0000-4000-8000-0000000000a3"
	esDomain   = "6f1c0000-0000-4000-8000-0000000000a4"
)

// runEmailSend runs the command against a fake API with stdin fed through cmd.SetIn. It returns the decoded JSON body of the one
// request, the number of requests and the command error.
func runEmailSend(t *testing.T, newCmd func(*rootFlags) *cobra.Command, wantPath, stdin string, args ...string) (body map[string]any, hits int, err error) {
	return runEmailSendWith(t, newCmd, &rootFlags{noCache: true}, wantPath, stdin, args...)
}

func runEmailSendWith(t *testing.T, newCmd func(*rootFlags) *cobra.Command, rf *rootFlags, wantPath, stdin string, args ...string) (body map[string]any, hits int, err error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != "POST" || r.URL.Path != "/api/v1"+wantPath {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		body = map[string]any{}
		if e := json.Unmarshal(raw, &body); e != nil {
			t.Errorf("body is not JSON: %q", raw)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	cmd := newCmd(rf)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)
	err = cmd.Execute()
	return
}

func TestEmailSendBodyUsesTheLiveKeyNames(t *testing.T) {
	body, hits, err := runEmailSend(t, newEmailSendCmd, "/email/send", "",
		"--contact-id", esContact, "--to", "lead@example.com", "--subject", "Following up", "--html", "<p>Hi</p>", "--text", "Hi",
		"--reply-to", "reps@example.com", "--send-domain-id", esDomain)
	if err != nil || hits != 1 {
		t.Fatalf("hits=%d err=%v", hits, err)
	}
	want := map[string]any{"contactId": esContact, "to": "lead@example.com", "subject": "Following up", "html": "<p>Hi</p>", "text": "Hi",
		"replyTo": "reps@example.com", "sendDomainId": esDomain}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("body = %v, want exactly %v", body, want)
	}
	for _, old := range []string{"body", "bodyHtml", "scheduledAt"} {
		if _, ok := body[old]; ok {
			t.Errorf("old key %q was sent", old)
		}
	}
}

func TestEmailSendMinimalBodySendsOnlyWhatWasGiven(t *testing.T) {
	body, hits, err := runEmailSend(t, newEmailSendCmd, "/email/send", "",
		"--contact-id", esContact, "--to", "lead@example.com", "--subject", "Hi", "--text", "Plain")
	want := map[string]any{"contactId": esContact, "to": "lead@example.com", "subject": "Hi", "text": "Plain"}
	if err != nil || hits != 1 || !reflect.DeepEqual(body, want) {
		t.Errorf("hits=%d err=%v body=%v, want %v", hits, err, body, want)
	}
}

// Everything the CLI would build must satisfy the spec before it leaves: no body, a bad address, a blank subject or a
// non-UUID contact never reach the API.
func TestEmailSendRefusesABodyTheSpecWouldRefuse(t *testing.T) {
	ok := []string{"--contact-id", esContact, "--to", "lead@example.com", "--subject", "Hi", "--html", "<p>x</p>"}
	with := func(flag, val string) []string {
		out := append([]string{}, ok...)
		for i := 0; i < len(out); i += 2 {
			if out[i] == flag {
				out[i+1] = val
			}
		}
		return out
	}
	cases := map[string][]string{
		"no body":       {"--contact-id", esContact, "--to", "lead@example.com", "--subject", "Hi"},
		"empty body":    append(append([]string{}, ok[:6]...), "--html", ""),
		"no at sign":    with("--to", "lead.example.com"),
		"blank subject": with("--subject", "   "),
		"bad contact":   with("--contact-id", "not-a-uuid"),
		"no to":         {"--contact-id", esContact, "--subject", "Hi", "--html", "x"},
		"old body flag": {"--contact-id", esContact, "--to", "lead@example.com", "--subject", "Hi", "--body", "x"},
	}
	for name, args := range cases {
		_, hits, err := runEmailSend(t, newEmailSendCmd, "/email/send", "", args...)
		if err == nil || hits != 0 {
			t.Errorf("%s: hits=%d err=%v, want a local refusal", name, hits, err)
		}
	}
}

func TestEmailSendBulkBodyUsesTheLiveKeyNames(t *testing.T) {
	body, hits, err := runEmailSend(t, newEmailSendBulkCmd, "/email/send-bulk", "",
		"--contact-ids", esContact+","+esContact2, "--subject", "Update", "--html", "<p>Hi</p>", "--text", "Hi", "--template-id", esTemplate,
		"--subject-edited=false", "--reply-to", "reps@example.com", "--send-domain-id", esDomain)
	if err != nil || hits != 1 {
		t.Fatalf("hits=%d err=%v", hits, err)
	}
	want := map[string]any{"contactIds": []any{esContact, esContact2}, "subject": "Update", "html": "<p>Hi</p>", "text": "Hi",
		"templateId": esTemplate, "subjectEdited": false, "replyTo": "reps@example.com", "sendDomainId": esDomain}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("body = %v, want exactly %v", body, want)
	}
	for _, old := range []string{"body", "bodyHtml"} {
		if _, ok := body[old]; ok {
			t.Errorf("old key %q was sent", old)
		}
	}
}

func TestEmailSendBulkTemplateOnlyBodyAndRefusals(t *testing.T) {
	body, hits, err := runEmailSend(t, newEmailSendBulkCmd, "/email/send-bulk", "", "--contact-ids", esContact, "--template-id", esTemplate)
	want := map[string]any{"contactIds": []any{esContact}, "templateId": esTemplate}
	if err != nil || hits != 1 || !reflect.DeepEqual(body, want) {
		t.Errorf("hits=%d err=%v body=%v, want %v", hits, err, body, want)
	}
	for name, args := range map[string][]string{
		"bad contact id": {"--contact-ids", esContact + ",nope", "--subject", "x", "--html", "y"},
		"bad template":   {"--contact-ids", esContact, "--template-id", "nope"},
		"old body flag":  {"--contact-ids", esContact, "--subject", "x", "--body", "y"},
	} {
		_, hits, err := runEmailSend(t, newEmailSendBulkCmd, "/email/send-bulk", "", args...)
		if err == nil || hits != 0 {
			t.Errorf("%s: hits=%d err=%v, want a local refusal", name, hits, err)
		}
	}
}

func TestEmailSendStdinPassesKnownKeysAndRefusesUnknownOnes(t *testing.T) {
	good := `{"contactId":"` + esContact + `","to":"a@b.co","subject":"S","html":"<p>x</p>","cc":[],"bcc":null}`
	body, hits, err := runEmailSend(t, newEmailSendCmd, "/email/send", good, "--stdin")
	if err != nil || hits != 1 || body["to"] != "a@b.co" || len(body) != 6 {
		t.Fatalf("known keys: hits=%d err=%v body=%v", hits, err, body)
	}
	for _, bad := range []string{
		`{"contactId":"` + esContact + `","subject":"S","body":"old","to":"a@b.co"}`,
		`{"contactId":"` + esContact + `","to":"a@b.co","subject":"S","html":"x","scheduledAt":"2026-10-23T00:00:00Z"}`,
		`{"contactId":"` + esContact + `","to":"a@b.co","subject":"S","html":"x","bodyHtml":"x"}`,
	} {
		_, hits, err := runEmailSend(t, newEmailSendCmd, "/email/send", bad, "--stdin")
		if err == nil || hits != 0 {
			t.Errorf("%s: hits=%d err=%v, want a local refusal", bad, hits, err)
			continue
		}
		if !strings.Contains(err.Error(), "accepted keys: bcc, cc, contactId, html, replyTo, sendDomainId, subject, text, to") || !strings.Contains(err.Error(), "nothing was sent") {
			t.Errorf("refusal does not list the accepted keys: %v", err)
		}
	}
}

func TestEmailSendBulkStdinPassesKnownKeysAndRefusesUnknownOnes(t *testing.T) {
	good := `{"contactIds":["` + esContact + `"],"templateId":"` + esTemplate + `","subjectEdited":true,"batchDelayMs":250}`
	body, hits, err := runEmailSend(t, newEmailSendBulkCmd, "/email/send-bulk", good, "--stdin")
	if err != nil || hits != 1 || len(body) != 4 {
		t.Fatalf("known keys: hits=%d err=%v body=%v", hits, err, body)
	}
	_, hits, err = runEmailSend(t, newEmailSendBulkCmd, "/email/send-bulk", `{"contactIds":["`+esContact+`"],"subject":"S","body":"old"}`, "--stdin")
	if err == nil || hits != 0 || !strings.Contains(err.Error(), `"body"`) || !strings.Contains(err.Error(), "accepted keys: batchDelayMs, contactIds") {
		t.Errorf("hits=%d err=%v, want a local refusal naming body and listing the accepted keys", hits, err)
	}
}

// Go CLI #324 r2: --dry-run validates exactly like a real send and previews only a body that would be posted. A dry run never
// reaches the server (hits stays 0), so the signal is the error: required and empty values are refused, a valid body is not.
func TestEmailSendDryRunValidatesLikeARealSend(t *testing.T) {
	dry := func() *rootFlags { return &rootFlags{noCache: true, dryRun: true} }
	run := func(newCmd func(*rootFlags) *cobra.Command, path string, args ...string) (int, error) {
		_, hits, err := runEmailSendWith(t, newCmd, dry(), path, "", args...)
		return hits, err
	}
	sendBad := map[string][]string{
		"no contact":          {"--to", "a@b.co", "--subject", "S", "--html", "x"},
		"no to":               {"--contact-id", esContact, "--subject", "S", "--html", "x"},
		"no subject":          {"--contact-id", esContact, "--to", "a@b.co", "--html", "x"},
		"no body":             {"--contact-id", esContact, "--to", "a@b.co", "--subject", "S"},
		"empty body":          {"--contact-id", esContact, "--to", "a@b.co", "--subject", "S", "--html", "", "--text", ""},
		"empty to":            {"--contact-id", esContact, "--to", "", "--subject", "S", "--html", "x"},
		"empty subject":       {"--contact-id", esContact, "--to", "a@b.co", "--subject", "", "--html", "x"},
		"empty contact":       {"--contact-id", "", "--to", "a@b.co", "--subject", "S", "--html", "x"},
		"subject past limit":  {"--contact-id", esContact, "--to", "a@b.co", "--subject", strings.Repeat("s", 999), "--html", "x"},
		"bad send domain":     {"--contact-id", esContact, "--to", "a@b.co", "--subject", "S", "--html", "x", "--send-domain-id", "nope"},
		"reply-to past limit": {"--contact-id", esContact, "--to", "a@b.co", "--subject", "S", "--html", "x", "--reply-to", strings.Repeat("r", 255)},
	}
	for name, args := range sendBad {
		if hits, err := run(newEmailSendCmd, "/email/send", args...); err == nil || hits != 0 {
			t.Errorf("send %s: hits=%d err=%v, want a local refusal under --dry-run", name, hits, err)
		}
	}
	if hits, err := run(newEmailSendCmd, "/email/send", "--contact-id", esContact, "--to", "a@b.co", "--subject", "S", "--text", "x"); err != nil || hits != 0 {
		t.Errorf("valid send under --dry-run: hits=%d err=%v", hits, err)
	}
	bulkBad := map[string][]string{
		"no contact ids":  {"--subject", "S", "--html", "x"},
		"bad contact id":  {"--contact-ids", "nope", "--subject", "S", "--html", "x"},
		"bad template id": {"--contact-ids", esContact, "--template-id", "nope"},
	}
	for name, args := range bulkBad {
		if hits, err := run(newEmailSendBulkCmd, "/email/send-bulk", args...); err == nil || hits != 0 {
			t.Errorf("send-bulk %s: hits=%d err=%v, want a local refusal under --dry-run", name, hits, err)
		}
	}
	if hits, err := run(newEmailSendBulkCmd, "/email/send-bulk", "--contact-ids", esContact, "--template-id", esTemplate); err != nil || hits != 0 {
		t.Errorf("valid send-bulk under --dry-run: hits=%d err=%v", hits, err)
	}
}

// The limits of the live spec hold on the flag path too (the unit test pins each one to the spec).
func TestEmailSendFlagsRefuseValuesPastTheSpecLimits(t *testing.T) {
	ok := []string{"--contact-id", esContact, "--to", "a@b.co", "--subject", "S", "--html", "x"}
	for name, extra := range map[string][]string{
		"reply-to":       {"--reply-to", strings.Repeat("r", 255)},
		"send-domain-id": {"--send-domain-id", "nope"},
	} {
		_, hits, err := runEmailSend(t, newEmailSendCmd, "/email/send", "", append(append([]string{}, ok...), extra...)...)
		if err == nil || hits != 0 {
			t.Errorf("%s: hits=%d err=%v, want a local refusal", name, hits, err)
		}
	}
	_, hits, err := runEmailSend(t, newEmailSendCmd, "/email/send", "", "--contact-id", esContact, "--to", "a@b.co", "--subject", strings.Repeat("s", 999), "--html", "x")
	if err == nil || hits != 0 {
		t.Errorf("subject past 998: hits=%d err=%v", hits, err)
	}
}
