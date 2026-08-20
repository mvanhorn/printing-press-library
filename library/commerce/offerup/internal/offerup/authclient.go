package offerup

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/config"
)

// ErrNotLoggedIn signals no captured OfferUp session is available via the
// generated cookie mechanism (config file or OFFERUP_COOKIE env var). The CLI
// maps it to a clear auth error (exit 4); live-dogfood treats it as a skip.
var ErrNotLoggedIn = errors.New("not logged in to OfferUp — run 'offerup-pp-cli auth login --chrome' (or set OFFERUP_COOKIE)")

// sessionCookie reads the captured OfferUp session cookie from the generated
// cookie store: the OFFERUP_COOKIE env var wins, then the cookie persisted by
// `auth login --chrome` / `auth set-token` in config.toml. It returns the raw
// "name=value; name=value" Cookie header value (never Bearer-prefixed — the
// authenticated GraphQL endpoint authenticates on the session cookie, not an
// Authorization header). The value is never logged.
func sessionCookie() (string, error) {
	cfg, err := config.Load("")
	if err != nil {
		return "", err
	}
	// config.Load maps OFFERUP_COOKIE into OfferupCookie and persists the
	// browser-captured cookie set into AccessToken (via SaveTokens). Either is
	// the raw Cookie header value; prefer the env override.
	if v := strings.TrimSpace(cfg.OfferupCookie); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(cfg.AccessToken); v != "" {
		return v, nil
	}
	return "", ErrNotLoggedIn
}

// authQueries holds the captured GraphQL query strings for OfferUp's
// authenticated operations, keyed by operationName. Captured from the live
// web app (full queries, not fragile persisted-query hashes).
//
//go:embed authqueries.json
var authQueriesRaw []byte

var authQueries map[string]string

func init() { _ = json.Unmarshal(authQueriesRaw, &authQueries) }

// gqlAuth POSTs an authenticated GraphQL operation to OfferUp's BFF with the
// captured session cookie and returns the response `data` object. The session
// cookie is the credential; the x-ou-operation-name header mirrors the web
// app but is not auth.
func (c *Client) gqlAuth(ctx context.Context, op string, vars map[string]any) (map[string]any, error) {
	q, ok := authQueries[op]
	if !ok {
		return nil, fmt.Errorf("unknown authenticated operation %q", op)
	}
	cookie, err := sessionCookie()
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(map[string]any{"operationName": op, "variables": vars, "query": q})
	if err != nil {
		return nil, err
	}
	url := c.baseURL + "/api/graphql"
	// Retry once on HTTP 429, mirroring the public fetchNextData path, so a
	// single transient throttle doesn't surface as a hard failure.
	for attempt := 0; attempt < 2; attempt++ {
		c.limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", chromeUA)
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Cookie", cookie)
		req.Header.Set("x-ou-operation-name", op)
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("calling %s: %w", op, err)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
			if attempt == 0 {
				wait := cliutil.RetryAfter(resp)
				select {
				case <-time.After(wait):
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return nil, &cliutil.RateLimitError{URL: url, RetryAfter: cliutil.RetryAfter(resp), Body: snippet(body)}
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s returned HTTP %d: %s", op, resp.StatusCode, snippet(body))
		}
		c.limiter.OnSuccess()
		var env struct {
			Data   map[string]any `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			return nil, fmt.Errorf("parsing %s response: %w", op, err)
		}
		if len(env.Errors) > 0 {
			msg := env.Errors[0].Message
			if msg == "" {
				msg = "authentication or request error"
			}
			return nil, fmt.Errorf("%s: %s (session may have expired — try 'offerup-pp-cli auth login --chrome')", op, msg)
		}
		return env.Data, nil
	}
	return nil, fmt.Errorf("unreachable: retry loop exited for %s", op)
}

// Account returns the authenticated user's own profile (tokens stripped).
func (c *Client) Account(ctx context.Context) (any, error) {
	d, err := c.gqlAuth(ctx, "GetUser", map[string]any{})
	if err != nil {
		return nil, err
	}
	return clean(dig(d, "me")), nil
}

// MyListings returns the authenticated user's active listings.
func (c *Client) MyListings(ctx context.Context, limit int) (any, error) {
	if limit <= 0 {
		limit = 20
	}
	d, err := c.gqlAuth(ctx, "GetMySellingListings", map[string]any{"limit": limit, "nextPageCursor": "", "sellFasterVariant": "control"})
	if err != nil {
		return nil, err
	}
	return clean(dig(d, "getMySellingItems", "listings")), nil
}

// ArchivedListings returns the authenticated user's archived/sold listings.
func (c *Client) ArchivedListings(ctx context.Context) (any, error) {
	d, err := c.gqlAuth(ctx, "GetMyArchivedListings", map[string]any{})
	if err != nil {
		return nil, err
	}
	return clean(dig(d, "archivedItems", "items")), nil
}

// SavedLists returns the authenticated user's saved/favorited lists.
func (c *Client) SavedLists(ctx context.Context) (any, error) {
	d, err := c.gqlAuth(ctx, "GetSavedLists", map[string]any{})
	if err != nil {
		return nil, err
	}
	return clean(dig(d, "savedLists")), nil
}

// Chats returns the authenticated user's message threads.
func (c *Client) Chats(ctx context.Context) (any, error) {
	d, err := c.gqlAuth(ctx, "GetChats", map[string]any{})
	if err != nil {
		return nil, err
	}
	return clean(dig(d, "getChats", "chats")), nil
}

// ChatDiscussion returns one conversation's messages by discussion id.
func (c *Client) ChatDiscussion(ctx context.Context, discussionID string) (any, error) {
	d, err := c.gqlAuth(ctx, "GetChatDiscussion", map[string]any{"input": map[string]any{"discussionId": discussionID}})
	if err != nil {
		return nil, err
	}
	return clean(d), nil
}

// MarkSold marks one of the user's listings as sold.
func (c *Client) MarkSold(ctx context.Context, id int64) (any, error) {
	d, err := c.gqlAuth(ctx, "MarkListingAsSold", map[string]any{"id": id})
	if err != nil {
		return nil, err
	}
	return clean(d), nil
}

// Archive archives one of the user's listings.
func (c *Client) Archive(ctx context.Context, id int64) (any, error) {
	d, err := c.gqlAuth(ctx, "ArchiveListing", map[string]any{"id": id})
	if err != nil {
		return nil, err
	}
	return clean(d), nil
}

// clean recursively drops GraphQL bookkeeping (__typename) and any sensitive
// credential fields (the authenticated `me` object embeds sessionToken /
// djangoToken / refreshToken — never surface those in output).
func clean(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			lk := strings.ToLower(k)
			if k == "__typename" || strings.Contains(lk, "token") || strings.Contains(lk, "password") || strings.Contains(lk, "secret") {
				continue
			}
			out[k] = clean(val)
		}
		return out
	case []any:
		for i := range t {
			t[i] = clean(t[i])
		}
		return t
	default:
		return v
	}
}
