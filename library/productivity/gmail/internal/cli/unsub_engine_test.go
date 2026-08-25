// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavioral acceptance tests for the unsubscribe engine, driven end to end
// through the real command tree on the shared fake-Gmail fixture, with a
// SECOND httptest server standing in as the sender's unsubscribe endpoint.
// Covers audit classification over synced data, plan freezing +
// ineligibility, run's token/tamper refusals, the exact hardened POST (body,
// content-type, no redirect following, no cookies/auth), the SSRF skip, the
// live DKIM skips, the attempt ledger, and verify's violator join.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

const (
	unsubTestSenderA = "news@letters.example"    // one-click verified
	unsubTestSenderB = "deals@shop.example"      // plain-url (no post header)
	unsubTestSenderC = "updates@culture.example" // mailto-only
	unsubTestURLA    = "https://unsub.letters.example/one-click/a1"
	unsubTestDKIMA   = "v=1; a=rsa-sha256; d=letters.example; s=news; h=From:Subject:List-Unsubscribe:List-Unsubscribe-Post; bh=abc; b=def"
	unsubTestAuthA   = "mx.google.com; dkim=pass header.i=@letters.example; spf=pass; dmarc=pass (p=REJECT) header.from=letters.example"
)

// oneClickHeadersA is sender A's full verified header set.
func oneClickHeadersA() [][2]string {
	return [][2]string{
		{"List-Unsubscribe", "<" + unsubTestURLA + ">, <mailto:u@letters.example>"},
		{"List-Unsubscribe-Post", "List-Unsubscribe=One-Click"},
		{"Authentication-Results", unsubTestAuthA},
		{"DKIM-Signature", unsubTestDKIMA},
	}
}

// seedUnsubMailbox adds the three unsubscribe test senders to the fake
// mailbox with recent internal dates (inside every --since window) and
// returns sender A's newest message id.
func seedUnsubMailbox(fx *engineFixture) string {
	base := time.Now().Add(-48 * time.Hour).UnixMilli()
	labelsUnread := []string{"INBOX", "CATEGORY_PROMOTIONS", "UNREAD"}
	labelsRead := []string{"INBOX", "CATEGORY_PROMOTIONS"}

	// Sender A: 4 messages, 2 unread, full one-click headers everywhere.
	for i := 1; i <= 4; i++ {
		labels := labelsRead
		if i%2 == 0 {
			labels = labelsUnread
		}
		fx.fake.addMsg(fmt.Sprintf("a%d", i), labels, fakeMsg{
			From:         "Letters News <" + unsubTestSenderA + ">",
			Subject:      fmt.Sprintf("Newsletter %d", i),
			InternalDate: base + int64(i)*1000,
			ThreadID:     fmt.Sprintf("ta%d", i),
			Size:         1000,
			ExtraHeaders: oneClickHeadersA(),
		})
	}
	// Sender B: 3 messages, https URL but no List-Unsubscribe-Post.
	for i := 1; i <= 3; i++ {
		fx.fake.addMsg(fmt.Sprintf("b%d", i), labelsRead, fakeMsg{
			From:         "Shop Deals <" + unsubTestSenderB + ">",
			Subject:      fmt.Sprintf("Deal %d", i),
			InternalDate: base + int64(i)*1000,
			ThreadID:     fmt.Sprintf("tb%d", i),
			Size:         500,
			ExtraHeaders: [][2]string{
				{"List-Unsubscribe", "<https://esp-tracker.example/u/b>"},
				{"Authentication-Results", unsubTestAuthA},
			},
		})
	}
	// Sender C: 3 messages, mailto only.
	for i := 1; i <= 3; i++ {
		fx.fake.addMsg(fmt.Sprintf("c%d", i), labelsRead, fakeMsg{
			From:         "Culture Updates <" + unsubTestSenderC + ">",
			Subject:      fmt.Sprintf("Update %d", i),
			InternalDate: base + int64(i)*1000,
			ThreadID:     fmt.Sprintf("tc%d", i),
			Size:         500,
			ExtraHeaders: [][2]string{
				{"List-Unsubscribe", "<mailto:leave@culture.example>"},
				{"Authentication-Results", unsubTestAuthA},
			},
		})
	}
	return "a4"
}

// unsubEndpoint is the second httptest server: the sender's one-click
// endpoint. It records every request verbatim.
type unsubEndpoint struct {
	mu       sync.Mutex
	requests []recordedUnsubPost
	status   int
	location string
	ts       *httptest.Server
}

type recordedUnsubPost struct {
	Method, Path, Host, Body, ContentType, Cookie, Authorization string
}

func newUnsubEndpoint(t *testing.T) *unsubEndpoint {
	t.Helper()
	ep := &unsubEndpoint{status: http.StatusOK}
	ep.ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		ep.mu.Lock()
		ep.requests = append(ep.requests, recordedUnsubPost{
			Method:        r.Method,
			Path:          r.URL.Path,
			Host:          r.Host,
			Body:          string(body),
			ContentType:   r.Header.Get("Content-Type"),
			Cookie:        r.Header.Get("Cookie"),
			Authorization: r.Header.Get("Authorization"),
		})
		status, location := ep.status, ep.location
		ep.mu.Unlock()
		if location != "" {
			w.Header().Set("Location", location)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(ep.ts.Close)
	return ep
}

func (ep *unsubEndpoint) setResponse(status int, location string) {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	ep.status = status
	ep.location = location
}

func (ep *unsubEndpoint) recorded() []recordedUnsubPost {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	return append([]recordedUnsubPost(nil), ep.requests...)
}

// unsubRewriteTransport routes the (already-guarded) POST to the local
// endpoint listener while preserving the original request untouched apart
// from the target — the http.Client wrapper (redirect policy, timeout,
// no-jar) and request construction stay production code.
type unsubRewriteTransport struct{ target *url.URL }

func (rt unsubRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r2 := req.Clone(req.Context())
	r2.Host = req.URL.Host // the server should see the ORIGINAL host
	r2.URL.Scheme = rt.target.Scheme
	r2.URL.Host = rt.target.Host
	return http.DefaultTransport.RoundTrip(r2)
}

// installUnsubSeams points DNS at resolveTo (mutable) and the transport at
// the endpoint, restoring both on cleanup.
func installUnsubSeams(t *testing.T, ep *unsubEndpoint, resolveTo map[string]string) {
	t.Helper()
	oldLookup, oldTransport := unsubLookupIP, unsubTransportOverride
	t.Cleanup(func() { unsubLookupIP, unsubTransportOverride = oldLookup, oldTransport })
	unsubLookupIP = func(ctx context.Context, host string) ([]net.IP, error) {
		addr, ok := resolveTo[host]
		if !ok {
			return nil, fmt.Errorf("test resolver: unknown host %s", host)
		}
		return []net.IP{net.ParseIP(addr)}, nil
	}
	target, err := url.Parse(ep.ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	unsubTransportOverride = unsubRewriteTransport{target: target}
}

// planUnsub runs `unsub plan` for the given senders and returns (sha, nonce, parsed).
func planUnsub(t *testing.T, fx *engineFixture, senders string) (string, string, map[string]any) {
	t.Helper()
	out, stderr, code := fx.runCLI(t, "unsub", "plan", "--account", "test", "--senders", senders)
	if code != 0 {
		t.Fatalf("unsub plan exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	parsed := mustParseJSON(t, out)
	sha, _ := parsed["plan_sha"].(string)
	nonce, _ := parsed["nonce"].(string)
	if sha == "" || nonce == "" {
		t.Fatalf("unsub plan output missing sha/nonce: %s", out)
	}
	return sha, nonce, parsed
}

func TestUnsubAudit_ClassifiesSyncedSenders(t *testing.T) {
	fx := newEngineFixture(t)
	seedUnsubMailbox(fx)

	if out, stderr, code := fx.runCLI(t, "sync", "--account", "test"); code != 0 {
		t.Fatalf("sync exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	out, stderr, code := fx.runCLI(t, "unsub", "audit", "--account", "test")
	if code != 0 {
		t.Fatalf("unsub audit exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	var rows []map[string]any
	if err := jsonUnmarshalString(out, &rows); err != nil {
		t.Fatalf("audit output not a row array: %v\n%s", err, out)
	}
	if len(rows) != 3 {
		t.Fatalf("audit rows = %d, want 3 (A, B, C)\n%s", len(rows), out)
	}
	// Ordered by count desc then sender asc: A(4), then B and C (3 each).
	a, b, c := rows[0], rows[1], rows[2]
	if a["sender"] != unsubTestSenderA || a["count"].(float64) != 4 {
		t.Fatalf("row 0 = %v, want %s count 4", a, unsubTestSenderA)
	}
	if a["classification"] != classUnsubOneClick {
		t.Fatalf("A classification = %v, want %s", a["classification"], classUnsubOneClick)
	}
	if a["url_or_mailto"] != unsubTestURLA || a["aligned"] != true {
		t.Fatalf("A url/aligned = %v/%v, want %s/true", a["url_or_mailto"], a["aligned"], unsubTestURLA)
	}
	if got := a["unread_rate"].(float64); got != 0.5 {
		t.Fatalf("A unread_rate = %v, want 0.5", got)
	}
	if b["sender"] != unsubTestSenderB || b["classification"] != classUnsubPlainURL {
		t.Fatalf("row 1 = %v, want %s plain-url", b, unsubTestSenderB)
	}
	if b["aligned"] != false {
		t.Fatalf("B aligned = %v, want false (esp-tracker.example != shop.example)", b["aligned"])
	}
	if got, _ := b["reason"].(string); !strings.Contains(got, "List-Unsubscribe-Post header missing") {
		t.Fatalf("B reason = %q", got)
	}
	if c["sender"] != unsubTestSenderC || c["classification"] != classUnsubMailtoOnly {
		t.Fatalf("row 2 = %v, want %s mailto-only", c, unsubTestSenderC)
	}
	if got, _ := c["url_or_mailto"].(string); got != "mailto:leave@culture.example" {
		t.Fatalf("C url_or_mailto = %q", got)
	}

	// The audit stamped the registrable unsub domain into the store.
	db := fx.openStore(t)
	m, err := db.GetMailMeta("test", "a4")
	if err != nil {
		t.Fatal(err)
	}
	if m.ListUnsubDomain != "letters.example" {
		t.Fatalf("a4 list_unsub_domain = %q, want letters.example", m.ListUnsubDomain)
	}

	// --select honors the field list under --agent.
	out, _, code = fx.runCLI(t, "unsub", "audit", "--account", "test", "--agent", "--select", "sender,classification")
	if code != 0 {
		t.Fatalf("audit --select exit = %d\n%s", code, out)
	}
	env := mustParseJSON(t, out)
	results := env["results"].([]any)
	first := results[0].(map[string]any)
	if len(first) != 2 || first["sender"] == nil || first["classification"] == nil {
		t.Fatalf("--select sender,classification row = %v", first)
	}
}

func TestUnsubPlanAndRun_EndToEnd(t *testing.T) {
	fx := newEngineFixture(t)
	newestA := seedUnsubMailbox(fx)
	ep := newUnsubEndpoint(t)
	resolveTo := map[string]string{"unsub.letters.example": "93.184.216.34"}
	installUnsubSeams(t, ep, resolveTo)

	if out, stderr, code := fx.runCLI(t, "sync", "--account", "test"); code != 0 {
		t.Fatalf("sync exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}

	// ---- plan: A freezes; B and a ghost are ineligible ----
	sha, nonce, parsed := planUnsub(t, fx, strings.Join([]string{unsubTestSenderA, unsubTestSenderB, "ghost@nowhere.example"}, ","))
	if got := parsed["total"].(float64); got != 1 {
		t.Fatalf("plan total = %v, want 1 (only sender A eligible)", got)
	}
	perGroup := parsed["per_group"].([]any)
	if len(perGroup) != 1 {
		t.Fatalf("per_group = %v, want 1 group", perGroup)
	}
	g0 := perGroup[0].(map[string]any)
	if g0["rule"] != unsubTestSenderA || g0["action"] != "unsub" {
		t.Fatalf("group 0 = %v, want sender A / action unsub", g0)
	}
	ineligible := parsed["ineligible"].([]any)
	if len(ineligible) != 2 {
		t.Fatalf("ineligible = %v, want B + ghost", ineligible)
	}
	if parsed["run_hint"] == nil || parsed["apply_hint"] != nil {
		t.Fatalf("plan hints wrong (want run_hint, no apply_hint): %s", parsed)
	}
	// The frozen group carries the exact URL and the newest message id.
	plan, err := loadPlanFile(fx.authDir, sha)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Groups[0].UnsubURL != unsubTestURLA || plan.Groups[0].Sender != unsubTestSenderA {
		t.Fatalf("frozen group = %+v", plan.Groups[0])
	}
	if len(plan.Groups[0].IDs) != 1 || plan.Groups[0].IDs[0] != newestA {
		t.Fatalf("frozen ids = %v, want [%s]", plan.Groups[0].IDs, newestA)
	}

	// ---- (e) wrong token: exit 4, ZERO posts, zero API calls ----
	fx.fake.resetLog()
	out, _, code := fx.runCLI(t, "unsub", "run", "--plan", sha, "--token", strings.Repeat("ab", 16))
	if code != 4 {
		t.Fatalf("wrong token exit = %d, want 4\n%s", code, out)
	}
	if n := len(ep.recorded()); n != 0 {
		t.Fatalf("wrong token caused %d endpoint requests, want 0", n)
	}
	if got := fx.fake.reqLog(); len(got) != 0 {
		t.Fatalf("wrong token made %d Gmail API calls, want 0: %v", len(got), got)
	}

	// ---- (f) tampered plan file: exit 4, zero posts ----
	planPath := filepath.Join(fx.authDir, "plans", sha+".json")
	orig, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, append(orig, ' '), 0o600); err != nil {
		t.Fatal(err)
	}
	out, _, code = fx.runCLI(t, "unsub", "run", "--plan", sha, "--token", nonce)
	if code != 4 {
		t.Fatalf("tampered plan exit = %d, want 4\n%s", code, out)
	}
	if n := len(ep.recorded()); n != 0 {
		t.Fatalf("tampered plan caused %d endpoint requests, want 0", n)
	}
	if err := os.WriteFile(planPath, orig, 0o600); err != nil {
		t.Fatal(err)
	}

	// ---- engine separation: 'cleanup apply' refuses an unsub plan without
	// burning the token or touching the mailbox ----
	fx.fake.resetLog()
	out, _, code = fx.runCLI(t, "cleanup", "apply", "--plan", sha, "--token", nonce)
	if code != 4 {
		t.Fatalf("cleanup apply on unsub plan exit = %d, want 4\n%s", code, out)
	}
	if n := fx.fake.countRequests("POST "); n != 0 {
		t.Fatalf("cleanup apply on unsub plan issued %d mutations, want 0", n)
	}

	// ---- (a) valid run: exactly one POST, exact body, 302 recorded, not followed ----
	ep.setResponse(http.StatusFound, "https://unsub.letters.example/confirmed")
	out, stderr, code := fx.runCLI(t, "unsub", "run", "--plan", sha, "--token", nonce)
	if code != 0 {
		t.Fatalf("unsub run exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	res := mustParseJSON(t, out)
	if res["posted"].(float64) != 1 || res["skipped"].(float64) != 0 || res["state"] != "done" {
		t.Fatalf("run result = %s", out)
	}
	senders := res["senders"].([]any)
	s0 := senders[0].(map[string]any)
	if s0["outcome"] != "posted" || s0["http_status"].(float64) != 302 || s0["sender"] != unsubTestSenderA {
		t.Fatalf("sender result = %v", s0)
	}
	reqs := ep.recorded()
	if len(reqs) != 1 {
		t.Fatalf("endpoint requests = %d, want exactly 1 (redirect must NOT be followed)", len(reqs))
	}
	r := reqs[0]
	if r.Method != "POST" || r.Path != "/one-click/a1" {
		t.Fatalf("request = %+v, want POST /one-click/a1", r)
	}
	if r.Body != "List-Unsubscribe=One-Click" {
		t.Fatalf("body = %q, want the exact RFC 8058 body", r.Body)
	}
	if r.ContentType != "application/x-www-form-urlencoded" {
		t.Fatalf("content-type = %q", r.ContentType)
	}
	if r.Cookie != "" || r.Authorization != "" {
		t.Fatalf("request leaked credentials: cookie=%q auth=%q", r.Cookie, r.Authorization)
	}
	if r.Host != "unsub.letters.example" {
		t.Fatalf("host = %q, want the original hostname", r.Host)
	}
	db := fx.openStore(t)
	attempts, err := db.ListMailUnsubAttempts("test")
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 {
		t.Fatalf("ledger attempts = %d, want 1", len(attempts))
	}
	if attempts[0].Sender != unsubTestSenderA || attempts[0].URL != unsubTestURLA || attempts[0].Status != "302" || attempts[0].PlanSha != sha {
		t.Fatalf("ledger row = %+v", attempts[0])
	}

	// Re-running the same token refuses (single use), no new posts.
	out, _, code = fx.runCLI(t, "unsub", "run", "--plan", sha, "--token", nonce)
	if code != 4 {
		t.Fatalf("re-used token exit = %d, want 4\n%s", code, out)
	}
	if n := len(ep.recorded()); n != 1 {
		t.Fatalf("re-used token grew endpoint requests to %d", n)
	}

	// ---- (b) sender resolving to loopback: skipped ssrf-guard, no POST ----
	resolveTo["unsub.letters.example"] = "127.0.0.1"
	sha2, nonce2, _ := planUnsub(t, fx, unsubTestSenderA)
	out, _, code = fx.runCLI(t, "unsub", "run", "--plan", sha2, "--token", nonce2)
	if code != 3 {
		t.Fatalf("ssrf run exit = %d, want 3 (partial)\n%s", code, out)
	}
	res = mustParseJSON(t, out)
	s0 = res["senders"].([]any)[0].(map[string]any)
	if s0["outcome"] != "skipped" || !strings.Contains(s0["reason"].(string), "ssrf-guard") {
		t.Fatalf("ssrf sender result = %v", s0)
	}
	if n := len(ep.recorded()); n != 1 {
		t.Fatalf("ssrf-guarded run still posted (endpoint requests = %d)", n)
	}
	attempts, _ = db.ListMailUnsubAttempts("test")
	if got := attempts[len(attempts)-1].Status; got != "skipped:ssrf-guard" {
		t.Fatalf("ssrf ledger status = %q, want skipped:ssrf-guard", got)
	}
	resolveTo["unsub.letters.example"] = "93.184.216.34"

	// ---- (c) live DKIM h= no longer covers list-unsubscribe-post: skipped ----
	fx.fake.setExtraHeaders(newestA, [][2]string{
		{"List-Unsubscribe", "<" + unsubTestURLA + ">, <mailto:u@letters.example>"},
		{"List-Unsubscribe-Post", "List-Unsubscribe=One-Click"},
		{"Authentication-Results", unsubTestAuthA},
		{"DKIM-Signature", "v=1; a=rsa-sha256; d=letters.example; s=news; h=From:Subject:List-Unsubscribe; bh=abc; b=def"},
	})
	sha3, nonce3, _ := planUnsub(t, fx, unsubTestSenderA)
	out, _, code = fx.runCLI(t, "unsub", "run", "--plan", sha3, "--token", nonce3)
	if code != 3 {
		t.Fatalf("dkim-coverage run exit = %d, want 3\n%s", code, out)
	}
	res = mustParseJSON(t, out)
	s0 = res["senders"].([]any)[0].(map[string]any)
	if s0["outcome"] != "skipped" || !strings.Contains(s0["reason"].(string), "DKIM h= tag list") {
		t.Fatalf("dkim-coverage sender result = %v", s0)
	}
	if n := len(ep.recorded()); n != 1 {
		t.Fatalf("dkim-coverage run still posted (endpoint requests = %d)", n)
	}

	// ---- (d) duplicate live DKIM-Signature headers: skipped ----
	fx.fake.setExtraHeaders(newestA, [][2]string{
		{"List-Unsubscribe", "<" + unsubTestURLA + ">, <mailto:u@letters.example>"},
		{"List-Unsubscribe-Post", "List-Unsubscribe=One-Click"},
		{"Authentication-Results", unsubTestAuthA},
		{"DKIM-Signature", unsubTestDKIMA},
		{"DKIM-Signature", unsubTestDKIMA},
	})
	sha4, nonce4, _ := planUnsub(t, fx, unsubTestSenderA)
	out, _, code = fx.runCLI(t, "unsub", "run", "--plan", sha4, "--token", nonce4)
	if code != 3 {
		t.Fatalf("duplicate-dkim run exit = %d, want 3\n%s", code, out)
	}
	res = mustParseJSON(t, out)
	s0 = res["senders"].([]any)[0].(map[string]any)
	if s0["outcome"] != "skipped" || !strings.Contains(s0["reason"].(string), "2 DKIM-Signature headers") {
		t.Fatalf("duplicate-dkim sender result = %v", s0)
	}
	if n := len(ep.recorded()); n != 1 {
		t.Fatalf("duplicate-dkim run still posted (endpoint requests = %d)", n)
	}
}

// TestUnsubRunCrash_RecoverIsHonest pins the recover path for an
// interrupted unsub run: an authorized 'unsub' apply with no chunks is
// reconciled to 'abandoned' with a note pointing at mail_unsub_ledger —
// never the mailbox-apply "nothing was sent" claim, because attempts may
// have gone out.
func TestUnsubRunCrash_RecoverIsHonest(t *testing.T) {
	fx := newEngineFixture(t)
	seedUnsubMailbox(fx)
	if _, _, code := fx.runCLI(t, "sync", "--account", "test"); code != 0 {
		t.Fatal("sync failed")
	}
	sha, nonce, _ := planUnsub(t, fx, unsubTestSenderA)

	// Simulate the crash: token burned + run authorized, then death before
	// any terminal state (unsub runs never write chunk intents).
	db := fx.openStore(t)
	applyID, err := db.AuthorizeMailApply(nonce, sha, "test", "unsub", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetMailApplyState(applyID, store.MailApplyStateApplying); err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertMailUnsubAttempt(store.MailUnsubAttempt{
		Account: "test", Sender: unsubTestSenderA, URL: unsubTestURLA, PlanSha: sha, Status: "unknown",
	}); err != nil {
		t.Fatal(err)
	}

	out, stderr, code := fx.runCLI(t, "cleanup", "recover")
	if code != 0 {
		t.Fatalf("recover exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	parsed := mustParseJSON(t, out)
	recovered := parsed["recovered"].([]any)
	if len(recovered) != 1 {
		t.Fatalf("recovered = %v, want the one crashed unsub run", recovered)
	}
	r0 := recovered[0].(map[string]any)
	if r0["state"] != store.MailApplyStateAbandoned {
		t.Fatalf("state = %v, want abandoned", r0["state"])
	}
	note, _ := r0["note"].(string)
	if !strings.Contains(note, "mail_unsub_ledger") || strings.Contains(note, "nothing was sent") {
		t.Fatalf("recover note = %q, want the unsub-honest wording", note)
	}
	// The 'unknown' attempt stays untouched — never auto-retried.
	attempts, err := db.ListMailUnsubAttempts("test")
	if err != nil || len(attempts) != 1 || attempts[0].Status != "unknown" {
		t.Fatalf("attempts after recover = %+v (err %v)", attempts, err)
	}
}

func TestUnsubVerify_ReportsViolators(t *testing.T) {
	fx := newEngineFixture(t)
	db := fx.openStore(t)
	now := time.Now().UTC()
	nowMs := now.UnixMilli()

	// A: 2xx post 5 days ago, arrivals after the 2-day grace -> violator.
	if _, err := db.InsertMailUnsubAttempt(store.MailUnsubAttempt{
		Account: "test", Sender: unsubTestSenderA, URL: unsubTestURLA,
		PostedAt: now.Add(-5 * 24 * time.Hour).Format(time.RFC3339), Status: "200",
	}); err != nil {
		t.Fatal(err)
	}
	// B: 2xx post, quiet since -> not reported.
	if _, err := db.InsertMailUnsubAttempt(store.MailUnsubAttempt{
		Account: "test", Sender: unsubTestSenderB, URL: "https://esp-tracker.example/u/b",
		PostedAt: now.Add(-5 * 24 * time.Hour).Format(time.RFC3339), Status: "204",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpsertMailMeta([]store.MailMeta{
		{Account: "test", ID: "v1", FromEmail: unsubTestSenderA, Subject: "still here", InternalDate: nowMs - 4*24*60*60*1000}, // in grace
		{Account: "test", ID: "v2", FromEmail: unsubTestSenderA, Subject: "and here", InternalDate: nowMs - 2*24*60*60*1000},
		{Account: "test", ID: "v3", FromEmail: unsubTestSenderA, Subject: "newest offender", InternalDate: nowMs - 1*24*60*60*1000},
	}); err != nil {
		t.Fatal(err)
	}

	out, stderr, code := fx.runCLI(t, "unsub", "verify", "--account", "test")
	if code != 0 {
		t.Fatalf("unsub verify exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	var rows []map[string]any
	if err := jsonUnmarshalString(out, &rows); err != nil {
		t.Fatalf("verify output not a row array: %v\n%s", err, out)
	}
	if len(rows) != 1 {
		t.Fatalf("violators = %d, want exactly sender A\n%s", len(rows), out)
	}
	v := rows[0]
	if v["sender"] != unsubTestSenderA || v["arrivals_since"].(float64) != 2 {
		t.Fatalf("violator = %v, want %s with 2 post-grace arrivals", v, unsubTestSenderA)
	}
	if v["newest_subject"] != "newest offender" {
		t.Fatalf("newest_subject = %v", v["newest_subject"])
	}
	if v["escalation_query"] != "from:"+unsubTestSenderA {
		t.Fatalf("escalation_query = %v", v["escalation_query"])
	}

	// --select spot check under --agent.
	out, _, code = fx.runCLI(t, "unsub", "verify", "--account", "test", "--agent", "--select", "sender,escalation_query")
	if code != 0 {
		t.Fatalf("verify --select exit = %d\n%s", code, out)
	}
	env := mustParseJSON(t, out)
	first := env["results"].([]any)[0].(map[string]any)
	if len(first) != 2 || first["escalation_query"] != "from:"+unsubTestSenderA {
		t.Fatalf("--select row = %v", first)
	}
}

// jsonUnmarshalString unmarshals a JSON string into v.
func jsonUnmarshalString(s string, v any) error {
	return json.Unmarshal([]byte(s), v)
}
