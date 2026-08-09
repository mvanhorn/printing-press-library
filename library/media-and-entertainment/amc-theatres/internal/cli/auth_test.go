// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"io"
	"strings"
	"testing"
	"time"
)

func TestReadToken(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "newline terminated", input: "secret-token\n", want: "secret-token"},
		{name: "EOF terminated", input: "secret-token", want: "secret-token"},
		{name: "CRLF terminated", input: "secret-token\r\n", want: "secret-token"},
		{name: "empty", input: "\n", wantErr: true},
		{name: "whitespace only", input: "   \n", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token, err := readToken(strings.NewReader(tc.input), &strings.Builder{}, false)
			if (err != nil) != tc.wantErr {
				t.Fatalf("readToken() error = %v, wantErr %v", err, tc.wantErr)
			}
			if token != tc.want {
				t.Fatalf("readToken() = %q, want %q", token, tc.want)
			}
		})
	}
}

func TestSetTokenRejectsCredentialArgument(t *testing.T) {
	t.Parallel()

	cmd := newAuthSetTokenCmd(&rootFlags{})
	if err := cmd.Args(cmd, []string{"secret-token"}); err == nil {
		t.Fatal("set-token accepted a credential in argv")
	}
}

func TestSetTokenNonInteractiveDoesNotReadOpenPipe(t *testing.T) {
	t.Parallel()

	reader, writer := io.Pipe()
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})

	cmd := newAuthSetTokenCmd(&rootFlags{agent: true})
	cmd.SetIn(reader)
	done := make(chan error, 1)
	go func() {
		done <- cmd.RunE(cmd, nil)
	}()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "does not accept --no-input or --agent") {
			t.Fatalf("set-token error = %v, want non-interactive rejection", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("set-token blocked while reading an open empty stdin pipe")
	}
}

func TestValidateTokenInputMode(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name           string
		isTerminal     bool
		nonInteractive bool
		wantErr        bool
	}{
		{name: "interactive terminal", isTerminal: true},
		{name: "piped interactive", isTerminal: false},
		{name: "piped agent rejected", nonInteractive: true, wantErr: true},
		{name: "agent terminal rejected", isTerminal: true, nonInteractive: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTokenInputMode(tc.isTerminal, tc.nonInteractive)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateTokenInputMode() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
