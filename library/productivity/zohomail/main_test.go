package zohomail

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type lineWriter struct {
	mu sync.Mutex
	ch chan string
}

func newLineWriter() *lineWriter {
	return &lineWriter{ch: make(chan string, 20)}
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, line := range strings.Split(string(p), "\n") {
		if strings.TrimSpace(line) != "" {
			w.ch <- line
		}
	}
	return len(p), nil
}

func TestAccountsUsesZohoAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/accounts" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"status":{"code":200},"data":[{"accountId":"123","mailboxAddress":"me@example.com"}]}`))
	}))
	defer srv.Close()

	t.Setenv("ZOHO_MAIL_ACCESS_TOKEN", "abc")
	t.Setenv("ZOHO_MAIL_BASE_URL", srv.URL)

	var out strings.Builder
	if err := run([]string{"accounts"}, &out, &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Zoho-oauthtoken abc" {
		t.Fatalf("auth header = %q", gotAuth)
	}
	if !strings.Contains(out.String(), "accountId") || !strings.Contains(out.String(), "me@example.com") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestSendPayload(t *testing.T) {
	var payload map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/api/accounts/123/messages" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"status":{"code":200},"data":{"message":"sent"}}`))
	}))
	defer srv.Close()

	t.Setenv("ZOHO_MAIL_ACCESS_TOKEN", "abc")
	t.Setenv("ZOHO_MAIL_BASE_URL", srv.URL)

	var out strings.Builder
	err := run([]string{"send", "--account-id", "123", "--from", "me@example.com", "--to", "you@example.com", "--subject", "Hi", "--content", "Body"}, &out, &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	if payload["fromAddress"] != "me@example.com" || payload["toAddress"] != "you@example.com" || payload["content"] != "Body" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestRefreshTokenFlow(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/v2/token" {
			t.Fatalf("token path = %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("grant_type") != "refresh_token" {
			t.Fatalf("grant_type = %s", r.Form.Get("grant_type"))
		}
		_, _ = w.Write([]byte(`{"access_token":"fresh"}`))
	}))
	defer tokenSrv.Close()

	var gotAuth string
	mailSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"status":{"code":200},"data":[]}`))
	}))
	defer mailSrv.Close()

	t.Setenv("ZOHO_REFRESH_TOKEN", "refresh")
	t.Setenv("ZOHO_CLIENT_ID", "id")
	t.Setenv("ZOHO_CLIENT_SECRET", "secret")
	t.Setenv("ZOHO_ACCOUNTS_BASE_URL", tokenSrv.URL)
	t.Setenv("ZOHO_MAIL_BASE_URL", mailSrv.URL)

	if err := run([]string{"accounts"}, &strings.Builder{}, &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Zoho-oauthtoken fresh" {
		t.Fatalf("auth header = %q", gotAuth)
	}
}

func TestSelfClientTokenOmitsRedirectURI(t *testing.T) {
	var sawRedirect bool
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		sawRedirect = r.Form.Get("redirect_uri") != ""
		_, _ = w.Write([]byte(`{"access_token":"short","refresh_token":"refresh"}`))
	}))
	defer tokenSrv.Close()

	t.Setenv("ZOHO_CLIENT_ID", "id")
	t.Setenv("ZOHO_CLIENT_SECRET", "secret")
	t.Setenv("ZOHO_ACCOUNTS_BASE_URL", tokenSrv.URL)

	var out strings.Builder
	if err := run([]string{"token", "--self-client", "--code", "abc"}, &out, &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	if sawRedirect {
		t.Fatal("self-client token exchange sent redirect_uri")
	}
	if !strings.Contains(out.String(), "refresh_token") {
		t.Fatalf("missing refresh token output: %s", out.String())
	}
}

func TestConfigureDiscoversDefaults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/accounts":
			_, _ = w.Write([]byte(`{"status":{"code":200},"data":[{"accountId":"123","mailboxAddress":"me@example.com","isDefaultAccount":true}]}`))
		case "/api/accounts/123/folders":
			_, _ = w.Write([]byte(`{"status":{"code":200},"data":[{"folderId":"in1","folderName":"Inbox","path":"/Inbox"},{"folderId":"sent1","folderName":"Sent","path":"/Sent"},{"folderId":"arch1","folderName":"Archive","path":"/Archive"}]}`))
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("PP_ZOHOMAIL_CONFIG", cfgPath)
	t.Setenv("ZOHO_MAIL_ACCESS_TOKEN", "abc")
	t.Setenv("ZOHO_MAIL_BASE_URL", srv.URL)

	var out strings.Builder
	if err := run([]string{"configure"}, &out, &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"account_id": "123"`) || !strings.Contains(string(b), `"inbox": "in1"`) {
		t.Fatalf("config = %s", string(b))
	}
	if strings.Contains(string(b), "abc") {
		t.Fatalf("access token persisted: %s", string(b))
	}
}

func TestInboxUsesConfiguredFolder(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/accounts/123/messages/view" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"status":{"code":200},"data":[{"messageId":"m1","subject":"Hi"}]}`))
	}))
	defer srv.Close()

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"mail_base":"`+srv.URL+`","access_token":"ignored","account_id":"123","folders":{"inbox":"in1"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PP_ZOHOMAIL_CONFIG", cfgPath)
	t.Setenv("ZOHO_MAIL_ACCESS_TOKEN", "abc")

	var out strings.Builder
	if err := run([]string{"inbox", "--limit", "5"}, &out, &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "folderId=in1") || !strings.Contains(gotQuery, "limit=5") {
		t.Fatalf("query = %s", gotQuery)
	}
	if !strings.Contains(out.String(), "messageId") || !strings.Contains(out.String(), "Hi") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestMessageRowsPreferMessageColumnsOverFolderID(t *testing.T) {
	body := []byte(`{"status":{"code":200},"data":[{"folderId":"in1","messageId":"m1","subject":"Hi","fromAddress":"me@example.com"}]}`)
	var out strings.Builder
	if err := writeFormatted(&out, body, "pretty"); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("output = %s", out.String())
	}
	if lines[0] != "messageId\tsubject\tfromAddress\tfolderId" {
		t.Fatalf("header = %q output = %s", lines[0], out.String())
	}
	if !strings.Contains(lines[1], "m1\tHi\tme@example.com\tin1") {
		t.Fatalf("row = %q output = %s", lines[1], out.String())
	}
}

func TestConfigureOfflineKnownIDs(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("PP_ZOHOMAIL_CONFIG", cfgPath)

	if err := run([]string{"configure", "--account-id", "123", "--inbox-folder-id", "in1", "--sent-folder-id", "sent1"}, &strings.Builder{}, &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !strings.Contains(got, `"account_id": "123"`) || !strings.Contains(got, `"inbox": "in1"`) || !strings.Contains(got, `"sent": "sent1"`) {
		t.Fatalf("config = %s", got)
	}
}

func TestLoginBrowserFlowSavesAuthAndDefaults(t *testing.T) {
	mailSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/accounts":
			if r.Header.Get("Authorization") != "Zoho-oauthtoken fresh" {
				t.Fatalf("auth header = %q", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`{"status":{"code":200},"data":[{"accountId":"123","isDefaultAccount":true}]}`))
		case "/api/accounts/123/folders":
			_, _ = w.Write([]byte(`{"status":{"code":200},"data":[{"folderId":"in1","folderName":"Inbox","path":"/Inbox"},{"folderId":"sent1","folderName":"Sent","path":"/Sent"}]}`))
		default:
			t.Fatalf("mail path = %s", r.URL.Path)
		}
	}))
	defer mailSrv.Close()

	var authSrv *httptest.Server
	authSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/v2/auth":
			redirect := r.URL.Query().Get("redirect_uri")
			state := r.URL.Query().Get("state")
			http.Redirect(w, r, redirect+"?code=browser-code&state="+state, http.StatusFound)
		case "/oauth/v2/token":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("grant_type") == "refresh_token" {
				_, _ = w.Write([]byte(`{"access_token":"fresh"}`))
				return
			}
			if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "browser-code" {
				t.Fatalf("code = %s", r.Form.Get("code"))
			}
			_, _ = w.Write([]byte(`{"access_token":"fresh","refresh_token":"refresh"}`))
		default:
			t.Fatalf("auth path = %s", r.URL.Path)
		}
	}))
	defer authSrv.Close()

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	out := newLineWriter()
	done := make(chan error, 1)
	cfg := config{
		AccountsBase: authSrv.URL,
		MailBase:     mailSrv.URL,
		ClientID:     "id",
		ClientSecret: "secret",
		ConfigPath:   cfgPath,
		Folders:      map[string]string{},
		HTTPClient:   authSrv.Client(),
	}
	go func() {
		done <- login(out, &strings.Builder{}, cfg, "http://localhost:0/callback", "ZohoMail.accounts.READ", time.Minute, false)
	}()

	var openLine string
	select {
	case openLine = <-out.ch:
	case <-time.After(time.Second):
		t.Fatal("login did not print auth URL")
	}
	if !strings.HasPrefix(openLine, "open\t") {
		t.Fatalf("open line = %q", openLine)
	}
	resp, err := authSrv.Client().Get(strings.TrimPrefix(openLine, "open\t"))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("login did not finish")
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{`"refresh_token": "refresh"`, `"client_id": "id"`, `"client_secret": "secret"`, `"account_id": "123"`, `"inbox": "in1"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in config: %s", want, got)
		}
	}
}

func TestLoginAcceptsClientCredentialsFlags(t *testing.T) {
	var out strings.Builder
	err := run([]string{"login", "--client-id", "id", "--client-secret", "secret", "--redirect-uri", "https://example.com/callback"}, &out, &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "login redirect URI must use http localhost") {
		t.Fatalf("err = %v", err)
	}
}

func TestClientSetupPrintsConsoleURL(t *testing.T) {
	var out strings.Builder
	if err := run([]string{"client-setup", "--no-open"}, &out, &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "open\thttps://accounts.zoho.com/developerconsole") ||
		!strings.Contains(got, "redirect_uri\thttp://localhost:53682/callback") ||
		!strings.Contains(got, "ZohoMail.accounts.READ") {
		t.Fatalf("output = %s", got)
	}
}

func TestAuthSavePersistsExistingRefreshToken(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("PP_ZOHOMAIL_CONFIG", cfgPath)
	var out strings.Builder
	err := run([]string{"auth-save", "--client-id", "id", "--client-secret", "secret-value", "--refresh-token", "refresh-value", "--no-discover"}, &out, &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{`"client_id": "id"`, `"client_secret": "secret-value"`, `"refresh_token": "refresh-value"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in config: %s", want, got)
		}
	}
	if strings.Contains(out.String(), "secret-value") || strings.Contains(out.String(), "refresh-value") {
		t.Fatalf("secret leaked in output: %s", out.String())
	}
}

func TestAuthRBWSavesFieldsWithoutPrintingSecrets(t *testing.T) {
	dir := t.TempDir()
	rbwPath := filepath.Join(dir, "rbw")
	script := `#!/bin/sh
case "$4" in
  ZOHO_CLIENT_ID) printf 'id-value' ;;
  ZOHO_CLIENT_SECRET) printf 'secret-value' ;;
  ZOHO_REFRESH_TOKEN) printf 'refresh-value' ;;
  *) exit 2 ;;
esac
`
	if err := os.WriteFile(rbwPath, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "config.json")
	t.Setenv("PP_ZOHOMAIL_CONFIG", cfgPath)
	var out strings.Builder
	err := run([]string{"auth-rbw", "--item", "Zoho Mail OAuth", "--rbw-bin", rbwPath, "--no-discover"}, &out, &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{`"client_id": "id-value"`, `"client_secret": "secret-value"`, `"refresh_token": "refresh-value"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in config: %s", want, got)
		}
	}
	if strings.Contains(out.String(), "secret-value") || strings.Contains(out.String(), "refresh-value") {
		t.Fatalf("secret leaked in output: %s", out.String())
	}
}

func TestCallbackHandlerDropsExtraValidCallback(t *testing.T) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	codeCh <- "already-received"
	handler := callbackHandler("state", codeCh, errCh)

	done := make(chan struct{})
	go func() {
		defer close(done)
		req := httptest.NewRequest(http.MethodGet, "/callback?state=state&code=second", nil)
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("callback handler blocked on full code channel")
	}
}
