package offerup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanStripsTypenameAndTokens(t *testing.T) {
	in := map[string]any{
		"__typename":   "User",
		"id":           "42",
		"name":         "Bob",
		"sessionToken": "stub-value-1",
		"djangoToken":  "stub-value-2",
		"refreshToken": "stub-value-3",
		"apiSecret":    "stub-value-4",
		"password":     "hunter2",
		"profile": map[string]any{
			"__typename": "UserProfile",
			"city":       "Seattle",
			"authToken":  "stub-nested",
		},
		"listings": []any{
			map[string]any{"__typename": "Listing", "id": "1", "sellerToken": "stub-seller"},
		},
	}
	out, ok := clean(in).(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "42", out["id"])
	assert.Equal(t, "Bob", out["name"])
	for _, leaked := range []string{"__typename", "sessionToken", "djangoToken", "refreshToken", "apiSecret", "password"} {
		_, present := out[leaked]
		assert.Falsef(t, present, "%s must be stripped", leaked)
	}
	prof := out["profile"].(map[string]any)
	assert.Equal(t, "Seattle", prof["city"])
	_, tok := prof["authToken"]
	assert.False(t, tok, "nested token must be stripped")
	listing := out["listings"].([]any)[0].(map[string]any)
	_, sellerTok := listing["sellerToken"]
	assert.False(t, sellerTok, "token inside array element must be stripped")
	_, tn := listing["__typename"]
	assert.False(t, tn)
}

// fakePressAuth writes an executable stub that prints a cookie header for the
// `cookies` subcommand, so CookieHeader resolves without real press-auth.
func fakePressAuth(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "press-auth")
	script := "#!/bin/sh\nif [ \"$1\" = cookies ]; then printf 'ou_sess=test-cookie'; fi\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}

func TestAccountAuthPathStripsTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "ou_sess=test-cookie", r.Header.Get("Cookie"))
		assert.Equal(t, "GetUser", r.Header.Get("x-ou-operation-name"))
		_, _ = w.Write([]byte(`{"data":{"me":{"__typename":"User","id":"7","sessionToken":"stub-value","profile":{"__typename":"UserProfile","name":"Pat"}}}}`))
	}))
	defer srv.Close()
	t.Setenv("OFFERUP_BASE_URL", srv.URL)
	t.Setenv("PRESS_AUTH_BIN", fakePressAuth(t))

	c := NewClient(5*time.Second, 0)
	v, err := c.Account(context.Background())
	require.NoError(t, err)
	out := v.(map[string]any)
	assert.Equal(t, "7", out["id"])
	_, leaked := out["sessionToken"]
	assert.False(t, leaked, "Account output must not leak sessionToken")
	assert.Equal(t, "Pat", out["profile"].(map[string]any)["name"])
}

func TestGqlAuthSurfacesGraphQLErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errors":[{"message":"unauthenticated"}]}`))
	}))
	defer srv.Close()
	t.Setenv("OFFERUP_BASE_URL", srv.URL)
	t.Setenv("PRESS_AUTH_BIN", fakePressAuth(t))

	c := NewClient(5*time.Second, 0)
	_, err := c.MyListings(context.Background(), 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthenticated")
}

func TestAuthQueriesEmbedded(t *testing.T) {
	for _, op := range []string{"GetUser", "GetMySellingListings", "GetMyArchivedListings", "GetSavedLists", "GetChats", "GetChatDiscussion", "MarkListingAsSold", "ArchiveListing"} {
		assert.NotEmptyf(t, authQueries[op], "embedded auth query %q must load", op)
	}
}
