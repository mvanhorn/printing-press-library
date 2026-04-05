package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// printJSON marshals v to JSON and prints to stdout.
func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// printTable prints rows as a tab-aligned table with headers.
func printTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	w.Flush()
}

// errorf returns a formatted error. Cobra prints it to stderr.
func errorf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

// infof prints a formatted info message to stderr (progress/status).
func infof(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
