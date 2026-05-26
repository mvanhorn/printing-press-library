// defillama-pp-mcp is the MCP stdio server entry point that ships in the
// Printing Press library bundle. It looks for the defillama-pp-cli binary
// alongside itself (or on $PATH) and delegates each tool call to it.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/mcp"
)

func main() {
	cli, err := resolveCLI()
	if err != nil {
		fmt.Fprintln(os.Stderr, "defillama-pp-mcp:", err)
		os.Exit(1)
	}
	if err := mcp.Run(os.Stdin, os.Stdout, os.Stderr, cli); err != nil {
		fmt.Fprintln(os.Stderr, "defillama-pp-mcp:", err)
		os.Exit(1)
	}
}

// resolveCLI returns the absolute path to defillama-pp-cli. Search order:
// 1) sibling binary in the same directory as defillama-pp-mcp
// 2) $DEFILLAMA_PP_CLI override (escape hatch)
// 3) $PATH lookup
func resolveCLI() (string, error) {
	if v := os.Getenv("DEFILLAMA_PP_CLI"); v != "" {
		return v, nil
	}
	self, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(self)
		candidate := filepath.Join(dir, "defillama-pp-cli")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, nil
		}
	}
	if p, err := exec.LookPath("defillama-pp-cli"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("defillama-pp-cli binary not found alongside MCP server, in $PATH, or via $DEFILLAMA_PP_CLI")
}
