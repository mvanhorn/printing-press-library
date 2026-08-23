// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"io"
	"testing"
)

func TestDeliverOutputWriterFileOmitsStdoutUnlessAlsoStdout(t *testing.T) {
	var stdout bytes.Buffer
	buf := &bytes.Buffer{}
	fileSink := DeliverSink{Scheme: "file", Target: "/tmp/out.json"}

	w := deliverOutputWriter(fileSink, false, &stdout, buf)
	if _, err := io.WriteString(w, "captured"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("file: wrote stdout %q", stdout.String())
	}
	if buf.String() != "captured" {
		t.Fatalf("file: buffer = %q", buf.String())
	}

	stdout.Reset()
	buf.Reset()
	w = deliverOutputWriter(fileSink, true, &stdout, buf)
	if _, err := io.WriteString(w, "both"); err != nil {
		t.Fatalf("write also-stdout: %v", err)
	}
	if stdout.String() != "both" || buf.String() != "both" {
		t.Fatalf("also-stdout stdout=%q buf=%q", stdout.String(), buf.String())
	}
}

func TestDeliverOutputWriterWebhookKeepsStdout(t *testing.T) {
	var stdout bytes.Buffer
	buf := &bytes.Buffer{}
	w := deliverOutputWriter(DeliverSink{Scheme: "webhook", Target: "https://example.com"}, false, &stdout, buf)
	if _, err := io.WriteString(w, "hook"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if stdout.String() != "hook" || buf.String() != "hook" {
		t.Fatalf("webhook stdout=%q buf=%q", stdout.String(), buf.String())
	}
}
