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
	s := server.NewMCPServer("chrome-history-pp-cli", "1.0.0", server.WithToolCapabilities(false))
	for _, t := range tools() {
		s.AddTool(t.tool, t.handler)
	}
	return s
}

func ServeStdio() error { return server.ServeStdio(NewServer()) }

type toolSpec struct {
	tool    mcp.Tool
	handler server.ToolHandlerFunc
}

func tools() []toolSpec {
	return []toolSpec{
		mk("search", "FTS search over URL/title/search terms", []arg{{"query", true, "Search query"}, {"domain", false, "Domain filter"}, {"device", false, "Device filter"}, {"since", false, "Since window"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"search"}
			args = appendFlag(args, "domain", reqStr(r, "domain"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return append(args, "--", reqStr(r, "query"))
		}),
		mk("list", "Recent history list", []arg{{"since", false, "Since window"}, {"until", false, "Until window"}, {"domain", false, "Domain filter"}, {"device", false, "Device filter"}, {"transition", false, "Transition filter"}, {"min_visits", false, "Minimum visits"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
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
		mk("domains", "Domain frequency ranking", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"domains"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("searches", "Search terms from browser history", []arg{{"since", false, "Since window"}, {"domain", false, "Domain filter"}, {"device", false, "Device filter"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"searches"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "domain", reqStr(r, "domain"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("downloads", "Download history", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
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
		mk("report", "Activity report", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"report"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("heatmap", "Hour x weekday activity grid", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"heatmap"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("journeys", "Chrome journeys cluster listing", []arg{{"since", false, "Since window"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"journeys"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("timeline", "Sessionized browsing timeline", []arg{{"since", false, "Since window/date"}, {"until", false, "Until date"}, {"device", false, "Device filter"}, {"gap", false, "Session gap"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"timeline"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "until", reqStr(r, "until"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "gap", reqStr(r, "gap"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("rabbitholes", "Detect productive-to-distracting drift", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"gap", false, "Session gap"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"rabbitholes"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "gap", reqStr(r, "gap"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("dwell", "Derived dwell-time estimate", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"gap", false, "Dwell cap gap"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"dwell"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "gap", reqStr(r, "gap"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("graph", "Navigation graph from visit referrers", []arg{{"since", false, "Since window"}, {"domain", false, "Domain filter"}, {"device", false, "Device filter"}, {"format", false, "json|dot"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"graph"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "domain", reqStr(r, "domain"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "format", reqStr(r, "format"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("profile", "Behavioral self-profile", []arg{{"since", false, "Since window"}, {"device", false, "Device filter"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"profile"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("devices", "List local/synced/imported/extension origin buckets", []arg{{"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"devices"}
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("topic", "Merge FTS and journeys by topic", []arg{{"name", true, "Topic name"}, {"since", false, "Since window"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"topic"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return append(args, "--", reqStr(r, "name"))
		}),
		mk("sql", "Run SELECT-only SQL on snapshot", []arg{{"query", true, "SELECT query"}, {"limit", false, "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"sql"}
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return append(args, "--", reqStr(r, "query"))
		}),
		mkWrite("sync", "Snapshot and index browser history", []arg{{"profile", false, "Profile name"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"sync"}
			args = appendFlag(args, "profile", reqStr(r, "profile"))
			return args
		}),
		mk("doctor", "Health-check source and snapshot", []arg{{"profile", false, "Profile name"}}, func(r mcp.CallToolRequest) []string {
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
	return toolSpec{tool: tool, handler: h}
}

func runSelf(args ...string) (string, error) {
	exe, err := osExecutable()
	if err != nil {
		return "", err
	}
	cmd := exec.Command(exe, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	if err != nil {
		if s := strings.TrimSpace(errBuf.String()); s != "" {
			return "", fmt.Errorf("%w: %s", err, s)
		}
		return "", err
	}
	return strings.TrimSpace(outBuf.String()), nil
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
