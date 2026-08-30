// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.

// Package wer computes word error rate between a reference phrase and a
// transcript. `voice verify` uses it to decide whether a cloned voice is
// intelligible enough to put in front of users.
package wer

import (
	"strings"
	"unicode"
)

// Verdict labels a word error rate.
const (
	VerdictPass = "pass"
	VerdictWarn = "warn"
	VerdictFail = "fail"
)

// PassThreshold and WarnThreshold bound the verdict bands.
const (
	PassThreshold = 0.15
	WarnThreshold = 0.30
)

// Normalize lowercases the text, drops punctuation, and splits on whitespace.
// Casing and punctuation are artifacts of the transcriber, not of the voice,
// so scoring them would report failures the listener would never hear.
func Normalize(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '\''
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.Trim(f, "'")
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// Rate returns the word error rate of hypothesis against reference:
// (substitutions + insertions + deletions) / reference word count.
//
// An empty reference returns 0 when the hypothesis is also empty and 1
// otherwise, so an empty transcript can never look like a perfect score.
func Rate(reference, hypothesis string) float64 {
	ref := Normalize(reference)
	hyp := Normalize(hypothesis)
	if len(ref) == 0 {
		if len(hyp) == 0 {
			return 0
		}
		return 1
	}
	return float64(editDistance(ref, hyp)) / float64(len(ref))
}

// Verdict maps a rate to its band.
func Verdict(rate float64) string {
	switch {
	case rate < PassThreshold:
		return VerdictPass
	case rate < WarnThreshold:
		return VerdictWarn
	default:
		return VerdictFail
	}
}

// editDistance is Levenshtein distance over word slices, computed with two
// rows so a long transcript does not allocate a full matrix.
func editDistance(a, b []string) int {
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
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
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
