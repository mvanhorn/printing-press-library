// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// HAND-WRITTEN — small file utility used by transcendence.go.

package cli

import (
	"io"
	"os"
)

// openCSVTarget opens path for writing, returning a closer that flushes the
// underlying file. Stdout is treated as non-closable.
func openCSVTarget(path string) (io.WriteCloser, error) {
	if path == "-" || path == "" {
		return nopWriteCloser{os.Stdout}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
