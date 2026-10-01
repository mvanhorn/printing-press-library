// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package tsadmin

import (
	"fmt"
	"strings"
)

// DiffLine is one line of a line-level diff. Op is "+", "-", or " ".
// OldLine/NewLine are 1-based line numbers (0 when not applicable).
type DiffLine struct {
	Op      string `json:"op"`
	OldLine int    `json:"old_line,omitempty"`
	NewLine int    `json:"new_line,omitempty"`
	Text    string `json:"text"`
}

// maxLCSCells bounds the LCS table; past it the middle section is reported
// as a full replacement rather than allocating a huge table.
const maxLCSCells = 4_000_000

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}

// LineDiff returns the line diff from a to b. Common prefix and suffix lines
// are trimmed before an LCS on the remainder, which keeps policy edits (a
// few changed lines in a long file) cheap.
func LineDiff(a, b string) []DiffLine {
	al, bl := splitLines(a), splitLines(b)
	pre := 0
	for pre < len(al) && pre < len(bl) && al[pre] == bl[pre] {
		pre++
	}
	suf := 0
	for suf < len(al)-pre && suf < len(bl)-pre && al[len(al)-1-suf] == bl[len(bl)-1-suf] {
		suf++
	}
	out := make([]DiffLine, 0, len(al)+len(bl))
	for i := 0; i < pre; i++ {
		out = append(out, DiffLine{Op: " ", OldLine: i + 1, NewLine: i + 1, Text: al[i]})
	}
	am, bm := al[pre:len(al)-suf], bl[pre:len(bl)-suf]
	out = append(out, middleDiff(am, bm, pre, pre)...)
	for k := 0; k < suf; k++ {
		ai, bi := len(al)-suf+k, len(bl)-suf+k
		out = append(out, DiffLine{Op: " ", OldLine: ai + 1, NewLine: bi + 1, Text: al[ai]})
	}
	return out
}

func middleDiff(a, b []string, aOff, bOff int) []DiffLine {
	n, m := len(a), len(b)
	out := make([]DiffLine, 0, n+m)
	if n == 0 || m == 0 || n*m > maxLCSCells {
		for i, l := range a {
			out = append(out, DiffLine{Op: "-", OldLine: aOff + i + 1, Text: l})
		}
		for j, l := range b {
			out = append(out, DiffLine{Op: "+", NewLine: bOff + j + 1, Text: l})
		}
		return out
	}
	lcs := make([][]int32, n+1)
	for i := range lcs {
		lcs[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			out = append(out, DiffLine{Op: " ", OldLine: aOff + i + 1, NewLine: bOff + j + 1, Text: a[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			out = append(out, DiffLine{Op: "-", OldLine: aOff + i + 1, Text: a[i]})
			i++
		default:
			out = append(out, DiffLine{Op: "+", NewLine: bOff + j + 1, Text: b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, DiffLine{Op: "-", OldLine: aOff + i + 1, Text: a[i]})
	}
	for ; j < m; j++ {
		out = append(out, DiffLine{Op: "+", NewLine: bOff + j + 1, Text: b[j]})
	}
	return out
}

// DiffChanges returns only the added and removed lines.
func DiffChanges(lines []DiffLine) (added, removed []DiffLine) {
	added, removed = make([]DiffLine, 0), make([]DiffLine, 0)
	for _, l := range lines {
		switch l.Op {
		case "+":
			added = append(added, l)
		case "-":
			removed = append(removed, l)
		}
	}
	return added, removed
}

// UnifiedDiff renders changed lines with `context` lines of surrounding
// context, in a compact unified style for terminals.
func UnifiedDiff(lines []DiffLine, context int) string {
	keep := make([]bool, len(lines))
	for i, l := range lines {
		if l.Op == " " {
			continue
		}
		for k := i - context; k <= i+context; k++ {
			if k >= 0 && k < len(lines) {
				keep[k] = true
			}
		}
	}
	var sb strings.Builder
	prevKept := true
	for i, l := range lines {
		if !keep[i] {
			prevKept = false
			continue
		}
		if !prevKept {
			sb.WriteString("...\n")
		}
		prevKept = true
		num := l.NewLine
		if l.Op == "-" {
			num = l.OldLine
		}
		fmt.Fprintf(&sb, "%s %4d | %s\n", l.Op, num, l.Text)
	}
	return sb.String()
}
