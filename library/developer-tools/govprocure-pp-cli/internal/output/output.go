package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
)

// Exit codes for agent-native use.
const (
	ExitOK        = 0
	ExitUsage     = 2
	ExitNotFound  = 3
	ExitAuth      = 4
	ExitAPI       = 5
	ExitRateLimit = 7
)

// Format controls output rendering mode.
type Format string

const (
	FormatAuto    Format = "auto"
	FormatTable   Format = "table"
	FormatJSON    Format = "json"
	FormatCSV     Format = "csv"
	FormatCompact Format = "compact"
)

// Options holds global output options set from CLI flags.
type Options struct {
	Format     Format
	Compact    bool
	AgentMode  bool
	Select     []string // field filter
	DryRun     bool
}

// IsTTY returns true if stdout is an interactive terminal.
func IsTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// EffectiveFormat resolves the actual format to use given options and TTY state.
func EffectiveFormat(opts *Options) Format {
	if opts.AgentMode {
		return FormatJSON
	}
	if opts.Format != FormatAuto && opts.Format != "" {
		return opts.Format
	}
	if !IsTTY() {
		return FormatJSON
	}
	if opts.Compact {
		return FormatCompact
	}
	return FormatTable
}

// PrintJSON writes v as pretty-printed JSON to stdout.
func PrintJSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "json encode error: %v\n", err)
	}
}

// PrintTable renders headers + rows as an ASCII table.
func PrintTable(headers []string, rows [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
	table.SetBorder(true)
	table.SetAutoWrapText(true)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("│")
	table.SetRowSeparator("─")
	table.SetHeaderLine(true)
	table.SetBorder(false)
	for _, row := range rows {
		table.Append(row)
	}
	table.Render()
}

// PrintCSV writes headers + rows as CSV to stdout.
func PrintCSV(headers []string, rows [][]string) {
	w := csv.NewWriter(os.Stdout)
	_ = w.Write(headers)
	for _, row := range rows {
		_ = w.Write(row)
	}
	w.Flush()
}

// FilterFields filters a map to only include the specified fields.
// If fields is empty, all fields are included.
func FilterFields(m map[string]interface{}, fields []string) map[string]interface{} {
	if len(fields) == 0 {
		return m
	}
	out := make(map[string]interface{}, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if v, ok := m[f]; ok {
			out[f] = v
		}
	}
	return out
}

// Truncate shortens a string to maxLen, adding "..." if needed.
func Truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// FormatDollars formats a float as a dollar amount string.
func FormatDollars(f float64) string {
	if f == 0 {
		return "N/A"
	}
	if f >= 1_000_000 {
		return fmt.Sprintf("$%.1fM", f/1_000_000)
	}
	if f >= 1_000 {
		return fmt.Sprintf("$%.1fK", f/1_000)
	}
	return fmt.Sprintf("$%.0f", f)
}

// Error prints an error message to stderr.
func Error(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
}

// Fatal prints an error to stderr and exits with the given code.
func Fatal(code int, format string, args ...interface{}) {
	Error(format, args...)
	os.Exit(code)
}

// Warn prints a warning to stderr.
func Warn(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "warn: "+format+"\n", args...)
}

// DryRun prints a dry-run message and returns.
func DryRunMsg(action string) {
	fmt.Printf("[dry-run] would: %s\n", action)
}
