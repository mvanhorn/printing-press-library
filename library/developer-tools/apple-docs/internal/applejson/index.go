// Copyright 2026 joseph-alvin-castillo. Licensed under Apache-2.0. See LICENSE.

package applejson

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FrameworkIndex is the parsed shape of /tutorials/data/index/<framework>.json.
type FrameworkIndex struct {
	SchemaVersion json.RawMessage         `json:"schemaVersion"`
	Languages     map[string][]*IndexNode `json:"interfaceLanguages"`
}

// IndexNode is one entry in the index tree. Leaves carry a Path; group
// markers carry only Title and Type=groupMarker.
type IndexNode struct {
	Title      string       `json:"title"`
	Path       string       `json:"path,omitempty"`
	Type       string       `json:"type"`
	Deprecated bool         `json:"deprecated,omitempty"`
	External   bool         `json:"external,omitempty"`
	Beta       bool         `json:"beta,omitempty"`
	Children   []*IndexNode `json:"children,omitempty"`
}

// ParseIndex decodes the raw index JSON.
func ParseIndex(raw json.RawMessage) (*FrameworkIndex, error) {
	var idx FrameworkIndex
	if err := json.Unmarshal(raw, &idx); err != nil {
		return nil, fmt.Errorf("parsing framework index: %w", err)
	}
	return &idx, nil
}

// WalkSwift walks the swift-language tree depth-first, calling fn on
// every non-groupMarker node. Group markers are skipped (they're
// section headings, not symbols).
func (f *FrameworkIndex) WalkSwift(fn func(node *IndexNode)) {
	if f.Languages == nil {
		return
	}
	roots := f.Languages["swift"]
	if roots == nil {
		// Fallback to whatever language is present.
		for _, v := range f.Languages {
			roots = v
			break
		}
	}
	for _, root := range roots {
		walk(root, fn)
	}
}

func walk(n *IndexNode, fn func(*IndexNode)) {
	if n == nil {
		return
	}
	if n.Type != "groupMarker" {
		fn(n)
	}
	for _, child := range n.Children {
		walk(child, fn)
	}
}

// PathStem returns the last segment of a doc path, stripping
// parameter labels: "/documentation/swiftui/view/onappear(perform:)"
// becomes "onappear". Used for likely-rename matching.
func PathStem(path string) string {
	path = strings.TrimSuffix(path, "/")
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return path
	}
	stem := path[idx+1:]
	if paren := strings.Index(stem, "("); paren > 0 {
		stem = stem[:paren]
	}
	return strings.ToLower(stem)
}

// LevenshteinClose reports whether two strings are within edit distance
// `maxDist`. Used to flag likely renames.
//
// Note: kept exported because the original-author tests live in the
// applejson package. Callers outside the package use [Levenshtein]
// directly with an explicit threshold.
func LevenshteinClose(a, b string, maxDist int) bool {
	if a == b {
		return true
	}
	if abs(len(a)-len(b)) > maxDist {
		return false
	}
	dist := Levenshtein(a, b)
	return dist <= maxDist
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Levenshtein returns the byte-level edit distance between two strings.
// Shared with cli/snapshot_diff.go's rename pairing — kept in one place to
// avoid two implementations drifting.
func Levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
