// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
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
			token, err := readToken(strings.NewReader(tc.input))
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
