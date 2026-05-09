// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package extract

import (
	"regexp"
	"strconv"
	"strings"
)

// Citation is a single statute reference parsed from a judgment body.
type Citation struct {
	Statute string `json:"statute"` // statute name, e.g. "毒品危害防制條例"
	Article int    `json:"article"` // article number; 0 when only the statute is named
}

// jidPattern matches in-body references to other judgments (the JID format).
var jidPattern = regexp.MustCompile(`\b[A-Z]{3,4}[A-Z]?,\d+,[\p{Han}\w]+,\d+,\d{8},\d+\b`)

// statuteArticlePattern matches Taiwan statute names ending in 法 / 條例 / 規則 /
// 準則 / 辦法 / 要點 followed by 第N條. The leading Han run is bounded to 2-10
// characters so it does not eat preceding verbs like 違反, 處, 應. Examples:
//   - 刑法第50條
//   - 毒品危害防制條例第17條
//   - 民法第71條至第98條
//   - 刑事訴訟法 第412條
var statuteArticlePattern = regexp.MustCompile(`([\p{Han}]{1,10}(?:法|條例|規則|準則|辦法|要點))\s*第\s*(\d+)\s*條`)

// statuteOnlyPattern matches bare statute references without an article number,
// e.g. "違反毒品危害防制條例". Less precise; only used as a fallback.
var statuteOnlyPattern = regexp.MustCompile(`([\p{Han}]{1,10}(?:法|條例|規則|準則|辦法|要點))(?:[^第]|$)`)

// stripLeadingVerbs removes common verbs that the statute regex may capture
// when they immediately precede a real statute name without a separator.
var leadingVerb = regexp.MustCompile(`^(?:被告違反|違反|按|依|依據|處|應依)`)

func normaliseStatute(s string) string {
	for {
		next := leadingVerb.ReplaceAllString(s, "")
		if next == s {
			return s
		}
		s = next
	}
}

// ExtractCitations returns the unique Citation list found in `body`. Statute
// names are normalised by trimming whitespace; article numbers are 0 for
// bare statute mentions.
func ExtractCitations(body string) []Citation {
	if body == "" {
		return nil
	}
	seen := make(map[string]Citation)
	for _, m := range statuteArticlePattern.FindAllStringSubmatch(body, -1) {
		statute := normaliseStatute(strings.TrimSpace(m[1]))
		if statute == "" {
			continue
		}
		article, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		key := statute + "#" + m[2]
		if _, ok := seen[key]; !ok {
			seen[key] = Citation{Statute: statute, Article: article}
		}
	}
	// Bare-statute fallback intentionally omitted: too many false positives
	// from common Chinese phrases ending in 法 (違法, 依法, 於法, 最高法, ...).
	// Statutes worth indexing are referenced with an explicit article number;
	// callers needing the bare-statute set can grep over judgment bodies via
	// `judgments get` and the FTS5 index.
	_ = statuteOnlyPattern // kept for documentation / future stricter pass
	out := make([]Citation, 0, len(seen))
	for _, c := range seen {
		out = append(out, c)
	}
	return out
}

// ExtractJIDReferences returns unique JIDs cited inside the judgment body.
// Useful for building the cited-by reverse index.
func ExtractJIDReferences(body string) []string {
	if body == "" {
		return nil
	}
	seen := make(map[string]struct{})
	for _, m := range jidPattern.FindAllString(body, -1) {
		seen[m] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for j := range seen {
		out = append(out, j)
	}
	return out
}
