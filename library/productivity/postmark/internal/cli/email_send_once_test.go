// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil/testenv"
)

func TestNovelEmailSendOnceHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"email", "send-once", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("email send-once --help error = %v", err)
	}
	for _, want := range []string{"email send-once [flags]", "Do NOT use this command when a repeat send is intended"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, out.String())
		}
	}
}

func TestSendOnceDerivedKey(t *testing.T) {
	base := sendOnceDerivedKey("jane@example.com, bob@example.com", "", "", "App <app@example.com>", "Your code", "", "", "", "")
	same := []string{
		sendOnceDerivedKey("Bob@Example.com,jane@example.com", "", "", "app@example.com", " Your code ", "", "", "outbound", ""),
		sendOnceDerivedKey("bob@example.com, jane@example.com", "", "", "APP@example.com", "Your code", "", "", "Outbound", ""),
	}
	for i, k := range same {
		if k != base {
			t.Errorf("variant %d key %s != %s", i, k, base)
		}
	}
	different := []string{
		sendOnceDerivedKey("jane@example.com", "", "", "app@example.com", "Your code", "", "", "", ""),
		sendOnceDerivedKey("jane@example.com, bob@example.com", "", "", "app@example.com", "Your new code", "", "", "", ""),
		sendOnceDerivedKey("jane@example.com, bob@example.com", "", "", "app@example.com", "Your code", "", "", "broadcast", ""),
	}
	for i, k := range different {
		if k == base {
			t.Errorf("different input %d produced the same key", i)
		}
	}
	tmplA := sendOnceDerivedKey("jane@example.com", "", "", "app@example.com", "", "password-reset", `{"b":2,"a":1}`, "", "")
	tmplB := sendOnceDerivedKey("jane@example.com", "", "", "app@example.com", "", "password-reset", `{ "a": 1, "b": 2 }`, "", "")
	if tmplA != tmplB {
		t.Error("model key order should not change the key")
	}
	withCc := sendOnceDerivedKey("jane@example.com, bob@example.com", "boss@example.com", "", "App <app@example.com>", "Your code", "", "", "", "")
	withBcc := sendOnceDerivedKey("jane@example.com, bob@example.com", "", "boss@example.com", "App <app@example.com>", "Your code", "", "", "", "")
	if withCc == base || withBcc == base || withCc == withBcc {
		t.Error("adding a Cc or Bcc recipient must change the key, and Cc and Bcc must not collide")
	}
	if !strings.HasPrefix(base, "sha256-") || len(base) > sendOnceMaxKeyLen {
		t.Errorf("key shape = %q", base)
	}
}

func TestSendOncePayload(t *testing.T) {
	path, p, err := sendOncePayload(sendOnceInput{from: "a@x.co", to: "b@y.co", template: "42", model: `{"name":"Jane"}`, stream: "outbound", metadata: map[string]string{"order": "7"}}, "k1")
	if err != nil || path != "/email/withTemplate" || p["TemplateId"] != int64(42) {
		t.Fatalf("numeric template: %s %v %v", path, p, err)
	}
	meta := p["Metadata"].(map[string]string)
	if meta[sendOnceMetadataKey] != "k1" || meta["order"] != "7" {
		t.Fatalf("metadata = %v", meta)
	}
	path, p, err = sendOncePayload(sendOnceInput{from: "a@x.co", to: "b@y.co", template: "password-reset", stream: "outbound"}, "k2")
	if err != nil || path != "/email/withTemplate" || p["TemplateAlias"] != "password-reset" {
		t.Fatalf("alias template: %s %v %v", path, p, err)
	}
	path, p, err = sendOncePayload(sendOnceInput{from: "a@x.co", to: "b@y.co", subject: "Hi", text: "body", stream: "outbound"}, "k3")
	if err != nil || path != "/email" || p["Subject"] != "Hi" || p["TextBody"] != "body" {
		t.Fatalf("plain: %s %v %v", path, p, err)
	}
	if _, _, err := sendOncePayload(sendOnceInput{template: "t", model: "[1]"}, "k"); err == nil {
		t.Fatal("non-object model should fail")
	}
}

func TestSendOnceParseMetadata(t *testing.T) {
	m, err := sendOnceParseMetadata([]string{"order=7", "note=a=b"})
	if err != nil || m["order"] != "7" || m["note"] != "a=b" {
		t.Fatalf("metadata = %v, %v", m, err)
	}
	if _, err := sendOnceParseMetadata([]string{"pp_idempotency_key=x"}); err == nil {
		t.Fatal("reserved key accepted")
	}
	if _, err := sendOnceParseMetadata([]string{"novalue"}); err == nil {
		t.Fatal("pair without = accepted")
	}
}

// sendOnceFake wires /servers for --server resolution plus an empty outbound
// search and a successful /email.
func sendOnceFake(t *testing.T) *postmarkFake {
	f := newPostmarkFake(t)
	f.reply("GET /servers", 200, postmarkServersPayload([3]any{1, "Main App", "tok-main"}))
	f.reply("GET /server", 200, map[string]any{"ID": 1, "Name": "Main App"})
	f.reply("GET /messages/outbound", 200, map[string]any{"TotalCount": 0, "Messages": []any{}})
	f.reply("POST /email", 200, map[string]any{"To": "jane@example.com", "SubmittedAt": "2026-09-30T12:00:00Z", "MessageID": "msg-1", "ErrorCode": 0, "Message": "OK"})
	return f
}

func countRequests(reqs []postmarkFakeRequest, method, path string) int {
	n := 0
	for _, r := range reqs {
		if r.Method == method && r.Path == path {
			n++
		}
	}
	return n
}

var sendOnceArgs = []string{"email", "send-once", "--key", "otp-4821", "--server", "Main App", "--from", "app@example.com", "--to", "jane@example.com", "--subject", "Your code", "--text", "Code: 4821", "--json"}

func TestSendOncePlanSendThenDuplicate(t *testing.T) {
	f := sendOnceFake(t)
	home := postmarkTestEnv(t, f)
	db := filepath.Join(home, "ledger.db")

	// Plan: searches Postmark, never posts.
	stdout, stderr, err := postmarkRun(t, append(sendOnceArgs, "--db", db)...)
	if err != nil {
		t.Fatalf("plan: %v\n%s", err, stderr)
	}
	var plan sendOnceResult
	postmarkResults(t, stdout, &plan)
	if !plan.WouldSend || plan.Sent || plan.Duplicate || plan.Dedupe.Postmark != "miss" {
		t.Fatalf("plan result = %+v", plan)
	}
	reqs := f.log()
	if countRequests(reqs, "POST", "/email") != 0 {
		t.Fatal("plan mode posted a send")
	}
	var search postmarkFakeRequest
	for _, r := range reqs {
		if r.Path == "/messages/outbound" {
			search = r
		}
	}
	if !strings.Contains(search.Query, "metadata_pp_idempotency_key=otp-4821") || !strings.Contains(search.Query, "recipient=jane%40example.com") || search.ServerToken != "tok-main" {
		t.Fatalf("outbound search = %+v", search)
	}

	// Send: posts once with the key in Metadata and records the ledger.
	stdout, stderr, err = postmarkRun(t, append(sendOnceArgs, "--db", db, "--send")...)
	if err != nil {
		t.Fatalf("send: %v\n%s", err, stderr)
	}
	var sent sendOnceResult
	postmarkResults(t, stdout, &sent)
	if !sent.Sent || sent.MessageID != "msg-1" || sent.Duplicate {
		t.Fatalf("send result = %+v", sent)
	}
	var body map[string]any
	for _, r := range f.log() {
		if r.Method == "POST" && r.Path == "/email" {
			_ = json.Unmarshal([]byte(r.Body), &body)
			if r.AccountTok != "" {
				t.Fatal("send carried the account token")
			}
		}
	}
	if meta, _ := body["Metadata"].(map[string]any); meta[sendOnceMetadataKey] != "otp-4821" {
		t.Fatalf("send body metadata = %v", body)
	}

	// Retry: answered by the ledger without any send or search.
	before := len(f.log())
	stdout, stderr, err = postmarkRun(t, append(sendOnceArgs, "--db", db, "--send")...)
	if err != nil {
		t.Fatalf("retry: %v\n%s", err, stderr)
	}
	var dup sendOnceResult
	postmarkResults(t, stdout, &dup)
	if !dup.Duplicate || dup.Source != sendOnceSourceLedger || dup.MessageID != "msg-1" || dup.Sent {
		t.Fatalf("retry result = %+v", dup)
	}
	after := f.log()[before:]
	if countRequests(after, "POST", "/email") != 0 || countRequests(after, "GET", "/messages/outbound") != 0 {
		t.Fatalf("retry reached Postmark: %+v", after)
	}
}

func TestSendOnceRemoteDuplicate(t *testing.T) {
	f := sendOnceFake(t)
	home := postmarkTestEnv(t, f)
	f.reply("GET /messages/outbound", 200, map[string]any{"TotalCount": 2, "Messages": []any{
		map[string]any{"MessageID": "other", "ReceivedAt": "2026-09-30T08:00:00-04:00", "Metadata": map[string]string{"pp_idempotency_key": "different"}},
		map[string]any{"MessageID": "prior-7", "ReceivedAt": "2026-09-30T08:01:00-04:00", "Metadata": map[string]string{"pp_idempotency_key": "otp-4821"}},
	}})
	stdout, stderr, err := postmarkRun(t, append(sendOnceArgs, "--db", filepath.Join(home, "l.db"), "--send")...)
	if err != nil {
		t.Fatalf("send-once: %v\n%s", err, stderr)
	}
	var res sendOnceResult
	postmarkResults(t, stdout, &res)
	if !res.Duplicate || res.Source != sendOnceSourcePostmark || res.MessageID != "prior-7" {
		t.Fatalf("result = %+v", res)
	}
	if countRequests(f.log(), "POST", "/email") != 0 {
		t.Fatal("remote duplicate still sent")
	}
}

func TestSendOnceIgnoresUnmatchedMetadata(t *testing.T) {
	f := sendOnceFake(t)
	home := postmarkTestEnv(t, f)
	// A search that ignored the metadata filter must not count as a duplicate.
	f.reply("GET /messages/outbound", 200, map[string]any{"TotalCount": 1, "Messages": []any{
		map[string]any{"MessageID": "unrelated", "Metadata": map[string]string{}},
	}})
	stdout, _, err := postmarkRun(t, append(sendOnceArgs, "--db", filepath.Join(home, "l.db"))...)
	if err != nil {
		t.Fatal(err)
	}
	var res sendOnceResult
	postmarkResults(t, stdout, &res)
	if res.Duplicate || !res.WouldSend {
		t.Fatalf("unrelated message treated as duplicate: %+v", res)
	}
}

func TestSendOnceInactiveRecipient(t *testing.T) {
	f := sendOnceFake(t)
	home := postmarkTestEnv(t, f)
	f.reply("POST /email", 422, `{"ErrorCode":406,"Message":"You tried to send to recipient(s) that have been marked as inactive."}`)
	stdout, _, err := postmarkRun(t, append(sendOnceArgs, "--db", filepath.Join(home, "l.db"), "--send")...)
	if err == nil || ExitCode(err) != 5 {
		t.Fatalf("want API error exit 5, got %v", err)
	}
	var serr sendOnceError
	postmarkResults(t, stdout, &serr)
	if serr.ErrorCode != 406 || serr.Error != "inactive_recipient" || !strings.Contains(serr.Next, `postmark-pp-cli diagnose jane@example.com --server 'Main App'`) {
		t.Fatalf("structured error = %+v", serr)
	}
}

func TestSendOnceRefusesUnderHarness(t *testing.T) {
	f := sendOnceFake(t)
	home := postmarkTestEnv(t, f)
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
	stdout, _, err := postmarkRun(t, append(sendOnceArgs, "--db", filepath.Join(home, "l.db"), "--send")...)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"refused":true`) {
		t.Fatalf("expected harness refusal, got %s", stdout)
	}
	if len(f.log()) != 0 {
		t.Fatalf("harness refusal still made requests: %+v", f.log())
	}
}

func TestSendOnceRequiresRecipient(t *testing.T) {
	f := sendOnceFake(t)
	postmarkTestEnv(t, f)
	_, _, err := postmarkRun(t, "email", "send-once", "--from", "a@x.co", "--subject", "s", "--text", "t", "--json")
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("missing --to should exit 2, got %v", err)
	}
}

func TestSendOnceDerivedKeyIncludesBody(t *testing.T) {
	a := sendOnceDerivedKey("jane@example.com", "", "", "app@example.com", "Your code", "", "", "", "Code: 1111\x00")
	b := sendOnceDerivedKey("jane@example.com", "", "", "app@example.com", "Your code", "", "", "", "Code: 2222\x00")
	if a == b {
		t.Fatalf("different bodies produced the same key %s", a)
	}
}

func TestSendOnceFailureDisposition(t *testing.T) {
	t.Run("server error keeps the key reserved", func(t *testing.T) {
		f := sendOnceFake(t)
		home := postmarkTestEnv(t, f)
		db := filepath.Join(home, "ledger.db")
		f.reply("POST /email", 503, map[string]any{"ErrorCode": 0, "Message": "unavailable"})

		_, stderr, err := postmarkRun(t, append(sendOnceArgs, "--db", db, "--send")...)
		if err == nil {
			t.Fatal("a 503 send must fail")
		}
		if !strings.Contains(stderr, "outcome is unknown") {
			t.Errorf("stderr should say the outcome is unknown: %q", stderr)
		}
		before := countRequests(f.log(), "POST", "/email")

		f.reply("POST /email", 200, map[string]any{"To": "jane@example.com", "MessageID": "msg-2", "ErrorCode": 0})
		stdout, stderr, err := postmarkRun(t, append(sendOnceArgs, "--db", db, "--send")...)
		if err != nil {
			t.Fatalf("retry: %v\n%s", err, stderr)
		}
		var res sendOnceResult
		postmarkResults(t, stdout, &res)
		if !res.Duplicate || !res.DeliveryUnknown || res.Dedupe.Ledger != "pending" || res.MessageID != "" || res.Sent {
			t.Fatalf("retry after an unknown outcome = %+v", res)
		}
		if got := countRequests(f.log(), "POST", "/email"); got != before {
			t.Fatalf("retry posted again after an unknown outcome (%d -> %d)", before, got)
		}
	})

	t.Run("definite rejection releases the key", func(t *testing.T) {
		f := sendOnceFake(t)
		home := postmarkTestEnv(t, f)
		db := filepath.Join(home, "ledger.db")
		f.reply("POST /email", 422, map[string]any{"ErrorCode": 300, "Message": "Invalid 'From' address"})

		if _, _, err := postmarkRun(t, append(sendOnceArgs, "--db", db, "--send")...); err == nil {
			t.Fatal("a 422 send must fail")
		}
		f.reply("POST /email", 200, map[string]any{"To": "jane@example.com", "MessageID": "msg-3", "ErrorCode": 0})
		stdout, stderr, err := postmarkRun(t, append(sendOnceArgs, "--db", db, "--send")...)
		if err != nil {
			t.Fatalf("corrected retry: %v\n%s", err, stderr)
		}
		var res sendOnceResult
		postmarkResults(t, stdout, &res)
		if !res.Sent || res.MessageID != "msg-3" {
			t.Fatalf("corrected retry after a rejection = %+v", res)
		}
	})
}

func TestSendOnceTwoTokensForOneServerShareTheLedger(t *testing.T) {
	f := sendOnceFake(t)
	home := postmarkTestEnv(t, f)
	db := filepath.Join(home, "ledger.db")
	args := []string{"email", "send-once", "--key", "otp-77", "--from", "app@example.com", "--to", "jane@example.com", "--subject", "Your code", "--text", "Code: 77", "--json", "--db", db, "--send"}

	t.Setenv("POSTMARK_SERVER_TOKEN", "tok-first")
	if _, stderr, err := postmarkRun(t, args...); err != nil {
		t.Fatalf("first send: %v\n%s", err, stderr)
	}
	t.Setenv("POSTMARK_SERVER_TOKEN", "tok-second")
	stdout, stderr, err := postmarkRun(t, args...)
	if err != nil {
		t.Fatalf("second send: %v\n%s", err, stderr)
	}
	var res sendOnceResult
	postmarkResults(t, stdout, &res)
	if !res.Duplicate || res.Source != sendOnceSourceLedger {
		t.Fatalf("a second token for the same server should hit the ledger: %+v", res)
	}
	if n := countRequests(f.log(), "POST", "/email"); n != 1 {
		t.Fatalf("POST /email count = %d, want 1", n)
	}
}
