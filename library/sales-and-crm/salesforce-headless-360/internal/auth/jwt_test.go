package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoginJWTPostsSignedAssertion(t *testing.T) {
	keyPath := writeTestRSAKey(t)
	var assertion string
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
		if got := r.Form.Get("grant_type"); got != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
			t.Fatalf("grant_type = %q", got)
		}
		assertion = r.Form.Get("assertion")
		return jsonResponse(`{"access_token":"jwt-access","instance_url":"https://jwt.example","scope":"api","token_type":"Bearer","expires_in":900}`), nil
	})}

	env := map[string]string{
		"SF360_JWT_KEY_PATH":  keyPath,
		"SF360_JWT_CLIENT_ID": "client-123",
		"SF360_JWT_USERNAME":  "user@example.com",
	}
	result, err := LoginJWT(context.Background(), JWTOptions{
		TokenURL:   "https://login.example/services/oauth2/token",
		HTTPClient: client,
		Env:        func(k string) string { return env[k] },
		Now:        func() time.Time { return time.Unix(1000, 0).UTC() },
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.AccessToken != "jwt-access" || result.AuthMethod != MethodJWT {
		t.Fatalf("unexpected result: %+v", result)
	}
	parts := strings.Split(assertion, ".")
	if len(parts) != 3 {
		t.Fatalf("assertion is not a JWT: %q", assertion)
	}
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var claims map[string]any
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		t.Fatal(err)
	}
	if claims["iss"] != "client-123" || claims["sub"] != "user@example.com" || claims["aud"] != "https://login.salesforce.com" {
		t.Fatalf("unexpected claims: %v", claims)
	}
	if int64(claims["exp"].(float64)) != time.Unix(1000, 0).Add(3*time.Minute).Unix() {
		t.Fatalf("unexpected exp: %v", claims["exp"])
	}
}

func TestLoginJWTMissingEnvNamesVariable(t *testing.T) {
	_, err := LoginJWT(context.Background(), JWTOptions{Env: func(string) string { return "" }})
	if err == nil || !strings.Contains(err.Error(), "SF360_JWT_KEY_PATH") {
		t.Fatalf("expected missing key path error, got %v", err)
	}
}

func TestLoginJWTUnreadableKeyIncludesPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.pem")
	env := map[string]string{
		"SF360_JWT_KEY_PATH":  path,
		"SF360_JWT_CLIENT_ID": "client-123",
		"SF360_JWT_USERNAME":  "user@example.com",
	}
	_, err := LoginJWT(context.Background(), JWTOptions{Env: func(k string) string { return env[k] }})
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("expected path in error, got %v", err)
	}
}

func writeTestRSAKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	path := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

type roundTrip func(*http.Request) (*http.Response, error)

func (rt roundTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt(r)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
