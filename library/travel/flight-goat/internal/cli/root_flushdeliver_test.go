// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(review-2026-08-01): tests for flushDeliver's three-way branching —
// success delivery, failure-with-buffered-partial delivery, and the
// delivery-error path that must not clobber a command error.

package cli

import (
	"bytes"
	"errors"
	"testing"
)

func TestFlushDeliver(t *testing.T) {
	orig := deliverFn
	defer func() { deliverFn = orig }()

	cmdErr := errors.New("batch stopped: rate limited")
	sinkErr := errors.New("webhook 503")

	t.Run("no sink buffer passes error through untouched", func(t *testing.T) {
		called := false
		deliverFn = func(DeliverSink, []byte, bool) error { called = true; return nil }
		if got := flushDeliver(&rootFlags{}, cmdErr); got != cmdErr {
			t.Fatalf("got %v, want cmdErr", got)
		}
		if called {
			t.Fatal("deliver must not run without a sink buffer")
		}
	})

	t.Run("failure with empty buffer skips delivery", func(t *testing.T) {
		called := false
		deliverFn = func(DeliverSink, []byte, bool) error { called = true; return nil }
		flags := &rootFlags{deliverBuf: &bytes.Buffer{}}
		if got := flushDeliver(flags, cmdErr); got != cmdErr {
			t.Fatalf("got %v, want cmdErr", got)
		}
		if called {
			t.Fatal("nothing was buffered; deliver must not run")
		}
	})

	t.Run("failure with buffered partial output delivers it", func(t *testing.T) {
		var delivered []byte
		deliverFn = func(_ DeliverSink, body []byte, _ bool) error { delivered = body; return nil }
		flags := &rootFlags{deliverBuf: bytes.NewBufferString(`{"partial":true}`)}
		if got := flushDeliver(flags, cmdErr); got != cmdErr {
			t.Fatalf("got %v, want cmdErr preserved", got)
		}
		if string(delivered) != `{"partial":true}` {
			t.Fatalf("delivered = %q, want the buffered envelope", delivered)
		}
	})

	t.Run("delivery failure surfaces only on a successful command", func(t *testing.T) {
		deliverFn = func(DeliverSink, []byte, bool) error { return sinkErr }
		flags := &rootFlags{deliverBuf: bytes.NewBufferString("x")}
		if got := flushDeliver(flags, nil); got != sinkErr {
			t.Fatalf("got %v, want sinkErr when the command succeeded", got)
		}
		flags = &rootFlags{deliverBuf: bytes.NewBufferString("x")}
		if got := flushDeliver(flags, cmdErr); got != cmdErr {
			t.Fatalf("got %v, want cmdErr to win over the delivery failure", got)
		}
	})

	t.Run("success with buffer delivers and stays nil", func(t *testing.T) {
		deliverFn = func(DeliverSink, []byte, bool) error { return nil }
		flags := &rootFlags{deliverBuf: bytes.NewBufferString("x")}
		if got := flushDeliver(flags, nil); got != nil {
			t.Fatalf("got %v, want nil", got)
		}
	})
}
