// Package cli implements all obs-pp-cli commands.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// Exit codes used throughout the CLI.
// code: 0 — success
// code: 1 — connection error (OBS not reachable)
// code: 2 — scene or source not found (404-equivalent)
// code: 3 — config missing
// code: 4 — invalid argument
// code: 5 — preflight check failed
const (
	// ExitOK is the success exit code.
	ExitOK = 0
	// ExitConnection is the exit code when OBS is unreachable. code: 1
	ExitConnection = 1
	// ExitNotFound is the exit code for missing resource. code: 2
	ExitNotFound = 2
	// ExitConfig is the exit code for missing or invalid config. code: 3
	ExitConfig = 3
	// ExitInvalidArg is the exit code for bad user input. code: 4
	ExitInvalidArg = 4
	// ExitPreflight is the exit code when preflight checks fail. code: 5
	ExitPreflight = 5
)

// isatty returns true if the given file descriptor is a terminal (TTY).
// Used to decide whether to render colour/emoji in output.
func isatty(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// colorsEnabled returns true if colour output should be used.
// Respects the NO_COLOR environment variable (https://no-color.org/) and
// the --no-color flag stored on the root command's flags.
func colorsEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if noColor, _ := RootCmd.Flags().GetBool("no-color"); noColor {
		return false
	}
	return isatty(os.Stdout)
}

// currentFormat returns the --format value from the root command flags.
func currentFormat() string {
	f, _ := RootCmd.PersistentFlags().GetString("format")
	if f == "" {
		return "plain"
	}
	return f
}

// isAgentMode returns true when --agent flag is set.
// In agent mode the CLI omits decorative output and emits JSON or plain text
// suitable for LLM consumption.
func isAgentMode() bool {
	v, _ := RootCmd.PersistentFlags().GetBool("agent")
	return v
}

// isDryRun returns true when --dry-run is set.
func isDryRun() bool {
	v, _ := RootCmd.PersistentFlags().GetBool("dry-run")
	return v
}

// printJSON marshals v to indented JSON and writes it to stdout.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// filterFields selects only the requested field names from a JSON-marshalled
// object. Used to implement --format=select output.
func filterFields(v any, fields []string) (map[string]any, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var all map[string]any
	if err := json.Unmarshal(b, &all); err != nil {
		return nil, err
	}
	out := make(map[string]any, len(fields))
	for _, f := range fields {
		if val, ok := all[f]; ok {
			out[f] = val
		}
	}
	return out, nil
}

// compactObjectFields removes fields with zero/nil/empty values from a map.
// Used in --agent mode to strip verbose fields that add noise for LLMs.
func compactObjectFields(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case string:
			if val != "" {
				out[k] = v
			}
		case float64:
			if val != 0 {
				out[k] = v
			}
		case bool:
			out[k] = v
		default:
			if v != nil {
				out[k] = v
			}
		}
	}
	return out
}

// newTabWriter creates a tab-aligned writer for columnar terminal output.
// Output is flushed to dst (typically os.Stdout) when Flush() is called.
func newTabWriter(dst io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(dst, 0, 0, 2, ' ', 0)
}

// formatMs formats a millisecond duration into human-readable H:MM:SS or M:SS.
func formatMs(ms float64) string {
	total := int(ms / 1000)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// truncate shortens s to at most max runes, appending "..." if truncated.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// hint wraps an error with a user-facing hint.
// hint: message is always lower-case, action-oriented.
func hint(err error, msg string) error {
	return fmt.Errorf("%w\nhint: %s", err, msg)
}

// parseSelectFields splits a comma-separated --select flag value.
func parseSelectFields(sel string) []string {
	if sel == "" {
		return nil
	}
	parts := strings.Split(sel, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// boolPtr is a pointer helper for goobs builder params.
func boolPtr(b bool) *bool { return &b }
