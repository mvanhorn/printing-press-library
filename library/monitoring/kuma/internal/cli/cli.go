// Package cli implements the kuma-pp-cli command surface.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	kuma "github.com/mvanhorn/printing-press-library/library/monitoring/kuma/internal/client"
)

// version is the single source of truth for the CLI version, shared by the
// version command, --version, and the agent-context surface.
const version = "2026.8.26"

const usage = `kuma-pp-cli — Uptime Kuma v2 operator CLI

Usage:
  kuma-pp-cli <command> [flags]

Commands:
  health            connectivity + auth check
  monitors          list monitors (--query substring)
  heartbeats        recent beats across monitors (--hours, default 3)
  incident-context  composite brief for one monitor (--monitor id|name)
  set-retries       change a monitor's maxretries (dry-run unless --yes)
  agent-context     emit structured JSON describing this CLI for agents
  version           print version

Global flags:
  --url, --username, --password   connection overrides (default from env)
  --json                          JSON output
`

type exitError struct{ code int }

func (e *exitError) Error() string { return fmt.Sprintf("exit %d", e.code) }

// Run executes the CLI and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer, env func(string) string) int {
	if len(args) < 1 {
		fmt.Fprint(stderr, usage)
		return ExitUsage
	}
	cmd := args[0]
	if cmd == "--version" || cmd == "-version" {
		fmt.Fprintln(stdout, "kuma-pp-cli "+version)
		return ExitOK
	}
	if cmd == "-h" || cmd == "--help" || cmd == "help" {
		fmt.Fprint(stdout, usage)
		return ExitOK
	}
	known := map[string]bool{
		"health": true, "monitors": true, "heartbeats": true,
		"incident-context": true, "set-retries": true, "version": true,
		"agent-context": true,
	}
	if !known[cmd] {
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", cmd, usage)
		return ExitUsage
	}
	if cmd == "version" {
		fmt.Fprintln(stdout, "kuma-pp-cli "+version)
		return ExitOK
	}
	// agent-context describes the CLI itself and must not require credentials
	// or a reachable server, so it is handled before any client construction.
	if cmd == "agent-context" {
		if err := runAgentContext(stdout, args[1:]); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return ExitError
		}
		return ExitOK
	}

	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	// Flag defaults are rendered verbatim by flag.PrintDefaults, so binding
	// credentials as defaults would print the username and password into
	// `<command> --help` output and into any harness log that captures it.
	// Declare them empty and resolve the environment separately.
	urlF := fs.String("url", "", "Kuma base URL (default $UPTIME_KUMA_URL)")
	userF := fs.String("username", "", "username (default $UPTIME_KUMA_USERNAME)")
	passF := fs.String("password", "", "password (default $UPTIME_KUMA_PASSWORD)")
	_ = fs.Bool("json", false, "JSON output") // reserved: machine output mode

	// `<command> --help` is a documented, successful query for that
	// subcommand's flags, so it must print to stdout and exit 0 rather than
	// being treated as a flag-parse error.
	for _, a := range args[1:] {
		if a == "-h" || a == "--help" || a == "help" {
			fmt.Fprintf(stdout, "kuma-pp-cli %s\n\n%s\n\nFlags:\n", cmd, commandSynopsis[cmd])
			fs.SetOutput(stdout)
			fs.PrintDefaults()
			fmt.Fprintf(stdout, "\nExamples:\n%s\n", commandExamples[cmd])
			return ExitOK
		}
	}

	baseURL, username, password := globalOverrides(args[1:],
		firstNonEmpty(*urlF, env("UPTIME_KUMA_URL")),
		firstNonEmpty(*userF, env("UPTIME_KUMA_USERNAME")),
		firstNonEmpty(*passF, env("UPTIME_KUMA_PASSWORD")),
	)
	baseURL = normalizeBaseURL(baseURL)
	*urlF = baseURL
	client := kuma.New(kuma.Config{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Username:   username,
		Password:   password,
		HTTPClient: &http.Client{Timeout: 90 * time.Second},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var err error
	switch cmd {
	case "health":
		err = runHealth(ctx, client, fs, args[1:], stdout, stderr, urlF)
	case "monitors":
		err = runMonitors(ctx, client, fs, args[1:], stdout, stderr)
	case "heartbeats":
		err = runHeartbeats(ctx, client, fs, args[1:], stdout, stderr)
	case "incident-context":
		err = runIncident(ctx, client, fs, args[1:], stdout, stderr)
	case "set-retries":
		err = runSetRetries(ctx, client, fs, args[1:], stdout, stderr)
	}
	return classifyErr(err, stderr)
}

func globalOverrides(args []string, baseURL, username, password string) (string, string, string) {
	values := map[string]*string{"url": &baseURL, "username": &username, "password": &password}
	for i := 0; i < len(args); i++ {
		for name, dst := range values {
			if strings.HasPrefix(args[i], "--"+name+"=") {
				*dst = strings.TrimPrefix(args[i], "--"+name+"=")
			} else if args[i] == "--"+name && i+1 < len(args) {
				i++
				*dst = args[i]
			}
		}
	}
	return baseURL, username, password
}

// commandSynopsis and commandExamples back the per-subcommand help output.
// Runnable examples are part of the help contract, not decoration: they are
// what makes `--help` usable to an operator and to an agent introspecting the
// CLI, so every command must carry at least one.
var commandSynopsis = map[string]string{
	"health":           "Check connectivity and authentication against the configured Uptime Kuma server.",
	"monitors":         "List configured monitors, optionally filtered by a case-insensitive substring.",
	"heartbeats":       "Show recent heartbeat history across monitors within a lookback window.",
	"incident-context": "Build a composite incident brief for a single monitor.",
	"set-retries":      "Change a monitor's retry count. Previews the change unless --yes is passed.",
}

var commandExamples = map[string]string{
	"health": "  # Verify credentials and reachability\n" +
		"  kuma-pp-cli health",
	"monitors": "  # List every monitor\n" +
		"  kuma-pp-cli monitors\n\n" +
		"  # Filter by name\n" +
		"  kuma-pp-cli monitors --query api",
	"heartbeats": "  # Last 3 hours across all monitors\n" +
		"  kuma-pp-cli heartbeats\n\n" +
		"  # Last 6 hours for one monitor\n" +
		"  kuma-pp-cli heartbeats --hours 6 --monitor-id 12",
	"incident-context": "  # Brief for a monitor by id\n" +
		"  kuma-pp-cli incident-context --monitor 12\n\n" +
		"  # Widen the timeline window\n" +
		"  kuma-pp-cli incident-context --monitor 12 --lookback-minutes 180",
	"set-retries": "  # Preview the change (no write)\n" +
		"  kuma-pp-cli set-retries --monitor 12 --maxretries 3\n\n" +
		"  # Apply it\n" +
		"  kuma-pp-cli set-retries --monitor 12 --maxretries 3 --yes",
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// normalizeBaseURL reduces a configured URL to the server origin. Operators
// commonly copy the address out of a browser sitting on a dashboard page, so
// the value arrives as ".../dashboard" or with a query string. The Socket.IO
// endpoint lives at the origin, and appending "/socket.io/" to a page path
// silently returns the dashboard HTML instead of an engine.io packet, which
// surfaces much later as a confusing handshake error.
func normalizeBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return trimmed
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return strings.TrimRight(trimmed, "/")
	}
	return u.Scheme + "://" + u.Host
}

func classifyErr(err error, stderr io.Writer) int {
	if err == nil {
		return ExitOK
	}
	var ee *exitError
	if errors.As(err, &ee) {
		if ee.code != ExitUsage && ee.code != ExitOK {
			fmt.Fprintln(stderr, "error:", err)
		}
		return ee.code
	}
	fmt.Fprintln(stderr, "error:", err)
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "auth failed"):
		return ExitAuth
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "timed out"), strings.Contains(msg, "deadline exceeded"):
		return ExitTimeout
	case strings.Contains(msg, "connection refused"), strings.Contains(msg, "no such host"),
		strings.Contains(msg, "handshake failed"):
		return ExitConnection
	default:
		return ExitError
	}
}
