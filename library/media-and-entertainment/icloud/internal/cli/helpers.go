// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
)

// ── error types ───────────────────────────────────────────────────────────────

type cliError struct {
	code int
	err  error
}

func (e *cliError) Error() string { return e.err.Error() }
func (e *cliError) Unwrap() error { return e.err }

func usageErr(err error) error   { return &cliError{code: 2, err: err} }
func notFoundErr(err error) error { return &cliError{code: 3, err: err} }
func configErr(err error) error  { return &cliError{code: 10, err: err} }

// ── output ────────────────────────────────────────────────────────────────────

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// newTabWriter returns a tabwriter that flushes to w with aligned columns.
func newTabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
}

// ── color ─────────────────────────────────────────────────────────────────────

func colorize(f *rootFlags, code, s string) string {
	if f.noColor || !isTerminal(os.Stdout) {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

func bold(f *rootFlags, s string) string   { return colorize(f, "1", s) }
func red(f *rootFlags, s string) string    { return colorize(f, "31", s) }
func yellow(f *rootFlags, s string) string { return colorize(f, "33", s) }
func green(f *rootFlags, s string) string  { return colorize(f, "32", s) }

// ── size formatting ───────────────────────────────────────────────────────────

func formatSize(f *rootFlags, gb float64) string {
	s := fmt.Sprintf("%.2f GB", gb)
	switch {
	case gb >= 2:
		return red(f, s)
	case gb >= 0.5:
		return yellow(f, s)
	default:
		return green(f, s)
	}
}

func formatSizeBytes(f *rootFlags, b int64) string {
	gb := float64(b) / (1 << 30)
	return formatSize(f, gb)
}
