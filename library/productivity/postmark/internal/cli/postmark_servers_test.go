// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/config"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPostmarkIsAccountPath(t *testing.T) {
	cases := map[string]bool{
		"/servers":                  true,
		"/servers/1234567":          true,
		"/domains/12/verifyDkim":    true,
		"/senders":                  true,
		"/templates/push":           true,
		"/data-removals/7":          true,
		"/server":                   false,
		"/templates":                false,
		"/templates/welcome":        false,
		"/messages/outbound":        false,
		"/message-streams/outbound": false,
		"/serversx":                 false,
	}
	for path, want := range cases {
		if got := postmarkIsAccountPath(path); got != want {
			t.Errorf("postmarkIsAccountPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestMatchPostmarkServer(t *testing.T) {
	refs := []postmarkServerRef{
		{ID: 1, Name: "Main App"},
		{ID: 2, Name: "Staging"},
		{ID: 3, Name: "Marketing Site"},
		{ID: 4, Name: "Billing"},
	}
	cases := []struct {
		want   string
		id     int64
		errSub string
	}{
		{want: "Main App", id: 1},
		{want: "main app", id: 1},
		{want: "3", id: 3},
		{want: "stag", id: 2},
		{want: "i", errSub: "more than one server"},
		{want: "Nope", errSub: "no server named"},
	}
	for _, tc := range cases {
		got, err := matchPostmarkServer(refs, tc.want)
		if tc.errSub != "" {
			if err == nil || !strings.Contains(err.Error(), tc.errSub) {
				t.Errorf("matchPostmarkServer(%q) err = %v, want containing %q", tc.want, err, tc.errSub)
			}
			continue
		}
		if err != nil || got.ID != tc.id {
			t.Errorf("matchPostmarkServer(%q) = %+v, %v; want ID %d", tc.want, got, err, tc.id)
		}
	}
}

func TestMaskPostmarkTokens(t *testing.T) {
	doc := map[string]any{
		"Name":      "Main App",
		"ApiTokens": []any{"00000000-0000-4000-8000-00000000c0de"},
		"Servers": []any{
			map[string]any{"ApiTokens": []any{"abc"}, "Name": "x"},
		},
	}
	maskPostmarkTokens(doc)
	if got := doc["ApiTokens"].([]any)[0]; got != "****c0de" {
		t.Errorf("top-level token = %v, want ****c0de", got)
	}
	nested := doc["Servers"].([]any)[0].(map[string]any)["ApiTokens"].([]any)[0]
	if nested != "****" {
		t.Errorf("short token = %v, want ****", nested)
	}
	if doc["Name"] != "Main App" {
		t.Errorf("non-token field changed: %v", doc["Name"])
	}
}

func TestPostmarkNeedsFirstPageOffset(t *testing.T) {
	mk := func(method, raw string) *http.Request {
		u, _ := url.Parse("https://api.postmarkapp.com" + raw)
		return &http.Request{Method: method, URL: u, Header: http.Header{}}
	}
	cases := []struct {
		req  *http.Request
		want bool
	}{
		{mk("GET", "/messages/outbound?count=100"), true},
		{mk("GET", "/templates?Count=100"), true},
		{mk("GET", "/messages/outbound?count=100&offset=200"), false},
		{mk("GET", "/templates?Count=100&Offset=0"), false},
		{mk("GET", "/server"), false},
		{mk("GET", "/email/bulk?count=20"), false},
		{mk("POST", "/email?count=1"), false},
	}
	for _, tc := range cases {
		if got := postmarkNeedsFirstPageOffset(tc.req); got != tc.want {
			t.Errorf("%s %s: got %v want %v", tc.req.Method, tc.req.URL, got, tc.want)
		}
	}
}

type recordingTransport struct{ last *http.Request }

func (r *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.last = req
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`)), Request: req}, nil
}

func TestPostmarkTokenRoutingTransport(t *testing.T) {
	rec := &recordingTransport{}
	rt := &postmarkTokenRoutingTransport{base: rec}
	send := func(path string) *http.Request {
		req, _ := http.NewRequest(http.MethodGet, "https://api.postmarkapp.com"+path, nil)
		req.Header.Set(postmarkServerTokenHeader, "server-token")
		req.Header.Set(postmarkAccountTokenHeader, "account-token")
		if _, err := rt.RoundTrip(req); err != nil {
			t.Fatalf("RoundTrip(%s): %v", path, err)
		}
		return rec.last
	}
	server := send("/server")
	if server.Header.Get(postmarkAccountTokenHeader) != "" || server.Header.Get(postmarkServerTokenHeader) == "" {
		t.Errorf("/server headers = %v, want server token only", server.Header)
	}
	account := send("/servers")
	if account.Header.Get(postmarkServerTokenHeader) != "" || account.Header.Get(postmarkAccountTokenHeader) == "" {
		t.Errorf("/servers headers = %v, want account token only", account.Header)
	}
}

type tokenBodyTransport struct{}

func (tokenBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body := `{"Name":"Main App","ApiTokens":["00000000-0000-4000-8000-00000000c0de"]}`
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

func TestPostmarkTokenMaskingIgnoresHeaderMarker(t *testing.T) {
	rt := &postmarkTokenRoutingTransport{base: tokenBodyTransport{}}
	read := func(req *http.Request) string {
		resp, err := rt.RoundTrip(req)
		if err != nil {
			t.Fatalf("RoundTrip: %v", err)
		}
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	injected, _ := http.NewRequest(http.MethodGet, "https://api.postmarkapp.com/server", nil)
	injected.Header.Set("X-Pp-Internal-Tokens", "1")
	if body := read(injected); strings.Contains(body, "00000000") {
		t.Errorf("header marker bypassed masking: %s", body)
	}
	internal, _ := http.NewRequestWithContext(withInternalTokens(context.Background()), http.MethodGet, "https://api.postmarkapp.com/server", nil)
	if body := read(internal); !strings.Contains(body, "00000000") {
		t.Errorf("internal context lookup was masked: %s", body)
	}
}

func TestKeepPostmarkArchiveOnFullSync(t *testing.T) {
	newSync := func() *cobra.Command {
		c := &cobra.Command{Use: "sync"}
		c.Flags().Bool("full", false, "")
		c.Flags().Bool("no-prune", false, "")
		return c
	}
	full := newSync()
	_ = full.Flags().Set("full", "true")
	if err := keepPostmarkArchiveOnFullSync(full); err != nil {
		t.Fatalf("sync --full: %v", err)
	}
	if got := full.Flags().Lookup("no-prune").Value.String(); got != "true" {
		t.Errorf("sync --full no-prune = %s, want true", got)
	}

	explicit := newSync()
	_ = explicit.Flags().Set("full", "true")
	_ = explicit.Flags().Set("no-prune", "false")
	err := keepPostmarkArchiveOnFullSync(explicit)
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("sync --full --no-prune=false: err = %v (exit %d), want a usage error", err, ExitCode(err))
	}

	incremental := newSync()
	_ = incremental.Flags().Set("no-prune", "false")
	if err := keepPostmarkArchiveOnFullSync(incremental); err != nil {
		t.Fatalf("incremental sync: %v", err)
	}
	if got := incremental.Flags().Lookup("no-prune").Value.String(); got != "false" {
		t.Errorf("non-full sync no-prune = %s, want false", got)
	}
}

func TestPostmarkTransportBlocksUnconfirmedSends(t *testing.T) {
	rec := &recordingTransport{}
	rt := &postmarkTokenRoutingTransport{base: rec}
	send := func(token string) error {
		req, _ := http.NewRequest(http.MethodPost, "https://api.postmarkapp.com/email/batch", strings.NewReader("[]"))
		req.Header.Set(postmarkServerTokenHeader, token)
		_, err := rt.RoundTrip(req)
		return err
	}
	postmarkSendAllowed.Store(false)
	if err := send("real-token"); err != errPostmarkSendNotConfirmed {
		t.Fatalf("unconfirmed send err = %v, want errPostmarkSendNotConfirmed", err)
	}
	if err := send(postmarkSandboxToken); err != nil {
		t.Fatalf("sandbox send err = %v, want nil", err)
	}
	postmarkSendAllowed.Store(true)
	defer postmarkSendAllowed.Store(false)
	if err := send("real-token"); err != nil {
		t.Fatalf("confirmed send err = %v, want nil", err)
	}
}

func TestServersTokensRevealsWithoutStoringAndListStaysMasked(t *testing.T) {
	f := newPostmarkFake(t)
	postmarkTestEnv(t, f)
	f.reply("GET /servers", 200, postmarkServersPayload([3]any{7, "Main App", "00000000-0000-4000-8000-00000000c0de"}))

	out, _, err := postmarkRun(t, "servers", "tokens", "Main App", "--json")
	if err != nil {
		t.Fatalf("servers tokens: %v", err)
	}
	if !strings.Contains(out, "00000000-0000-4000-8000-00000000c0de") {
		t.Errorf("servers tokens did not print the full token: %s", out)
	}

	out, _, err = postmarkRun(t, "servers", "list", "--count", "10", "--offset", "0", "--json")
	if err != nil {
		t.Fatalf("servers list: %v", err)
	}
	if strings.Contains(out, "00000000") || !strings.Contains(out, "****c0de") {
		t.Errorf("servers list leaked or failed to mask the token: %s", out)
	}

	if data, err := os.ReadFile(defaultDBPath("postmark-pp-cli")); err == nil && strings.Contains(string(data), "00000000") {
		t.Errorf("unmasked token reached the local store")
	}
}

func TestSyncFromAnotherServerWalksEverything(t *testing.T) {
	f := newPostmarkFake(t)
	home := postmarkTestEnv(t, f)
	f.reply("GET /servers", 200, postmarkServersPayload([3]any{1, "Main App", "tok-main"}, [3]any{2, "Staging", "tok-staging"}))
	db := filepath.Join(home, "fresh-dir", "archive.db") // the directory does not exist yet
	const note = "belonged to another server"
	run := func(server string) string {
		t.Helper()
		_, stderr, err := postmarkRun(t, "sync", "--server", server, "--resources", "servers", "--db", db)
		if err != nil {
			t.Fatalf("sync --server %s: %v\n%s", server, err, stderr)
		}
		return stderr
	}
	if stderr := run("Main App"); strings.Contains(stderr, note) {
		t.Fatalf("first sync into an empty archive should resume normally:\n%s", stderr)
	}
	if stderr := run("Staging"); !strings.Contains(stderr, note) {
		t.Fatalf("switching servers should reset the checkpoints:\n%s", stderr)
	}
	if stderr := run("Staging"); strings.Contains(stderr, note) {
		t.Fatalf("a second sync of the same server should resume:\n%s", stderr)
	}
}

func TestSetPostmarkServerTokenClearsOverrides(t *testing.T) {
	cfg := &config.Config{AuthHeaderVal: "saved-token", Headers: map[string]string{"x-postmark-server-token": "static-token", "X-Other": "keep"}}
	if got := postmarkEffectiveServerToken(cfg); got != "static-token" {
		t.Fatalf("effective token = %q, want the static header", got)
	}
	setPostmarkServerToken(cfg, "tok-staging")
	if got := postmarkEffectiveServerToken(cfg); got != "tok-staging" {
		t.Fatalf("after selecting a server the effective token = %q", got)
	}
	if cfg.AuthHeaderVal != "" || cfg.Headers["X-Other"] != "keep" || len(cfg.Headers) != 1 {
		t.Fatalf("overrides not cleared correctly: auth_header=%q headers=%v", cfg.AuthHeaderVal, cfg.Headers)
	}
	a := postmarkServerScope(&client.Client{Config: &config.Config{PostmarkServerToken: "tok-a"}})
	b := postmarkServerScope(&client.Client{Config: &config.Config{AuthHeaderVal: "tok-a"}})
	if a == "" || a != b || strings.Contains(a, "tok-a") {
		t.Fatalf("scope should hash the token actually sent: %q vs %q", a, b)
	}
}

func TestImportRefusesDataRemovals(t *testing.T) {
	f := newPostmarkFake(t)
	home := postmarkTestEnv(t, f)
	input := filepath.Join(home, "removals.jsonl")
	if err := os.WriteFile(input, []byte(`{"RequestedBy":"privacy@example.com","RequestedFor":"jane@example.com"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, resource := range []string{"data_removals", "data-removals"} {
		_, _, err := postmarkRun(t, "import", resource, "--input", input)
		if err == nil || ExitCode(err) != 2 {
			t.Fatalf("import %s: err = %v (exit %d), want a usage error", resource, err, ExitCode(err))
		}
	}
	for _, r := range f.log() {
		if r.Method == "POST" {
			t.Fatalf("import reached the API: %+v", r)
		}
	}
}

func TestPostmarkSyncLockWaitEndsWithContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.db.sync.lock")
	release, err := acquirePostmarkSyncLock(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	// A second open file handle stands in for another process.
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if _, err := acquirePostmarkSyncLock(ctx, path); err == nil || !strings.Contains(err.Error(), "another sync is still using this archive") {
		t.Fatalf("second acquire = %v, want a busy error once the context ends", err)
	}
	release()
	again, err := acquirePostmarkSyncLock(context.Background(), path)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	again()
}
