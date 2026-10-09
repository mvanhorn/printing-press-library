// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

// safeTextWriter strips C0 and C1 control characters (except newline and
// tab, which the human tables use) from text written to a terminal, so a
// remote locker name or note cannot carry terminal escape sequences.
// Partial UTF-8 sequences are held until the next write.
type safeTextWriter struct {
	w       io.Writer
	pending []byte
}

func newSafeTextWriter(w io.Writer) *safeTextWriter { return &safeTextWriter{w: w} }

func (s *safeTextWriter) Write(p []byte) (int, error) {
	buf := append(s.pending, p...)
	cut := len(buf)
	// Hold back an incomplete rune at the end (at most 3 bytes).
	for i := len(buf) - 1; i >= 0 && i >= len(buf)-3; i-- {
		if utf8.RuneStart(buf[i]) {
			if !utf8.FullRune(buf[i:]) {
				cut = i
			}
			break
		}
	}
	s.pending = append([]byte(nil), buf[cut:]...)
	if _, err := io.WriteString(s.w, stripControl(string(buf[:cut]))); err != nil {
		return 0, err
	}
	return len(p), nil
}

// stripControl removes C0 (except \n and \t), DEL and C1 control characters,
// and invalid UTF-8 bytes.
func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return r
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f):
			return -1
		case r == utf8.RuneError:
			return -1
		case unicode.Is(unicode.Cf, r):
			// Bidi overrides and other format controls can reorder a row.
			return -1
		}
		return r
	}, s)
}
