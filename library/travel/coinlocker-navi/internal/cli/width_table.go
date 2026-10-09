// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

// widthTable is a human table that pads cells by terminal display width.
// text/tabwriter counts runes, so rows with Japanese names (two columns per
// character) push the later columns out of line.
type widthTable struct {
	rows [][]string
}

// add appends one row. Tabs and newlines in a cell become spaces, and
// control characters are removed before the cell is measured.
func (t *widthTable) add(cells ...string) {
	row := make([]string, len(cells))
	for i, c := range cells {
		c = strings.NewReplacer("\t", " ", "\r\n", " ", "\n", " ").Replace(c)
		row[i] = stripControl(c)
	}
	t.rows = append(t.rows, row)
}

// write prints the rows with two spaces between columns. The last column is
// not padded.
func (t *widthTable) write(w io.Writer) error {
	var widths []int
	for _, r := range t.rows {
		for i, c := range r {
			if i >= len(widths) {
				widths = append(widths, 0)
			}
			if n := displayWidth(c); n > widths[i] {
				widths[i] = n
			}
		}
	}
	for _, r := range t.rows {
		var b strings.Builder
		for i, c := range r {
			b.WriteString(c)
			if i < len(r)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-displayWidth(c)+2))
			}
		}
		if _, err := fmt.Fprintln(w, b.String()); err != nil {
			return err
		}
	}
	return nil
}

// displayWidth is the number of terminal columns s takes: two for East Asian
// wide and fullwidth characters, zero for combining marks, one otherwise.
func displayWidth(s string) int {
	n := 0
	for _, r := range s {
		n += runeWidth(r)
	}
	return n
}

func runeWidth(r rune) int {
	switch {
	case unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || r == 0x200B:
		return 0
	case r >= 0x1100 && r <= 0x115F, // Hangul Jamo
		r >= 0x2E80 && r <= 0x303E,   // CJK radicals, punctuation
		r >= 0x3041 && r <= 0x33FF,   // kana, CJK symbols
		r >= 0x3400 && r <= 0x4DBF,   // CJK extension A
		r >= 0x4E00 && r <= 0x9FFF,   // CJK unified ideographs
		r >= 0xA000 && r <= 0xA4CF,   // Yi
		r >= 0xAC00 && r <= 0xD7A3,   // Hangul syllables
		r >= 0xF900 && r <= 0xFAFF,   // CJK compatibility ideographs
		r >= 0xFE30 && r <= 0xFE4F,   // CJK compatibility forms
		r >= 0xFF00 && r <= 0xFF60,   // fullwidth forms
		r >= 0xFFE0 && r <= 0xFFE6,   // fullwidth signs
		r >= 0x1F300 && r <= 0x1F64F, // emoji
		r >= 0x20000 && r <= 0x3FFFD: // CJK extensions B and later
		return 2
	}
	return 1
}

// truncateWidth cuts s to at most max display columns, ending with "..."
// when it cuts.
func truncateWidth(s string, max int) string {
	if displayWidth(s) <= max {
		return s
	}
	limit := max - 3
	if limit < 0 {
		limit = 0
	}
	var b strings.Builder
	n := 0
	for _, r := range s {
		rw := runeWidth(r)
		if n+rw > limit {
			break
		}
		b.WriteRune(r)
		n += rw
	}
	if max < 3 {
		return b.String()
	}
	return b.String() + "..."
}
