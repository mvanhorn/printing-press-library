// Copyright 2026 drdriftwood. Licensed under Apache-2.0. See LICENSE.

// Package client wraps the local Obsidian CLI (`obsidian`, v1.12+) as a
// virtual REST client. V1 is read-only: every Get* call dispatches to
// `exec.Command("obsidian", ...)` against the active vault. Writes return
// a 405 error pending the upstream markdown-patch frontmatter-corruption
// fix.
//
// The HTTPClient/BaseURL/BasePath fields are retained so generated
// helpers compile without modification; no HTTP request is ever issued.
package client

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/config"
)

// BinaryResponseHeader is referenced by internal/mcp/tools.go to ask for
// raw bytes. Retained for compile compatibility; subprocess dispatch
// always returns JSON-shaped data.
const BinaryResponseHeader = "X-Binary-Response"

// Client is the subprocess-backed virtual REST client.
type Client struct {
	BaseURL    string
	BasePath   string
	Config     *config.Config
	HTTPClient *http.Client
	DryRun     bool
	NoCache    bool
	cacheDir   string
	limiter    *cliutil.AdaptiveLimiter
}

// APIError reports a virtual-HTTP failure with structured status so
// classifyAPIError can map to a typed exit code.
type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("%s %s: %s", e.Method, e.Path, e.Body)
	}
	return fmt.Sprintf("%s %s returned status %d", e.Method, e.Path, e.StatusCode)
}

func New(cfg *config.Config, timeout time.Duration, rateLimit float64) *Client {
	homeDir, _ := os.UserHomeDir()
	cacheDir := filepath.Join(homeDir, ".cache", "obsidian-pp-cli")
	return &Client{
		BaseURL:    "subprocess://obsidian",
		BasePath:   "",
		Config:     cfg,
		HTTPClient: &http.Client{Timeout: timeout},
		cacheDir:   cacheDir,
		limiter:    cliutil.NewAdaptiveLimiter(rateLimit),
	}
}

func (c *Client) RateLimit() float64 { return c.limiter.Rate() }

// Get dispatches a virtual GET to the obsidian subprocess.
//
// pp:client-call — every Get* method here shells out to the real
// `obsidian` binary. Stdout is parsed as JSON when the underlying command
// supports `format=json`; otherwise the route wraps text/TSV output into
// a JSON shape the framework handlers can consume.
func (c *Client) Get(path string, params map[string]string) (json.RawMessage, error) {
	return c.GetWithHeaders(path, params, nil)
}

func (c *Client) GetWithHeaders(path string, params map[string]string, _ map[string]string) (json.RawMessage, error) {
	if !c.NoCache && !c.DryRun && c.cacheDir != "" {
		if cached, ok := c.readCache(path, params); ok {
			return cached, nil
		}
	}
	if c.DryRun {
		return c.dryRunDispatch(path, params)
	}
	c.limiter.Wait()
	result, _, err := c.dispatch("GET", path, params)
	if err == nil {
		c.limiter.OnSuccess()
		if !c.NoCache && c.cacheDir != "" {
			c.writeCache(path, params, result)
		}
	}
	return result, err
}

func (c *Client) GetNoCache(path string, params map[string]string) (json.RawMessage, error) {
	return c.GetWithHeadersNoCache(path, params, nil)
}

func (c *Client) GetWithHeadersNoCache(path string, params map[string]string, _ map[string]string) (json.RawMessage, error) {
	if c.DryRun {
		return c.dryRunDispatch(path, params)
	}
	c.limiter.Wait()
	result, _, err := c.dispatch("GET", path, params)
	if err == nil {
		c.limiter.OnSuccess()
		if !c.NoCache && c.cacheDir != "" {
			c.writeCache(path, params, result)
		}
	}
	return result, err
}

func (c *Client) ProbeGet(path string) (int, error) {
	_, status, err := c.dispatch("GET", path, nil)
	return status, err
}

// Write methods all return a read-only-V1 error. V2 will replace these.

func (c *Client) Post(path string, _ any) (json.RawMessage, int, error) {
	return nil, 0, readOnlyErr("POST", path)
}
func (c *Client) PostWithParams(path string, _ map[string]string, _ any) (json.RawMessage, int, error) {
	return nil, 0, readOnlyErr("POST", path)
}
func (c *Client) PostWithHeaders(path string, _ any, _ map[string]string) (json.RawMessage, int, error) {
	return nil, 0, readOnlyErr("POST", path)
}
func (c *Client) PostWithParamsAndHeaders(path string, _ map[string]string, _ any, _ map[string]string) (json.RawMessage, int, error) {
	return nil, 0, readOnlyErr("POST", path)
}
func (c *Client) Delete(path string) (json.RawMessage, int, error) {
	return nil, 0, readOnlyErr("DELETE", path)
}
func (c *Client) DeleteWithParams(path string, _ map[string]string) (json.RawMessage, int, error) {
	return nil, 0, readOnlyErr("DELETE", path)
}
func (c *Client) DeleteWithHeaders(path string, _ map[string]string) (json.RawMessage, int, error) {
	return nil, 0, readOnlyErr("DELETE", path)
}
func (c *Client) DeleteWithParamsAndHeaders(path string, _ map[string]string, _ map[string]string) (json.RawMessage, int, error) {
	return nil, 0, readOnlyErr("DELETE", path)
}
func (c *Client) Put(path string, _ any) (json.RawMessage, int, error) {
	return nil, 0, readOnlyErr("PUT", path)
}
func (c *Client) PutWithParams(path string, _ map[string]string, _ any) (json.RawMessage, int, error) {
	return nil, 0, readOnlyErr("PUT", path)
}
func (c *Client) PutWithHeaders(path string, _ any, _ map[string]string) (json.RawMessage, int, error) {
	return nil, 0, readOnlyErr("PUT", path)
}
func (c *Client) PutWithParamsAndHeaders(path string, _ map[string]string, _ any, _ map[string]string) (json.RawMessage, int, error) {
	return nil, 0, readOnlyErr("PUT", path)
}
func (c *Client) Patch(path string, _ any) (json.RawMessage, int, error) {
	return nil, 0, readOnlyErr("PATCH", path)
}
func (c *Client) PatchWithParams(path string, _ map[string]string, _ any) (json.RawMessage, int, error) {
	return nil, 0, readOnlyErr("PATCH", path)
}
func (c *Client) PatchWithHeaders(path string, _ any, _ map[string]string) (json.RawMessage, int, error) {
	return nil, 0, readOnlyErr("PATCH", path)
}
func (c *Client) PatchWithParamsAndHeaders(path string, _ map[string]string, _ any, _ map[string]string) (json.RawMessage, int, error) {
	return nil, 0, readOnlyErr("PATCH", path)
}

func readOnlyErr(method, path string) error {
	return &APIError{
		Method:     method,
		Path:       path,
		StatusCode: 405,
		Body:       "obsidian-pp-cli V1 is read-only; writes are deferred to V2 pending the upstream markdown-patch frontmatter-corruption fix",
	}
}

// --- cache (path-keyed, 5-minute TTL) -----------------------------------

func (c *Client) cacheKey(path string, params map[string]string) string {
	key := path
	if c.Config != nil && c.Config.Path != "" {
		key += "|cfg=" + c.Config.Path
	}
	paramKeys := make([]string, 0, len(params))
	for k := range params {
		paramKeys = append(paramKeys, k)
	}
	sort.Strings(paramKeys)
	for _, k := range paramKeys {
		key += "|" + k + "=" + params[k]
	}
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:8])
}

func (c *Client) readCache(path string, params map[string]string) (json.RawMessage, bool) {
	cacheFile := filepath.Join(c.cacheDir, c.cacheKey(path, params)+".json")
	info, err := os.Stat(cacheFile)
	if err != nil || time.Since(info.ModTime()) > 5*time.Minute {
		return nil, false
	}
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, false
	}
	return json.RawMessage(data), true
}

func (c *Client) writeCache(path string, params map[string]string, data json.RawMessage) {
	_ = os.MkdirAll(c.cacheDir, 0o755)
	cacheFile := filepath.Join(c.cacheDir, c.cacheKey(path, params)+".json")
	_ = os.WriteFile(cacheFile, []byte(data), 0o644)
}

// --- subprocess dispatch ------------------------------------------------

// route maps a virtual path pattern to an obsidian subcommand spec.
type route struct {
	pattern       string
	obsidianCmd   string
	fileFromPath  string            // path-param name to forward as file=
	extraPathArgs map[string]string // path-param name -> obsidian arg name (e.g. "property" -> "name")
	forceFormat   string            // set format=<value> when the caller didn't
	textKey       string            // wrap raw stdout as {"<textKey>": "<stdout>"} JSON
	boolFlagNames []string          // params that flip to bare obsidian flags when true
	wrapAsArray   bool              // split stdout by newlines into a JSON string array
}

// routes is the read-only V1 dispatch table. Mirrors the 14 GET endpoints
// in obsidian-openapi.yaml. Writes (POST/PUT/DELETE/PATCH) are V2.
var routes = []route{
	{
		pattern:      "/notes/{name}",
		obsidianCmd:  "read",
		fileFromPath: "name",
		textKey:      "content",
	},
	{
		pattern:       "/notes/{name}/backlinks",
		obsidianCmd:   "backlinks",
		fileFromPath:  "name",
		forceFormat:   "json",
		boolFlagNames: []string{"counts", "total"},
	},
	{
		pattern:       "/notes/{name}/links",
		obsidianCmd:   "links",
		fileFromPath:  "name",
		boolFlagNames: []string{"total"},
		wrapAsArray:   true,
	},
	{
		pattern:       "/notes/{name}/properties/{property}",
		obsidianCmd:   "property:read",
		fileFromPath:  "name",
		extraPathArgs: map[string]string{"property": "name"},
		textKey:       "value",
	},
	{
		pattern:       "/search",
		obsidianCmd:   "search",
		forceFormat:   "json",
		boolFlagNames: []string{"total", "case"},
	},
	{
		pattern:       "/search/context",
		obsidianCmd:   "search:context",
		forceFormat:   "json",
		boolFlagNames: []string{"case"},
	},
	{
		pattern:       "/tags",
		obsidianCmd:   "tags",
		forceFormat:   "json",
		boolFlagNames: []string{"counts", "total"},
	},
	{
		pattern:       "/tasks",
		obsidianCmd:   "tasks",
		forceFormat:   "json",
		boolFlagNames: []string{"done", "todo", "total"},
	},
	{
		pattern:       "/orphans",
		obsidianCmd:   "orphans",
		boolFlagNames: []string{"total", "all"},
		wrapAsArray:   true,
	},
	{
		pattern:       "/deadends",
		obsidianCmd:   "deadends",
		boolFlagNames: []string{"total", "all"},
		wrapAsArray:   true,
	},
	{
		pattern:       "/unresolved",
		obsidianCmd:   "unresolved",
		forceFormat:   "json",
		boolFlagNames: []string{"counts", "total", "verbose"},
	},
	{
		pattern:     "/vault",
		obsidianCmd: "vault",
		textKey:     "info",
	},
	{
		pattern:       "/files",
		obsidianCmd:   "files",
		boolFlagNames: []string{"total"},
		wrapAsArray:   true,
	},
	{
		pattern:       "/folders",
		obsidianCmd:   "folders",
		boolFlagNames: []string{"total"},
		wrapAsArray:   true,
	},
}

func (c *Client) dispatch(method, path string, params map[string]string) (json.RawMessage, int, error) {
	if method != "GET" {
		return nil, 405, readOnlyErr(method, path)
	}
	r, pathArgs, ok := matchRoute(path)
	if !ok {
		return nil, 404, &APIError{Method: method, Path: path, StatusCode: 404, Body: "no obsidian command mapped to " + path}
	}
	return runObsidian(r, buildArgs(r, pathArgs, params))
}

func (c *Client) dryRunDispatch(path string, params map[string]string) (json.RawMessage, error) {
	r, pathArgs, ok := matchRoute(path)
	if !ok {
		return json.RawMessage(`{"dry_run":true,"error":"unmapped path"}`), nil
	}
	args := buildArgs(r, pathArgs, params)
	out, _ := json.Marshal(map[string]any{
		"dry_run":    true,
		"subprocess": append([]string{"obsidian", r.obsidianCmd}, args...),
	})
	return json.RawMessage(out), nil
}

func matchRoute(path string) (*route, map[string]string, bool) {
	for i := range routes {
		r := &routes[i]
		if args, ok := matchPattern(r.pattern, path); ok {
			return r, args, true
		}
	}
	return nil, nil, false
}

// matchPattern matches "/notes/{name}/backlinks" against an actual URL-
// escaped path and returns the unescaped path-param map.
func matchPattern(pattern, path string) (map[string]string, bool) {
	pp := strings.Split(strings.Trim(pattern, "/"), "/")
	ap := strings.Split(strings.Trim(path, "/"), "/")
	if len(pp) != len(ap) {
		return nil, false
	}
	out := map[string]string{}
	for i, p := range pp {
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			decoded, err := url.PathUnescape(ap[i])
			if err != nil {
				decoded = ap[i]
			}
			out[strings.TrimSuffix(strings.TrimPrefix(p, "{"), "}")] = decoded
			continue
		}
		if p != ap[i] {
			return nil, false
		}
	}
	return out, true
}

// buildArgs constructs the obsidian subprocess argument list for a route.
// Path params and query params both render as `key=value` pairs (or bare
// flags for declared boolean names). Spaces in values are passed as-is to
// exec.Command — no shell quoting is needed because each arg is a
// separate slice element.
func buildArgs(r *route, pathArgs, params map[string]string) []string {
	args := make([]string, 0, len(pathArgs)+len(params)+2)
	if r.fileFromPath != "" {
		if v, ok := pathArgs[r.fileFromPath]; ok && v != "" {
			args = append(args, "file="+v)
		}
	}
	for src, dst := range r.extraPathArgs {
		if v, ok := pathArgs[src]; ok && v != "" {
			args = append(args, dst+"="+v)
		}
	}
	if v, ok := params["path"]; ok && v != "" {
		args = append(args, "path="+v)
	}
	boolSet := make(map[string]bool, len(r.boolFlagNames))
	for _, b := range r.boolFlagNames {
		boolSet[b] = true
	}
	// When a route requires JSON output (so framework handlers can parse
	// the response), force format=<r.forceFormat> unconditionally — the
	// generated handler always emits format=tsv (the spec default) into
	// params, which would otherwise override us and return un-parseable
	// TSV to handlers that expect JSON. Users still pick their *output*
	// format via the framework --json / --csv / --plain flags downstream.
	formatForced := r.forceFormat != ""
	if formatForced {
		args = append(args, "format="+r.forceFormat)
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := params[k]
		if k == "path" {
			continue
		}
		if k == "format" && formatForced {
			continue
		}
		if boolSet[k] {
			if v == "true" || v == "1" {
				args = append(args, k)
			}
			continue
		}
		if v == "" {
			continue
		}
		args = append(args, k+"="+v)
	}
	return args
}

// runObsidian executes `obsidian <subcmd> <args...>` and shapes stdout to
// the route's contract.
func runObsidian(r *route, args []string) (json.RawMessage, int, error) {
	full := append([]string{r.obsidianCmd}, args...)
	cmd := exec.Command("obsidian", full...)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		stderr := ""
		if errors.As(err, &ee) {
			stderr = string(ee.Stderr)
		}
		body := strings.TrimSpace(stderr)
		if body == "" {
			body = err.Error()
		}
		status := 500
		bl := strings.ToLower(body)
		switch {
		case strings.Contains(bl, "not found"):
			status = 404
		case strings.Contains(bl, "no active vault"), strings.Contains(bl, "no vault"):
			status = 503
		}
		return nil, status, &APIError{
			Method:     "GET",
			Path:       "/" + r.obsidianCmd,
			StatusCode: status,
			Body:       body,
		}
	}
	stdout := bytes.TrimRight(out, "\n")

	if r.textKey != "" {
		shape, mErr := json.Marshal(map[string]string{r.textKey: string(stdout)})
		if mErr != nil {
			return nil, 500, mErr
		}
		return json.RawMessage(shape), 200, nil
	}
	if r.wrapAsArray {
		lines := strings.Split(string(stdout), "\n")
		items := make([]string, 0, len(lines))
		for _, ln := range lines {
			s := strings.TrimSpace(ln)
			if s != "" {
				items = append(items, s)
			}
		}
		shape, mErr := json.Marshal(items)
		if mErr != nil {
			return nil, 500, mErr
		}
		return json.RawMessage(shape), 200, nil
	}
	if !json.Valid(stdout) {
		// Last-resort fallback for endpoints that should have returned
		// JSON but didn't (e.g. obsidian printed a startup notice). Wrap
		// the text so callers can still parse the envelope.
		shape, _ := json.Marshal(map[string]string{"text": string(stdout)})
		return json.RawMessage(shape), 200, nil
	}
	return json.RawMessage(stdout), 200, nil
}
