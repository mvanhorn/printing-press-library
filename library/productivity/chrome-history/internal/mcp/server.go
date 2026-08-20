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
	cmdArgs func(mcp.CallToolRequest) []string
}

func tools() []toolSpec {
	return []toolSpec{
		mk("search", "FTS search over URL/title/search terms", []arg{{name: "query", required: true, desc: "Search query"}, {name: "domain", desc: "Domain filter"}, {name: "device", desc: "Device filter"}, {name: "since", desc: "Since window"}, {name: "limit", desc: "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"search"}
			args = appendFlag(args, "domain", reqStr(r, "domain"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return append(args, "--", reqStr(r, "query"))
		}),
		mk("list", "Recent history list", []arg{{name: "since", desc: "Since window"}, {name: "until", desc: "Until window"}, {name: "domain", desc: "Domain filter"}, {name: "device", desc: "Device filter"}, {name: "transition", desc: "Transition filter"}, {name: "min_visits", desc: "Minimum visits"}, {name: "limit", desc: "Row limit"}}, func(r mcp.CallToolRequest) []string {
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
		mk("domains", "Domain frequency ranking", []arg{{name: "since", desc: "Since window"}, {name: "device", desc: "Device filter"}, {name: "limit", desc: "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"domains"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("searches", "Search terms from browser history", []arg{{name: "since", desc: "Since window"}, {name: "domain", desc: "Domain filter"}, {name: "device", desc: "Device filter"}, {name: "limit", desc: "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"searches"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "domain", reqStr(r, "domain"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("downloads", "Download history", []arg{{name: "since", desc: "Since window"}, {name: "device", desc: "Device filter"}, {name: "limit", desc: "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"downloads"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("visited", "Check whether a URL/domain was visited", []arg{{name: "target", required: true, desc: "URL or domain"}, {name: "limit", desc: "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"visited"}
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return append(args, "--", reqStr(r, "target"))
		}),
		mk("report", "Activity report", []arg{{name: "since", desc: "Since window"}, {name: "device", desc: "Device filter"}, {name: "limit", desc: "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"report"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("heatmap", "Hour x weekday activity grid", []arg{{name: "since", desc: "Since window"}, {name: "device", desc: "Device filter"}, {name: "limit", desc: "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"heatmap"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("journeys", "Chrome journeys cluster listing", []arg{{name: "since", desc: "Since window"}, {name: "limit", desc: "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"journeys"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("timeline", "Sessionized browsing timeline", []arg{{name: "since", desc: "Since window/date"}, {name: "until", desc: "Until date"}, {name: "device", desc: "Device filter"}, {name: "gap", desc: "Session gap"}, {name: "limit", desc: "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"timeline"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "until", reqStr(r, "until"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "gap", reqStr(r, "gap"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("rabbitholes", "Detect productive-to-distracting drift", []arg{{name: "since", desc: "Since window"}, {name: "device", desc: "Device filter"}, {name: "gap", desc: "Session gap"}, {name: "limit", desc: "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"rabbitholes"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "gap", reqStr(r, "gap"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("dwell", "Derived dwell-time estimate", []arg{{name: "since", desc: "Since window"}, {name: "device", desc: "Device filter"}, {name: "gap", desc: "Dwell cap gap"}, {name: "limit", desc: "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"dwell"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "gap", reqStr(r, "gap"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("graph", "Navigation graph from visit referrers", []arg{{name: "since", desc: "Since window"}, {name: "domain", desc: "Domain filter"}, {name: "device", desc: "Device filter"}, {name: "format", desc: "json|dot"}, {name: "limit", desc: "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"graph"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "domain", reqStr(r, "domain"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "format", reqStr(r, "format"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("profile", "Behavioral self-profile", []arg{{name: "since", desc: "Since window"}, {name: "device", desc: "Device filter"}, {name: "limit", desc: "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"profile"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "device", reqStr(r, "device"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("devices", "List local/synced/imported/extension origin buckets", []arg{{name: "limit", desc: "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"devices"}
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return args
		}),
		mk("topic", "Merge FTS and journeys by topic", []arg{{name: "name", required: true, desc: "Topic name"}, {name: "since", desc: "Since window"}, {name: "limit", desc: "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"topic"}
			args = appendFlag(args, "since", reqStr(r, "since"))
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return append(args, "--", reqStr(r, "name"))
		}),
		mk("sql", "Run SELECT-only SQL on the active store; archive mode exposes url/time/title history tables", []arg{{name: "query", required: true, desc: "SELECT query"}, {name: "limit", desc: "Row limit"}}, func(r mcp.CallToolRequest) []string {
			args := []string{"sql"}
			args = appendFlag(args, "limit", reqStr(r, "limit"))
			return append(args, "--", reqStr(r, "query"))
		}),
		mkWrite("sync", "Snapshot and index browser history", []arg{{name: "profile", desc: "Profile name"}, {name: "accumulate", desc: "Also append into the durable archive (archive mode)", isBool: true}}, func(r mcp.CallToolRequest) []string {
			args := []string{"sync"}
			args = appendFlag(args, "profile", reqStr(r, "profile"))
			if reqBool(r, "accumulate") {
				args = append(args, "--accumulate")
			}
			return args
		}),
		mk("archive_status", "Show accumulating-archive status (enabled?, counts, baseline).", nil, func(r mcp.CallToolRequest) []string {
			return []string{"archive", "status"}
		}),
		mkWrite("archive_enable", "Enable the durable accumulating archive (seeds from the current snapshot) so history survives Chrome's pruning/clears.", nil, func(r mcp.CallToolRequest) []string {
			return []string{"archive", "enable"}
		}),
		mkWrite("archive_disable", "Stop accumulating into the archive but keep the archive file.", nil, func(r mcp.CallToolRequest) []string {
			return []string{"archive", "disable"}
		}),
		mk("doctor", "Health-check source and snapshot", []arg{{name: "profile", desc: "Profile name"}}, func(r mcp.CallToolRequest) []string {
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
	isBool   bool
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
		propOpts := []mcp.PropertyOption{mcp.Description(a.desc)}
		if a.required {
			propOpts = append(propOpts, mcp.Required())
		}
		if a.isBool {
			opts = append(opts, mcp.WithBoolean(a.name, propOpts...))
		} else {
			opts = append(opts, mcp.WithString(a.name, propOpts...))
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

func reqBool(r mcp.CallToolRequest, k string) bool {
	v, _ := r.GetArguments()[k]
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func appendFlag(args []string, flag, val string) []string {
	if strings.TrimSpace(val) == "" {
		return args
	}
	return append(args, "--"+flag, val)
}
