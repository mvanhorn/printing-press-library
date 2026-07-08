package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

const defaultTokenURL = "https://login.salesforce.com/services/oauth2/token"

type JWTOptions struct {
	TokenURL   string
	HTTPClient *http.Client
	Env        func(string) string
	Now        func() time.Time
}

func LoginJWT(ctx context.Context, opts JWTOptions) (*Result, error) {
	env := opts.Env
	if env == nil {
		env = os.Getenv
	}
	keyPath := env("SF360_JWT_KEY_PATH")
	if keyPath == "" {
		return nil, MissingEnvError("SF360_JWT_KEY_PATH")
	}
	clientID := env("SF360_JWT_CLIENT_ID")
	if clientID == "" {
		return nil, MissingEnvError("SF360_JWT_CLIENT_ID")
	}
	username := env("SF360_JWT_USERNAME")
	if username == "" {
		return nil, MissingEnvError("SF360_JWT_USERNAME")
	}
	key, err := readPrivateKey(keyPath)
	if err != nil {
		return nil, err
	}
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	assertion, err := signJWT(key, clientID, username, now())
	if err != nil {
		return nil, err
	}
	tokenURL := opts.TokenURL
	if tokenURL == "" {
		tokenURL = defaultTokenURL
	}
	resp, err := postForm(ctx, opts.HTTPClient, tokenURL, url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	})
	if err != nil {
		return nil, &Error{Kind: "token_http", Message: "exchanging JWT bearer assertion", Err: err}
	}
	return parseTokenHTTPResponse(resp, MethodJWT)
}

func readPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &Error{Kind: "jwt_key", Message: "reading JWT private key", Path: path, Err: err}
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, &Error{Kind: "jwt_key", Message: "JWT private key is not PEM encoded", Path: path}
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, &Error{Kind: "jwt_key", Message: "parsing JWT private key", Path: path, Err: err}
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, &Error{Kind: "jwt_key", Message: "JWT private key must be RSA", Path: path}
	}
	return key, nil
}

func signJWT(key *rsa.PrivateKey, clientID, username string, now time.Time) (string, error) {
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	claims := map[string]any{
		"iss": clientID,
		"sub": username,
		"aud": "https://login.salesforce.com",
		"exp": now.Add(3 * time.Minute).Unix(),
	}
	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", fmt.Errorf("signing JWT assertion: %w", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}
