package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

var cliCandidates = []string{
	"/Applications/HomeClaw.app/Contents/MacOS/homeclaw-cli",
	"/Applications/HomeClaw.app/Contents/Resources/homeclaw-cli",
}

var mcpCandidates = []string{
	"/Applications/HomeClaw.app/Contents/Resources/mcp-server.js",
	"/Applications/HomeClaw.app/Contents/Resources/mcp-server/dist/server.js",
}

func firstExisting(candidates []string) string {
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func main() {
	if runtime.GOOS != "darwin" {
		fmt.Fprintln(os.Stderr, "homeclaw-pp-cli requires macOS and the HomeClaw app")
		os.Exit(2)
	}
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		runMCP(os.Args[2:])
		return
	}
	path := os.Getenv("HOMECLAW_CLI_PATH")
	if path == "" {
		path = firstExisting(cliCandidates)
	}
	if path == "" {
		fmt.Fprintln(os.Stderr, "HomeClaw CLI not found; install HomeClaw or set HOMECLAW_CLI_PATH")
		os.Exit(2)
	}
	run(path, os.Args[1:]...)
}

func runMCP(args []string) {
	server := os.Getenv("HOMECLAW_MCP_SERVER_PATH")
	if server == "" {
		server = firstExisting(mcpCandidates)
	}
	if server == "" {
		fmt.Fprintln(os.Stderr, "HomeClaw MCP server not found; install HomeClaw or set HOMECLAW_MCP_SERVER_PATH")
		os.Exit(2)
	}
	node := os.Getenv("HOMECLAW_NODE_PATH")
	if node == "" {
		node = firstExisting([]string{"/opt/homebrew/bin/node", "/usr/local/bin/node", "/usr/bin/node"})
	}
	if node == "" {
		fmt.Fprintln(os.Stderr, "Node.js not found; set HOMECLAW_NODE_PATH")
		os.Exit(2)
	}
	_ = filepath.Clean(server)
	run(node, append([]string{server}, args...)...)
}

func run(path string, args ...string) {
	command := exec.Command(path, args...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
