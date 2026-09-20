package nepraparse

import (
	"strings"
	"unicode"
)

// nbsp is U+00A0. It is the single most dangerous character in these files:
// a cell whose whole content is &nbsp; (or a literal 0xA0 byte) looks
// non-empty to a naive reader and then fails numeric parsing in a way that
// invites a fallback to zero.
//
// The hazard is in the DECODE, not in the trim. Once the byte has been
// correctly decoded from windows-1252 to U+00A0, strings.TrimSpace DOES strip
// it, because unicode.IsSpace covers U+00A0. What does not survive a naive
// read is the byte itself: 0xA0 is not valid UTF-8, so a file read without
// decoding yields U+FFFD per byte, unicode.IsSpace(U+FFFD) is false, and the
// cell stays non-empty through every trim in the standard library. Measured:
// strings.TrimSpace(string([]byte{0xA0})) == "\xa0", not "".
//
// So this package decodes first (see charset.go) and then collapses
// explicitly here — explicitly because collapseText must also turn NBSP into
// a SEPARATOR in the middle of a string, which trimming the ends cannot do:
// ">Export\n  to K.Electric<" and the NBSP-padded company names both depend
// on internal collapsing, not just trimming.
const nbsp = ' '

// collapseText normalises the text of one already-tag-free, already-unescaped
// cell: every Unicode space (NBSP included) becomes an ASCII space, runs of
// whitespace collapse to one space, and the result is trimmed.
//
// Collapsing happens AFTER markup removal, never with a line-based regex,
// because the real cell text wraps across source lines:
// ">Name of\n  Companies<", ">Natural Gas/\n  Furnace Oil<",
// ">Export\n  to K.Electric<".
func collapseText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range s {
		if r == nbsp || unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}
