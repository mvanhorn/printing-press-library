// Code-orchestration thin MCP surface for Dub.
//
// Two tools cover the entire 53-endpoint API: dub_search to discover
// endpoints by natural-language query, and dub_execute to invoke one by
// its endpoint_id. This collapses the API to ~1K tokens of tool definitions
// while preserving full coverage. Pattern source: Anthropic 2026-04-22
// "Building agents that reach production systems with MCP" (Cloudflare's
// MCP server uses the same shape for ~2,500 endpoints).
//
// This file was hand-written in polish (the spec is an upstream Speakeasy
// OpenAPI without a `mcp:` block). The endpoint registry is embedded from
// tools-manifest.json so adding/removing API endpoints does not require a
// separate registry update — the manifest is the single source of truth.

package mcp

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

//go:embed embedded_manifest.json
var embeddedManifest []byte

// codeOrchEndpoint captures the slice of endpoint metadata the search +
// execute pair needs at runtime.
type codeOrchEndpoint struct {
	ID          string   `json:"id"`
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	Summary     string   `json:"summary"`
	Positional  []string `json:"positional"`
	Description string   `json:"description"`
	keywords    []string
}

type manifestParam struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Location    string `json:"location"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type manifestTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Method      string          `json:"method"`
	Path        string          `json:"path"`
	Params      []manifestParam `json:"params"`
}

type manifestRoot struct {
	Tools []manifestTool `json:"tools"`
}

var codeOrchEndpoints []codeOrchEndpoint

func init() {
	var m manifestRoot
	if err := json.Unmarshal(embeddedManifest, &m); err != nil {
		// Embedded manifest is generated at build time; failure here means
		// the file was malformed when embedded. Best-effort: leave the
		// registry empty so search returns 0 results rather than panicking.
		return
	}
	codeOrchEndpoints = make([]codeOrchEndpoint, 0, len(m.Tools))
	for _, t := range m.Tools {
		var positional []string
		for _, p := range t.Params {
			if p.Location == "path" {
				positional = append(positional, p.Name)
			}
		}
		codeOrchEndpoints = append(codeOrchEndpoints, codeOrchEndpoint{
			ID:          t.Name,
			Method:      t.Method,
			Path:        t.Path,
			Summary:     truncSummary(t.Description, 240),
			Description: t.Description,
			Positional:  positional,
			keywords:    codeOrchKeywords(t.Name, t.Description, t.Path),
		})
	}
}

// truncSummary keeps the first sentence or 240 chars, whichever is shorter.
func truncSummary(s string, max int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, ".!?"); i > 0 && i < max {
		return s[:i+1]
	}
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

// codeOrchStopwords are words too common to contribute useful ranking
// signal — without filtering, "is" and "in" inside endpoint descriptions
// match every query term that contains those bigrams, polluting results.
var codeOrchStopwords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true,
	"at": true, "be": true, "by": true, "for": true, "from": true,
	"has": true, "in": true, "is": true, "it": true, "its": true,
	"of": true, "on": true, "or": true, "that": true, "the": true,
	"this": true, "to": true, "was": true, "will": true, "with": true,
	"your": true, "you": true, "any": true, "all": true,
}

// codeOrchKeywords produces the lowercase token stream used for search
// ranking. Tokens shorter than 3 characters are dropped (too noisy for
// substring matching), and a small stopword set is filtered out.
func codeOrchKeywords(name, description, path string) []string {
	raw := strings.ToLower(name + " " + description + " " + path)
	raw = strings.Map(func(r rune) rune {
		switch r {
		case '_', '-', '/', '{', '}', '.', ',', ':', ';':
			return ' '
		}
		return r
	}, raw)
	out := make([]string, 0, 16)
	seen := map[string]bool{}
	for _, tok := range strings.Fields(raw) {
		if len(tok) < 3 || codeOrchStopwords[tok] || seen[tok] {
			continue
		}
		seen[tok] = true
		out = append(out, tok)
	}
	return out
}

// RegisterCodeOrchestrationTools registers dub_search + dub_execute on the
// MCP server. Called from cmd/dub-pp-mcp/main.go alongside RegisterTools()
// so cloud-agent hosts can reach the entire API in 2 tools while local
// agents that prefer typed tools still have the full 53-tool surface.
func RegisterCodeOrchestrationTools(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("dub_search",
			mcplib.WithDescription("Search the Dub API for endpoints matching a natural-language query. Returns a ranked list of {endpoint_id, method, path, summary} entries. Call this first to find the endpoint to execute."),
			mcplib.WithString("query", mcplib.Required(), mcplib.Description("Natural-language description of what you want to do (e.g. \"create a short link\", \"list partner commissions\").")),
			mcplib.WithNumber("limit", mcplib.Description("Max endpoints to return (default 10).")),
		),
		handleCodeOrchSearch,
	)
	s.AddTool(
		mcplib.NewTool("dub_execute",
			mcplib.WithDescription("Execute one Dub API endpoint by its endpoint_id (from dub_search). Path placeholders match by name; remaining params become query string on GET/DELETE or JSON body on POST/PUT/PATCH."),
			mcplib.WithString("endpoint_id", mcplib.Required(), mcplib.Description("Endpoint identifier returned by dub_search (e.g. \"links_create\").")),
			mcplib.WithObject("params", mcplib.Description("Parameters for the endpoint.")),
		),
		handleCodeOrchExecute,
	)
}

func handleCodeOrchSearch(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	query, ok := args["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return mcplib.NewToolResultError("query is required"), nil
	}
	limit := 10
	if v, ok := args["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}

	terms := codeOrchKeywords("", query, "")
	type scored struct {
		ep    *codeOrchEndpoint
		score int
	}
	results := make([]scored, 0, len(codeOrchEndpoints))
	for i := range codeOrchEndpoints {
		ep := &codeOrchEndpoints[i]
		score := 0
		for _, t := range terms {
			for _, kw := range ep.keywords {
				if kw == t {
					score += 2
				} else if strings.Contains(kw, t) || strings.Contains(t, kw) {
					score++
				}
			}
		}
		if score > 0 {
			results = append(results, scored{ep: ep, score: score})
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].score > results[j].score })
	if len(results) > limit {
		results = results[:limit]
	}

	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		out = append(out, map[string]any{
			"endpoint_id": r.ep.ID,
			"method":      r.ep.Method,
			"path":        r.ep.Path,
			"summary":     r.ep.Summary,
			"score":       r.score,
		})
	}
	data, _ := json.Marshal(map[string]any{"count": len(out), "results": out})
	return mcplib.NewToolResultText(string(data)), nil
}

func handleCodeOrchExecute(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	id, ok := args["endpoint_id"].(string)
	if !ok || id == "" {
		return mcplib.NewToolResultError("endpoint_id is required (call dub_search first)"), nil
	}

	var ep *codeOrchEndpoint
	for i := range codeOrchEndpoints {
		if codeOrchEndpoints[i].ID == id {
			ep = &codeOrchEndpoints[i]
			break
		}
	}
	if ep == nil {
		return mcplib.NewToolResultError(fmt.Sprintf("unknown endpoint_id %q — call dub_search to discover valid ids", id)), nil
	}

	params, _ := args["params"].(map[string]any)
	if params == nil {
		params = map[string]any{}
	}

	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	path := ep.Path
	for _, p := range ep.Positional {
		if v, ok := params[p]; ok {
			path = strings.ReplaceAll(path, "{"+p+"}", fmt.Sprintf("%v", v))
			delete(params, p)
		}
	}

	query := map[string]string{}
	if ep.Method == "GET" || ep.Method == "DELETE" {
		for k, v := range params {
			query[k] = fmt.Sprintf("%v", v)
		}
	}

	var data json.RawMessage
	switch ep.Method {
	case "GET":
		data, err = c.Get(path, query)
	case "DELETE":
		data, _, err = c.Delete(path)
	case "POST":
		data, _, err = c.Post(path, params)
	case "PUT":
		data, _, err = c.Put(path, params)
	case "PATCH":
		data, _, err = c.Patch(path, params)
	default:
		return mcplib.NewToolResultError(fmt.Sprintf("unsupported method %q", ep.Method)), nil
	}
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcplib.NewToolResultText(string(data)), nil
}
