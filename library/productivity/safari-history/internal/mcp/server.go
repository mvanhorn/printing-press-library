package mcp

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func NewServer() *server.MCPServer {
	s := server.NewMCPServer("safari-history-pp-cli", "1.0.0", server.WithToolCapabilities(false))
	for _, t := range tools() {
		s.AddTool(t.tool, t.handler)
	}
	return s
}

func ServeStdio() error { return server.ServeStdio(NewServer()) }

type toolSpec struct {
	tool    mcp.Tool
	handler server.ToolHandlerFunc
	cmdArgs func(mcp.CallToolRequest) []string
}

func tools() []toolSpec {
	return []toolSpec{
		mk("search", "Full-text (FTS5) search over visited URLs, page titles, and search terms in the local cached store; no sync required. Required: query. Optional: domain, device, since, limit. Prefer this for keyword lookups; use 'visited' when checking one URL/domain.", []arg{{"query", true, "Search query"}, {"domain", false, "Domain filter"}, {"device", false, "Device filter"}, {"since", false, "Since window"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"search"}
			args = appendFlag(args, "domain", reqStr(r, "domain"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return append(args, "--", reqStr(r, "query"))
		}),
		mk("list", "List recent individual visits from the local cached store; no sync required. Optional filters: since/until window, domain, device, transition, min_visits, limit. Use 'domains' or 'report' for aggregates instead of raw rows.", []arg{{"since", false, "Since window"}, {"until", false, "Until window"}, {"domain", false, "Domain filter"}, {"device", false, "Device filter"}, {"transition", false, "Transition filter"}, {"min_visits", false, "Minimum visits"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"list"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "until", reqStr(r, "until"))
			args = appendFlag(args, "domain", reqStr(r, "domain"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "transition", reqStr(r, "transition"))
			args = appendFlag(args, "min-visits", reqStr(r, "min_visits"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("domains", "Rank domains from the local cached store over the --since window; no sync required. Optional: since, device, limit. Returns page counts, visit sums, and category per domain.", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"domains"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("searches", "Query the local cached store for search-engine terms; no sync required. Optional: since, domain, device, limit. Note: unavailable on Safari, which omits search terms from History.db (reports unavailable).", []arg{{"since", false, "Since window"}, {"domain", false, "Domain filter"}, {"device", false, "Device filter"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"searches"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "domain", reqStr(r, "domain"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("downloads", "Query the local cached store for downloads; no sync required. Optional: since, device, limit. Note: unavailable on Safari, which omits downloads from History.db (reports unavailable).", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"downloads"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("visited", "Check the local cached store for whether a URL/domain was visited; no sync required.", []arg{{"target", true, "URL or domain"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"visited"}
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return append(args, "--", reqStr(r, "target"))
		}),
		mk("report", "Summarize browsing activity from the local cached store; no sync required. Optional: since, device, limit. Includes per-day/hour counts, top domains, and productivity split.", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"report"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("heatmap", "Render a weekday-by-hour activity heatmap from the local cached store; no sync required. Optional: since, device, limit. Returns a 7x24 grid of visit counts.", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"heatmap"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("journeys", "Query cached browser topic clusters; no sync required. Optional: since, limit. Note: unavailable on Safari, which has no journeys tables (reports unavailable); use 'topic' for FTS-based topic grouping instead.", []arg{{"since", false, "Since window"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"journeys"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("timeline", "Reconstruct sessions from the local cached store; no sync required. Optional: since, until, device, gap, limit. Use for 'what was I doing on <day>' narratives.", []arg{{"since", false, "Since window/date"}, {"until", false, "Until date"}, {"device", false, "Device filter"}, {"gap", false, "Session gap"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"timeline"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "until", reqStr(r, "until"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "gap", reqStr(r, "gap"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("rabbitholes", "Query cached sessions for productive-to-distracting drift; no sync required. Optional: since, device, gap, limit. Note: unavailable on Safari, which omits navigation transition types from History.db.", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"gap", false, "Session gap"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"rabbitholes"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "gap", reqStr(r, "gap"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("dwell", "Estimate time-on-site per domain from the local cached store; no sync required. Optional: since, device, gap, limit. Note: an inference from visit gaps, not a precise measurement.", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"gap", false, "Dwell cap gap"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"dwell"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "gap", reqStr(r, "gap"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("graph", "Build a navigation graph from the local cached store; no sync required. Optional: since, domain, device, format (json|dot), limit. Note: edges are sparse on Safari.", []arg{{"since", false, "Since window"}, {"domain", false, "Domain filter"}, {"device", false, "Device filter"}, {"format", false, "json|dot"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"graph"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "domain", reqStr(r, "domain"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "format", reqStr(r, "format"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("profile", "Summarize browsing patterns from the local cached store; no sync required. Optional: since, device, limit. Higher-level summary; use 'report' for raw per-day/per-hour counts.", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"profile"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("devices", "List visit-origin buckets from the local cached store; no sync required. Optional: limit. Safari reports a local origin and a synced bucket, with no per-device identity.", []arg{{"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"devices"}
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("icloud-tabs", "List synced iCloud tabs open on the user's other Apple devices (read-only; --refresh is intentionally not exposed over MCP because it activates Safari)", []arg{{"summary", false, "Per-device tab counts instead of per-tab rows (true/false)"}, {"device_name", false, "Filter to devices whose name contains this substring"}, {"pinned", false, "Only pinned tabs (true/false)"}, {"limit", false, "Row limit (default unlimited)"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"icloud-tabs"}
			if strings.EqualFold(reqStr(r, "summary"), "true") {
				args = append(args, "--summary")
			}
			if strings.EqualFold(reqStr(r, "pinned"), "true") {
				args = append(args, "--pinned")
			}
			args = appendFlag(args, "device-name", reqStr(r, "device_name"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("topic", "Gather cached pages about a named topic via full-text matches; no sync required. Required: name. Optional: since, limit. Returns matching pages grouped under the topic.", []arg{{"name", true, "Topic name"}, {"since", false, "Since window"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"topic"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return append(args, "--", reqStr(r, "name"))
		}),
		mk("sql", "Run a read-only SELECT query against the local cached store; no sync required. Required: query (non-SELECT rejected). Optional: limit. Enforced read-only via PRAGMA query_only.", []arg{{"query", true, "SELECT query"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"sql"}
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return append(args, "--", reqStr(r, "query"))
		}),
		mkWrite("sync", "Refresh the local cache from the live Safari history DB and rebuild FTS. Only needed when cached results are known-stale; read tools query the cached store without sync. Optional: profile, accumulate (append into durable archive). Writes local state.", []arg{{"profile", false, "Profile name"}, {"accumulate", false, "Also append into the durable archive (archive mode); pass true to enable"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"sync"}
			args = appendFlag(args, "profile", reqStr(r, "profile"))
			if strings.EqualFold(reqStr(r, "accumulate"), "true") {
				args = append(args, "--accumulate")
			}
			return args
		}),
		mk("archive_status", "Show accumulating-archive status and whether archive.db is queryable offline; no sync required.", nil, func(r mcp.CallToolRequest) []string {
			return []string{"archive", "status"}
		}),
		mkWrite("archive_enable", "Enable the durable accumulating archive by seeding it from the current snapshot.", nil, func(r mcp.CallToolRequest) []string {
			return []string{"archive", "enable"}
		}),
		mkWrite("archive_disable", "Stop accumulating into the archive but keep the archive file.", nil, func(r mcp.CallToolRequest) []string {
			return []string{"archive", "disable"}
		}),
		mk("doctor", "Health-check live refresh access plus cached snapshot/archive readability. Optional: profile. If source_db is missing but cached_store is queryable, answer from search/sql/domains/list/report without sync.", []arg{{"profile", false, "Profile name"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"doctor"}
			args = appendFlag(args, "profile", reqStr(r, "profile"))
			return args
		}),
	}
}

type arg struct {
	name     string
	required bool
	desc     string
}

// mk builds a read-only tool (the common case: every query command).
func mk(name, desc string, args []arg, cmdArgs func(mcp.CallToolRequest) []string) toolSpec {
	return mkTool(name, desc, true, args, cmdArgs)
}

// mkWrite builds a tool that mutates local state (e.g. sync writes the snapshot
// DB and rebuilds the FTS index), so it must not advertise readOnlyHint.
func mkWrite(name, desc string, args []arg, cmdArgs func(mcp.CallToolRequest) []string) toolSpec {
	return mkTool(name, desc, false, args, cmdArgs)
}

func mkTool(name, desc string, readOnly bool, args []arg, cmdArgs func(mcp.CallToolRequest) []string) toolSpec {
	opts := []mcp.ToolOption{mcp.WithDescription(desc), mcp.WithReadOnlyHintAnnotation(readOnly)}
	for _, a := range args {
		if a.required {
			opts = append(opts, mcp.WithString(a.name, mcp.Required(), mcp.Description(a.desc)))
		} else {
			opts = append(opts, mcp.WithString(a.name, mcp.Description(a.desc)))
		}
	}
	tool := mcp.NewTool(name, opts...)
	h := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		base := cmdArgs(req)
		// Place --json immediately after the subcommand name so it is parsed as a
		// flag even when the builder ends with a "--" positional terminator
		// (everything after "--" is treated as a positional arg by cobra).
		args := make([]string, 0, len(base)+1)
		args = append(args, base[0], "--json")
		args = append(args, base[1:]...)
		out, err := runSelf(args...)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("%v: %s", err, out)), nil
		}
		return mcp.NewToolResultText(out), nil
	}
	return toolSpec{tool: tool, handler: h, cmdArgs: cmdArgs}
}

func runSelf(args ...string) (string, error) {
	exe, err := osExecutable()
	if err != nil {
		return "", err
	}
	// #nosec G204 -- the MCP server dispatches to its OWN binary (os.Executable);
	// args are built from validated tool inputs by the per-tool cmdArgs builders,
	// not assembled into a shell string. This is the CLI-as-engine pattern.
	cmd := exec.Command(exe, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText != "" {
			return out, fmt.Errorf("%w: %s", err, errText)
		}
		return out, err
	}
	return out, nil
}

var osExecutable = os.Executable

func reqStr(r mcp.CallToolRequest, k string) string {
	v, _ := r.GetArguments()[k]
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func appendFlag(args []string, flag, val string) []string {
	if strings.TrimSpace(val) == "" {
		return args
	}
	return append(args, "--"+flag, val)
}
