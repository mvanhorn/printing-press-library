// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package shield

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

type Tranche struct {
	Index    int    `json:"index"`
	FromLine int    `json:"from_line"`
	ToLine   int    `json:"to_line"`
	Text     string `json:"-"`
}

type TrancheManifest struct {
	Tranches []TrancheSummary `json:"tranches"`
}

type TrancheSummary struct {
	Index       int            `json:"index"`
	FromLine    int            `json:"from_line"`
	ToLine      int            `json:"to_line"`
	EntityCount int            `json:"entity_count"`
	EntityKinds map[string]int `json:"entity_kinds"`
}

func SplitRecords(input string, maxBytes int) ([]Tranche, error) {
	if maxBytes <= 0 {
		maxBytes = 64 * 1024
	}
	if tranches, ok := splitCSV(input, maxBytes); ok {
		return tranches, nil
	}
	return splitLines(input, maxBytes)
}

func splitCSV(input string, maxBytes int) ([]Tranche, bool) {
	reader := csv.NewReader(strings.NewReader(input))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil || len(records) == 0 || len(records[0]) < 2 {
		return nil, false
	}
	header := records[0]
	var out []Tranche
	var batch [][]string
	from := 2
	flush := func(to int) {
		if len(batch) == 0 {
			return
		}
		var b bytes.Buffer
		w := csv.NewWriter(&b)
		_ = w.Write(header)
		_ = w.WriteAll(batch)
		w.Flush()
		out = append(out, Tranche{Index: len(out) + 1, FromLine: from, ToLine: to, Text: b.String()})
		batch = nil
		from = to + 1
	}
	for i, row := range records[1:] {
		batch = append(batch, row)
		var b bytes.Buffer
		w := csv.NewWriter(&b)
		_ = w.Write(header)
		_ = w.WriteAll(batch)
		w.Flush()
		if b.Len() > maxBytes && len(batch) > 1 {
			last := batch[len(batch)-1]
			batch = batch[:len(batch)-1]
			flush(i + 1)
			batch = [][]string{last}
			from = i + 2
		}
	}
	flush(len(records))
	return out, true
}

func splitLines(input string, maxBytes int) ([]Tranche, error) {
	scanner := bufio.NewScanner(strings.NewReader(input))
	scanner.Buffer(make([]byte, 1024), max(maxBytes*4, len(input)+1))
	var out []Tranche
	var b strings.Builder
	from := 1
	line := 0
	for scanner.Scan() {
		line++
		next := scanner.Text() + "\n"
		if b.Len()+len(next) > maxBytes && b.Len() > 0 {
			out = append(out, Tranche{Index: len(out) + 1, FromLine: from, ToLine: line - 1, Text: b.String()})
			b.Reset()
			from = line
		}
		if len(next) > maxBytes {
			for _, part := range splitOversized(next, maxBytes) {
				if b.Len() > 0 {
					out = append(out, Tranche{Index: len(out) + 1, FromLine: from, ToLine: line - 1, Text: b.String()})
					b.Reset()
				}
				out = append(out, Tranche{Index: len(out) + 1, FromLine: line, ToLine: line, Text: part})
			}
			from = line + 1
			continue
		}
		b.WriteString(next)
	}
	if b.Len() > 0 {
		out = append(out, Tranche{Index: len(out) + 1, FromLine: from, ToLine: line, Text: b.String()})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("split lines: %w", err)
	}
	if len(out) == 0 {
		out = []Tranche{{Index: 1, FromLine: 1, ToLine: 1, Text: input}}
	}
	return out, nil
}

func splitOversized(s string, maxBytes int) []string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return []string{s}
	}
	var out []string
	for start := 0; start < len(s); {
		end := utf8SplitEnd(s, start, maxBytes)
		if end < len(s) {
			if dot := strings.LastIndexAny(s[start:end], ".!?;"); dot > maxBytes/2 {
				end = start + dot + 1
			}
		}
		out = append(out, s[start:end])
		if end == len(s) {
			break
		}
		start = end
	}
	return out
}

func utf8SplitEnd(s string, start, maxBytes int) int {
	end := min(start+maxBytes, len(s))
	for end > start && end < len(s) && !utf8.RuneStart(s[end]) {
		end--
	}
	if end == start {
		_, size := utf8.DecodeRuneInString(s[start:])
		if size == 0 {
			return len(s)
		}
		return min(start+size, len(s))
	}
	return end
}

func ManifestJSON(m TrancheManifest) json.RawMessage {
	raw, _ := json.MarshalIndent(m, "", "  ")
	return raw
}
