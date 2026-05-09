// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package extract

import (
	"regexp"
	"strconv"
	"strings"
)

// SentenceKind classifies the criminal-sentence form parsed from a 主文 block.
type SentenceKind string

const (
	SentencePrison     SentenceKind = "imprisonment" // 有期徒刑 / 無期徒刑
	SentenceDetention  SentenceKind = "detention"    // 拘役
	SentenceFine       SentenceKind = "fine"         // 罰金
	SentenceLifePrison SentenceKind = "life_prison"  // 無期徒刑
	SentenceProbation  SentenceKind = "probation"    // 緩刑
	SentenceLabor      SentenceKind = "social_labor" // 社會勞動 / 易服社會勞動
	SentenceUnknown    SentenceKind = "unknown"
)

// Sentence is a single penalty parsed from a judgment's 主文 (verdict).
type Sentence struct {
	Kind         SentenceKind
	PrisonMonths int    // total months for SentencePrison
	FineNTD      int    // 罰金 amount in NTD
	Probation    int    // 緩刑 years
	Raw          string // the literal substring matched (for audit)
}

// chineseDigit maps simple Chinese numerals + common compounds to integers.
// Sentencing language typically uses ASCII digits in modern judgments, but
// older ones occasionally use 一二三四五六七八九十.
var chineseDigit = map[rune]int{
	'零': 0, '〇': 0, '一': 1, '二': 2, '三': 3, '四': 4,
	'五': 5, '六': 6, '七': 7, '八': 8, '九': 9, '十': 10,
}

// parseChineseInt converts a short Chinese-numeral string (e.g. "十一" = 11,
// "二十" = 20) to int. Returns 0 on failure.
func parseChineseInt(s string) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	r := []rune(s)
	switch len(r) {
	case 1:
		if v, ok := chineseDigit[r[0]]; ok {
			return v
		}
	case 2:
		// "十X" = 10+X, "X十" = X*10
		if r[0] == '十' {
			if v, ok := chineseDigit[r[1]]; ok {
				return 10 + v
			}
		}
		if r[1] == '十' {
			if v, ok := chineseDigit[r[0]]; ok {
				return v * 10
			}
		}
	case 3:
		// "X十Y" = X*10+Y
		if r[1] == '十' {
			a, okA := chineseDigit[r[0]]
			b, okB := chineseDigit[r[2]]
			if okA && okB {
				return a*10 + b
			}
		}
	}
	return 0
}

var (
	prisonYearMonth = regexp.MustCompile(`有期徒刑\s*([\d一二三四五六七八九十]+)\s*年(?:\s*([\d一二三四五六七八九十]+)\s*[月個月])?`)
	prisonMonthOnly = regexp.MustCompile(`有期徒刑\s*([\d一二三四五六七八九十]+)\s*個?月`)
	lifePrison      = regexp.MustCompile(`無期徒刑`)
	detention       = regexp.MustCompile(`拘役\s*([\d一二三四五六七八九十]+)\s*日`)
	fine            = regexp.MustCompile(`(?:罰金|併科罰金)\s*(?:新台幣|新臺幣)?\s*([\d,]+)\s*元`)
	probation       = regexp.MustCompile(`緩刑\s*([\d一二三四五六七八九十]+)\s*年`)
	socialLabor     = regexp.MustCompile(`易服\s*社會勞動`)
)

// ExtractSentences scans a 主文 (verdict) block and returns every sentence
// pattern detected. Each match is independent — a single judgment may contain
// multiple sentences (e.g. 有期徒刑 + 罰金 + 緩刑).
func ExtractSentences(verdict string) []Sentence {
	if verdict == "" {
		return nil
	}
	var out []Sentence
	if lifePrison.FindString(verdict) != "" {
		out = append(out, Sentence{Kind: SentenceLifePrison, Raw: "無期徒刑"})
	}
	for _, m := range prisonYearMonth.FindAllStringSubmatch(verdict, -1) {
		years := parseChineseInt(m[1])
		months := 0
		if len(m) > 2 && m[2] != "" {
			months = parseChineseInt(m[2])
		}
		out = append(out, Sentence{
			Kind:         SentencePrison,
			PrisonMonths: years*12 + months,
			Raw:          m[0],
		})
	}
	// Month-only form (e.g. "有期徒刑6個月") not already matched by year+month.
	for _, m := range prisonMonthOnly.FindAllStringSubmatch(verdict, -1) {
		// Skip ranges that include a year (already handled by prisonYearMonth).
		if strings.Contains(m[0], "年") {
			continue
		}
		months := parseChineseInt(m[1])
		out = append(out, Sentence{
			Kind:         SentencePrison,
			PrisonMonths: months,
			Raw:          m[0],
		})
	}
	for _, m := range detention.FindAllStringSubmatch(verdict, -1) {
		days := parseChineseInt(m[1])
		// store detention days as fractional months (rounded down)
		out = append(out, Sentence{
			Kind:         SentenceDetention,
			PrisonMonths: days / 30,
			Raw:          m[0],
		})
	}
	for _, m := range fine.FindAllStringSubmatch(verdict, -1) {
		amount := strings.ReplaceAll(m[1], ",", "")
		n, _ := strconv.Atoi(amount)
		out = append(out, Sentence{
			Kind:    SentenceFine,
			FineNTD: n,
			Raw:     m[0],
		})
	}
	for _, m := range probation.FindAllStringSubmatch(verdict, -1) {
		years := parseChineseInt(m[1])
		out = append(out, Sentence{
			Kind:      SentenceProbation,
			Probation: years,
			Raw:       m[0],
		})
	}
	if socialLabor.FindString(verdict) != "" {
		out = append(out, Sentence{Kind: SentenceLabor, Raw: "易服社會勞動"})
	}
	return out
}

// PrimarySentence returns the most-significant sentence from the list:
// life > prison > detention > fine > probation > labor > unknown. Used for
// histogram bucketing where one number per judgment is wanted.
func PrimarySentence(ss []Sentence) Sentence {
	priority := map[SentenceKind]int{
		SentenceLifePrison: 6,
		SentencePrison:     5,
		SentenceDetention:  4,
		SentenceFine:       3,
		SentenceProbation:  2,
		SentenceLabor:      1,
	}
	var best Sentence
	bestRank := -1
	for _, s := range ss {
		r := priority[s.Kind]
		if r > bestRank {
			best = s
			bestRank = r
		}
	}
	if bestRank < 0 {
		return Sentence{Kind: SentenceUnknown}
	}
	return best
}
