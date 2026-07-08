package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const (
	MethodSFFallthrough = "sf_fallthrough"
	MethodJWT           = "jwt"
	MethodOAuthWeb      = "oauth_web"
)

// Result is the normalized credential material returned by every auth flow.
type Result struct {
	AccessToken  string
	RefreshToken string
	InstanceURL  string
	Scope        string
	TokenType    string
	ExpiresIn    int
	AuthMethod   string
}

type TokenRef struct {
	Service string `json:"service"`
	Key     string `json:"key"`
}

type StoredToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	InstanceURL  string    `json:"instance_url,omitempty"`
	Scope        string    `json:"scope,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

type Error struct {
	Kind     string
	Message  string
	Hint     string
	Variable string
	Path     string
	Err      error
}

func (e *Error) Error() string {
	msg := e.Message
	if e.Variable != "" {
		msg += ": " + e.Variable
	}
	if e.Path != "" {
		msg += ": " + e.Path
	}
	if e.Hint != "" {
		msg += " (hint: " + e.Hint + ")"
	}
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return msg
}

func (e *Error) Unwrap() error { return e.Err }

func MissingEnvError(variable string) error {
	return &Error{Kind: "missing_env", Message: "missing required environment variable", Variable: variable}
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	InstanceURL  string `json:"instance_url"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

func decodeTokenResponse(r io.Reader, method string) (*Result, error) {
	var tr tokenResponse
	if err := json.NewDecoder(r).Decode(&tr); err != nil {
		return nil, &Error{Kind: "token_parse", Message: "parsing token response", Err: err}
	}
	if tr.AccessToken == "" {
		return nil, &Error{Kind: "token_parse", Message: "token response missing access_token"}
	}
	return &Result{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		InstanceURL:  tr.InstanceURL,
		Scope:        tr.Scope,
		TokenType:    tr.TokenType,
		ExpiresIn:    tr.ExpiresIn,
		AuthMethod:   method,
	}, nil
}

func postForm(ctx context.Context, hc *http.Client, endpoint string, values url.Values) (*http.Response, error) {
	if hc == nil {
		hc = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, stringsReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return hc.Do(req)
}

func parseTokenHTTPResponse(resp *http.Response, method string) (*Result, error) {
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, &Error{
			Kind:    "token_http",
			Message: fmt.Sprintf("token endpoint returned HTTP %d", resp.StatusCode),
			Err:     fmt.Errorf("%s", string(body)),
		}
	}
	return decodeTokenResponse(resp.Body, method)
}

func RefreshToken(ctx context.Context, hc *http.Client, tokenURL, clientID, clientSecret, refreshToken string) (*Result, error) {
	values := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
	}
	if clientSecret != "" {
		values.Set("client_secret", clientSecret)
	}
	resp, err := postForm(ctx, hc, tokenURL, values)
	if err != nil {
		return nil, &Error{Kind: "token_http", Message: "refreshing access token", Err: err}
	}
	return parseTokenHTTPResponse(resp, "")
}

func StoreToken(ref TokenRef, token StoredToken) error {
	if ref.Service == "" || ref.Key == "" {
		return &Error{Kind: "token_ref", Message: "tokenRef requires service and key"}
	}
	path, err := tokenPath(ref)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating token store: %w", err)
	}
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling token: %w", err)
	}
	// File-backed storage remains the dependency-free token store until the
	// project adopts an OS keychain package.
	return os.WriteFile(path, data, 0o600)
}

func LoadStoredToken(ref TokenRef) (*StoredToken, error) {
	path, err := tokenPath(ref)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var token StoredToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("parsing stored token: %w", err)
	}
	return &token, nil
}

func DeleteStoredToken(ref TokenRef) error {
	path, err := tokenPath(ref)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func tokenPath(ref TokenRef) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home dir: %w", err)
	}
	service := filepath.Base(ref.Service)
	key := filepath.Base(ref.Key)
	return filepath.Join(home, ".salesforce-headless-360-pp-cli", "tokens", service, key+".json"), nil
}

type stringReader struct{ s string }

func stringsReader(s string) *stringReader { return &stringReader{s: s} }

func (r *stringReader) Read(p []byte) (int, error) {
	if r.s == "" {
		return 0, io.EOF
	}
	n := copy(p, r.s)
	r.s = r.s[n:]
	return n, nil
}
