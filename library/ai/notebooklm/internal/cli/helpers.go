// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "fmt"

// Typed exit codes for agents: code: 2 usage, code: 3 not found, code: 4 auth,
// code: 5 api, code: 6 conflict, code: 7 rate limit, code: 10 config.

// compactObjectFields and stripVerboseFields are implemented in output.go for --compact JSON.

func dryRunOK(flags *rootFlags) bool {
	return flags != nil && flags.dryRun
}

func applyAgentDefaults(flags *rootFlags) {
	if flags == nil || !flags.agent {
		return
	}
	flags.asJSON = true
	flags.noInput = true
	flags.yes = true
}

func dryRunMessage(cmd string) {
	if colorEnabled() {
		fmt.Printf("dry-run: %s\n", cmd)
		return
	}
	fmt.Printf("dry-run: %s\n", cmd)
}
