// PATCH: shared helper resolving the three-credential model (PAT / publishable / service_role) for the novel auth-admin, pgrst, and storage commands.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// projectSurface holds the credentials and base URL for talking to a single
// Supabase project's runtime APIs (Auth, Storage, PostgREST, Functions).
// The Management-API client emitted by the generator is unrelated — it
// targets api.supabase.com and uses a Personal Access Token.
type projectSurface struct {
	BaseURL    string // https://<ref>.supabase.co
	APIKey     string // SUPABASE_PUBLISHABLE_KEY or SUPABASE_ANON_KEY
	SecretKey  string // SUPABASE_SERVICE_ROLE_KEY or SUPABASE_SECRET_KEY (optional)
	ProjectRef string // <ref> derived from BaseURL
	httpClient *http.Client
}

// newProjectSurface reads project-surface credentials from env vars.
// Returns an error with a clear hint when required env is missing.
func newProjectSurface(requireSecret bool) (*projectSurface, error) {
	baseURL := strings.TrimRight(os.Getenv("SUPABASE_URL"), "/")
	if baseURL == "" {
		return nil, configErr(fmt.Errorf("SUPABASE_URL not set; export SUPABASE_URL=https://<ref>.supabase.co"))
	}
	if !strings.HasPrefix(baseURL, "https://") && !strings.HasPrefix(baseURL, "http://") {
		return nil, configErr(fmt.Errorf("SUPABASE_URL must include scheme: got %q, expected https://<ref>.supabase.co", baseURL))
	}
	apiKey := os.Getenv("SUPABASE_PUBLISHABLE_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("SUPABASE_ANON_KEY")
	}
	secretKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	if secretKey == "" {
		secretKey = os.Getenv("SUPABASE_SECRET_KEY")
	}
	if requireSecret && secretKey == "" {
		return nil, configErr(fmt.Errorf("SUPABASE_SERVICE_ROLE_KEY (or SUPABASE_SECRET_KEY) required for this command (Auth Admin, RLS-bypassing reads, etc.); the publishable key is RLS-only"))
	}
	if !requireSecret && apiKey == "" && secretKey == "" {
		return nil, configErr(fmt.Errorf("set SUPABASE_PUBLISHABLE_KEY (or legacy SUPABASE_ANON_KEY) for project API calls"))
	}
	return &projectSurface{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		SecretKey:  secretKey,
		ProjectRef: parseProjectRef(baseURL),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// parseProjectRef extracts the project ref from a Supabase project URL.
// https://<ref>.supabase.co → <ref>
func parseProjectRef(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	if i := strings.Index(host, "."); i > 0 {
		return host[:i]
	}
	return host
}

// do executes an HTTP request against the project surface. Uses the secret
// key when useSecret is true (required for Auth Admin and RLS-bypassing
// operations), else uses the publishable/anon key.
func (p *projectSurface) do(ctx context.Context, method, path string, body io.Reader, useSecret bool) ([]byte, int, error) {
	full := p.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, full, body)
	if err != nil {
		return nil, 0, fmt.Errorf("building request: %w", err)
	}
	key := p.APIKey
	if useSecret && p.SecretKey != "" {
		key = p.SecretKey
	}
	if key == "" {
		return nil, 0, configErr(fmt.Errorf("no project API key available for %s", path))
	}
	// Supabase project APIs require both apikey header AND Authorization Bearer
	// (with the same value for service_role to unlock admin endpoints).
	req.Header.Set("apikey", key)
	req.Header.Set("Authorization", "Bearer "+key)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return respBody, resp.StatusCode, &projectAPIError{Method: method, Path: path, StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	return respBody, resp.StatusCode, nil
}

// projectAPIError carries HTTP status for typed exit codes from project-surface calls.
// Distinct from client.APIError which serves the Management API path.
type projectAPIError struct {
	Method     string
	Path       string
	StatusCode int
	Body       string
}

func (e *projectAPIError) Error() string {
	return fmt.Sprintf("%s %s returned HTTP %d: %s", e.Method, e.Path, e.StatusCode, truncate(e.Body, 200))
}

// ensure json is referenced
var _ = json.Marshal
