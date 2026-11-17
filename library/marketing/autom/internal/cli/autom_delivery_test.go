// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/serp"
)

func TestDeliveryHooks(t *testing.T) {
	ran := 0
	afterDelivery(func() error { ran++; return nil })
	if err := runDeliveryHooks(true); err != nil || ran != 0 {
		t.Fatalf("skipped hooks ran=%d err=%v, want 0 and nil", ran, err)
	}
	if err := runDeliveryHooks(false); err != nil || ran != 0 {
		t.Fatalf("hooks were not cleared after a skip (ran=%d)", ran)
	}
	afterDelivery(func() error { ran++; return nil })
	afterDelivery(func() error { return errors.New("store down") })
	if err := runDeliveryHooks(false); err == nil || ran != 1 {
		t.Fatalf("ran=%d err=%v, want 1 and the hook error", ran, err)
	}
	if len(deliveryHooks) != 0 {
		t.Fatalf("hooks left after a run: %d", len(deliveryHooks))
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

// A report that cannot be written must surface the error, so news watch
// returns before marking the articles as seen.
func TestWriteNewsWatchReportsOutputError(t *testing.T) {
	view := newsWatchView{Query: "Autom", New: []serp.Result{{Title: "T", URL: "https://example.com/a"}}}
	if err := writeNewsWatch(failingWriter{}, &rootFlags{asJSON: true}, view); err == nil {
		t.Fatal("writeNewsWatch to a failing writer returned nil")
	}
}
