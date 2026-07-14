// Package mcp exposes the mt5-pp-cli command tree as MCP tools over stdio.
//
// Architecture: each MCP tool handler re-enters the existing cobra root in
// the same process with a synthesised argv plus --agent. Stdout (JSON) becomes
// the tool result; non-zero exits become error results carrying the exit code
// and stderr. No subprocesses; no logic duplication; the safety pipeline (live
// gate, hash-confirm, audit log) applies identically because we are running
// the same handlers the user runs from the shell.
package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	cli "github.com/mvanhorn/printing-press-library/library/other/mt5/internal/cli"
)

// Tool-call log destination. Optional — set via SetLogWriter from the binary's
// --log-file flag. Used so Claude Desktop users can tail a file to see what
// the agent is asking the CLI to do without diving into the host's stderr.
var (
	logMu sync.Mutex
	logW  io.Writer
)

// SetLogWriter installs a writer that dispatch() will append a per-call line
// to. Pass nil to disable. Safe to call from main before serving.
func SetLogWriter(w io.Writer) {
	logMu.Lock()
	defer logMu.Unlock()
	logW = w
}

func writeLogLine(args []string, exitCode int, errSummary string) {
	logMu.Lock()
	defer logMu.Unlock()
	if logW == nil {
		return
	}
	if errSummary != "" {
		fmt.Fprintf(logW, "%s mt5-pp-cli %s  exit=%d  err=%q\n",
			time.Now().UTC().Format(time.RFC3339), strings.Join(args, " "), exitCode, errSummary)
	} else {
		fmt.Fprintf(logW, "%s mt5-pp-cli %s  exit=%d\n",
			time.Now().UTC().Format(time.RFC3339), strings.Join(args, " "), exitCode)
	}
}

// ServerVersion is the version reported by the MCP server. It tracks
// cli.Version so a single -ldflags stamp at build time updates both binaries.
func ServerVersion() string { return cli.Version }

// NewServer builds the MCP server and registers every tool. The server is
// ready to be served over a transport.
func NewServer() *server.MCPServer {
	s := server.NewMCPServer(
		"mt5-pp-mcp",
		ServerVersion(),
		server.WithToolCapabilities(true),
	)

	for _, t := range tools() {
		s.AddTool(t.tool, t.handler)
	}
	return s
}

// ListToolNames returns every registered tool name. Used by 'mt5-pp-mcp
// --list-tools' for smoke tests and documentation.
func ListToolNames() []string {
	regs := tools()
	out := make([]string, 0, len(regs))
	for _, t := range regs {
		out = append(out, t.tool.Name)
	}
	return out
}

// dispatch re-enters cli.NewRootCmd() with the given argv plus --agent so
// stdout is JSON. Returns an MCP success result on exit 0, an error result
// with exit code annotated otherwise.
//
// Each dispatch builds a fresh cobra tree — no shared state between calls.
// The bridge subprocess starts and stops per-call (≈50ms overhead) which is
// fine for the interactive cadence of an MCP client.
func dispatch(ctx context.Context, args ...string) (*mcp.CallToolResult, error) {
	args = append([]string{}, args...)
	args = append(args, "--agent")

	root := cli.NewRootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)

	if err := root.ExecuteContext(ctx); err != nil {
		var ex *cli.ExitErr
		code := 1
		if errors.As(err, &ex) {
			code = ex.Code
		}
		writeLogLine(args, code, err.Error())
		return mcp.NewToolResultError(fmt.Sprintf(
			"mt5-pp-cli %s\nexit %d (%s)\n%s\n%s",
			strings.Join(args, " "),
			code,
			exitName(code),
			err.Error(),
			strings.TrimSpace(stderr.String()),
		)), nil
	}
	writeLogLine(args, 0, "")
	// Success: keep stdout parseable for JSON-producing tools. The one stderr
	// case that must be surfaced on a zero exit is the audit persistence warning:
	// the broker action already happened, but the local audit record failed.
	if auditWarning := auditStderrWarning(stderr.String()); auditWarning != "" {
		return mcp.NewToolResultText(stdout.String() + "\n--- audit warning ---\n" + auditWarning), nil
	}
	return mcp.NewToolResultText(stdout.String()), nil
}

func auditStderrWarning(stderr string) string {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	kept := make([]string, 0, len(lines))
	keepContinuation := false
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "AUDIT ") {
			kept = append(kept, line)
			keepContinuation = true
			continue
		}
		if keepContinuation && strings.HasPrefix(line, "  ") {
			kept = append(kept, line)
			continue
		}
		keepContinuation = false
	}
	return strings.Join(kept, "\n")
}

// exitName puts a name on the documented exit codes so MCP clients can
// distinguish "broker rejected" from "safety rejected" without parsing text.
func exitName(code int) string {
	switch code {
	case 0:
		return "ok"
	case 2:
		return "usage"
	case 3:
		return "not_found"
	case 4:
		return "auth"
	case 5:
		return "broker_rejected"
	case 6:
		return "safety_rejected"
	case 7:
		return "rate_limited"
	case 10:
		return "config"
	case 11:
		return "terminal_down"
	default:
		return "error"
	}
}

// ─── tool registry ──────────────────────────────────────────────────────────

type toolReg struct {
	tool    mcp.Tool
	handler server.ToolHandlerFunc
}

// Tool-annotation policy: mcp-go's default is (readOnly=false, destructive=true)
// because it can't infer intent. We override per-tool below — readOnly=true
// flags tools the agent can call freely; destructive=false on safe tools and
// on local-only writes (sync, backtest run); destructive=true only on tools
// that send broker traffic.
//
// tools returns every MCP tool. Adding a new tool means one entry here; the
// dispatch helper handles the rest.
func tools() []toolReg {
	return []toolReg{
		// Foundation
		{
			tool: mcp.NewTool("mt5_doctor",
				mcp.WithDescription("Run a preflight check: Python, MetaTrader5 package, terminal, login, store writability, safety mode. Non-zero exit means something blocks live commands; the detail field carries remediation."),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false),
			),
			handler: func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return dispatch(ctx, "doctor")
			},
		},
		{
			tool: mcp.NewTool("mt5_account_info",
				mcp.WithDescription("Print account balance, equity, margin, leverage, currency, trade mode. Useful to confirm which broker account the CLI will operate against and whether it is real or demo."),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false),
			),
			handler: func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return dispatch(ctx, "account", "info")
			},
		},
		{
			tool: mcp.NewTool("mt5_terminal_info",
				mcp.WithDescription("Print MT5 terminal build, data path, and connected/trade-allowed flags."),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false),
			),
			handler: func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return dispatch(ctx, "terminal", "info")
			},
		},

		// Live reads
		{
			tool: mcp.NewTool("mt5_symbols_list",
				mcp.WithDescription("List symbols from the local mirror, optionally filtered by a comma-separated glob (e.g. 'EUR*,XAU*'). Requires a prior 'mt5_sync_all' call."),
				mcp.WithString("filter", mcp.Description("Comma-separated glob filter (default: *)")),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false)),
			handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				filter := stringArg(req, "filter", "*")
				return dispatch(ctx, "symbols", "list", "--filter", filter)
			},
		},
		{
			tool: mcp.NewTool("mt5_quote",
				mcp.WithDescription("Get a live bid/ask/spread/last-tick quote for a symbol. Hits the bridge (≈50ms) and does not require sync."),
				mcp.WithString("symbol", mcp.Required(), mcp.Description("Symbol (use broker's exact name — JustMarkets demo uses '.s' suffix e.g. 'EURUSD.s')")),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false)),
			handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				sym, err := requireString(req, "symbol")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				return dispatch(ctx, "quote", "--symbol", sym)
			},
		},
		{
			tool: mcp.NewTool("mt5_positions_list",
				mcp.WithDescription("Snapshot currently open positions. Live, not from the mirror."),
				mcp.WithString("symbol", mcp.Description("Optional symbol filter")),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false)),
			handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				args := []string{"positions", "list"}
				if s := stringArg(req, "symbol", ""); s != "" {
					args = append(args, "--symbol", s)
				}
				return dispatch(ctx, args...)
			},
		},
		{
			tool: mcp.NewTool("mt5_orders_list",
				mcp.WithDescription("Snapshot pending orders. Live, not from the mirror."),
				mcp.WithString("symbol", mcp.Description("Optional symbol filter")),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false)),
			handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				args := []string{"orders", "list"}
				if s := stringArg(req, "symbol", ""); s != "" {
					args = append(args, "--symbol", s)
				}
				return dispatch(ctx, args...)
			},
		},
		{
			tool: mcp.NewTool("mt5_history_deals",
				mcp.WithDescription("Read historical deals from the local mirror over a window. Requires a prior sync over the same window."),
				mcp.WithString("from", mcp.Description("ISO date or relative (e.g. '30d', '2024-01-01'); default 30d")),
				mcp.WithString("to", mcp.Description("ISO date or relative; default 'now'")),
				mcp.WithString("symbol", mcp.Description("Optional symbol filter")),
				mcp.WithString("magic", mcp.Description("Optional magic-number filter")),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false)),
			handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				args := []string{"history", "deals",
					"--from", stringArg(req, "from", "30d"),
					"--to", stringArg(req, "to", "now"),
				}
				if s := stringArg(req, "symbol", ""); s != "" {
					args = append(args, "--symbol", s)
				}
				if m := stringArg(req, "magic", ""); m != "" {
					args = append(args, "--magic", m)
				}
				return dispatch(ctx, args...)
			},
		},
		{
			tool: mcp.NewTool("mt5_risk_preview",
				mcp.WithDescription("Project margin and ±pip P&L for a hypothetical position. Read-only, hits the bridge for live price."),
				mcp.WithString("symbol", mcp.Required(), mcp.Description("Symbol")),
				mcp.WithNumber("volume", mcp.Required(), mcp.Description("Lots (e.g. 0.10)")),
				mcp.WithNumber("pips", mcp.Description("Pip distance for ±P&L (default 100)")),
				mcp.WithString("side", mcp.Description("'buy' or 'sell' (default 'buy')")),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false)),
			handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				sym, err := requireString(req, "symbol")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				vol, err := requireFloat(req, "volume")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				pips := floatArg(req, "pips", 100)
				args := []string{"risk", "preview",
					"--symbol", sym,
					"--volume", fmtF(vol),
					"--pips", fmtF(pips),
					"--side", stringArg(req, "side", "buy"),
				}
				return dispatch(ctx, args...)
			},
		},

		// Algo
		{
			tool: mcp.NewTool("mt5_stats_summary",
				mcp.WithDescription("Compute win rate, profit factor, expectancy, max drawdown, Sharpe & Sortino from the mirror's deals table over --since."),
				mcp.WithString("since", mcp.Description("Window (default 30d)")),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false)),
			handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return dispatch(ctx, "stats", "summary", "--since", stringArg(req, "since", "30d"))
			},
		},
		{
			tool: mcp.NewTool("mt5_sql",
				mcp.WithDescription("Run a read-only SQL query against the local mirror. Tables: accounts, symbols, ticks, bars_M1..MN1, orders, history_orders, positions, deals, calendar_events, features, backtests, audit. INSERT/UPDATE/DELETE are blocked — this tool is read-only by design; the human can run those via the CLI if needed."),
				mcp.WithString("query", mcp.Required(), mcp.Description("SQL SELECT/explain statement")),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false)),
			handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				q, err := requireString(req, "query")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				if looksLikeSQLWrite(q) {
					return mcp.NewToolResultError("mt5_sql is read-only; INSERT/UPDATE/DELETE/DROP/ALTER/CREATE blocked. Ask the human to run those from the CLI with --write."), nil
				}
				return dispatch(ctx, "sql", q)
			},
		},

		// Sync
		{
			tool: mcp.NewTool("mt5_sync_all",
				mcp.WithDescription("Mirror MT5 data into the local SQLite store: symbols, positions, orders, history deals/orders. Optional bars and ticks. First sync over a wide window can take minutes; subsequent runs are incremental."),
				mcp.WithString("since", mcp.Description("Lower bound (default 30d)")),
				mcp.WithBoolean("with_bars", mcp.Description("Also sync bars (slow without --only-symbols)")),
				mcp.WithBoolean("with_ticks", mcp.Description("Also sync ticks (huge — use only_symbols)")),
				mcp.WithString("only_symbols", mcp.Description("Comma-separated symbols to restrict bars/ticks")),
				mcp.WithString("bars_tfs", mcp.Description("Comma-separated TFs for --bars (default M5,H1,D1)")),
				mcp.WithDestructiveHintAnnotation(false)),
			handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				args := []string{"sync", "all", "--since", stringArg(req, "since", "30d")}
				if boolArg(req, "with_bars", false) {
					args = append(args, "--bars")
				}
				if boolArg(req, "with_ticks", false) {
					args = append(args, "--ticks")
				}
				if s := stringArg(req, "only_symbols", ""); s != "" {
					args = append(args, "--only-symbols", s)
				}
				if s := stringArg(req, "bars_tfs", ""); s != "" {
					args = append(args, "--bars-tf", s)
				}
				return dispatch(ctx, args...)
			},
		},

		// Writes — read-only preview
		{
			tool: mcp.NewTool("mt5_order_check",
				mcp.WithDescription("Preflight an order without sending it. Returns broker's predicted margin, fees, retcode (10009 = would succeed). Read-only, no safety gate."),
				mcp.WithString("symbol", mcp.Required()),
				mcp.WithString("side", mcp.Required(), mcp.Description("'buy' or 'sell'")),
				mcp.WithNumber("volume", mcp.Required(), mcp.Description("Lots")),
				mcp.WithNumber("sl", mcp.Description("Stop-loss price (optional)")),
				mcp.WithNumber("tp", mcp.Description("Take-profit price (optional)")),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false)),
			handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				sym, err := requireString(req, "symbol")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				side, err := requireString(req, "side")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				vol, err := requireFloat(req, "volume")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				args := []string{"order", "check", "--symbol", sym, "--side", side, "--volume", fmtF(vol)}
				if v, ok := optFloat(req, "sl"); ok {
					args = append(args, "--sl", fmtF(v))
				}
				if v, ok := optFloat(req, "tp"); ok {
					args = append(args, "--tp", fmtF(v))
				}
				return dispatch(ctx, args...)
			},
		},

		// Writes — order_send (dry-run by default; --confirm to execute)
		{
			tool: mcp.NewTool("mt5_order_send",
				mcp.WithDescription(`Send a market order through the full safety pipeline.

WITHOUT 'confirm': dry-run. Returns a SHA-256 intent hash. Surface the dry-run summary to the human and ask for explicit approval.
WITH 'confirm' set to the printed hash: executes if the hash matches within the 60–120s bucketed window (the hash is valid for the current 60s bucket and the previous one, so the user has the full 60s minimum to type it) and (for real accounts) MT5_LIVE=1 + i_understand_this_is_live are set.

The hash binds symbol/side/volume/sl/tp/deviation — not live price — so it survives normal tick movement but a wider deviation can't be slipped through under a hash shown for a tighter one. The safety layer rejects with exit 6 if the gate isn't passed; the broker rejects with exit 5.`),
				mcp.WithString("symbol", mcp.Required()),
				mcp.WithString("side", mcp.Required(), mcp.Description("'buy' or 'sell'")),
				mcp.WithNumber("volume", mcp.Required(), mcp.Description("Lots")),
				mcp.WithNumber("sl", mcp.Description("Stop-loss price (optional)")),
				mcp.WithNumber("tp", mcp.Description("Take-profit price (optional)")),
				mcp.WithString("comment", mcp.Description("Order comment shown to the broker")),
				mcp.WithNumber("magic", mcp.Description("Magic number for EA-tagged orders")),
				mcp.WithString("confirm", mcp.Description("SHA-256 hash from the prior dry-run. Required to actually send.")),
				mcp.WithBoolean("i_understand_this_is_live", mcp.Description("Required on real accounts (trade_mode=2). The 'MT5_LIVE=1' env var must also be set by the human in the MCP host config."))),
			handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				sym, err := requireString(req, "symbol")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				side, err := requireString(req, "side")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				vol, err := requireFloat(req, "volume")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				args := []string{"order", "send", "--symbol", sym, "--side", side, "--volume", fmtF(vol)}
				if v, ok := optFloat(req, "sl"); ok {
					args = append(args, "--sl", fmtF(v))
				}
				if v, ok := optFloat(req, "tp"); ok {
					args = append(args, "--tp", fmtF(v))
				}
				if v := stringArg(req, "comment", ""); v != "" {
					args = append(args, "--comment", v)
				}
				if v, ok := optFloat(req, "magic"); ok {
					args = append(args, "--magic", strconv.FormatInt(int64(v), 10))
				}
				if v := stringArg(req, "confirm", ""); v != "" {
					args = append(args, "--confirm", v)
				}
				if boolArg(req, "i_understand_this_is_live", false) {
					args = append(args, "--i-understand-this-is-live")
				}
				return dispatch(ctx, args...)
			},
		},

		// Writes — close_all (dry-run by default; --confirm to execute)
		{
			tool: mcp.NewTool("mt5_close_all",
				mcp.WithDescription(`Bulk-close all open positions matching a SQL predicate.

WITHOUT 'confirm': dry-run. Returns the candidate ticket list and a SHA-256 intent hash. Surface the candidate list to the human before requesting confirmation.
WITH 'confirm': closes each candidate sequentially. If any individual close is rejected the overall exit is 5 (broker_rejected); positions that did close stay closed (no transactional rollback across the broker).

The intent hash covers ticket list + filter — not live profit — so it survives ticks. Each ticket is audited individually.`),
				mcp.WithString("filter", mcp.Required(), mcp.Description("SQL WHERE clause on the live positions snapshot, e.g. \"profit < 0\" or \"symbol = 'EURUSD.s' AND volume <= 0.05\"")),
				mcp.WithString("confirm", mcp.Description("SHA-256 hash from the prior dry-run. Required to actually close.")),
				mcp.WithBoolean("i_understand_this_is_live", mcp.Description("Required on real accounts."))),
			handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				filter, err := requireString(req, "filter")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				args := []string{"close", "all", "--filter", filter}
				if v := stringArg(req, "confirm", ""); v != "" {
					args = append(args, "--confirm", v)
				}
				if boolArg(req, "i_understand_this_is_live", false) {
					args = append(args, "--i-understand-this-is-live")
				}
				return dispatch(ctx, args...)
			},
		},

		// Quant
		{
			tool: mcp.NewTool("mt5_backtest_run",
				mcp.WithDescription("Run a built-in strategy against historical bars from the local mirror. v1 ships 'sma-cross' (long-only SMA crossover with --fast/--slow). Persists the run row to the backtests table."),
				mcp.WithString("symbol", mcp.Required()),
				mcp.WithString("tf", mcp.Description("Timeframe (default H1)")),
				mcp.WithString("from", mcp.Description("Start ISO date / relative (default 1y)")),
				mcp.WithString("to", mcp.Description("End (default now)")),
				mcp.WithNumber("deposit", mcp.Description("Starting equity (default 10000)")),
				mcp.WithNumber("fast", mcp.Description("Fast SMA period (default 20)")),
				mcp.WithNumber("slow", mcp.Description("Slow SMA period (default 50)")),
				mcp.WithDestructiveHintAnnotation(false)),
			handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				sym, err := requireString(req, "symbol")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				args := []string{"backtest", "run", "--strategy", "sma-cross",
					"--symbol", sym,
					"--tf", stringArg(req, "tf", "H1"),
					"--from", stringArg(req, "from", "1y"),
					"--to", stringArg(req, "to", "now"),
					"--deposit", fmtF(floatArg(req, "deposit", 10000)),
					"--fast", strconv.Itoa(int(floatArg(req, "fast", 20))),
					"--slow", strconv.Itoa(int(floatArg(req, "slow", 50))),
				}
				return dispatch(ctx, args...)
			},
		},
		{
			tool: mcp.NewTool("mt5_backtest_list",
				mcp.WithDescription("List prior backtest runs stored in the local mirror."),
				mcp.WithNumber("limit", mcp.Description("Max rows (default 25)")),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false)),
			handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return dispatch(ctx, "backtest", "list", "--limit", strconv.Itoa(int(floatArg(req, "limit", 25))))
			},
		},

		// Audit
		{
			tool: mcp.NewTool("mt5_audit_tail",
				mcp.WithDescription("Show the most recent N entries from the append-only write audit log. Covers every dry-run, confirmed write, and rejection."),
				mcp.WithNumber("limit", mcp.Description("Max rows (default 50)")),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithDestructiveHintAnnotation(false)),
			handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return dispatch(ctx, "audit", "tail", "--limit", strconv.Itoa(int(floatArg(req, "limit", 50))))
			},
		},
	}
}

// ─── arg helpers ───────────────────────────────────────────────────────────

func stringArg(req mcp.CallToolRequest, key, def string) string {
	args := req.GetArguments()
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return def
}

func requireString(req mcp.CallToolRequest, key string) (string, error) {
	args := req.GetArguments()
	v, ok := args[key]
	if !ok {
		return "", fmt.Errorf("missing required argument %q", key)
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return "", fmt.Errorf("argument %q must be a non-empty string", key)
	}
	return s, nil
}

func floatArg(req mcp.CallToolRequest, key string, def float64) float64 {
	args := req.GetArguments()
	if v, ok := args[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return def
}

func optFloat(req mcp.CallToolRequest, key string) (float64, bool) {
	args := req.GetArguments()
	if v, ok := args[key]; ok {
		if f, ok := v.(float64); ok {
			return f, true
		}
	}
	return 0, false
}

func requireFloat(req mcp.CallToolRequest, key string) (float64, error) {
	args := req.GetArguments()
	v, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("missing required argument %q", key)
	}
	f, ok := v.(float64)
	if !ok {
		return 0, fmt.Errorf("argument %q must be numeric", key)
	}
	return f, nil
}

func boolArg(req mcp.CallToolRequest, key string, def bool) bool {
	args := req.GetArguments()
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

func fmtF(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// looksLikeSQLWrite is a fast-path UX guard on the MCP mt5_sql tool. The
// load-bearing gate is that dispatch routes mt5_sql to the CLI sql command
// without --write, which opens store.OpenReadOnly (mode=ro + query_only).
// We drop PRAGMA so read-only PRAGMAs (table_info, schema_version) and
// pragma_table_info() table-valued functions stay accessible from the
// agent — those have legitimate diagnostic uses.
func looksLikeSQLWrite(q string) bool {
	first := strings.ToUpper(strings.TrimSpace(q))
	for _, kw := range []string{"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE", "REPLACE", "TRUNCATE"} {
		if strings.HasPrefix(first, kw) {
			return true
		}
	}
	return false
}
