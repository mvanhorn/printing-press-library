package cli

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMintPortalSessionMatchesOfficialContract(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"jti":"request-id","sub":"account-id"}`))
	accountToken := header + "." + payload + ".account-signature"
	now := time.Unix(1_700_000_000, 0)

	session, err := mintPortalSession(accountToken, "user-id", "school", "@admin:layers-agenda", now)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(session, ".")
	if len(parts) != 3 {
		t.Fatalf("got %d JWT parts", len(parts))
	}
	mac := hmac.New(sha256.New, []byte("account-signature"))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if got, want := parts[2], base64.RawURLEncoding.EncodeToString(mac.Sum(nil)); got != want {
		t.Fatalf("signature = %q, want %q", got, want)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var claims portalSessionClaims
	if err := json.Unmarshal(decoded, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Kind != "session:portal" || claims.UserID != "user-id" || claims.Community != "school" || claims.PortalAlias != "@admin:layers-agenda" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.JTI != "request-id" || claims.Subject != "account-id" || claims.IssuedAt != now.Unix() {
		t.Fatalf("unexpected provenance claims: %+v", claims)
	}
}

func TestMintPortalSessionRejectsNonJWT(t *testing.T) {
	if _, err := mintPortalSession("not-a-jwt", "user", "school", "portal", time.Now()); err == nil {
		t.Fatal("expected malformed token error")
	}
}
