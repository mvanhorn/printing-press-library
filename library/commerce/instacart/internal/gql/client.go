package gql

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/store"
)

// GraphQLEndpoint is the US endpoint, kept for reference. Requests use
// Endpoint(), which follows the configured regional host.
const (
	GraphQLEndpoint = "https://www.instacart.com/graphql"
)

// Endpoint returns the GraphQL URL for the active Instacart host.
func Endpoint() string { return auth.BaseURL() + "/graphql" }

type Client struct {
	HTTP      *http.Client
	Cfg       *config.Config
	Session   *auth.Session
	Store     *store.Store
	Debug     bool
	UserAgent string
}

type Extensions struct {
	PersistedQuery *PersistedQuery `json:"persistedQuery,omitempty"`
}

type PersistedQuery struct {
	Version    int    `json:"version"`
	Sha256Hash string `json:"sha256Hash"`
}

type Response struct {
	Data       json.RawMessage `json:"data,omitempty"`
	Errors     []GraphQLError  `json:"errors,omitempty"`
	Extensions map[string]any  `json:"extensions,omitempty"`
	StatusCode int             `json:"-"`
	RawBody    []byte          `json:"-"`
}

type GraphQLError struct {
	Message    string         `json:"message"`
	Path       []any          `json:"path,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

func (e GraphQLError) Error() string { return e.Message }

func NewClient(sess *auth.Session, cfg *config.Config, st *store.Store) *Client {
	ua := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"
	if cfg != nil && cfg.UserAgent != "" {
		ua = cfg.UserAgent
	}
	return &Client{
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		Cfg:       cfg,
		Session:   sess,
		Store:     st,
		UserAgent: ua,
	}
}

// Query runs a persisted GraphQL query via GET. The operation must have a
// hash in the local store or explicitly provided.
func (c *Client) Query(ctx context.Context, opName string, variables any) (*Response, error) {
	return c.call(ctx, http.MethodGet, opName, variables, "")
}

// Mutation runs a persisted GraphQL mutation via POST.
func (c *Client) Mutation(ctx context.Context, opName string, variables any, query string) (*Response, error) {
	return c.call(ctx, http.MethodPost, opName, variables, query)
}

func (c *Client) call(ctx context.Context, method, opName string, variables any, query string) (*Response, error) {
	if c.Session == nil {
		return nil, errors.New("no session (run `instacart auth login`)")
	}

	hash := ""
	if c.Store != nil {
		h, err := c.Store.LookupOp(opName)
		if err == nil {
			hash = h
		}
	}
	if hash == "" && method == http.MethodGet {
		return nil, fmt.Errorf("no persisted query hash known for %q -- run `instacart capture` to refresh", opName)
	}

	varsJSON, err := json.Marshal(variables)
	if err != nil {
		return nil, fmt.Errorf("marshal variables: %w", err)
	}

	var req *http.Request
	if method == http.MethodGet {
		// GET + persisted query: hash in extensions, no query text.
		ext := Extensions{PersistedQuery: &PersistedQuery{Version: 1, Sha256Hash: hash}}
		extJSON, _ := json.Marshal(ext)
		u, err := url.Parse(Endpoint())
		if err != nil {
			return nil, err
		}
		q := u.Query()
		q.Set("operationName", opName)
		q.Set("variables", string(varsJSON))
		q.Set("extensions", string(extJSON))
		u.RawQuery = q.Encode()
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
	} else {
		// POST: if we have query text, send it as a plain graphql request
		// (no persistedQuery extension — Instacart returns
		// PersistedQueryNotSupported when both are sent). If no query text
		// but we have a hash, send just the hash.
		bodyMap := map[string]any{
			"operationName": opName,
			"variables":     json.RawMessage(varsJSON),
		}
		if query != "" {
			bodyMap["query"] = query
		} else if hash != "" {
			ext := Extensions{PersistedQuery: &PersistedQuery{Version: 1, Sha256Hash: hash}}
			extJSON, _ := json.Marshal(ext)
			bodyMap["extensions"] = json.RawMessage(extJSON)
		} else {
			return nil, fmt.Errorf("mutation %q has no query text and no persisted hash", opName)
		}
		body, err := json.Marshal(bodyMap)
		if err != nil {
			return nil, err
		}
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, Endpoint(), bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Origin", auth.BaseURL())
	req.Header.Set("Referer", auth.BaseURL()+"/")
	req.Header.Set("X-Client-Identifier", "web")
	req.Header.Set("apollographql-client-name", "@instacart/marketplace-web")
	req.Header.Set("apollographql-client-version", "unknown")
	c.Session.ApplyToRequest(req)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var out Response
	out.StatusCode = resp.StatusCode
	out.RawBody = raw
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			return &out, fmt.Errorf("decode response (status %d): %w", resp.StatusCode, err)
		}
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &out, fmt.Errorf("auth rejected (HTTP %d): session may be expired, re-run `instacart auth login`", resp.StatusCode)
	}
	// Handle stale persisted queries. Instacart returns:
	//   - PersistedQueryNotFound  - hash not in server allowlist; Apollo's default
	//     behavior is to retry with full query text
	//   - PersistedQueryNotSupported - server rejects both hash-only AND hash+query
	//     combinations for this operation. Happens after a bundle rotation.
	if len(out.Errors) > 0 {
		for _, e := range out.Errors {
			msg := strings.ToLower(e.Message)
			if strings.Contains(msg, "persistedquerynotfound") || strings.Contains(msg, "persisted query not found") {
				if query != "" && method == http.MethodGet {
					// Retry the query as a POST with full query text (Apollo-style APQ fallback).
					return c.call(ctx, http.MethodPost, opName, variables, query)
				}
				return &out, fmt.Errorf("%w: operation %q", ErrHashStale, opName)
			}
			if strings.Contains(msg, "persistedquerynotsupported") || strings.Contains(msg, "persisted_query_not_supported") {
				return &out, fmt.Errorf("%w: operation %q", ErrHashStale, opName)
			}
		}
		return &out, fmt.Errorf("graphql errors: %s", out.Errors[0].Message)
	}
	return &out, nil
}

// ErrHashStale is returned when the server rejects an operation because its
// persisted query hash no longer matches the current web bundle. Callers
// should wrap this into a user-facing message that points at `instacart
// capture` (or eventually `instacart capture --live`) to refresh.
var ErrHashStale = errors.New("persisted query hash is stale (Instacart rolled a new bundle) -- run `instacart capture` to refresh")

func Sha256(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
