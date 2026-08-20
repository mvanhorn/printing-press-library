// Hand-authored: minimal, dependency-free progress indicator. Not generated.
package cli

import (
	"fmt"
	"io"
	"os"
)

// progress is a minimal progress indicator for long-running commands. It writes
// a single self-rewriting status line to stderr — never stdout — so --json and
// other machine output stays clean and pipeable. It auto-disables when stderr is
// not a terminal or when any machine-output flag is set; all methods are safe to
// call on a disabled (or nil) indicator.
type progress struct {
	w       io.Writer
	label   string
	total   int
	enabled bool
}

// newProgress returns a progress indicator for an operation processing `total`
// items (pass 0 when the total is unknown). Whether it actually draws is decided
// up front by progressEnabled.
func newProgress(flags *rootFlags, label string, total int) *progress {
	return &progress{
		w:       os.Stderr,
		label:   label,
		total:   total,
		enabled: progressEnabled(flags),
	}
}

// progressEnabled reports whether a progress line may be drawn: only for an
// interactive stderr with no machine-output flag set. This mirrors emit's
// machine-format gate so scripts, pipes, agents, and --json never see progress
// noise.
func progressEnabled(flags *rootFlags) bool {
	return progressAllowedByFlags(flags) && isTerminal(os.Stderr)
}

// progressAllowedByFlags reports whether the output flags permit a progress line,
// independent of whether stderr is a terminal. Any machine-output flag (or a
// --select projection) disables it so --json, pipes, and agents stay clean.
func progressAllowedByFlags(flags *rootFlags) bool {
	if flags == nil {
		return false
	}
	if flags.asJSON || flags.agent || flags.compact || flags.csv || flags.quiet || flags.plain {
		return false
	}
	if flags.selectFields != "" {
		return false
	}
	return true
}

// update redraws the status line with n items processed. No-op when disabled.
func (p *progress) update(n int) {
	if p == nil || !p.enabled {
		return
	}
	if p.total > 0 {
		fmt.Fprintf(p.w, "\r%s %d/%d", p.label, n, p.total)
	} else {
		fmt.Fprintf(p.w, "\r%s %d", p.label, n)
	}
}

// done clears the status line. No-op when disabled. Safe to call once the
// operation finishes (or is abandoned).
func (p *progress) done() {
	if p == nil || !p.enabled {
		return
	}
	// Carriage return + clear-to-end-of-line erases the transient status so it
	// does not linger above the command's real output.
	fmt.Fprint(p.w, "\r\033[K")
}
