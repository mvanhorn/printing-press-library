// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package granola

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/cliutil"
)

// InternalBaseURL is Granola's internal API root. The API has both /v1
// and /v2 surfaces; the typed methods below pin to whichever surface the
// community clients use successfully today.
const InternalBaseURL = "https://api.granola.ai"

// granolaUserAgent identifies the CLI to the internal API. Granola returns
// "Unsupported client" without a recognizable User-Agent; the form
// "Granola/<ver> (<os>; <client-name>)" passes the gate as of 2026-05.
const granolaClientVersion = "7.299.0"
const granolaPlatform = "darwin"
const granolaUserAgent = "Granola/7.299.0 (macOS; granola-pp-cli)"

// InternalClient is the typed wrapper for Granola's internal API. It is
// auto-discovering: NewInternalClient reads the token from the Granola
// desktop app's on-disk state, rotates it on 401 exactly once, and threads
// through the rotated pair for subsequent calls in this process.
type InternalClient struct {
	baseURL    string
	httpClient *http.Client
	limiter    *cliutil.AdaptiveLimiter
	// refresh-once mutex protects the on-401 refresh path so a burst of
	// parallel requests doesn't all hit WorkOS in lockstep.
	refreshMu sync.Mutex
}

// defaultInternalAPIRateLimit is a conservative default for Granola's internal
// API. The real ceiling is unpublished, so we pace at 2 req/s and let the
// AdaptiveLimiter halve on observed 429s.
const defaultInternalAPIRateLimit = 2.0

// NewInternalClient builds an InternalClient. It does NOT touch the
// network; the first request lazily loads the token.
func NewInternalClient() (*InternalClient, error) {
	return &InternalClient{
		baseURL:    InternalBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		limiter:    cliutil.NewAdaptiveLimiter(defaultInternalAPIRateLimit),
	}, nil
}

// SetHTTPClient swaps the HTTP client (used by tests).
func (c *InternalClient) SetHTTPClient(h *http.Client) {
	c.httpClient = h
}

// SetBaseURL overrides the base URL (used by tests pointing at httptest
// servers).
func (c *InternalClient) SetBaseURL(u string) {
	c.baseURL = u
}

// post sends a JSON POST to path, handling gzip, refresh-on-401, and the
// canonical Granola headers. Returns the raw decompressed body bytes.
func (c *InternalClient) post(path string, body any) ([]byte, error) {
	return c.postAttempt(path, body, false)
}

func (c *InternalClient) postAttempt(path string, body any, isRetry bool) ([]byte, error) {
	access, _, err := LoadAccessToken()
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}
	req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("User-Agent", granolaUserAgent)
	req.Header.Set("X-Client-Version", granolaClientVersion)
	req.Header.Set("X-Granola-Platform", granolaPlatform)

	// Pace requests via the AdaptiveLimiter. Granola's internal API has no
	// published rate limit; the limiter starts at defaultInternalAPIRateLimit
	// req/s and self-adjusts on observed 429s.
	c.limiter.Wait()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("granola %s: %w", path, err)
	}
	defer resp.Body.Close()

	// Typed 429 handling — surface throttling as cliutil.RateLimitError so the
	// caller can distinguish "rate limited" from "no data" rather than silently
	// returning an empty body. Honor Retry-After when present; otherwise the
	// AdaptiveLimiter halves its rate for subsequent requests.
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		wait := cliutil.RetryAfter(resp)
		fmt.Fprintf(os.Stderr, "granola %s: rate limited, AdaptiveLimiter at %.1f req/s, Retry-After=%s\n", path, c.limiter.Rate(), wait)
		drained, _ := io.ReadAll(resp.Body)
		return nil, &cliutil.RateLimitError{
			URL:        c.baseURL + path,
			RetryAfter: wait,
			Body:       truncate(string(drained), 500),
		}
	}
	c.limiter.OnSuccess()

	var reader io.Reader = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gz, gerr := gzip.NewReader(resp.Body)
		if gerr != nil {
			return nil, fmt.Errorf("granola %s: gzip: %w", path, gerr)
		}
		defer gz.Close()
		reader = gz
	}
	respBody, _ := io.ReadAll(reader)

	if resp.StatusCode == http.StatusUnauthorized && !isRetry {
		// Single-shot refresh-and-retry. WorkOS rotates refresh tokens
		// single-use; we mutex to prevent stampedes.
		c.refreshMu.Lock()
		// Re-check inside the lock: another goroutine may have already
		// refreshed.
		newAccess, _, _ := LoadAccessToken()
		if newAccess == access {
			refresh, rerr := LoadRefreshToken()
			if rerr != nil {
				c.refreshMu.Unlock()
				return nil, fmt.Errorf("granola %s: 401 and no refresh token available: %w", path, rerr)
			}
			if _, err := RefreshAccessToken(refresh); err != nil {
				c.refreshMu.Unlock()
				return nil, fmt.Errorf("granola %s: refresh failed: %w", path, err)
			}
		}
		c.refreshMu.Unlock()
		return c.postAttempt(path, body, true)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("granola %s: status %d: %s", path, resp.StatusCode, truncate(string(respBody), 500))
	}
	return respBody, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}

// PATCH(api-catalog-refresh): endpoints for the catalog surfaces. Named
// constants because each one appears three times -- the request, the shape
// error, and the test that pins the path -- and a typo in any of them reads
// as "Granola changed the payload" rather than "we asked the wrong URL".
//
// The document-list surface is v2. /v1/get-document-lists answers 404 "Not
// implemented" on the live API, so the fallback below is a legacy-install
// courtesy, not the primary path.
const (
	recipesEndpoint               = "/v1/get-recipes"
	panelTemplatesEndpoint        = "/v1/get-panel-templates"
	documentListsEndpoint         = "/v2/get-document-lists"
	documentListsLegacyEndpoint   = "/v1/get-document-lists"
	documentListsNotImplemented   = "status 404"
	shapeErrorBodySampleMaxLength = 200
)

// APIShapeError reports a response body the client could not map onto any
// known shape.
//
// Typed rather than a bare fmt.Errorf so a caller that makes several calls in
// one pass can tell "Granola changed this payload" apart from a transport,
// auth or rate-limit failure -- the first needs a code change here, the others
// need the user to do something. Endpoint is a field and not only prose in the
// message for the same reason: the catalog refresh calls three endpoints
// together, and "unrecognized response shape" without a name leaves both the
// operator and the test assertion guessing which one drifted.
type APIShapeError struct {
	// Endpoint is the API path whose response could not be parsed.
	Endpoint string
	// Body is a truncated sample of what came back, so a bug report carries
	// the actual drift rather than a description of it.
	Body string
}

func (e *APIShapeError) Error() string {
	return fmt.Sprintf("granola %s: unrecognized response shape: %s", e.Endpoint, e.Body)
}

func shapeError(endpoint string, raw []byte) error {
	return &APIShapeError{Endpoint: endpoint, Body: truncate(string(raw), shapeErrorBodySampleMaxLength)}
}

// GetDocumentsRequest is the body for /v2/get-documents.
type GetDocumentsRequest struct {
	Limit                  int  `json:"limit,omitempty"`
	Offset                 int  `json:"offset,omitempty"`
	IncludeLastViewedPanel bool `json:"include_last_viewed_panel,omitempty"`
}

// GetDocumentsResponse is the response from /v2/get-documents.
type GetDocumentsResponse struct {
	Docs       []Document `json:"docs,omitempty"`
	Cursor     string     `json:"cursor,omitempty"`
	HasMore    bool       `json:"has_more,omitempty"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

// GetDocuments calls /v2/get-documents and returns the documents. The
// internal API paginates; the caller is responsible for offset/limit
// management.
func (c *InternalClient) GetDocuments(limit, offset int, includePanel bool) ([]Document, error) {
	env, err := c.GetDocumentsPage(limit, offset, includePanel)
	if err != nil {
		return nil, err
	}
	return env.Docs, nil
}

// GetDocumentsPage calls /v2/get-documents and returns the full
// response envelope including pagination flags (HasMore, NextCursor).
// Callers that page through the endpoint should prefer this method so
// they can terminate on has_more=false rather than guessing from
// returned-row count. Both response shapes Granola ships (bare array
// and wrapped {docs:[]}) are handled; bare-array responses surface as
// HasMore=false because the bare shape carries no pagination metadata.
func (c *InternalClient) GetDocumentsPage(limit, offset int, includePanel bool) (GetDocumentsResponse, error) {
	if limit <= 0 {
		limit = 100
	}
	body := GetDocumentsRequest{Limit: limit, Offset: offset, IncludeLastViewedPanel: includePanel}
	raw, err := c.post("/v2/get-documents", body)
	if err != nil {
		return GetDocumentsResponse{}, err
	}
	var env GetDocumentsResponse
	if err := json.Unmarshal(raw, &env); err == nil && env.Docs != nil {
		return env, nil
	}
	var arr []Document
	if err := json.Unmarshal(raw, &arr); err == nil {
		return GetDocumentsResponse{Docs: arr}, nil
	}
	return GetDocumentsResponse{}, fmt.Errorf("granola get-documents: unrecognized response shape: %s", truncate(string(raw), 200))
}

// GetDocumentsBatch calls /v1/get-documents-batch with the given ids.
func (c *InternalClient) GetDocumentsBatch(ids []string) ([]Document, error) {
	body := map[string]any{"document_ids": ids}
	raw, err := c.post("/v1/get-documents-batch", body)
	if err != nil {
		return nil, err
	}
	var env struct {
		Docs []Document `json:"docs"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && env.Docs != nil {
		return env.Docs, nil
	}
	var arr []Document
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	return nil, fmt.Errorf("granola get-documents-batch: unrecognized response shape: %s", truncate(string(raw), 200))
}

// GetDocumentTranscript fetches the transcript for one doc.
func (c *InternalClient) GetDocumentTranscript(id string) ([]TranscriptSegment, error) {
	body := map[string]string{"document_id": id}
	raw, err := c.post("/v1/get-document-transcript", body)
	if err != nil {
		return nil, err
	}
	// Two shapes: bare array, or {transcript:[]}.
	var env struct {
		Transcript []TranscriptSegment `json:"transcript"`
		Segments   []TranscriptSegment `json:"segments"`
	}
	if err := json.Unmarshal(raw, &env); err == nil {
		if env.Transcript != nil {
			return env.Transcript, nil
		}
		if env.Segments != nil {
			return env.Segments, nil
		}
	}
	var arr []TranscriptSegment
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	return nil, fmt.Errorf("granola get-document-transcript: unrecognized response shape")
}

// GetDocumentPanels fetches AI panels for one doc. Returns map of
// panel slug or template id -> rendered markdown content.
func (c *InternalClient) GetDocumentPanels(documentID string) (map[string]string, error) {
	body := map[string]string{"document_id": documentID}
	raw, err := c.post("/v1/get-document-panels", body)
	if err != nil {
		return nil, err
	}
	// Granola has used multiple shapes here historically. The current
	// shape is a list of panel objects.
	type panelObj struct {
		ID             string          `json:"id,omitempty"`
		Slug           string          `json:"slug,omitempty"`
		Title          string          `json:"title,omitempty"`
		TemplateID     string          `json:"template_id,omitempty"`
		TemplateSlug   string          `json:"template_slug,omitempty"`
		Content        json.RawMessage `json:"content,omitempty"`
		MarkdownOutput string          `json:"markdown_output,omitempty"`
		Output         string          `json:"output,omitempty"`
	}
	out := map[string]string{}
	var arr []panelObj
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		for _, p := range arr {
			key := firstNonEmpty(p.Slug, p.TemplateSlug, p.ID, p.TemplateID, p.Title)
			val := firstNonEmpty(p.MarkdownOutput, p.Output)
			if val == "" && len(p.Content) > 0 {
				// Content might be TipTap JSON.
				if rendered, rerr := Render(p.Content); rerr == nil {
					val = rendered
				} else {
					val = string(p.Content)
				}
			}
			if key != "" {
				out[key] = val
			}
		}
		return out, nil
	}
	var env struct {
		Panels []panelObj `json:"panels"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && env.Panels != nil {
		for _, p := range env.Panels {
			key := firstNonEmpty(p.Slug, p.TemplateSlug, p.ID, p.TemplateID, p.Title)
			val := firstNonEmpty(p.MarkdownOutput, p.Output)
			if val == "" && len(p.Content) > 0 {
				if rendered, rerr := Render(p.Content); rerr == nil {
					val = rendered
				} else {
					val = string(p.Content)
				}
			}
			if key != "" {
				out[key] = val
			}
		}
		return out, nil
	}
	// Last shape: map.
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asMap); err == nil {
		for k, v := range asMap {
			var s string
			if err := json.Unmarshal(v, &s); err == nil {
				out[k] = s
			} else {
				out[k] = string(v)
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("granola get-document-panels: unrecognized response shape")
}

// GetWorkspaces lists workspaces.
func (c *InternalClient) GetWorkspaces() ([]Workspace, error) {
	raw, err := c.post("/v1/get-workspaces", map[string]any{})
	if err != nil {
		return nil, err
	}
	var env struct {
		Workspaces []Workspace `json:"workspaces"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && env.Workspaces != nil {
		return env.Workspaces, nil
	}
	var arr []Workspace
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	return nil, fmt.Errorf("granola get-workspaces: unrecognized response shape")
}

// GetDocumentLists lists folders, with each list's membership inline on
// DocumentListMetadata.Documents. Tries /v2 first then falls back to /v1 on
// 404 — Granola has shipped both.
//
// PATCH(api-catalog-refresh): the fallback used to fire on any 4xx, which
// meant a 401 on /v2 was retried against a /v1 that answers 404 "Not
// implemented" — so an expired credential surfaced as a missing endpoint and
// the real remedy never reached the user. Only a 404 means "this surface is
// not here"; every other 4xx is about the caller, not the path.
func (c *InternalClient) GetDocumentLists() ([]DocumentListMetadata, error) {
	raw, err := c.post(documentListsEndpoint, map[string]any{})
	if err != nil {
		if !strings.Contains(err.Error(), documentListsNotImplemented) {
			return nil, err
		}
		raw, err = c.post(documentListsLegacyEndpoint, map[string]any{})
		if err != nil {
			return nil, err
		}
	}
	return parseDocumentLists(raw)
}

// parseDocumentLists accepts both shapes Granola has shipped: the live
// {"lists":[...]} envelope and the bare array older builds returned.
func parseDocumentLists(raw []byte) ([]DocumentListMetadata, error) {
	var env struct {
		Lists []DocumentListMetadata `json:"lists"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && env.Lists != nil {
		return env.Lists, nil
	}
	var arr []DocumentListMetadata
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	return nil, shapeError(documentListsEndpoint, raw)
}

// RecipeCatalog is the /v1/get-recipes payload: every recipe collection the
// account can see, plus the usage counters keyed by recipe id.
//
// UnlistedRecipes and DefaultRecipes are decoded but deliberately not
// hydrated into the Cache: the desktop cache has no counterpart key for
// either, and writing rows that only ever appear on API-refreshed installs
// would make the two sync paths disagree about what the catalog contains
// while nothing retires the extras. They are kept on the typed surface so the
// live shape stays documented at the call site.
type RecipeCatalog struct {
	UserRecipes     []Recipe
	SharedRecipes   []Recipe
	PublicRecipes   []Recipe
	UnlistedRecipes []Recipe
	DefaultRecipes  []Recipe
	// Usage is keyed by recipe id.
	Usage map[string]RecipeUsage
}

// GetRecipes calls /v1/get-recipes. One call returns the whole catalog, so
// unlike transcripts there is nothing to page or budget.
func (c *InternalClient) GetRecipes() (RecipeCatalog, error) {
	raw, err := c.post(recipesEndpoint, map[string]any{})
	if err != nil {
		return RecipeCatalog{}, err
	}
	return parseRecipeCatalog(raw)
}

// parseRecipeCatalog decodes the get-recipes envelope key by key so one
// unreadable collection cannot cost the caller the other four.
func parseRecipeCatalog(raw []byte) (RecipeCatalog, error) {
	cat := RecipeCatalog{Usage: map[string]RecipeUsage{}}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return cat, shapeError(recipesEndpoint, raw)
	}
	buckets := []struct {
		key string
		dst *[]Recipe
	}{
		{"userRecipes", &cat.UserRecipes},
		{"sharedRecipes", &cat.SharedRecipes},
		{"publicRecipes", &cat.PublicRecipes},
		{"unlistedRecipes", &cat.UnlistedRecipes},
		{"defaultRecipes", &cat.DefaultRecipes},
	}
	recognized := false
	for _, b := range buckets {
		v, ok := probe[b.key]
		if !ok {
			continue
		}
		recognized = true
		_ = json.Unmarshal(v, b.dst)
	}
	if v, ok := probe["recipesUsage"]; ok {
		recognized = true
		cat.Usage = parseRecipesUsage(v)
	}
	if !recognized {
		// An object with none of the six known keys is not "an account with
		// no recipes" — every account gets defaultRecipes. It is a payload we
		// no longer understand, and reporting it as empty would quietly wipe
		// the catalog on the next sync.
		return cat, shapeError(recipesEndpoint, raw)
	}
	return cat, nil
}

// parseRecipesUsage decodes the recipesUsage dict.
//
// total_count is json.Number rather than string or int64 because the wire
// carries it as a bare number while the desktop cache stores it as a string,
// and json.Number is the one type encoding/json fills from either. Every
// consumer of RecipeUsage.TotalCount runs fmt.Sscanf over it, so the value
// stored here has to be decimal digits: hand back "" or a float rendering and
// both the store write and `recipes list --top-usage` silently read zero.
func parseRecipesUsage(raw json.RawMessage) map[string]RecipeUsage {
	out := map[string]RecipeUsage{}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return out
	}
	for id, entry := range entries {
		if id == "" {
			continue
		}
		var u struct {
			RecipeID   string      `json:"recipe_id,omitempty"`
			TotalCount json.Number `json:"total_count,omitempty"`
			LastUsedAt string      `json:"last_used_at,omitempty"`
		}
		if err := json.Unmarshal(entry, &u); err != nil {
			// One unreadable counter, not a broken catalog.
			continue
		}
		recipeID := u.RecipeID
		if recipeID == "" {
			// The dict key is the recipe id; the record does not always
			// repeat it, and the cache-path record leaves it blank too.
			recipeID = id
		}
		out[id] = RecipeUsage{
			RecipeID:   recipeID,
			TotalCount: u.TotalCount.String(),
			LastUsedAt: u.LastUsedAt,
		}
	}
	return out
}

// GetPanelTemplates calls /v1/get-panel-templates. The live response is a
// bare list; the envelope shapes are accepted too because every other
// collection endpoint on this API has shipped both at some point.
func (c *InternalClient) GetPanelTemplates() ([]PanelTemplate, error) {
	raw, err := c.post(panelTemplatesEndpoint, map[string]any{})
	if err != nil {
		return nil, err
	}
	return parsePanelTemplates(raw)
}

func parsePanelTemplates(raw []byte) ([]PanelTemplate, error) {
	var arr []PanelTemplate
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	var env struct {
		PanelTemplates []PanelTemplate `json:"panelTemplates"`
		Templates      []PanelTemplate `json:"templates"`
	}
	if err := json.Unmarshal(raw, &env); err == nil {
		if env.PanelTemplates != nil {
			return env.PanelTemplates, nil
		}
		if env.Templates != nil {
			return env.Templates, nil
		}
	}
	return nil, shapeError(panelTemplatesEndpoint, raw)
}

// UpdateDocument modifies a document. Used by delete (set deleted_at) and
// restore (clear deleted_at).
func (c *InternalClient) UpdateDocument(id string, fields map[string]any) error {
	body := map[string]any{"document_id": id}
	for k, v := range fields {
		body[k] = v
	}
	_, err := c.post("/v1/update-document", body)
	return err
}

// DeleteDocument soft-deletes a document by setting deleted_at to now.
func (c *InternalClient) DeleteDocument(id string) error {
	return c.UpdateDocument(id, map[string]any{
		"deleted_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// RestoreDocument clears deleted_at.
func (c *InternalClient) RestoreDocument(id string) error {
	return c.UpdateDocument(id, map[string]any{
		"deleted_at": nil,
	})
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if x != "" {
			return x
		}
	}
	return ""
}
