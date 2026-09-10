package cli

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/internal/cliutil/testenv"
)

// TestDeliverCapturedOutputRespectsDryRun is the regression guard for the
// ungated post-run hook: --dry-run must not perform the --deliver side effect,
// while a real run must still deliver.
func TestDeliverCapturedOutputRespectsDryRun(t *testing.T) {
	testenv.Isolate(t)

	dir := t.TempDir()

	flagsWith := func(t *testing.T, spec string, dryRun bool) *rootFlags {
		t.Helper()
		sink, err := ParseDeliverSink(spec)
		if err != nil {
			t.Fatalf("ParseDeliverSink(%q): %v", spec, err)
		}
		buf := &bytes.Buffer{}
		buf.WriteString(`{"result":"ok"}`)
		return &rootFlags{dryRun: dryRun, deliverSink: sink, deliverBuf: buf}
	}

	// A file sink must not be written during a dry run.
	dryRunSink := filepath.Join(dir, "dry-run.json")
	if err := deliverCapturedOutput(flagsWith(t, "file:"+dryRunSink, true)); err != nil {
		t.Fatalf("dry-run delivery: %v", err)
	}
	if _, statErr := os.Stat(dryRunSink); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("--dry-run --deliver file: wrote %s (stat error = %v)", dryRunSink, statErr)
	}

	// A webhook sink must not be dialed during a dry run: the unroutable URL
	// would fail the POST and surface as an error.
	if err := deliverCapturedOutput(flagsWith(t, "webhook:http://127.0.0.1:1/hook", true)); err != nil {
		t.Fatalf("dry-run webhook delivery: %v", err)
	}

	// A real run still delivers, byte for byte.
	realSink := filepath.Join(dir, "real.json")
	if err := deliverCapturedOutput(flagsWith(t, "file:"+realSink, false)); err != nil {
		t.Fatalf("real-run delivery: %v", err)
	}
	body, readErr := os.ReadFile(realSink) // #nosec G304 -- test-owned temp path.
	if readErr != nil {
		t.Fatalf("a real run must still deliver: %v", readErr)
	}
	if string(body) != `{"result":"ok"}` {
		t.Fatalf("delivered body = %q, want the captured buffer", body)
	}

	// No captured buffer and no sink is a no-op, not a panic.
	if err := deliverCapturedOutput(nil); err != nil {
		t.Fatalf("nil flags: %v", err)
	}
	if err := deliverCapturedOutput(&rootFlags{}); err != nil {
		t.Fatalf("empty flags: %v", err)
	}
}
