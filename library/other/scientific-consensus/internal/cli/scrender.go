// Hand-authored rendering helpers shared by the scientific-consensus engine
// commands. Not generated.
package cli

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

var reCache sync.Map // pattern -> *regexp.Regexp

// reMatch reports whether s matches pattern, caching the compiled regexp.
func reMatch(pattern, s string) bool {
	v, ok := reCache.Load(pattern)
	if !ok {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		reCache.Store(pattern, re)
		v = re
	}
	return v.(*regexp.Regexp).MatchString(s)
}

// emit writes v as machine output (JSON, honoring --select/--compact/--csv) when
// any machine-format flag is set or stdout is not a TTY; otherwise it calls
// human to render a terminal-friendly view.
func emit(cmd *cobra.Command, flags *rootFlags, v any, human func(w io.Writer)) error {
	out := cmd.OutOrStdout()
	if flags.asJSON || flags.agent || flags.compact || flags.csv || flags.quiet || flags.plain || flags.selectFields != "" || !isTerminal(out) {
		return printJSONFiltered(out, v, flags)
	}
	human(out)
	return nil
}

// bar renders a simple proportional bar for human pyramid/distribution views.
func bar(count, max, width int) string {
	if max <= 0 || count <= 0 {
		return ""
	}
	n := count * width / max
	if n < 1 {
		n = 1
	}
	return strings.Repeat("█", n)
}

// requireQuery returns the joined positional args as the query, or a usage error
// when none were provided and no flags were set (help-only invocation handled by
// the caller's help branch).
func requireQuery(args []string) (string, error) {
	q := strings.TrimSpace(strings.Join(args, " "))
	if q == "" {
		return "", usageErr(fmt.Errorf("a query argument is required"))
	}
	return q, nil
}
