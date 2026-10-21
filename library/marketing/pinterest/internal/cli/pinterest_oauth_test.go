// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/pinterest/internal/cliutil/testenv"
)

func TestOAuthTokenCommandUsesBasicClientAuth(t *testing.T) {
	testenv.Isolate(t)
	var gotAuth, gotPath string
	srv := newLoopbackTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		_, _ = w.Write([]byte(`{"access_token":"x","token_type":"bearer"}`))
	}))
	t.Setenv("PINTEREST_BASE_URL", srv.URL)
	t.Setenv("PINTEREST_ACCESS_TOKEN", "user-bearer-token")
	t.Setenv("PINTEREST_CLIENT_ID", "app-id")
	t.Setenv("PINTEREST_CLIENT_SECRET", "app-secret")
	if out, err := runPinterestCmd(t, "", "oauth", "token", "--grant-type", "refresh_token", "--refresh-token", "r", "--json"); err != nil {
		t.Fatalf("oauth token: %v\n%s", err, out)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("app-id:app-secret"))
	if gotPath != "/oauth/token" || gotAuth != want {
		t.Fatalf("path=%q auth=%q, want Basic client auth (not the bearer token)", gotPath, gotAuth)
	}
}

func TestOAuthTokenCommandRequiresClientCredentials(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("PINTEREST_ACCESS_TOKEN", "user-bearer-token")
	t.Setenv("PINTEREST_CLIENT_ID", "")
	t.Setenv("PINTEREST_CLIENT_SECRET", "")
	_, err := runPinterestCmd(t, "", "oauth", "token-revoke", "--token", "t", "--token-type-hint", "access_token", "--json")
	if err == nil || !strings.Contains(err.Error(), "PINTEREST_CLIENT_ID") {
		t.Fatalf("expected a client-credentials usage error, got %v", err)
	}
}

func TestExportReadConfigUsesBookmark(t *testing.T) {
	c := resourceReadConfigFor("boards")
	if c.cursorParam != "bookmark" || c.nextCursorPath != "bookmark" || c.paginationType != "cursor" {
		t.Fatalf("boards export config = %+v, want bookmark cursor", c)
	}
}
