// Package mcp implements a minimal stdio JSON-RPC 2.0 MCP server.
// Each tool wraps an existing CLI subcommand by re-invoking the binary with
// the appropriate args and --json flag. Output is captured and returned as
// MCP "text" content.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Run reads JSON-RPC requests from in (newline-delimited), writes responses to
// out, and logs to errOut. It blocks until in is closed.
//
// cliBin is the path to the defillama-pp-cli binary used to execute each tool
// call. If empty, the running executable is used (the in-process `mcp`
// subcommand path). The separate defillama-pp-mcp binary passes its sibling
// CLI path here.
func Run(in io.Reader, out io.Writer, errOut io.Writer, cliBin ...string) error {
	bin := ""
	if len(cliBin) > 0 && cliBin[0] != "" {
		bin = cliBin[0]
	} else {
		b, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locate binary: %w", err)
		}
		bin = b
	}
	s := &server{bin: bin, out: out, err: errOut}
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 1<<16), 8<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		s.handle(line)
	}
	return scanner.Err()
}

type server struct {
	bin string
	out io.Writer
	err io.Writer
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (s *server) handle(line string) {
	var req request
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		s.respondErr(nil, -32700, "parse error: "+err.Error())
		return
	}
	switch req.Method {
	case "initialize":
		s.respond(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "defillama-pp-cli", "version": "0.1"},
		})
	case "notifications/initialized":
		// no response needed
	case "tools/list":
		s.respond(req.ID, map[string]any{"tools": toolList()})
	case "tools/call":
		s.handleCall(req)
	default:
		s.respondErr(req.ID, -32601, "method not found: "+req.Method)
	}
}

func (s *server) handleCall(req request) {
	var call struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &call); err != nil {
		s.respondErr(req.ID, -32602, "invalid params: "+err.Error())
		return
	}
	args, err := buildArgs(call.Name, call.Arguments)
	if err != nil {
		s.respondErr(req.ID, -32602, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.bin, args...)
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf
	output, runErr := cmd.Output()
	if runErr != nil {
		msg := runErr.Error()
		if e := strings.TrimSpace(stderrBuf.String()); e != "" {
			msg = e
		}
		s.respond(req.ID, map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "error: " + msg + "\n" + string(output)},
			},
			"isError": true,
		})
		return
	}
	s.respond(req.ID, map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(output)}},
	})
}

func (s *server) respond(id json.RawMessage, result any) {
	b, _ := json.Marshal(response{JSONRPC: "2.0", ID: id, Result: result})
	fmt.Fprintln(s.out, string(b))
}

func (s *server) respondErr(id json.RawMessage, code int, msg string) {
	b, _ := json.Marshal(response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}})
	fmt.Fprintln(s.out, string(b))
}

// --- tool metadata ---

func toolList() []map[string]any {
	return []map[string]any{
		simpleTool("defillama_top",
			"Top protocols by TVL, optionally per-chain with fees/revenue/volume.",
			map[string]any{
				"chain":  stringProp("filter by chain (e.g. arbitrum)"),
				"sort":   stringProp("tvl|fees|revenue|volume|change_1d|change_7d"),
				"limit":  intProp("max rows (default 20)"),
				"with":   stringProp("comma list of extras: fees,revenue,volume"),
				"category":     stringProp("filter by category"),
				"include_cex":  boolProp("include CEX protocols when chain is set"),
			}),
		simpleTool("defillama_compare",
			"Side-by-side comparison of protocols (TVL/fees/revenue/volume).",
			map[string]any{
				"protocols": arrayStringProp("protocol slugs to compare", true),
				"metrics":   stringProp("comma list: tvl,fees,revenue,volume,mcap,change_1d,change_7d"),
				"period":    stringProp("optional time-series window: 7d|30d|90d|180d|1y|all"),
			}),
		simpleTool("defillama_yields",
			"Yield pools, filtered by chain/project/min-tvl/stablecoin.",
			map[string]any{
				"chain":            stringProp("filter by chain"),
				"project":          stringProp("filter by project"),
				"min_tvl":          numberProp("minimum TVL in USD"),
				"stablecoin_only":  boolProp("only stablecoin pools"),
				"sort":             stringProp("tvl|apy|apy_base"),
				"limit":            intProp("max rows (default 50)"),
			}),
		simpleTool("defillama_stables",
			"Stablecoin overview or per-chain breakdown.",
			map[string]any{
				"chain": stringProp("show per-chain breakdown"),
				"asset": stringProp("show detail for a specific stablecoin symbol or name"),
				"with":  stringProp("extras: dominance"),
				"flow":  boolProp("compute per-chain supply deltas over period"),
				"period": stringProp("for flow: 7d|30d|90d|180d|1y|all"),
			}),
		simpleTool("defillama_fees",
			"Protocol fees and revenue.",
			map[string]any{
				"protocol": stringProp("protocol slug"),
				"chain":    stringProp("chain filter"),
				"sort":     stringProp("fees|revenue"),
				"history":  boolProp("historical mode"),
				"period":   stringProp("7d|30d|90d|180d|1y|all"),
			}),
		simpleTool("defillama_dexs",
			"DEX volume overview.",
			map[string]any{
				"protocol": stringProp("specific DEX slug"),
				"chain":    stringProp("chain filter"),
				"sort":     stringProp("volume_24h|volume_7d|volume_30d"),
				"limit":    intProp("max rows"),
				"with":     stringProp("extras: market-share"),
				"history":  boolProp("with a protocol, show historical"),
				"period":   stringProp("7d|30d|90d|180d|1y|all"),
			}),
		simpleTool("defillama_profile",
			"Deep dive on a single protocol.",
			map[string]any{
				"protocol": stringPropReq("protocol slug"),
				"period":   stringProp("add historical section over the period"),
			}),
		simpleTool("defillama_price",
			"Live token price (pass-through, no mirror).",
			map[string]any{
				"coins": stringPropReq("comma-separated coin ids in chain:address or coingecko:id format"),
				"at":    stringProp("historical timestamp (unix seconds)"),
			}),
		simpleTool("defillama_sql",
			"Read-only SELECT against the local SQLite mirror.",
			map[string]any{
				"query": stringPropReq("SELECT statement; DDL/DML are rejected"),
			}),
		simpleTool("defillama_sync",
			"Trigger a sync. Without args, runs a full free-tier sync.",
			map[string]any{
				"domain":    stringProp("limit to a single domain (e.g. fees, pools)"),
				"protocol":  stringProp("sync per-protocol history"),
				"chain":     stringProp("sync chain TVL history"),
				"backfill":  boolProp("with --protocol, fetch full history"),
				"status":    boolProp("just print last sync times"),
			}),
	}
}

func simpleTool(name, desc string, props map[string]any) map[string]any {
	required := []string{}
	for k, v := range props {
		if m, ok := v.(map[string]any); ok && m["__required"] == true {
			required = append(required, k)
			delete(m, "__required")
		}
	}
	schema := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return map[string]any{"name": name, "description": desc, "inputSchema": schema}
}

func stringProp(desc string) map[string]any  { return map[string]any{"type": "string", "description": desc} }
func stringPropReq(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc, "__required": true}
}
func intProp(desc string) map[string]any    { return map[string]any{"type": "integer", "description": desc} }
func numberProp(desc string) map[string]any { return map[string]any{"type": "number", "description": desc} }
func boolProp(desc string) map[string]any   { return map[string]any{"type": "boolean", "description": desc} }
func arrayStringProp(desc string, required bool) map[string]any {
	m := map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": desc}
	if required {
		m["__required"] = true
	}
	return m
}

// buildArgs converts a tool name + JSON args into CLI argv. Tools always force
// --json so MCP output is structured.
func buildArgs(toolName string, args map[string]any) ([]string, error) {
	out := []string{"--json"}
	str := func(k string) string {
		if v, ok := args[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}
	intv := func(k string) (int, bool) {
		if v, ok := args[k]; ok {
			switch x := v.(type) {
			case float64:
				return int(x), true
			case int:
				return x, true
			case string:
				if n, err := strconv.Atoi(x); err == nil {
					return n, true
				}
			}
		}
		return 0, false
	}
	floatv := func(k string) (float64, bool) {
		if v, ok := args[k]; ok {
			switch x := v.(type) {
			case float64:
				return x, true
			case int:
				return float64(x), true
			case string:
				if n, err := strconv.ParseFloat(x, 64); err == nil {
					return n, true
				}
			}
		}
		return 0, false
	}
	boolv := func(k string) bool {
		v, ok := args[k]
		if !ok {
			return false
		}
		b, _ := v.(bool)
		return b
	}

	addStr := func(flag, val string) {
		if val != "" {
			out = append(out, flag, val)
		}
	}
	addLim := func() {
		if n, ok := intv("limit"); ok {
			out = append(out, "--limit", strconv.Itoa(n))
		}
	}

	switch toolName {
	case "defillama_top":
		out = append(out, "top")
		addStr("--chain", str("chain"))
		addStr("--sort", str("sort"))
		addStr("--with", str("with"))
		addStr("--category", str("category"))
		if boolv("include_cex") {
			out = append(out, "--include-cex")
		}
		addLim()
	case "defillama_compare":
		out = append(out, "compare")
		protos, _ := args["protocols"].([]any)
		if len(protos) == 0 {
			return nil, fmt.Errorf("compare requires `protocols` array")
		}
		for _, p := range protos {
			if s, ok := p.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		addStr("--metrics", str("metrics"))
		addStr("--period", str("period"))
	case "defillama_yields":
		out = append(out, "yields", "top")
		addStr("--chain", str("chain"))
		addStr("--project", str("project"))
		addStr("--sort", str("sort"))
		if v, ok := floatv("min_tvl"); ok {
			out = append(out, "--min-tvl", strconv.FormatFloat(v, 'f', -1, 64))
		}
		if boolv("stablecoin_only") {
			out = append(out, "--stablecoin-only")
		}
		addLim()
	case "defillama_stables":
		out = append(out, "stables")
		if boolv("flow") {
			out = []string{"--json", "stables", "flow"}
			addStr("--period", str("period"))
		} else {
			addStr("--chain", str("chain"))
			addStr("--with", str("with"))
			if asset := str("asset"); asset != "" {
				out = append(out, asset)
			}
		}
		addLim()
	case "defillama_fees":
		out = append(out, "fees")
		if p := str("protocol"); p != "" {
			out = append(out, p)
		}
		addStr("--chain", str("chain"))
		addStr("--sort", str("sort"))
		if boolv("history") {
			out = append(out, "--history")
		}
		addStr("--period", str("period"))
	case "defillama_dexs":
		out = append(out, "dexs")
		if p := str("protocol"); p != "" {
			out = append(out, p)
		}
		addStr("--chain", str("chain"))
		addStr("--sort", str("sort"))
		addStr("--with", str("with"))
		if boolv("history") {
			out = append(out, "--history")
		}
		addStr("--period", str("period"))
		addLim()
	case "defillama_profile":
		p := str("protocol")
		if p == "" {
			return nil, fmt.Errorf("profile requires `protocol`")
		}
		out = append(out, "profile", p)
		addStr("--period", str("period"))
	case "defillama_price":
		coins := str("coins")
		if coins == "" {
			return nil, fmt.Errorf("price requires `coins`")
		}
		out = append(out, "price", coins)
		addStr("--at", str("at"))
	case "defillama_sql":
		q := str("query")
		if q == "" {
			return nil, fmt.Errorf("sql requires `query`")
		}
		out = append(out, "sql", q)
	case "defillama_sync":
		out = append(out, "sync")
		addStr("--domain", str("domain"))
		addStr("--protocol", str("protocol"))
		addStr("--chain", str("chain"))
		if boolv("backfill") {
			out = append(out, "--backfill")
		}
		if boolv("status") {
			out = append(out, "--status")
		}
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
	return out, nil
}
