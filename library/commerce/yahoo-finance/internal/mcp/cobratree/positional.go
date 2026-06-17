// Local patch: fix positional argument handling in cobratree shell-out tools.
// Filed for upstream: cobratree walker passes positional args as --flag value,
// but cobra commands expect them as bare positional values (e.g. `search TSLA`
// not `search --query TSLA`). This file adds the extraction and routing logic.

package cobratree

import (
	"regexp"
	"strings"
)

var positionalNameRe = regexp.MustCompile(`<([^>]+)>`)

// parsePositionalArgNames extracts required positional argument names from a
// cobra Use string. Returns nil for variadic commands so the caller falls back
// to the generic "args" field instead.
//
// Variadic signals:
//   - repeated positional name: "compare <symbol> <symbol> [...]"
//   - name ending in "...":     "gather <file...>"
//   - explicit "[...]" token:   "cmd <a> <b> [...]"
//
// Examples:
//
//	"search <query> [flags]"           -> ["query"]
//	"sparkline <symbol>"               -> ["symbol"]
//	"fx <from> <to>"                   -> ["from", "to"]
//	"compare <symbol> <symbol> [...]"  -> nil  (variadic, repeated name)
//	"gather <file...>"                 -> nil  (variadic, ellipsis name)
//	"cmd <a> <b> [...]"                -> nil  (variadic, explicit [...])
func parsePositionalArgNames(use string) []string {
	if strings.Contains(use, "[...]") {
		return nil
	}
	matches := positionalNameRe.FindAllStringSubmatch(use, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(matches))
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		name := m[1]
		if seen[name] || strings.HasSuffix(name, "...") {
			return nil // variadic command
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}
