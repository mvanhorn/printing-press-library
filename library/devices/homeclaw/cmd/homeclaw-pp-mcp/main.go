package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var mcpCandidates = []string{
	"/Applications/HomeClaw.app/Contents/Resources/mcp-server.js",
	"/Applications/HomeClaw.app/Contents/Resources/mcp-server/dist/server.js",
}

func main() {
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

	command := exec.Command(node, append([]string{filepath.Clean(server)}, os.Args[1:]...)...)
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

func firstExisting(candidates []string) string {
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}
