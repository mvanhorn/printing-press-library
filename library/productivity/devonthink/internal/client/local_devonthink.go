// Copyright 2026 rowdy and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const devonthinkAppID = "DNtp"

func (c *Client) isLocalDEVONthink() bool {
	if c == nil {
		return false
	}
	baseURL := strings.TrimSpace(c.BaseURL)
	if baseURL == "" {
		return true
	}
	switch strings.ToLower(baseURL) {
	case "local", "devonthink", "devonthink://local":
		return true
	}
	return false
}

func (c *Client) doLocalDEVONthink(ctx context.Context, method, path string, params map[string]string, body any, _ map[string]string, _ bool) (json.RawMessage, int, error) {
	if c.DryRun {
		return localJSON(map[string]any{
			"status":  "dry-run",
			"dry_run": true,
			"method":  method,
			"path":    path,
			"params":  params,
			"body":    body,
			"message": "Would execute against the local DEVONthink app on this Mac.",
		})
	}

	if isMutatingVerb(method) && !(path == "/mcp/call" && localMCPToolLooksReadOnly(params["tool"])) {
		return localJSON(map[string]any{
			"status":  "blocked",
			"method":  method,
			"path":    path,
			"message": "Local write support is intentionally gated in this build. Re-run with --dry-run to preview, or use batch plan/apply after reviewing the generated plan.",
		})
	}

	switch {
	case path == "/" || path == "/runtime/doctor":
		return localJSON(c.localRuntimeDoctor(ctx))
	case path == "/databases":
		return localJSON(c.localDatabases(ctx))
	case path == "/records/search":
		return localJSON(c.localRecordSearch(ctx, params))
	case path == "/records/lookup":
		return localJSON(c.localRecordLookup(ctx, params))
	case path == "/records/create" || path == "/records/update" || path == "/records/move":
		return localJSON(c.localBlockedOperation(method, path))
	case path == "/selection":
		return localJSON(c.localSelection(ctx))
	case path == "/selection/snapshot":
		return localJSON(c.localSelectionSnapshot(ctx, params))
	case path == "/inventory/export":
		return c.localInventoryExport(ctx, params)
	case path == "/groups/tree":
		return localJSON(c.localGroupsTree(ctx, params))
	case path == "/graph/links":
		return localJSON(c.localGraphLinks(ctx, params))
	case path == "/graph/audit":
		return localJSON(c.localGraphAudit(ctx, params))
	case path == "/tags/analyze":
		return localJSON(c.localTagsAnalyze(ctx, params))
	case path == "/sheets/get":
		return localJSON(c.localSheet(ctx, params))
	case path == "/ai/ask" || path == "/ai/summarize":
		return localJSON(c.localAIRead(path, params))
	case path == "/mcp/tools":
		return c.localMCPTools(ctx)
	case path == "/mcp/schema":
		return c.localMCPSchema(ctx)
	case path == "/mcp/call":
		return c.localMCPCall(ctx, params, body)
	case path == "/batch/plan":
		return localJSON(c.localBatchPlan(params, body))
	case path == "/batch/apply":
		return localJSON(c.localBatchApply(params, body))
	case path == "/ledger":
		return localJSON(c.localLedgerList(params))
	case strings.HasPrefix(path, "/ledger/"):
		return localJSON(c.localLedgerShow(path))
	case path == "/privacy/audit":
		return localJSON(c.localPrivacyAudit(ctx, params))
	case path == "/mirror/sync":
		return localJSON(c.localMirrorSync(params))
	case path == "/mirror/search":
		return localJSON(c.localMirrorSearch(ctx, params))
	case path == "/context/pack":
		return localJSON(c.localContextPack(ctx, params))
	case strings.HasPrefix(path, "/records/"):
		return localJSON(c.localRecordPath(ctx, path, params))
	case path == "/ingest/file" || path == "/ingest/url" || path == "/media/ocr" || path == "/media/transcribe":
		return localJSON(c.localBlockedOperation(method, path))
	default:
		return localJSON(map[string]any{
			"status":  "unsupported",
			"path":    path,
			"message": "No local DEVONthink adapter exists for this endpoint yet.",
		})
	}
}

func localJSON(v any) (json.RawMessage, int, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, 0, err
	}
	return json.RawMessage(data), http.StatusOK, nil
}

func (c *Client) localRuntimeDoctor(ctx context.Context) map[string]any {
	version, versionErr := devonthinkVersion(ctx)
	toolsPath, toolsOK := devonthinkMCPToolsPath()
	mcpRunning := false
	if raw, ok, err := c.localMCPToolJSON(ctx, "is_running", map[string]any{}); err == nil && ok {
		var running map[string]any
		if json.Unmarshal(raw, &running) == nil {
			if value, _ := running["running"].(bool); value {
				mcpRunning = true
			}
		}
	}
	return map[string]any{
		"app": map[string]any{
			"name":          "DEVONthink",
			"bundle_id":     "com.devon-technologies.think3",
			"running":       versionErr == nil || mcpRunning,
			"version":       version,
			"applescript":   versionErr == nil,
			"error":         errorString(versionErr),
			"local_only":    true,
			"network_scope": "this Mac or user-controlled LAN only",
		},
		"mcp": map[string]any{
			"tools_file":       toolsPath,
			"tools_file_found": toolsOK,
			"http_ok":          mcpRunning,
			"note":             "The official DEVONthink MCP server is local. This CLI uses local automation by default and can expose MCP metadata when available.",
		},
		"privacy": map[string]any{
			"default_transport": "local",
			"cloud_required":    false,
		},
	}
}

func (c *Client) localDatabases(ctx context.Context) []map[string]any {
	if raw, ok, err := c.localMCPToolJSON(ctx, "get_databases", map[string]any{}); err == nil && ok {
		var databases []map[string]any
		if json.Unmarshal(raw, &databases) == nil {
			return databases
		}
	}
	names, err := devonthinkDatabaseNames(ctx)
	if err != nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		out = append(out, map[string]any{
			"name":       name,
			"local_only": true,
		})
	}
	return out
}

func (c *Client) localRecordSearch(ctx context.Context, params map[string]string) []map[string]any {
	query := firstNonEmpty(params["query"], "kind:document")
	args := map[string]any{"query": query}
	if limit, ok := intParam(params, "limit"); ok {
		args["limit"] = limit
	}
	if offset, ok := intParam(params, "offset"); ok {
		args["offset"] = offset
	}
	if sort := strings.TrimSpace(params["sort"]); sort != "" {
		args["sort"] = sort
	}
	if database := strings.TrimSpace(params["database"]); database != "" {
		args["database_uuid"] = database
	}
	if group := strings.TrimSpace(params["group"]); group != "" {
		args["group_uuid"] = group
	}
	if raw, ok, err := c.localMCPToolJSON(ctx, "search_records", args); err == nil && ok {
		var envelope struct {
			Results []map[string]any `json:"results"`
		}
		if json.Unmarshal(raw, &envelope) == nil {
			return envelope.Results
		}
		var direct []map[string]any
		if json.Unmarshal(raw, &direct) == nil {
			return direct
		}
	}
	return []map[string]any{}
}

func (c *Client) localRecordLookup(ctx context.Context, params map[string]string) map[string]any {
	args := map[string]any{}
	for _, key := range []string{"name", "url", "path", "location", "filename", "comment"} {
		if value := strings.TrimSpace(params[key]); value != "" {
			args[key] = value
			break
		}
	}
	if len(args) > 0 {
		if raw, ok, err := c.localMCPToolJSON(ctx, "lookup_records", args); err == nil && ok {
			var matches []map[string]any
			if json.Unmarshal(raw, &matches) == nil {
				return map[string]any{"matches": matches, "query": args}
			}
		}
	}
	return map[string]any{
		"matches": []map[string]any{},
		"query":   params,
		"note":    "Lookup is local-only. Live record enumeration will use DEVONthink automation or MCP as that bridge is expanded.",
	}
}

func (c *Client) localSelection(ctx context.Context) []map[string]any {
	if raw, ok, err := c.localMCPToolJSON(ctx, "get_selected_records", map[string]any{}); err == nil && ok {
		var records []map[string]any
		if json.Unmarshal(raw, &records) == nil {
			return records
		}
	}
	records, _ := devonthinkSelectedRecordNames(ctx)
	return records
}

func (c *Client) localSelectionSnapshot(ctx context.Context, params map[string]string) map[string]any {
	records := c.localSelection(ctx)
	warnings := []string{}
	return map[string]any{
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"name":       firstNonEmpty(params["name"], "selection-snapshot"),
		"records":    records,
		"count":      len(records),
		"warnings":   warnings,
	}
}

func (c *Client) localInventoryExport(ctx context.Context, params map[string]string) (json.RawMessage, int, error) {
	format := firstNonEmpty(params["format"], "maintenance")
	warnings := []string{}
	databases := c.localDatabases(ctx)
	if len(databases) == 0 {
		warnings = append(warnings, "No databases were returned by local MCP/automation.")
	}
	query := firstNonEmpty(params["query"], "kind:document")
	limit := 100
	if parsed, ok := intParam(params, "limit"); ok && parsed > 0 {
		limit = parsed
	}
	documents := c.localRecordSearch(ctx, map[string]string{
		"query": query,
		"limit": strconv.Itoa(limit),
	})
	for i := range documents {
		delete(documents[i], "path")
	}
	payload := map[string]any{
		"schema": map[string]any{
			"name":    "devonthink-pp-cli.inventory",
			"version": 1,
			"format":  format,
		},
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"source":       "devonthink-pp-cli",
		"local_only":   true,
		"databases":    databases,
		"groups":       []map[string]any{},
		"documents":    documents,
		"tags":         []map[string]any{},
		"query":        query,
		"limit":        limit,
		"warnings":     warnings,
		"maintenance_contract": map[string]any{
			"consumer": "Rowdy/DEVONthink-ai-maintenance",
			"role":     "mechanical inventory export; filing and triage policy stay outside the core CLI",
		},
	}
	data, status, err := localJSON(payload)
	if err != nil {
		return data, status, err
	}
	if output := strings.TrimSpace(params["output"]); output != "" {
		if err := writeLocalOutput(output, data); err != nil {
			return nil, 0, err
		}
	}
	return data, status, nil
}

func (c *Client) localGroupsTree(ctx context.Context, params map[string]string) map[string]any {
	args := map[string]any{}
	if uuid := firstNonEmpty(params["uuid"], params["root"], params["root_uuid"]); uuid != "" {
		args["uuid"] = uuid
	}
	if depth, ok := intParam(params, "max_depth"); ok {
		args["max_depth"] = depth
	}
	if include := strings.TrimSpace(params["include_documents"]); include != "" {
		args["include_documents"] = strings.EqualFold(include, "true") || include == "1"
	}
	if len(args) > 0 {
		if raw, ok, err := c.localMCPToolJSON(ctx, "get_group_tree", args); err == nil && ok {
			var tree map[string]any
			if json.Unmarshal(raw, &tree) == nil {
				return tree
			}
		}
	}
	return map[string]any{
		"database": params["database"],
		"root": map[string]any{
			"name":     firstNonEmpty(params["root"], "/"),
			"children": []map[string]any{},
		},
		"warnings": []string{"Live group traversal is not enabled in the local adapter yet."},
	}
}

func (c *Client) localGraphLinks(ctx context.Context, params map[string]string) map[string]any {
	uuid := firstNonEmpty(params["uuid"], params["record"])
	if uuid != "" {
		args := map[string]any{"uuid": uuid}
		if direction := strings.TrimSpace(params["direction"]); direction != "" {
			args["direction"] = direction
		}
		if kind := strings.TrimSpace(params["kind"]); kind != "" {
			args["kind"] = kind
		}
		if raw, ok, err := c.localMCPToolJSON(ctx, "get_record_links", args); err == nil && ok {
			var links map[string]any
			if json.Unmarshal(raw, &links) == nil {
				return links
			}
		}
	}
	return map[string]any{
		"record":   firstNonEmpty(params["uuid"], params["record"]),
		"incoming": []map[string]any{},
		"outgoing": []map[string]any{},
	}
}

func (c *Client) localGraphAudit(ctx context.Context, params map[string]string) map[string]any {
	records := c.localRecordSearch(ctx, map[string]string{
		"query": firstNonEmpty(params["query"], "item:tagged"),
		"limit": firstNonEmpty(params["limit"], "25"),
	})
	return map[string]any{
		"query":      params["query"],
		"checked_at": time.Now().UTC().Format(time.RFC3339),
		"issues":     []map[string]any{},
		"sample":     records,
		"warnings":   []string{"Graph audit is metadata-only in this build; no record content was exported."},
	}
}

func (c *Client) localTagsAnalyze(ctx context.Context, params map[string]string) map[string]any {
	args := map[string]any{}
	if database := strings.TrimSpace(params["database"]); database != "" {
		args["database_uuid"] = database
	}
	if raw, ok, err := c.localMCPToolJSON(ctx, "list_database_tags", args); err == nil && ok {
		var tags any
		if json.Unmarshal(raw, &tags) == nil {
			return map[string]any{
				"database": params["database"],
				"tags":     tags,
			}
		}
	}
	return map[string]any{
		"database": params["database"],
		"tags":     []map[string]any{},
		"summary": map[string]any{
			"count":      0,
			"duplicates": 0,
			"orphans":    0,
		},
	}
}

func (c *Client) localSheet(ctx context.Context, params map[string]string) map[string]any {
	if uuid := strings.TrimSpace(params["uuid"]); uuid != "" {
		if raw, ok, err := c.localMCPToolJSON(ctx, "get_record_text", map[string]any{"uuid": uuid}); err == nil && ok {
			var text any
			if json.Unmarshal(raw, &text) == nil {
				return map[string]any{"uuid": uuid, "text": text}
			}
		}
	}
	return map[string]any{
		"uuid":    params["uuid"],
		"columns": []string{},
		"rows":    []map[string]any{},
	}
}

func (c *Client) localAIRead(path string, params map[string]string) map[string]any {
	return map[string]any{
		"status":  "blocked",
		"path":    path,
		"query":   params["query"],
		"message": "AI-backed DEVONthink reads require an explicit local MCP-backed implementation so content exposure can be audited first.",
	}
}

func (c *Client) localMCPTools(ctx context.Context) (json.RawMessage, int, error) {
	if raw, ok, err := c.localMCPListTools(ctx); err == nil && ok {
		var parsed any
		if json.Unmarshal(raw, &parsed) == nil {
			return localJSON(map[string]any{
				"source": "http://127.0.0.1",
				"tools":  parsed,
			})
		}
	}
	path, ok := devonthinkMCPToolsPath()
	if !ok {
		return localJSON(map[string]any{"tools": []map[string]any{}, "error": "mcp-tools.json not found"})
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path comes from fixed DEVONthink application bundle candidates.
	if err != nil {
		return nil, 0, err
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, 0, err
	}
	return localJSON(map[string]any{
		"source": path,
		"tools":  parsed,
	})
}

func (c *Client) localMCPSchema(ctx context.Context) (json.RawMessage, int, error) {
	return c.localMCPTools(ctx)
}

func (c *Client) localMCPCall(ctx context.Context, params map[string]string, body any) (json.RawMessage, int, error) {
	tool := firstNonEmpty(params["tool"], params["name"])
	if tool == "" {
		return localJSON(map[string]any{
			"status":  "blocked",
			"message": "Pass a local MCP tool name.",
		})
	}
	args := map[string]any{}
	if rawArgs := strings.TrimSpace(params["args"]); rawArgs != "" {
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			return nil, 0, err
		}
	}
	if bodyMap, ok := body.(map[string]any); ok && len(args) == 0 {
		if raw, ok := bodyMap["args"].(string); ok && strings.TrimSpace(raw) != "" {
			if err := json.Unmarshal([]byte(raw), &args); err != nil {
				looseArgs, looseOK := parseLooseMCPArgs(raw)
				if !looseOK {
					return nil, 0, err
				}
				args = looseArgs
			}
		} else {
			args = bodyMap
		}
	}
	if raw, ok, err := c.localMCPToolJSON(ctx, tool, args); err != nil {
		return nil, 0, err
	} else if ok {
		return raw, http.StatusOK, nil
	}
	if text, ok, err := c.localMCPToolText(ctx, tool, args); err != nil {
		return nil, 0, err
	} else if ok {
		return localJSON(map[string]any{"tool": tool, "text": text})
	}
	return localJSON(map[string]any{
		"status":  "empty",
		"tool":    tool,
		"message": "The local MCP tool returned no content.",
	})
}

func (c *Client) localBatchPlan(params map[string]string, body any) map[string]any {
	return map[string]any{
		"status":      "planned",
		"created_at":  time.Now().UTC().Format(time.RFC3339),
		"source":      params["source"],
		"actions":     []map[string]any{},
		"input":       body,
		"review_note": "No write actions were generated by the local adapter.",
	}
}

func (c *Client) localBatchApply(params map[string]string, body any) map[string]any {
	return map[string]any{
		"status":  "blocked",
		"plan":    params["plan"],
		"input":   body,
		"message": "Batch apply is disabled until the CLI has a confirmed local write bridge and review ledger.",
	}
}

func (c *Client) localLedgerList(params map[string]string) []map[string]any {
	return []map[string]any{}
}

func (c *Client) localLedgerShow(path string) map[string]any {
	return map[string]any{
		"id":      strings.TrimPrefix(path, "/ledger/"),
		"status":  "not-found",
		"actions": []map[string]any{},
	}
}

func (c *Client) localPrivacyAudit(ctx context.Context, params map[string]string) map[string]any {
	query := firstNonEmpty(params["query"], "kind:document")
	limit := firstNonEmpty(params["limit"], "25")
	records := c.localRecordSearch(ctx, map[string]string{"query": query, "limit": limit})
	return map[string]any{
		"query":      query,
		"limit":      limit,
		"checked_at": time.Now().UTC().Format(time.RFC3339),
		"local_only": true,
		"summary": map[string]any{
			"records":              len(records),
			"estimated_characters": 0,
			"external_calls":       0,
		},
		"records": records,
		"risks":   []map[string]any{},
		"warnings": []string{
			"No record content was exported by this audit.",
			"Use this command before AI workflows to make content exposure explicit.",
		},
	}
}

func (c *Client) localMirrorSync(params map[string]string) map[string]any {
	return map[string]any{
		"status":      "not_implemented",
		"mirror_path": params["path"],
		"synced":      0,
		"updated":     0,
		"deleted":     0,
		"warnings": []string{
			"The local SQLite mirror backend is not active in this build; no rows were synced.",
			"Use live records search or inventory export for current DEVONthink metadata.",
		},
	}
}

func (c *Client) localMirrorSearch(_ context.Context, params map[string]string) []map[string]any {
	query := firstNonEmpty(params["query"], params["q"])
	return []map[string]any{{
		"uuid":    "mirror-not-implemented",
		"name":    "Local mirror backend is not active",
		"path":    "query:" + query,
		"query":   query,
		"source":  "local-mirror",
		"status":  "not_implemented",
		"warning": "The local SQLite mirror backend is not active in this build; use records search for live MCP metadata.",
	}}
}

func (c *Client) localContextPack(ctx context.Context, params map[string]string) map[string]any {
	query := params["query"]
	records := []map[string]any{}
	if query != "" {
		records = c.localRecordSearch(ctx, map[string]string{
			"query": query,
			"limit": firstNonEmpty(params["limit"], "10"),
		})
	} else {
		records = c.localSelection(ctx)
	}
	markdown := "# DEVONthink Context Pack\n\n"
	if query != "" {
		markdown += "Query: " + query + "\n\n"
	}
	if len(records) == 0 {
		markdown += "No matching records were available through the current local adapter.\n"
	} else {
		for _, record := range records {
			name, _ := record["name"].(string)
			uuid, _ := record["uuid"].(string)
			location, _ := record["location"].(string)
			markdown += "- " + firstNonEmpty(name, uuid, "record")
			if location != "" {
				markdown += " (" + location + ")"
			}
			if uuid != "" {
				markdown += " [" + uuid + "]"
			}
			markdown += "\n"
		}
	}
	return map[string]any{
		"created_at":    time.Now().UTC().Format(time.RFC3339),
		"query":         query,
		"token_budget":  firstNonEmpty(params["token_budget"], params["token-budget"], "4000"),
		"record_count":  len(records),
		"records":       records,
		"markdown":      markdown,
		"truncated":     false,
		"privacy_scope": "local",
	}
}

func (c *Client) localRecordPath(ctx context.Context, path string, params map[string]string) map[string]any {
	uuid := recordIDFromPath(path)
	switch {
	case strings.HasSuffix(path, "/content"):
		args := map[string]any{"uuid": uuid}
		if query := strings.TrimSpace(params["query"]); query != "" {
			args["query"] = query
		}
		if maxTokens, ok := intParam(params, "max_token_count"); ok {
			args["max_token_count"] = maxTokens
		}
		if value, ok, err := c.localMCPToolValue(ctx, "extract_record_content", args); err == nil && ok {
			return map[string]any{
				"uuid":    uuid,
				"content": value,
				"format":  firstNonEmpty(params["format"], "devonthink-extract"),
			}
		}
		return map[string]any{
			"uuid":     uuid,
			"content":  "",
			"format":   firstNonEmpty(params["format"], "text"),
			"warnings": []string{"Live content extraction is not enabled in the local adapter yet."},
		}
	case strings.HasSuffix(path, "/related"):
		if value, ok, err := c.localMCPToolValue(ctx, "find_similar_records", map[string]any{"uuid": uuid, "limit": 25}); err == nil && ok {
			return map[string]any{"uuid": uuid, "related": value}
		}
		return map[string]any{"uuid": uuid, "related": []map[string]any{}}
	case strings.HasSuffix(path, "/highlights"):
		if value, ok, err := c.localMCPToolValue(ctx, "extract_record_highlights", map[string]any{"uuid": uuid}); err == nil && ok {
			return map[string]any{"uuid": uuid, "highlights": value}
		}
		return map[string]any{"uuid": uuid, "highlights": []map[string]any{}}
	case strings.HasSuffix(path, "/versions"):
		if value, ok, err := c.localMCPToolValue(ctx, "get_record_versions", map[string]any{"uuid": uuid}); err == nil && ok {
			return map[string]any{"uuid": uuid, "versions": value}
		}
		return map[string]any{"uuid": uuid, "versions": []map[string]any{}}
	default:
		if value, ok, err := c.localMCPToolValue(ctx, "get_record_properties", map[string]any{"uuid": uuid}); err == nil && ok {
			if record, ok := value.(map[string]any); ok {
				return record
			}
			return map[string]any{"uuid": uuid, "record": value}
		}
		return map[string]any{
			"uuid":      uuid,
			"item_link": "x-devonthink-item://" + uuid,
			"local":     true,
			"warnings":  []string{"Live record metadata is not enabled in the local adapter yet."},
		}
	}
}

func (c *Client) localBlockedOperation(method, path string) map[string]any {
	return map[string]any{
		"status":  "blocked",
		"method":  method,
		"path":    path,
		"message": "This local operation needs an explicit safe-write implementation before it can modify DEVONthink.",
	}
}

func (c *Client) localMCPToolJSON(ctx context.Context, tool string, args map[string]any) (json.RawMessage, bool, error) {
	text, ok, err := c.localMCPToolText(ctx, tool, args)
	if err != nil || !ok {
		return nil, false, err
	}
	text = strings.TrimSpace(text)
	if text == "" || !json.Valid([]byte(text)) {
		return nil, false, nil
	}
	return json.RawMessage(text), true, nil
}

func (c *Client) localMCPToolValue(ctx context.Context, tool string, args map[string]any) (any, bool, error) {
	text, ok, err := c.localMCPToolText(ctx, tool, args)
	if err != nil || !ok {
		return nil, false, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", true, nil
	}
	if json.Valid([]byte(text)) {
		var value any
		if err := json.Unmarshal([]byte(text), &value); err != nil {
			return nil, false, err
		}
		return value, true, nil
	}
	return text, true, nil
}

func (c *Client) localMCPToolText(ctx context.Context, tool string, args map[string]any) (string, bool, error) {
	endpoint, token, err := devonthinkMCPEndpointAndToken()
	if err != nil {
		return "", false, err
	}
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      tool,
			"arguments": args,
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", false, err
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false, fmt.Errorf("local MCP %s returned HTTP %d", tool, resp.StatusCode)
	}
	var envelope struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", false, err
	}
	if envelope.Error != nil {
		return "", false, fmt.Errorf("local MCP %s error %d: %s", tool, envelope.Error.Code, envelope.Error.Message)
	}
	if len(envelope.Result.Content) == 0 {
		return "", false, nil
	}
	text := envelope.Result.Content[0].Text
	if envelope.Result.IsError {
		return text, false, fmt.Errorf("local MCP %s failed: %s", tool, text)
	}
	return text, true, nil
}

func (c *Client) localMCPListTools(ctx context.Context) (json.RawMessage, bool, error) {
	endpoint, token, err := devonthinkMCPEndpointAndToken()
	if err != nil {
		return nil, false, err
	}
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]any{},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, false, err
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false, fmt.Errorf("local MCP tools/list returned HTTP %d", resp.StatusCode)
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, false, err
	}
	if envelope.Error != nil {
		return nil, false, fmt.Errorf("local MCP tools/list error %d: %s", envelope.Error.Code, envelope.Error.Message)
	}
	return envelope.Result, true, nil
}

func devonthinkVersion(ctx context.Context) (string, error) {
	out, err := runOSA(ctx, `tell application id "`+devonthinkAppID+`" to get version`)
	return strings.TrimSpace(out), err
}

func devonthinkDatabaseNames(ctx context.Context) ([]string, error) {
	out, err := runOSA(ctx, `set oldDelimiters to AppleScript's text item delimiters
set AppleScript's text item delimiters to (ASCII character 30)
tell application id "`+devonthinkAppID+`" to set devonthinkNames to name of databases
set joinedNames to devonthinkNames as text
set AppleScript's text item delimiters to oldDelimiters
return joinedNames`)
	if err != nil {
		return nil, err
	}
	return splitOSAList(out), nil
}

func devonthinkSelectedRecordNames(ctx context.Context) ([]map[string]any, error) {
	out, err := runOSA(ctx, `set oldDelimiters to AppleScript's text item delimiters
set AppleScript's text item delimiters to (ASCII character 30)
tell application id "`+devonthinkAppID+`" to set devonthinkNames to name of selected records
set joinedNames to devonthinkNames as text
set AppleScript's text item delimiters to oldDelimiters
return joinedNames`)
	if err != nil {
		return []map[string]any{}, err
	}
	names := splitOSAList(out)
	records := make([]map[string]any, 0, len(names))
	for _, name := range names {
		records = append(records, map[string]any{"name": name})
	}
	return records, nil
}

func runOSA(ctx context.Context, script string) (string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, "osascript", "-e", script) // #nosec G204 -- command is fixed to local osascript; script strings are internal adapter snippets.
	out, err := cmd.CombinedOutput()
	if timeoutCtx.Err() != nil {
		return "", timeoutCtx.Err()
	}
	if err != nil {
		return strings.TrimSpace(string(out)), err
	}
	return string(out), nil
}

func splitOSAList(out string) []string {
	out = strings.TrimSpace(out)
	if out == "" || out == "missing value" {
		return []string{}
	}
	parts := []string{out}
	if strings.Contains(out, "\x1e") {
		parts = strings.Split(out, "\x1e")
	}
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

func devonthinkMCPToolsPath() (string, bool) {
	candidates := []string{
		"/Applications/DEVONthink.app/Contents/Resources/mcp-tools.json",
		"/Applications/DEVONthink.app/Contents/Library/LoginItems/DEVONthink MCP.app/Contents/Resources/mcp-tools.json",
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
	}
	return "", false
}

func devonthinkMCPEndpointAndToken() (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	configPath := filepath.Join(home, "Library", "Application Support", "DEVONthink", "MCP", "config.json")
	data, err := os.ReadFile(configPath) // #nosec G304 -- path is the fixed DEVONthink MCP config under the user's home directory.
	if err != nil {
		return "", "", err
	}
	var cfg struct {
		Auth struct {
			BearerToken string `json:"bearerToken"`
		} `json:"auth"`
		Server struct {
			Port int    `json:"port"`
			Name string `json:"name"`
		} `json:"server"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", "", err
	}
	port := cfg.Server.Port
	if port == 0 {
		port = 8420
	}
	return "http://127.0.0.1:" + strconv.Itoa(port) + "/mcp", cfg.Auth.BearerToken, nil
}

func writeLocalOutput(path string, data []byte) error {
	if path == "-" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func recordIDFromPath(path string) string {
	rest := strings.TrimPrefix(path, "/records/")
	if idx := strings.Index(rest, "/"); idx >= 0 {
		return rest[:idx]
	}
	return rest
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func intParam(params map[string]string, key string) (int, bool) {
	if params == nil {
		return 0, false
	}
	value := strings.TrimSpace(params[key])
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func localMCPToolLooksReadOnly(tool string) bool {
	tool = strings.ToLower(strings.TrimSpace(tool))
	if tool == "" {
		return false
	}
	for _, prefix := range []string{"get_", "list_", "search_", "extract_", "find_", "is_"} {
		if strings.HasPrefix(tool, prefix) {
			return true
		}
	}
	switch tool {
	case "lookup_records":
		return true
	}
	return false
}

func parseLooseMCPArgs(raw string) (map[string]any, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}, true
	}
	out := map[string]any{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		sep := "="
		if !strings.Contains(part, sep) {
			sep = ":"
		}
		key, value, ok := strings.Cut(part, sep)
		if !ok {
			return nil, false
		}
		key = strings.Trim(strings.TrimSpace(key), `"'`)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			return nil, false
		}
		out[key] = coerceLooseMCPValue(value)
	}
	return out, true
}

func coerceLooseMCPValue(value string) any {
	if value == "" {
		return ""
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	if strings.EqualFold(value, "true") {
		return true
	}
	if strings.EqualFold(value, "false") {
		return false
	}
	return value
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprint(err)
}
