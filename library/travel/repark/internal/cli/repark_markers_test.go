package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/repark/internal/cliutil/testenv"
)

func TestRawMarkersRejectInvalidWindowBeforeFetch(t *testing.T) {
	for _, window := range []string{"bad", "C34,135N40W120S20E150", "C19,135N19.001W134.999S18.999E135.001"} {
		for _, dryRun := range []bool{false, true} {
			testenv.Isolate(t)
			cmd := RootCmd()
			args := []string{"site", "markers", "--range=" + window, "--json"}
			if dryRun {
				args = append(args, "--dry-run")
			}
			cmd.SetArgs(args)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			cmd.SetContext(ctx)
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			err := cmd.Execute()
			var ce *cliError
			if !errors.As(err, &ce) || ce.code != 2 {
				t.Fatalf("range %q, preview %v: expected usage refusal before source IO, got %v", window, dryRun, err)
			}
		}
	}
}
