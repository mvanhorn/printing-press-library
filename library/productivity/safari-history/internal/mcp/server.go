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
		mk("search", "Full-text (FTS5) search over visited URLs, page titles, and search terms, ranked by relevance. Required: query. Optional: domain, device, since, limit. Returns matching visits with URL, title, visit time, and visit count. Prefer this for keyword lookups; use 'visited' when checking one specific URL/domain.", []arg{{"query", true, "Search query"}, {"domain", false, "Domain filter"}, {"device", false, "Device filter"}, {"since", false, "Since window"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"search"}
			args = appendFlag(args, "domain", reqStr(r, "domain"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return append(args, "--", reqStr(r, "query"))
		}),
		mk("list", "List recent individual visits from the synced snapshot, newest first. Optional filters: since/until window, domain, device, transition, min_visits, limit. Returns per-visit URL, title, visit time, and visit count. Use 'domains' or 'report' for aggregates instead of raw rows.", []arg{{"since", false, "Since window"}, {"until", false, "Until window"}, {"domain", false, "Domain filter"}, {"device", false, "Device filter"}, {"transition", false, "Transition filter"}, {"min_visits", false, "Minimum visits"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
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
		mk("domains", "Rank the most-visited registrable domains over the --since window, with page counts, visit sums, and a productive/neutral/distracting category per domain. Optional: since, device, limit. Returns one ranked row per domain.", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"domains"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("searches", "List the keyword search-engine queries the user ran over the --since window, optionally filtered by --domain. Optional: since, domain, device, limit. Returns each query term with its engine and visit time. Note: unavailable on Safari, which omits search terms from History.db (reports unavailable).", []arg{{"since", false, "Since window"}, {"domain", false, "Domain filter"}, {"device", false, "Device filter"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"searches"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "domain", reqStr(r, "domain"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("downloads", "List downloaded files over the --since window with filename, size, MIME type, and download state. Optional: since, device, limit. Note: unavailable on Safari, which omits downloads from History.db (reports unavailable).", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"downloads"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("visited", "Check whether a URL/domain was visited", []arg{{"target", true, "URL or domain"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"visited"}
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return append(args, "--", reqStr(r, "target"))
		}),
		mk("report", "Summarize browsing activity over the --since window: per-day and per-hour visit counts, top domains, and a productive/neutral/distracting split. Optional: since, device, limit. Use 'profile' for a higher-level behavioral summary instead.", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"report"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("heatmap", "Render a weekday-by-hour activity heatmap of visit counts over the --since window. Optional: since, device, limit. Returns a 7x24 grid of counts (request --json for the raw grid). Shows when during the week the user browses most.", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"heatmap"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("journeys", "List the browser's own topic clusters (Journeys) over the --since window, with cluster labels and top pages. Optional: since, limit. Note: unavailable on Safari, which has no journeys tables (reports unavailable); use 'topic' for FTS-based topic grouping instead.", []arg{{"since", false, "Since window"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"journeys"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("timeline", "Reconstruct ordered browsing sessions for a date or since/until window, splitting into sessions on a --gap idle threshold (e.g. 30m). Optional: since, until, device, gap, limit. Returns sessions with their ordered page visits. Use for 'what was I doing on <day>' narratives.", []arg{{"since", false, "Since window/date"}, {"until", false, "Until date"}, {"device", false, "Device filter"}, {"gap", false, "Session gap"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"timeline"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "until", reqStr(r, "until"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "gap", reqStr(r, "gap"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("rabbitholes", "Find browsing sessions over the --since window that started on productive pages and drifted into distracting ones, split on a --gap idle threshold. Optional: since, device, gap, limit. Note: unavailable on Safari, which omits navigation transition types from History.db (reports unavailable).", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"gap", false, "Session gap"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"rabbitholes"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "gap", reqStr(r, "gap"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("dwell", "Estimate time-on-site per domain over the --since window by differencing consecutive visit timestamps, capping each visit's dwell at --gap (default 30m). Optional: since, device, gap, limit. Returns ranked domains with estimated total dwell. Note: an inference from visit gaps, not a precise measurement.", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"gap", false, "Dwell cap gap"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"dwell"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "gap", reqStr(r, "gap"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("graph", "Build a navigation graph of page nodes and referrer edges (which page led to which) over the --since window. Optional: since, domain, device, format (json|dot for Graphviz), limit. Note: edges are sparse on Safari, which lacks from_visit referrer links in History.db.", []arg{{"since", false, "Since window"}, {"domain", false, "Domain filter"}, {"device", false, "Device filter"}, {"format", false, "json|dot"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"graph"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "domain", reqStr(r, "domain"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "format", reqStr(r, "format"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("profile", "Summarize the user's browsing self over the --since window: peak hours, busiest weekday, top domains, and the productive/neutral/distracting split. Optional: since, device, limit. The highest-level behavioral summary; use 'report' for raw per-day/per-hour counts.", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"profile"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("devices", "List visit-origin buckets (this-device vs synced) with visit counts, first/last seen, and top domains. Optional: limit. Note: Safari reports a single local origin and a synced bucket, with no per-device identity (unlike Chrome). Use the 'device' filter on other commands to scope by origin.", []arg{{"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
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
		mk("topic", "Gather everything the user browsed about a named topic over the --since window via full-text page matches, merging in the browser's Journeys clusters when the source has them. Required: name. Optional: since, limit. Returns the matching pages grouped under the topic.", []arg{{"name", true, "Topic name"}, {"since", false, "Since window"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"topic"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return append(args, "--", reqStr(r, "name"))
		}),
		mk("sql", "Run a read-only SELECT query against the snapshot's history tables for custom analysis the other tools don't cover. Required: query (non-SELECT statements are rejected). Optional: limit. Returns the result rows as JSON. The connection is enforced read-only via PRAGMA query_only.", []arg{{"query", true, "SELECT query"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"sql"}
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return append(args, "--", reqStr(r, "query"))
		}),
		mkWrite("sync", "Snapshot the live Safari history DB into the local cache and rebuild the FTS index; run this first so the read tools have fresh data. Optional: profile, accumulate (pass true to also append into the durable archive). Writes local state (not read-only). Returns the synced page/visit counts.", []arg{{"profile", false, "Profile name"}, {"accumulate", false, "Also append into the durable archive (archive mode); pass true to enable"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"sync"}
			args = appendFlag(args, "profile", reqStr(r, "profile"))
			if strings.EqualFold(reqStr(r, "accumulate"), "true") {
				args = append(args, "--accumulate")
			}
			return args
		}),
		mk("archive_status", "Show accumulating-archive status (enabled, counts, baseline).", nil, func(r mcp.CallToolRequest) []string {
			return []string{"archive", "status"}
		}),
		mkWrite("archive_enable", "Enable the durable accumulating archive by seeding it from the current snapshot.", nil, func(r mcp.CallToolRequest) []string {
			return []string{"archive", "enable"}
		}),
		mkWrite("archive_disable", "Stop accumulating into the archive but keep the archive file.", nil, func(r mcp.CallToolRequest) []string {
			return []string{"archive", "disable"}
		}),
		mk("doctor", "Health-check the Safari history source and the local snapshot: reports whether the source DB is reachable, snapshot freshness, row counts, and archive state. Optional: profile. Run this to diagnose empty results or 'db not found' errors before other tools.", []arg{{"profile", false, "Profile name"}}, func(r mcp.CallToolRequest) []string {
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
