package omnilogic

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	AuthLoginURL   = "https://services-gamma.haywardcloud.net/auth-service/v2/login"
	AuthRefreshURL = "https://services-gamma.haywardcloud.net/auth-service/v2/refresh"
	HaywardAppID   = "tzwqg83jvkyurxblidnepmachs"

	OpsURL = "https://www.haywardomnilogic.com/HAAPI/HomeAutomation/API.ashx"
)

// AuthState is the persisted token cache. The same shape is written to disk
// and consumed by the client constructor on next run.
type AuthState struct {
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	UserID       string    `json:"user_id"`
	Email        string    `json:"email,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	SavedAt      time.Time `json:"saved_at"`
}

func (s *AuthState) Valid() bool {
	return s != nil && s.Token != "" && time.Now().Before(s.ExpiresAt)
}

// defaultAuthCachePath returns the per-user token cache path. It lives in the
// CLI's config dir so the user can blow it away by deleting the config dir.
func defaultAuthCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "" // best effort; caller falls back to in-memory state
	}
	return filepath.Join(home, ".config", "hayward-omnilogic-pp-cli", "auth.json")
}

func loadAuthState(path string) (*AuthState, error) {
	if path == "" {
		return nil, errors.New("no auth cache path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s AuthState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func saveAuthState(path string, s *AuthState) error {
	if path == "" {
		return errors.New("no auth cache path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	s.SavedAt = time.Now()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

type loginPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token        string      `json:"token"`
	RefreshToken string      `json:"refreshToken"`
	UserID       json.Number `json:"userID"`
}

type refreshPayload struct {
	RefreshToken string `json:"refresh_token"`
}

// login executes the stage-1 REST JSON login. Email is what `HAYWARD_USER`
// holds in post-Oct-2025 v2 auth.
func (c *Client) login() (*AuthState, error) {
	body, _ := json.Marshal(loginPayload{Email: c.email, Password: c.password})
	req, _ := http.NewRequest("POST", AuthLoginURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-HAYWARD-APP-ID", HaywardAppID)
	if c.limiter != nil {
		c.limiter.Wait()
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()
	rawBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 429 {
		if c.limiter != nil {
			c.limiter.OnRateLimit()
		}
		return nil, &AuthError{StatusCode: resp.StatusCode, Body: "login rate-limited; retry after backoff: " + string(rawBody)}
	}
	if resp.StatusCode != 200 {
		return nil, &AuthError{StatusCode: resp.StatusCode, Body: string(rawBody)}
	}
	if c.limiter != nil {
		c.limiter.OnSuccess()
	}
	var lr loginResponse
	if err := json.Unmarshal(rawBody, &lr); err != nil {
		return nil, fmt.Errorf("parsing login response: %w", err)
	}
	if lr.Token == "" {
		return nil, &AuthError{StatusCode: resp.StatusCode, Body: "login response had no token"}
	}
	return &AuthState{
		Token:        lr.Token,
		RefreshToken: lr.RefreshToken,
		UserID:       lr.UserID.String(),
		Email:        c.email,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}, nil
}

// refresh trades a refresh_token for a fresh access token. Falls back to a
// full re-login if refresh fails.
func (c *Client) refresh(state *AuthState) (*AuthState, error) {
	if state == nil || state.RefreshToken == "" {
		return c.login()
	}
	body, _ := json.Marshal(refreshPayload{RefreshToken: state.RefreshToken})
	req, _ := http.NewRequest("POST", AuthRefreshURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-HAYWARD-APP-ID", HaywardAppID)
	if c.limiter != nil {
		c.limiter.Wait()
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return c.login()
	}
	defer resp.Body.Close()
	rawBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 429 {
		if c.limiter != nil {
			c.limiter.OnRateLimit()
		}
		return c.login()
	}
	if resp.StatusCode != 200 {
		return c.login()
	}
	if c.limiter != nil {
		c.limiter.OnSuccess()
	}
	var lr loginResponse
	if err := json.Unmarshal(rawBody, &lr); err != nil {
		return c.login()
	}
	// Refresh endpoint may return a fresh refreshToken or not — preserve the
	// existing one if absent.
	rt := lr.RefreshToken
	if rt == "" {
		rt = state.RefreshToken
	}
	uid := lr.UserID.String()
	if uid == "" {
		uid = state.UserID
	}
	return &AuthState{
		Token:        lr.Token,
		RefreshToken: rt,
		UserID:       uid,
		Email:        state.Email,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}, nil
}

// AuthError signals the v2 auth endpoint rejected the request. Carries the
// HTTP status and the body so `doctor` can surface actionable diagnostics.
type AuthError struct {
	StatusCode int
	Body       string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("hayward auth failed (HTTP %d): %s", e.StatusCode, e.Body)
}
