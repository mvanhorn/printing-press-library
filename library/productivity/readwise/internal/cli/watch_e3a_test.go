package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
)

type e3Transport func(*http.Request) (*http.Response, error)

func (f e3Transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func e3DB(t *testing.T) (*store.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}
func TestE3RecipientConsent(t *testing.T) {
	testenv.Isolate(t)
	db, _ := e3DB(t)
	for _, text := range []bool{true, false} {
		if _, err := db.AddWebhook(context.Background(), "https://example.com/hook", "", "*", text); err != nil {
			t.Fatal(err)
		}
	}
	var bodies []string
	old := webhookClientFactory
	webhookClientFactory = func() *http.Client {
		return &http.Client{Transport: e3Transport(func(r *http.Request) (*http.Response, error) {
			b, _ := io.ReadAll(r.Body)
			bodies = append(bodies, string(b))
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		})}
	}
	defer func() { webhookClientFactory = old }()
	for _, kind := range []string{"docs", "highlights"} {
		bodies = nil
		body := []byte(`{"items":[{"id":"1","text":"private","summary":"private"}]}`)
		if _, err := fireWatchWebhooks(context.Background(), io.Discard, db, kind, body, 1); err != nil {
			t.Fatal(err)
		}
		if len(bodies) != 2 || !strings.Contains(bodies[0], "private") || strings.Contains(bodies[1], "text") || strings.Contains(bodies[1], "summary") {
			t.Fatalf("recipient consent violated: %v", bodies)
		}
	}
}
func e3Run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	return out.String(), err
}

func TestE3WatchBaselineAndCap(t *testing.T) {
	testenv.Isolate(t)
	db, path := e3DB(t)
	ctx := context.Background()
	// Use authoritative upserts so tests don't depend on private store internals.
	for i := 1; i <= 205; i++ {
		raw := json.RawMessage(fmt.Sprintf(`{"id":"%d","title":"title","location":"new"}`, i))
		if _, _, err := db.UpsertBatch("documents", []json.RawMessage{raw}); err != nil {
			t.Fatal(err)
		}
	}
	for _, existing := range []bool{false, true} {
		name := fmt.Sprint(existing)
		args := []string{"watch", "add", "--db", path, "--name", name, "--kind", "docs"}
		if existing {
			args = append(args, "--include-existing")
		}
		if out, err := e3Run(t, args...); err != nil {
			t.Fatalf("add %s: %v %s", name, err, out)
		}
		w, err := db.GetWatch(ctx, name)
		if err != nil {
			t.Fatal(err)
		}
		seen, err := db.SeenItemIDs(ctx, w.ID, "docs")
		if err != nil {
			t.Fatal(err)
		}
		want := 205
		if existing {
			want = 0
		}
		if len(seen) != want {
			t.Fatalf("baseline %s = %d want %d", name, len(seen), want)
		}
		if existing {
			diff, err := runWatchDiff(ctx, db, w, false, false)
			if err != nil {
				t.Fatal(err)
			}
			if diff.NewCount != 200 || diff.DeferredCount != 5 {
				t.Fatalf("batch unbounded: %+v", diff)
			}
			out, err := e3Run(t, "watch", "run", "--db", path, "--name", name)
			if err != nil || !strings.Contains(out, "5 deferred") {
				t.Fatalf("cap not reported: %s %v", out, err)
			}
		}
	}
	if _, err := buildDocPayload("w", []store.DocItem{{Summary: strings.Repeat("x", 256*1024)}}, true); err == nil {
		t.Fatal("oversize payload accepted")
	}
}

func TestE3DeliveryFailure(t *testing.T) {
	testenv.Isolate(t)
	db, path := e3DB(t)
	ctx := context.Background()
	_, _, err := db.UpsertBatch("documents", []json.RawMessage{json.RawMessage(`{"id":"doc","title":"title"}`)})
	if err != nil {
		t.Fatal(err)
	}
	id, err := db.AddWatch(ctx, "failure", "docs", "{}")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	status := 503
	old := webhookClientFactory
	defer func() { webhookClientFactory = old }()
	webhookClientFactory = func() *http.Client {
		return &http.Client{Transport: e3Transport(func(r *http.Request) (*http.Response, error) {
			calls++
			code := status
			if status == 0 {
				code = 503
				if calls == 2 {
					code = 200
				}
			}
			return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		})}
	}
	// No matching receiver is a visible failure and retains seen state.
	out, err := e3Run(t, "watch", "run", "--db", path, "--notify", "--strict")
	if err == nil || !strings.Contains(out, "no matching webhook succeeded") {
		t.Fatalf("missing hooks hidden: %s %v", out, err)
	}
	seen, _ := db.SeenItemIDs(ctx, id, "docs")
	if len(seen) != 0 {
		t.Fatal("no matching hooks marked seen")
	}
	if _, err := db.AddWebhook(ctx, "https://example.com/hook", "", "docs", false); err != nil {
		t.Fatal(err)
	}
	out, err = e3Run(t, "watch", "run", "--db", path, "--notify", "--strict")
	if err == nil || calls != 2 {
		t.Fatalf("failure or retry hidden: calls=%d err=%v out=%s", calls, err, out)
	}
	seen, _ = db.SeenItemIDs(ctx, id, "docs")
	if len(seen) != 0 {
		t.Fatal("failed notification marked seen")
	}
	calls = 0
	status = 0
	out, err = e3Run(t, "watch", "run", "--db", path, "--notify", "--strict")
	if err != nil || calls != 2 {
		t.Fatalf("bounded retry did not recover: calls=%d err=%v out=%s", calls, err, out)
	}
	seen, _ = db.SeenItemIDs(ctx, id, "docs")
	if !seen["doc"] {
		t.Fatal("successful retry not marked seen")
	}
	hooks, err := db.ListWebhooks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hooks {
		if _, err := db.RemoveWebhook(ctx, h.ID); err != nil {
			t.Fatal(err)
		}
	}
	out, err = e3Run(t, "watch", "run", "--db", path, "--notify", "--strict")
	if err == nil || !strings.Contains(out, "no matching webhook") {
		t.Fatalf("empty watch with no hooks silent: %s %v", out, err)
	}
}

func TestE3WebhookDisclosure(t *testing.T) {
	testenv.Isolate(t)
	cmd := newWebhookAddCmd(&rootFlags{})
	for _, field := range []string{"id", "title", "author", "url", "text", "summary", "reading history"} {
		if !strings.Contains(cmd.Long, field) {
			t.Fatalf("help omits %s: %s", field, cmd.Long)
		}
	}
	cmd.SetArgs([]string{"https://example.com/hook", "--include-text"})
	cmd.SetIn(strings.NewReader("n\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatal("declined consent accepted")
	}
	for _, field := range []string{"title", "author", "url", "text", "summary", "reading history", "This endpoint WILL receive highlight text / document summaries."} {
		if !strings.Contains(out.String(), field) {
			t.Fatalf("consent omits %s: %s", field, out.String())
		}
	}
	noninteractive := newWebhookAddCmd(&rootFlags{noInput: true})
	noninteractive.SetArgs([]string{"https://example.com/hook"})
	if err := noninteractive.Execute(); err == nil {
		t.Fatal("noninteractive consent bypassed")
	}
	body, err := buildHLPayload("w", []store.HLItem{{ID: "1"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"title"`, `"author"`, `"url"`} {
		if !strings.Contains(string(body), field) {
			t.Fatalf("always sent field missing: %s", body)
		}
	}
}

func TestE3WebhookBlockedRanges(t *testing.T) {
	for _, ip := range []string{"100.64.0.1", "100.127.255.254", "0.1.2.3", "192.0.0.8", "198.18.0.1", "198.19.255.254", "240.1.2.3", "64:ff9b::808:808", "2002:a00:1::", "2002:c0a8:101::", "2002:6440:1::"} {
		if !isWatchBlockedIP(net.ParseIP(ip)) {
			t.Errorf("unsafe IP permitted: %s", ip)
		}
	}
	for _, ip := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111", "2002:808:808::"} {
		if isWatchBlockedIP(net.ParseIP(ip)) {
			t.Errorf("public IP rejected: %s", ip)
		}
	}
}

func TestE3WebhookRedirectSignature(t *testing.T) {
	client := safeWebhookClient()
	original, _ := http.NewRequest("POST", "https://example.com/secret", nil)
	same, _ := http.NewRequest("POST", "https://example.com/next", nil)
	same.Header.Set("X-Readwise-Signature", "signature")
	if err := client.CheckRedirect(same, []*http.Request{original}); err != nil || same.Header.Get("X-Readwise-Signature") == "" {
		t.Fatal("same host signature lost", err)
	}
	cross, _ := http.NewRequest("POST", "https://other.example/next", nil)
	cross.Header.Set("X-Readwise-Signature", "signature")
	if err := client.CheckRedirect(cross, []*http.Request{original}); err != nil {
		t.Fatal(err)
	}
	if cross.Header.Get("X-Readwise-Signature") != "" {
		t.Fatal("signature leaked across hosts")
	}
	// Go can re-copy initial headers on a later redirect; never resurrect consent.
	later, _ := http.NewRequest("POST", "https://other.example/third", nil)
	later.Header.Set("X-Readwise-Signature", "signature")
	if err := client.CheckRedirect(later, []*http.Request{original, cross}); err != nil {
		t.Fatal(err)
	}
	if later.Header.Get("X-Readwise-Signature") != "" {
		t.Fatal("signature resurrected on redirect chain")
	}
}

func TestE3WebhookURLRedaction(t *testing.T) {
	testenv.Isolate(t)
	db, path := e3DB(t)
	ctx := context.Background()
	target := "https://user:password@example.com/capability-token?key=query-token#fragment-token"
	if got := redactWebhookURL(target); got != "https://example.com/…" {
		t.Fatal(got)
	}
	id, err := db.AddWebhook(ctx, target, "", "*", true)
	if err != nil {
		t.Fatal(err)
	}
	assertClean := func(out string) {
		t.Helper()
		for _, secret := range []string{"password", "capability-token", "query-token", "fragment-token"} {
			if strings.Contains(out, secret) {
				t.Fatalf("URL secret exposed: %s", out)
			}
		}
	}
	out, err := e3Run(t, "webhook", "list", "--db", path)
	if err != nil {
		t.Fatal(err)
	}
	assertClean(out)
	out, err = e3Run(t, "webhook", "list", "--db", path, "--json")
	if err != nil || !strings.Contains(out, "capability-token") {
		t.Fatal("JSON URL contract lost", out, err)
	}
	old := webhookClientFactory
	defer func() { webhookClientFactory = old }()
	webhookClientFactory = func() *http.Client {
		return &http.Client{Transport: e3Transport(func(r *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("transport failed at %s", r.URL.String())
		})}
	}
	err = postWebhook(ctx, target, "", []byte("{}"))
	if err == nil {
		t.Fatal("transport unexpectedly succeeded")
	}
	assertClean(err.Error())
	if err := validWebhookURL("https://example.com/%bad-secret%"); err == nil {
		t.Fatal("invalid URL accepted")
	} else {
		assertClean(err.Error())
	}
	var log bytes.Buffer
	t.Setenv(cliutil.VerifyEnvVar, "1")
	if _, err := fireWatchWebhooks(ctx, &log, db, "docs", []byte("{}"), 1); err != nil {
		t.Fatal(err)
	}
	assertClean(log.String())
	t.Setenv(cliutil.VerifyEnvVar, "")
	webhookClientFactory = func() *http.Client {
		return &http.Client{Transport: e3Transport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		})}
	}
	out, err = e3Run(t, "webhook", "ping", fmt.Sprint(id), "--db", path)
	if err != nil {
		t.Fatal(err)
	}
	assertClean(out)
	_, _, err = db.UpsertBatch("documents", []json.RawMessage{json.RawMessage(`{"id":"redaction-doc"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.AddWatch(ctx, "redaction", "docs", "{}"); err != nil {
		t.Fatal(err)
	}
	out, err = e3Run(t, "watch", "run", "--notify", "--db", path)
	if err != nil {
		t.Fatal(err)
	}
	assertClean(out)
}

func TestE3WebhookSecretStdin(t *testing.T) {
	testenv.Isolate(t)
	db, path := e3DB(t)
	ctx := context.Background()
	cmd := newWebhookAddCmd(&rootFlags{yes: true})
	cmd.SetArgs([]string{"https://example.com/hook", "--db", path, "--secret-stdin"})
	cmd.SetIn(strings.NewReader("stdin-signing-secret\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	hooks, err := db.ListWebhooks(ctx)
	if err != nil || len(hooks) != 1 {
		t.Fatal(hooks, err)
	}
	hook, err := db.GetWebhook(ctx, hooks[0].ID)
	if err != nil || hook.Secret != "stdin-signing-secret" {
		t.Fatal("stdin secret not stored", err)
	}
	if strings.Contains(out.String(), "stdin-signing-secret") {
		t.Fatal("secret echoed")
	}
	conflict := newWebhookAddCmd(&rootFlags{yes: true})
	conflict.SetArgs([]string{"https://example.com/hook", "--db", path, "--secret-stdin", "--secret", "argv-secret"})
	conflict.SetIn(strings.NewReader("stdin-signing-secret\n"))
	if err := conflict.Execute(); err == nil {
		t.Fatal("conflicting secret sources accepted")
	}
}

func TestFPartialHookFailure(t *testing.T) {
	testenv.Isolate(t)
	db, path := e3DB(t)
	ctx := context.Background()
	if _, _, err := db.UpsertBatch("documents", []json.RawMessage{json.RawMessage(`{"id":"doc","title":"title"}`)}); err != nil {
		t.Fatal(err)
	}
	id, err := db.AddWatch(ctx, "partial", "docs", "{}")
	if err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"https://example.com/ok", "https://example.com/fail?secret=private"} {
		if _, err := db.AddWebhook(ctx, url, "", "docs", false); err != nil {
			t.Fatal(err)
		}
	}
	old := webhookClientFactory
	defer func() { webhookClientFactory = old }()
	calls := 0
	webhookClientFactory = func() *http.Client {
		return &http.Client{Transport: e3Transport(func(r *http.Request) (*http.Response, error) {
			calls++
			status := 200
			if r.URL.Path == "/fail" {
				status = 503
			}
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		})}
	}
	out, err := e3Run(t, "watch", "run", "--db", path, "--notify", "--strict")
	if err == nil || calls != 3 || !strings.Contains(out, "webhook #2 failed") || strings.Contains(out, "private") {
		t.Fatalf("partial delivery hidden: calls=%d out=%s err=%v", calls, out, err)
	}
	seen, err := db.SeenItemIDs(ctx, id, "docs")
	if err != nil || len(seen) != 0 {
		t.Fatalf("partial delivery consumed items: %v %v", seen, err)
	}
}

func TestFUnsyncedWatchWarning(t *testing.T) {
	testenv.Isolate(t)
	db, path := e3DB(t)
	out, err := e3Run(t, "watch", "add", "--db", path, "--name", "new", "--kind", "docs")
	if err != nil || !strings.Contains(out, "never completed a sync") {
		t.Fatalf("missing baseline warning: %s %v", out, err)
	}
	if err := db.SaveSyncState("documents", "", 0); err != nil {
		t.Fatal(err)
	}
	out, err = e3Run(t, "watch", "add", "--db", path, "--name", "synced", "--kind", "docs")
	if err != nil || strings.Contains(out, "never completed a sync") {
		t.Fatalf("incorrect baseline warning: %s %v", out, err)
	}
}

func TestFWatchBatchShrinks(t *testing.T) {
	testenv.Isolate(t)
	db, path := e3DB(t)
	ctx := context.Background()
	items := []json.RawMessage{}
	for i := 0; i < 8; i++ {
		raw, err := json.Marshal(map[string]any{"id": fmt.Sprintf("doc%d", i), "title": "title", "summary": strings.Repeat("s", 70000)})
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, raw)
	}
	if _, _, err := db.UpsertBatch("documents", items); err != nil {
		t.Fatal(err)
	}
	id, err := db.AddWatch(ctx, "large", "docs", "{}")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddWebhook(ctx, "https://example.com/hook", "", "docs", true); err != nil {
		t.Fatal(err)
	}
	old := webhookClientFactory
	defer func() { webhookClientFactory = old }()
	delivered := 0
	webhookClientFactory = func() *http.Client {
		return &http.Client{Transport: e3Transport(func(r *http.Request) (*http.Response, error) {
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if len(raw) > 256*1024 {
				t.Fatalf("oversized body: %d", len(raw))
			}
			var payload struct {
				Items []json.RawMessage `json:"items"`
			}
			if err := json.Unmarshal(raw, &payload); err != nil {
				t.Fatal(err)
			}
			delivered += len(payload.Items)
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		})}
	}
	for run := 0; run < 8; run++ {
		out, err := e3Run(t, "watch", "run", "--db", path, "--notify", "--strict")
		if err != nil {
			t.Fatalf("stuck batch: %s %v", out, err)
		}
	}
	seen, err := db.SeenItemIDs(ctx, id, "docs")
	if err != nil || len(seen) != 8 || delivered != 8 {
		t.Fatalf("seen=%d delivered=%d err=%v", len(seen), delivered, err)
	}
}
