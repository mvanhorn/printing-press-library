// Hand-authored test for PATCH(clickup-errors-print-once).

package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// main.go is the single place a returned error is printed; Cobra itself must
// stay silent or the user sees every error twice.
func TestRootCommandDoesNotPrintReturnedErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.pdf")
	cases := map[string][]string{
		"missing upload file": {"task", "attach", "abc123xyz", missing},
		"missing file arg":    {"task", "attach", "abc123xyz"},
		"unknown flag":        {"task", "get", "abc123xyz", "--no-such-flag"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			var flags rootFlags
			root := newRootCmd(&flags)
			var stdout, stderr bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.SetArgs(args)
			err := root.Execute()
			if err == nil {
				t.Fatal("expected an error")
			}
			if strings.Contains(stderr.String(), err.Error()) || strings.Contains(stderr.String(), "Error:") {
				t.Errorf("Cobra printed the error itself (main.go prints it again):\n%s", stderr.String())
			}
		})
	}
}
