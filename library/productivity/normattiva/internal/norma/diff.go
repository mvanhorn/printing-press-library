// Copyright 2026 Luigi Pastore and contributors. Licensed under Apache-2.0. See LICENSE.

package norma

import (
	"strings"
)

// DiffOp is one run of a word-level diff.
type DiffOp struct {
	Kind string `json:"kind"` // "equal", "delete", "insert"
	Text string `json:"text"`
}

// maxDiffCells bounds the LCS table (about 32 MB of ints).
const maxDiffCells = 4_000_000

// tokenize splits text into words and whitespace runs so the diff can be
// re-joined without losing line breaks.
func tokenize(s string) []string {
	var toks []string
	var cur strings.Builder
	inSpace := false
	for _, r := range s {
		sp := r == ' ' || r == '\n' || r == '\t'
		if cur.Len() > 0 && sp != inSpace {
			toks = append(toks, cur.String())
			cur.Reset()
		}
		inSpace = sp
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		toks = append(toks, cur.String())
	}
	return toks
}

// WordDiff computes a minimal word-level diff (LCS) between a and b.
func WordDiff(a, b string) []DiffOp {
	x, y := tokenize(a), tokenize(b)
	n, m := len(x), len(y)
	// Trim common prefix/suffix to keep the LCS table small.
	pre := 0
	for pre < n && pre < m && x[pre] == y[pre] {
		pre++
	}
	suf := 0
	for suf < n-pre && suf < m-pre && x[n-1-suf] == y[m-1-suf] {
		suf++
	}
	xs, ys := x[pre:n-suf], y[pre:m-suf]
	if len(xs)*len(ys) > maxDiffCells {
		// Too large for a word-level table: report the changed middle as one
		// replacement instead of allocating gigabytes.
		var ops []DiffOp
		for _, op := range []DiffOp{{"equal", strings.Join(x[:pre], "")}, {"delete", strings.Join(xs, "")}, {"insert", strings.Join(ys, "")}, {"equal", strings.Join(x[n-suf:], "")}} {
			if op.Text != "" {
				ops = append(ops, op)
			}
		}
		return ops
	}
	lcs := make([][]int, len(xs)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(ys)+1)
	}
	for i := len(xs) - 1; i >= 0; i-- {
		for j := len(ys) - 1; j >= 0; j-- {
			if xs[i] == ys[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	var ops []DiffOp
	add := func(kind, text string) {
		if text == "" {
			return
		}
		if k := len(ops); k > 0 && ops[k-1].Kind == kind {
			ops[k-1].Text += text
			return
		}
		ops = append(ops, DiffOp{Kind: kind, Text: text})
	}
	add("equal", strings.Join(x[:pre], ""))
	i, j := 0, 0
	for i < len(xs) && j < len(ys) {
		switch {
		case xs[i] == ys[j]:
			add("equal", xs[i])
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			add("delete", xs[i])
			i++
		default:
			add("insert", ys[j])
			j++
		}
	}
	for ; i < len(xs); i++ {
		add("delete", xs[i])
	}
	for ; j < len(ys); j++ {
		add("insert", ys[j])
	}
	add("equal", strings.Join(x[n-suf:], ""))
	return groupHunks(ops)
}

// groupHunks turns interleaved runs like [-a-]{+b+} [-c-]{+d+} into one
// readable [-a c-]{+b d+} pair: whitespace-only equal runs between changes
// are folded into both sides of the hunk.
func groupHunks(ops []DiffOp) []DiffOp {
	var out []DiffOp
	var del, ins strings.Builder
	pending := ""
	flush := func() {
		if del.Len() > 0 {
			out = append(out, DiffOp{Kind: "delete", Text: del.String()})
		}
		if ins.Len() > 0 {
			out = append(out, DiffOp{Kind: "insert", Text: ins.String()})
		}
		del.Reset()
		ins.Reset()
	}
	inHunk := func() bool { return del.Len() > 0 || ins.Len() > 0 }
	for i, op := range ops {
		switch op.Kind {
		case "delete":
			del.WriteString(pending + op.Text)
			if pending != "" && ins.Len() > 0 {
				ins.WriteString(pending)
			}
			pending = ""
		case "insert":
			ins.WriteString(pending + op.Text)
			if pending != "" && del.Len() > 0 {
				del.WriteString(pending)
			}
			pending = ""
		default:
			nextChanges := i+1 < len(ops) && ops[i+1].Kind != "equal"
			if inHunk() && nextChanges && strings.TrimSpace(op.Text) == "" {
				pending = op.Text
				continue
			}
			flush()
			out = append(out, DiffOp{Kind: "equal", Text: pending + op.Text})
			pending = ""
		}
	}
	flush()
	if pending != "" {
		out = append(out, DiffOp{Kind: "equal", Text: pending})
	}
	return out
}

// DiffStats counts inserted and deleted words.
func DiffStats(ops []DiffOp) (inserted, deleted int) {
	for _, op := range ops {
		w := len(strings.Fields(op.Text))
		switch op.Kind {
		case "insert":
			inserted += w
		case "delete":
			deleted += w
		}
	}
	return
}

// RenderDiff renders ops inline: deletions as [-...-], insertions as {+...+}.
// When color is true, ANSI red/green is used instead of the markers.
func RenderDiff(ops []DiffOp, color bool) string {
	var b strings.Builder
	for _, op := range ops {
		switch op.Kind {
		case "equal":
			b.WriteString(op.Text)
		case "delete":
			if color {
				b.WriteString("\x1b[31;9m" + op.Text + "\x1b[0m")
			} else {
				b.WriteString("[-" + op.Text + "-]")
			}
		case "insert":
			if color {
				b.WriteString("\x1b[32;1m" + op.Text + "\x1b[0m")
			} else {
				b.WriteString("{+" + op.Text + "+}")
			}
		}
	}
	return b.String()
}
