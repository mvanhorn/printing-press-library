// Copyright 2026 Nimrod Astarhan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written extras for the root command — injected after the generated tree is built.
// Safe to keep across reprints; do NOT add generated-output code here.

package cli

import (
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/dbt-cloud/internal/config"
)

// init injects DBT_CLOUD_ACCOUNT_ID into os.Args for generated commands that
// require account_id as their first positional argument, when the user has
// the env var set but didn't supply an explicit account_id on the command line.
// This fires before cobra parses os.Args, making the env-var default
// transparent to generated RunE functions that just read args[0].
func init() {
	accountID := config.AccountID("")
	if accountID == "" {
		return
	}
	os.Args = injectAccountIDIntoArgs(os.Args, accountID)
}

// injectAccountIDIntoArgs returns a new args slice (copy of os.Args) with
// accountID inserted after the last command-word token for known subcommands
// that require account_id as their first positional argument, when that
// argument is not already present.
func injectAccountIDIntoArgs(args []string, accountID string) []string {
	if len(args) < 2 {
		return args
	}

	// Known subcommand paths (space-joined cobra command words, no binary name)
	// whose first positional argument is account_id. Derived from pp:path
	// annotations containing "{account_id}" in the generated tree.
	accountIDFirstArg := map[string]bool{
		"runs list":                               true,
		"runs retrieve":                           true,
		"runs artifacts list-run":                 true,
		"runs artifacts retrieve-run":             true,
		"runs cancel run":                         true,
		"runs retry retrieve-run-failure-details": true,
		"runs retry run":                          true,
		"jobs list":                               true,
		"jobs retrieve":                           true,
		"jobs create":                             true,
		"jobs update":                             true,
		"jobs destroy":                            true,
		"jobs artifacts retrieve-job":             true,
		"jobs rerun retry-failed-job":             true,
		"jobs run trigger-job":                    true,
		"steps":                                   true,
	}

	// Collect non-flag, non-digit command word tokens.
	// Also record whether the next token after words is a digit (account_id already present).
	subArgs := args[1:]
	var cmdWords []string
	nextIsDigit := false
	for _, tok := range subArgs {
		if strings.HasPrefix(tok, "-") {
			break
		}
		if isAllDigits(tok) {
			nextIsDigit = true
			break // token is a numeric positional (account_id already present)
		}
		cmdWords = append(cmdWords, tok)
	}
	if len(cmdWords) == 0 {
		return args
	}

	// Try longest match first — find the command path that matches.
	for pathLen := len(cmdWords); pathLen >= 1; pathLen-- {
		candidate := strings.Join(cmdWords[:pathLen], " ")
		if !accountIDFirstArg[candidate] {
			continue
		}

		// Matched. If the token immediately following cmdWords[:pathLen] is
		// a digit-only token, account_id is already present — skip injection.
		if pathLen == len(cmdWords) && nextIsDigit {
			return args // already has an account_id
		}
		if pathLen < len(cmdWords) {
			// There's a non-digit word token after the matched path — it's a
			// deeper subcommand. Keep searching for a longer path match.
			continue
		}

		// account_id is absent — inject it after the command path.
		insertAt := 1 + pathLen
		newArgs := make([]string, 0, len(args)+1)
		newArgs = append(newArgs, args[:insertAt]...)
		newArgs = append(newArgs, accountID)
		newArgs = append(newArgs, args[insertAt:]...)
		return newArgs
	}
	return args
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
