package cobratree

import (
	"strings"
	"testing"
)

// Child output can carry API strings such as console names, which a stranger
// can influence, so control bytes must not reach a terminal unescaped.
func TestCLIFailureDetailEscapesControlBytes(t *testing.T) {
	got := cliFailureDetail("", "console \x1b[2Jname\nsecond line")
	if strings.Contains(got, "\x1b") {
		t.Fatalf("escape byte survived: %q", got)
	}
	if !strings.Contains(got, "\n") {
		t.Fatalf("line breaks should keep their shape: %q", got)
	}
	for _, hint := range stderrHintLines("warning: bad\x1b[2Jname") {
		if strings.Contains(hint, "\x1b") {
			t.Fatalf("escape byte survived in hint: %q", hint)
		}
	}
}
