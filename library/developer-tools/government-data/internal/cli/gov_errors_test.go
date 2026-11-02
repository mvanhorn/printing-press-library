package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/cliutil"
	"testing"
)

func TestGovernmentErrorCodes(t *testing.T) {
	for _, v := range []struct {
		err  error
		code int
	}{{fmt.Errorf("SEC HTTP 404; set SEC_USER_AGENT"), 3}, {fmt.Errorf("SEC HTTP 403; set SEC_USER_AGENT"), 4}, {&cliutil.RateLimitError{URL: "https://efts.sec.gov"}, 7}, {fmt.Errorf("set SEC_USER_AGENT"), 10}} {
		if got := ExitCode(govClassify(v.err)); got != v.code {
			t.Fatalf("%v: got %d want %d", v.err, got, v.code)
		}
	}
}
