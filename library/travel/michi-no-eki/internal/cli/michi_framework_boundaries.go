package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var michiReadOnlyCommands = map[string]bool{
	"catalog": true, "guidance": true, "find": true, "nearby": true,
	"station": true, "readiness": true, "notices": true, "notice": true,
	"station-notices": true, "compare": true, "snapshot": true, "changes": true,
	"stations": true, "bulletins": true,
}

// Suppress automatic journal/correction writes only for source workflows.
// Explicit recall, teach, forget and other optional local commands retain their
// normal semantics; this never sets the blanket no-learn switch.
func michiReadOnlyInvocation(root, executed *cobra.Command, invocationErr error) bool {
	if root != nil && executed == root && invocationErr == nil {
		// Cobra resolves flag-only successful invocations to root help.
		// Unknown commands/flags keep their normal learning error behavior.
		if command, _, err := root.Find(os.Args[1:]); err == nil && command == root {
			return true
		}
	}
	if executed != nil && commandIsHelpInvocation(executed) {
		return true
	}
	if executed != nil && executed != root {
		parts := strings.Fields(executed.CommandPath())
		return len(parts) > 1 && michiReadOnlyCommands[parts[1]]
	}
	if root != nil {
		if command, _, err := root.Find(os.Args[1:]); err == nil && command != nil && command != root {
			parts := strings.Fields(command.CommandPath())
			return len(parts) > 1 && michiReadOnlyCommands[parts[1]]
		}
	}
	if root == nil {
		return false
	}
	chain := journalVerbChain(root, executed)
	return len(chain) > 0 && michiReadOnlyCommands[chain[0]]
}
