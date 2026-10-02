// Hand-authored terminal-safety coverage for remote OpenAlex titles.

package cli

import (
	"strings"
	"testing"
	"unicode"
)

func TestSafeTerminalTextRemovesControlCharacters(t *testing.T) {
	got := safeTerminalText("safe\x1b]52;c;payload\a title\nnext")
	if strings.ContainsFunc(got, unicode.IsControl) {
		t.Fatalf("safeTerminalText retained a control character: %q", got)
	}
	if got != "safe ]52;c;payload title next" {
		t.Fatalf("safeTerminalText = %q", got)
	}
}
