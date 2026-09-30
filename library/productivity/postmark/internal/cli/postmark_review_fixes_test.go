// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/config"
)

func TestTemplatesCheckRefusesSymlinkedContent(t *testing.T) {
	srv, requests := newFakePostmarkServer(t, templateRoutes(nil, map[string]any{}))
	root := t.TempDir()
	welcome := templateContent{Name: "Welcome", Alias: "welcome", Subject: "Hi", HtmlBody: "<p>Hi</p>", TemplateType: templateTypeStandard}
	if _, err := writeTemplateFiles(root, welcome, nil); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(t.TempDir(), "private.txt")
	if err := os.WriteFile(secret, []byte("PRIVATE-FILE-CONTENT"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := templateLocalDir(root, "welcome", templateTypeStandard)
	if err := os.Symlink(secret, filepath.Join(dir, templateTextFileName)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	out, _, err := runWithServerToken(t, srv.URL, "templates", "check", "--dir", root, "--json")
	if err == nil || ExitCode(err) != templatesCheckFailedExit {
		t.Fatalf("a refused symlink must fail the check: err = %v (exit %d)", err, ExitCode(err))
	}
	for _, r := range requests() {
		if strings.Contains(r.Body, "PRIVATE-FILE-CONTENT") {
			t.Fatalf("check uploaded a file outside the template folder via %s %s", r.Method, r.Path)
		}
	}
	if !strings.Contains(out, "symbolic link") {
		t.Fatalf("check should report the refused symlink:\n%s", out)
	}
}

func TestTemplatesPullDoesNotWriteThroughSymlink(t *testing.T) {
	templates := []templateDetail{{TemplateId: 1, Name: "Welcome", Alias: "welcome", Subject: "Hi", HtmlBody: "<p>from server</p>", TemplateType: templateTypeStandard}}
	srv, _ := newFakePostmarkServer(t, templateRoutes(templates, map[string]any{}))
	root := filepath.Join(t.TempDir(), "tpl")
	dir := templateLocalDir(root, "welcome", templateTypeStandard)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "keep.html")
	if err := os.WriteFile(target, []byte("ORIGINAL"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, templateHTMLFileName)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	out, _, pullErr := runWithServerToken(t, srv.URL, "templates", "pull", root, "--json")
	if pullErr == nil && !strings.Contains(out, "symbolic link") {
		t.Fatalf("pull should fail or report the refused symlink:\n%s", out)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "ORIGINAL" {
		t.Fatalf("pull wrote through the symlink: %q, %v", got, err)
	}
}

func TestTemplatesPushPruneHeldAfterFailedCreate(t *testing.T) {
	remote := []templateDetail{{TemplateId: 9, Name: "Receipt", Alias: "receipt", Subject: "Receipt", HtmlBody: "<p>r</p>", TemplateType: templateTypeStandard}}
	routes := pushRoutes(remote)
	routes["POST /templates"] = func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"ErrorCode": 1101, "Message": "invalid template"})
	}
	srv, requests := newFakePostmarkServer(t, routes)
	root := t.TempDir()
	writeLocalFixture(t, root, []templateDetail{{Name: "Receipt v2", Alias: "receipt-v2", Subject: "Receipt", HtmlBody: "<p>r2</p>", TemplateType: templateTypeStandard}})

	out, _, err := runWithServerToken(t, srv.URL, "templates", "push", root, "--prune", "--yes", "--json")
	if err == nil {
		t.Fatalf("a failed create must fail the push:\n%s", out)
	}
	for _, r := range requests() {
		if r.Method == http.MethodDelete {
			t.Fatalf("prune deleted %s after the replacement failed to upload", r.Path)
		}
	}
	assertPruneHeld(t, out)
}

// assertPruneHeld requires a planned delete that was held back, not skipped
// for some unrelated reason.
func assertPruneHeld(t *testing.T, out string) {
	t.Helper()
	var view templatePushView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("push output is not JSON: %v\n%s", err, out)
	}
	deletes := 0
	for _, it := range view.Items {
		if it.Action != pushActionDelete {
			continue
		}
		deletes++
		if it.Status != pushStatusNotAttempted || !strings.Contains(it.Error, "not deleted") {
			t.Fatalf("delete item should be held: %+v", it)
		}
	}
	if deletes == 0 {
		t.Fatalf("expected a held delete in the plan:\n%s", out)
	}
}

func TestEmailSendRefusesDeliveryUnderHarness(t *testing.T) {
	f := newPostmarkFake(t)
	postmarkTestEnv(t, f)
	t.Setenv("POSTMARK_SERVER_TOKEN", "server-token")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
	f.reply("POST /email", 200, map[string]any{"ErrorCode": 0, "MessageID": "m-1"})
	stdout, _, err := postmarkRun(t, "email", "send", "--from", "sender@example.com", "--to", "jane@example.com", "--subject", "Hi", "--text-body", "Hello", "--send", "--json")
	if err != nil {
		t.Fatalf("harness refusal should exit cleanly: %v", err)
	}
	var refusal harnessRefusalResult
	if jerr := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &refusal); jerr != nil || !refusal.Refused {
		t.Fatalf("expected a structured refusal, got %q (%v)", stdout, jerr)
	}
	if n := countPosts(f, "/email"); n != 0 {
		t.Fatalf("email send --send delivered %d message(s) under a harness", n)
	}
}

func TestPostmarkTokenCacheKeyVariesByAccount(t *testing.T) {
	a := &client.Client{BaseURL: "https://api.postmarkapp.com", Config: &config.Config{PostmarkAccountToken: "account-a"}}
	b := &client.Client{BaseURL: "https://api.postmarkapp.com", Config: &config.Config{PostmarkAccountToken: "account-b"}}
	if postmarkTokenCacheKey(a, "Production") == postmarkTokenCacheKey(b, "Production") {
		t.Fatal("cached server tokens must not be shared across account tokens")
	}
	if postmarkTokenCacheKey(a, "Production") != postmarkTokenCacheKey(a, " production ") {
		t.Fatal("the same account and server name should share a cache entry")
	}
	if strings.Contains(postmarkTokenCacheKey(a, "Production"), "account-a") {
		t.Fatal("the cache key must not contain the account token")
	}
}

func TestRedactURLSecrets(t *testing.T) {
	// Built at runtime so no credential-shaped URL literal sits in the source.
	withPassword := (&url.URL{Scheme: "https", User: url.UserPassword("user", "pw-fixture"), Host: "hooks.example.com", Path: "/pm", RawQuery: "token=abc&stream=outbound"}).String()
	cases := map[string]string{
		withPassword:                   (&url.URL{Scheme: "https", User: url.UserPassword("user", "REDACTED"), Host: "hooks.example.com", Path: "/pm", RawQuery: "stream=outbound&token=REDACTED"}).String(),
		"https://hooks.example.com/pm": "https://hooks.example.com/pm",
		"https://svc@hooks.example.com/pm?Signature=xyz": "https://svc@hooks.example.com/pm?Signature=REDACTED",
	}
	for in, want := range cases {
		got := redactURLSecrets(in)
		if got != want {
			t.Errorf("redactURLSecrets(%q) = %q, want %q", in, got, want)
		}
		if strings.Contains(got, "pw-fixture") || strings.Contains(got, "=abc") || strings.Contains(got, "=xyz") {
			t.Errorf("secret survived in %q", got)
		}
	}
}

func TestSendOnceModelKeepsLargeIntegers(t *testing.T) {
	_, payload, err := sendOncePayload(sendOnceInput{to: "jane@example.com", from: "app@example.com", template: "receipt", model: `{"id":9007199254740993}`}, "k")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(payload["TemplateModel"])
	if !strings.Contains(string(b), "9007199254740993") {
		t.Fatalf("template model lost precision: %s", b)
	}
	a := sendOnceDerivedKey("jane@example.com", "", "", "app@example.com", "", "receipt", `{"id":9007199254740993}`, "", "")
	c := sendOnceDerivedKey("jane@example.com", "", "", "app@example.com", "", "receipt", `{"id":9007199254740992}`, "", "")
	if a == c {
		t.Fatal("distinct large integers in --model must produce distinct keys")
	}
}

func TestPostmarkEasternUsesDaylightTime(t *testing.T) {
	summer := postmarkEasternTimestamp(time.Date(2026, 7, 1, 16, 0, 0, 0, time.UTC))
	if summer != "2026-07-01T12:00:00" {
		t.Fatalf("summer Eastern timestamp = %s, want EDT (UTC-4)", summer)
	}
}

func TestDiagnoseLocalSaysSentWithoutDeliveryEvidence(t *testing.T) {
	d := &diagnoseServer{
		Messages: []diagnoseMessage{{MessageID: "m-1", Subject: "Reset", Status: "Sent", Source: postmarkSourceLocal, Events: []diagnoseEvent{}, at: time.Now()}},
		Bounces:  []diagnoseBounce{}, Suppressions: []diagnoseSuppression{},
	}
	diagnoseDecide("jane@example.com", d)
	if d.Verdict != verdictSent || !strings.Contains(d.Reason, "unconfirmed") {
		t.Fatalf("archived Sent message without delivery events = %q (%s), want %q", d.Verdict, d.Reason, verdictSent)
	}
}

func TestPulseByTagRejectsServerSelection(t *testing.T) {
	f := newPostmarkFake(t)
	postmarkTestEnv(t, f)
	_, _, err := postmarkRun(t, "pulse", "--by", "tag", "--server", "Main App", "--json")
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("pulse --by tag --server: err = %v (exit %d), want a usage error", err, ExitCode(err))
	}
}

func TestRecipientDomainsOpenRateUsesTheSameMessages(t *testing.T) {
	now := time.Now()
	msgs := []recipientDomainMessage{{MessageID: "new", Recipients: []string{"jane@example.com"}, ReceivedAt: now, TrackOpens: true}}
	opens := []recipientDomainOpen{{Recipient: "jane@example.com", MessageID: "new", ReceivedAt: now}}
	for i := 0; i < 100; i++ {
		opens = append(opens, recipientDomainOpen{Recipient: "jane@example.com", MessageID: "old-" + strings.Repeat("x", i%3) + string(rune('a'+i%26)), ReceivedAt: now})
	}
	rows := recipientDomainsCompute(msgs, nil, opens, nil, now.Add(-time.Hour), 0)
	if len(rows) != 1 || rows[0].OpenRatePct == nil || *rows[0].OpenRatePct != 100 {
		t.Fatalf("open rate should count only opens of the window's tracked sends: %+v", rows)
	}
}

func TestDomainsHealthFailsWhenEveryDomainLookupFails(t *testing.T) {
	f := newPostmarkFake(t)
	postmarkTestEnv(t, f)
	f.reply("GET /domains", 200, map[string]any{"TotalCount": 1, "Domains": []any{map[string]any{"ID": 7, "Name": "mail.example.com"}}})
	f.reply("GET /domains/7", 404, map[string]any{"ErrorCode": 510, "Message": "not found"})
	f.reply("GET /senders", 200, map[string]any{"TotalCount": 0, "SenderSignatures": []any{}})
	_, _, err := postmarkRun(t, "domains", "health", "--json")
	if err == nil || ExitCode(err) != 5 {
		t.Fatalf("domains health with every lookup failing: err = %v (exit %d), want exit 5", err, ExitCode(err))
	}
}

func TestBounceRepairSearchesTheRequestedStream(t *testing.T) {
	f := newPostmarkFake(t)
	postmarkTestEnv(t, f)
	f.reply("GET /servers", 200, postmarkServersPayload([3]any{1, "Main App", "tok-main"}))
	f.reply("GET /bounces", 200, map[string]any{"TotalCount": 0, "Bounces": []any{}})
	if _, stderr, err := postmarkRun(t, "bounces", "reactivate", "--server", "Main App", "--stream", "broadcast", "--json"); err != nil {
		t.Fatalf("reactivate: %v\n%s", err, stderr)
	}
	found := false
	for _, r := range f.log() {
		if r.Path == "/bounces" && strings.Contains(r.Query, "messagestream=broadcast") {
			found = true
		}
	}
	if !found {
		t.Fatalf("bounce search did not pass messagestream=broadcast: %+v", f.log())
	}
}

func TestInlineLocalLayout(t *testing.T) {
	local := []localTemplate{{Meta: templateMeta{Alias: "base", TemplateType: templateTypeLayout}, HtmlBody: "<main>{{{ @content }}}</main>", TextBody: "-- {{{@content}}} --"}}
	c := templateContent{Alias: "welcome", TemplateType: templateTypeStandard, LayoutTemplate: "base", HtmlBody: "<p>Hi</p>", TextBody: "Hi"}
	merged, ok, err := inlineLocalLayout(c, local)
	if err != nil || !ok || merged.LayoutTemplate != "" || merged.HtmlBody != "<main><p>Hi</p></main>" || merged.TextBody != "-- Hi --" {
		t.Fatalf("merged = %+v, ok = %v, err = %v", merged, ok, err)
	}
	if _, ok, err := inlineLocalLayout(templateContent{TemplateType: templateTypeStandard, LayoutTemplate: "remote-only"}, local); ok || err != nil {
		t.Fatalf("a layout that is not in the folder must not be inlined (ok=%v err=%v)", ok, err)
	}
	for _, body := range []string{"<main>no placeholder</main>", "{{{ @content }}}{{{ @content }}}"} {
		bad := []localTemplate{{Meta: templateMeta{Alias: "base", TemplateType: templateTypeLayout}, HtmlBody: body}}
		if _, ok, err := inlineLocalLayout(c, bad); ok || err == nil || !strings.Contains(err.Error(), "exactly one") {
			t.Fatalf("layout %q should be rejected, got ok=%v err=%v", body, ok, err)
		}
	}
}

func TestTemplatesPushPruneHeldOnConflictExits6(t *testing.T) {
	remote := []templateDetail{{TemplateId: 9, Name: "Receipt", Alias: "receipt", Subject: "Receipt", HtmlBody: "<p>r</p>", TemplateType: templateTypeStandard}}
	srv, requests := newFakePostmarkServer(t, pushRoutes(remote))
	root := t.TempDir()
	// The renamed template has no Subject, so the plan marks it a conflict.
	writeLocalFixture(t, root, []templateDetail{{Name: "Receipt v2", Alias: "receipt-v2", HtmlBody: "<p>r2</p>", TemplateType: templateTypeStandard}})
	out, _, err := runWithServerToken(t, srv.URL, "templates", "push", root, "--prune", "--yes", "--json")
	if err == nil || ExitCode(err) != 6 {
		t.Fatalf("push with a conflicted template: err = %v (exit %d), want exit 6\n%s", err, ExitCode(err), out)
	}
	if got := mutatingRequests(requests()); len(got) != 0 {
		t.Fatalf("no writes expected when the only local template conflicts, got %v", got)
	}
	assertPruneHeld(t, out)
}

func TestTemplatesPushRefusesSymlinkedTemplateFolder(t *testing.T) {
	remote := []templateDetail{
		{TemplateId: 1, Name: "Welcome", Alias: "welcome", Subject: "Hi", HtmlBody: "<p>Hi</p>", TemplateType: templateTypeStandard},
		{TemplateId: 2, Name: "Receipt", Alias: "receipt", Subject: "Receipt", HtmlBody: "<p>r</p>", TemplateType: templateTypeStandard},
	}
	srv, requests := newFakePostmarkServer(t, pushRoutes(remote))
	root := t.TempDir()
	writeLocalFixture(t, root, remote[:1])
	elsewhere := t.TempDir()
	writeLocalFixture(t, elsewhere, remote[1:])
	link := templateLocalDir(root, "receipt", templateTypeStandard)
	if err := os.Symlink(templateLocalDir(elsewhere, "receipt", templateTypeStandard), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, _, err := runWithServerToken(t, srv.URL, "templates", "push", root, "--prune", "--yes", "--json")
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("a symlinked template folder must block --yes: err = %v (exit %d)", err, ExitCode(err))
	}
	for _, r := range requests() {
		if r.Method == http.MethodDelete {
			t.Fatalf("pruned %s because its folder was a symlink", r.Path)
		}
	}
}

func TestResendBlockedFailsWhenEveryLookupFails(t *testing.T) {
	f := newPostmarkFake(t)
	postmarkTestEnv(t, f)
	f.reply("GET /servers", 200, postmarkServersPayload([3]any{1, "Main App", "tok-main"}))
	f.reply("GET /bounces", 200, map[string]any{"TotalCount": 2, "Bounces": []any{
		map[string]any{"ID": 1, "Type": "HardBounce", "Email": "a@example.com", "MessageID": "m-1", "Inactive": true, "CanActivate": true, "BouncedAt": time.Now().Format(time.RFC3339)},
		map[string]any{"ID": 2, "Type": "HardBounce", "Email": "b@example.com", "MessageID": "m-2", "Inactive": true, "CanActivate": false, "BouncedAt": time.Now().Format(time.RFC3339)},
	}})
	f.reply("GET /messages/outbound/m-1/details", 400, map[string]any{"ErrorCode": 300, "Message": "bad request"})
	_, _, err := postmarkRun(t, "bounces", "resend-blocked", "--server", "Main App", "--json")
	if err == nil || ExitCode(err) != 5 {
		t.Fatalf("every attempted lookup failed: err = %v (exit %d), want exit 5", err, ExitCode(err))
	}
}

func TestRedactURLSecretsFailsClosedOnBadQuery(t *testing.T) {
	got := redactURLSecrets("https://hooks.example.com/pm?token=SECRET;mode=1")
	if strings.Contains(got, "SECRET") {
		t.Fatalf("a malformed query leaked its value: %q", got)
	}
}

func TestSendOnceModelRejectsTrailingContent(t *testing.T) {
	for _, model := range []string{`{"id":1} garbage`, `{"a":1}{"b":2}`} {
		if _, _, err := sendOncePayload(sendOnceInput{to: "jane@example.com", from: "app@example.com", template: "receipt", model: model}, "k"); err == nil {
			t.Errorf("--model %q should be rejected", model)
		}
	}
}

func TestPostmarkTokenCacheKeyHonorsAccountHeader(t *testing.T) {
	base := &config.Config{PostmarkAccountToken: "account-a"}
	overridden := &config.Config{PostmarkAccountToken: "account-a", Headers: map[string]string{"x-postmark-account-token": "account-b"}}
	a := postmarkTokenCacheKey(&client.Client{Config: base}, "Production")
	b := postmarkTokenCacheKey(&client.Client{Config: overridden}, "Production")
	if a == b {
		t.Fatal("an account-token header override must change the cache key")
	}
}

func TestConflictingCredentialHeadersAreRefused(t *testing.T) {
	cfg := &config.Config{Headers: map[string]string{"X-Postmark-Account-Token": "account-a", "x-postmark-account-token": "account-b"}}
	if err := postmarkCredentialHeaderConflict(cfg); err == nil || ExitCode(err) != 10 {
		t.Fatalf("conflicting account-token headers: err = %v (exit %d), want a config error", err, ExitCode(err))
	}
	same := &config.Config{Headers: map[string]string{"X-Postmark-Account-Token": "account-a", "x-postmark-account-token": "account-a"}}
	if err := postmarkCredentialHeaderConflict(same); err != nil {
		t.Fatalf("identical duplicates are unambiguous: %v", err)
	}
}

func TestSendOnceSameNamedServersOnDifferentAccountsDoNotCollide(t *testing.T) {
	f := sendOnceFake(t)
	home := postmarkTestEnv(t, f)
	db := filepath.Join(home, "ledger.db")
	// Both tokens belong to servers named "Main App", but on different accounts.
	f.handle("GET /server", func(r *http.Request, _ string) (int, any) {
		id := 1
		if r.Header.Get("X-Postmark-Server-Token") == "tok-other-account" {
			id = 2
		}
		return 200, map[string]any{"ID": id, "Name": "Main App"}
	})
	args := []string{"email", "send-once", "--key", "otp-shared", "--from", "app@example.com", "--to", "jane@example.com", "--subject", "Code", "--text", "1", "--json", "--db", db, "--send"}
	for _, token := range []string{"tok-first-account", "tok-other-account"} {
		t.Setenv("POSTMARK_SERVER_TOKEN", token)
		stdout, stderr, err := postmarkRun(t, args...)
		if err != nil {
			t.Fatalf("send with %s: %v\n%s", token, err, stderr)
		}
		var res sendOnceResult
		postmarkResults(t, stdout, &res)
		if res.Duplicate || !res.Sent {
			t.Fatalf("send with %s should not be a duplicate of another account's server: %+v", token, res)
		}
	}
	if n := countRequests(f.log(), "POST", "/email"); n != 2 {
		t.Fatalf("POST /email count = %d, want 2", n)
	}
}

func TestDiagnoseArchivedMessagePastRetentionOffersNoLiveLookup(t *testing.T) {
	old := &diagnoseServer{
		Messages: []diagnoseMessage{{MessageID: "m-old", Subject: "Reset", Status: "Sent", Source: postmarkSourceLocal, Events: []diagnoseEvent{}, at: time.Now().Add(-60 * 24 * time.Hour)}},
		Bounces:  []diagnoseBounce{}, Suppressions: []diagnoseSuppression{},
	}
	diagnoseDecide("jane@example.com", old)
	if old.Next != "" || !strings.Contains(old.Reason, "45-day retention") {
		t.Fatalf("a message past retention should not suggest messages get: next=%q reason=%q", old.Next, old.Reason)
	}
	recent := &diagnoseServer{
		Messages: []diagnoseMessage{{MessageID: "m-new", Subject: "Reset", Status: "Sent", Source: postmarkSourceLocal, Events: []diagnoseEvent{}, at: time.Now().Add(-2 * 24 * time.Hour)}},
		Bounces:  []diagnoseBounce{}, Suppressions: []diagnoseSuppression{},
	}
	diagnoseDecide("jane@example.com", recent)
	if !strings.Contains(recent.Next, "messages get m-new") {
		t.Fatalf("a recent archived message should suggest the live lookup: %q", recent.Next)
	}
}
