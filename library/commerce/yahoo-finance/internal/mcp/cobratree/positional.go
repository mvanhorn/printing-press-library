// Local patch: fix positional argument handling in cobratree shell-out tools.
// Filed for upstream: cobratree walker passes positional args as --flag value,
// but cobra commands expect them as bare positional values (e.g. `search TSLA`
// not `search --query TSLA`). This file adds the extraction and routing logic.

package cobratree

import "regexp"

var positionalNameRe = regexp.MustCompile(`<([^>]+)>`)

// parsePositionalArgNames extracts required positional argument names from a
// cobra Use string. Returns nil for variadic commands (repeated names or "...")
// so the caller falls back to the generic "args" field instead.
//
// Examples:
//
//	"search <query> [flags]"           -> ["query"]
//	"sparkline <symbol>"               -> ["symbol"]
//	"compare <symbol> <symbol> [...]"  -> nil  (variadic, repeated name)
func parsePositionalArgNames(use string) []string {
	matches := positionalNameRe.FindAllStringSubmatch(use, -1)
	if len(matches) == 0 {
		return nil
	}
	// Variadic: repeated positional name or explicit "..." → fall back to args.
	seen := make(map[string]bool, len(matches))
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		name := m[1]
		if seen[name] {
			return nil // duplicate → variadic command
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}
