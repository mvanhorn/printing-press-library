package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/passive-indices/internal/cliutil"
)

func TestClassifyAPIErrorUpstreamHTMLIsNotAuth(t *testing.T) {
	raw := fmt.Errorf("GET constituents: %w", &cliutil.UpstreamHTMLError{URL: "https://www.niftyindices.com/x.csv", Expected: "CSV or JSON payload", Attempts: 5})
	err := classifyAPIError(raw, &rootFlags{})
	if got := ExitCode(err); got != 9 {
		t.Fatalf("exit code = %d, want 9", got)
	}
	if strings.Contains(err.Error(), "not authenticated") {
		t.Errorf("message must not mention auth: %q", err)
	}
	if got := ExitCode(authErr(fmt.Errorf("x"))); got == 9 {
		t.Errorf("auth exit code collides with upstream HTML code")
	}
}
