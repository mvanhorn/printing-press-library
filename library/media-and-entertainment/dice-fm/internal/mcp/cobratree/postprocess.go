// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Output post-processing hook for cobratree-mirrored CLI commands.
//
// The cobratree mirror is generic infrastructure (it shells out to the CLI and
// returns stdout verbatim). Some mirrored commands emit personal data
// (`fans top`/`fans profile`/`fans optin`, `door list`), which must be
// pseudonymized at the MCP boundary before reaching the model — but the
// redaction policy belongs to the application (internal/mcp), not to this
// generic walker. This registry lets the application register a per-command
// post-processor and an extra include_pii schema option without cobratree
// importing the pseudonymizer.
package cobratree

import (
	"strings"
	"sync"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// OutputPostProcessor transforms a mirrored command's stdout before it is
// returned to the MCP caller. args is the structured tool-call argument map
// (e.g. to read include_pii). It must return the (possibly transformed) output.
type OutputPostProcessor func(stdout string, args map[string]any) (string, error)

var (
	postProcMu        sync.RWMutex
	outputPostProcs   = map[string]OutputPostProcessor{}
	extraToolOptions  = map[string][]mcplib.ToolOption{}
	descriptionSuffix = map[string]string{}
	forcedCLIArgs     = map[string][]string{}
)

// pathKey joins a command path into the registry key (space-separated, the same
// shape a user types — e.g. "fans top", "door list").
func pathKey(path []string) string {
	return strings.Join(path, " ")
}

// RegisterOutputPostProcessor registers pp for the mirrored command at the given
// space-separated command path (e.g. "fans top"). Idempotent: a second call for
// the same path replaces the first.
func RegisterOutputPostProcessor(commandPath string, pp OutputPostProcessor) {
	postProcMu.Lock()
	defer postProcMu.Unlock()
	outputPostProcs[commandPath] = pp
}

// RegisterForcedCLIArgs adds command-line args that are always included before
// a registered post-processor sees stdout.
func RegisterForcedCLIArgs(commandPath string, args ...string) {
	postProcMu.Lock()
	defer postProcMu.Unlock()
	forcedCLIArgs[commandPath] = append([]string{}, args...)
}

// RegisterExtraToolOption adds an extra MCP tool option (e.g. an include_pii
// bool arg) to the mirrored command at commandPath. Multiple options accumulate.
func RegisterExtraToolOption(commandPath string, opt mcplib.ToolOption) {
	postProcMu.Lock()
	defer postProcMu.Unlock()
	extraToolOptions[commandPath] = append(extraToolOptions[commandPath], opt)
}

// RegisterDescriptionSuffix appends suffix to the mirrored command's MCP
// description (e.g. the "returns personal data" notice), so a host can gate
// auto-approval. Idempotent per path.
func RegisterDescriptionSuffix(commandPath, suffix string) {
	postProcMu.Lock()
	defer postProcMu.Unlock()
	descriptionSuffix[commandPath] = suffix
}

func lookupPostProcessor(commandPath string) (OutputPostProcessor, bool) {
	postProcMu.RLock()
	defer postProcMu.RUnlock()
	pp, ok := outputPostProcs[commandPath]
	return pp, ok
}

func lookupExtraToolOptions(commandPath string) []mcplib.ToolOption {
	postProcMu.RLock()
	defer postProcMu.RUnlock()
	return extraToolOptions[commandPath]
}

func lookupDescriptionSuffix(commandPath string) string {
	postProcMu.RLock()
	defer postProcMu.RUnlock()
	return descriptionSuffix[commandPath]
}

func lookupForcedCLIArgs(commandPath string) []string {
	postProcMu.RLock()
	defer postProcMu.RUnlock()
	return append([]string{}, forcedCLIArgs[commandPath]...)
}
