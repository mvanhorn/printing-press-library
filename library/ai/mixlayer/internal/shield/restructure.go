// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package shield

import (
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type RestructureOptions struct {
	BucketNumerics bool
	CoarsenDates   string
	DropColumns    []string
}

func Restructure(input string, opts RestructureOptions) (string, error) {
	reader := csv.NewReader(strings.NewReader(input))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err == nil && len(records) > 0 && len(records[0]) > 1 {
		return restructureCSV(records, opts)
	}
	return restructureText(input, opts), nil
}

func restructureCSV(records [][]string, opts RestructureOptions) (string, error) {
	header := records[0]
	drops := map[int]bool{}
	dropNames := map[string]bool{}
	for _, d := range opts.DropColumns {
		dropNames[strings.ToLower(strings.TrimSpace(d))] = true
	}
	for i, h := range header {
		if dropNames[strings.ToLower(strings.TrimSpace(h))] {
			drops[i] = true
		}
	}
	var b strings.Builder
	w := csv.NewWriter(&b)
	for _, row := range records {
		out := make([]string, 0, len(row))
		for i, cell := range row {
			if drops[i] {
				continue
			}
			out = append(out, coarsenCell(cell, opts))
		}
		if err := w.Write(out); err != nil {
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func restructureText(input string, opts RestructureOptions) string {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		lines[i] = coarsenCell(line, opts)
	}
	return strings.Join(lines, "\n")
}

func coarsenCell(cell string, opts RestructureOptions) string {
	out := cell
	if opts.CoarsenDates != "" {
		out = coarsenDates(out, opts.CoarsenDates)
	}
	if opts.BucketNumerics && regexp.MustCompile(`^\s*\d+(?:\.\d+)?\s*$`).MatchString(out) {
		out = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`).ReplaceAllStringFunc(out, bucketNumber)
	}
	return out
}

func coarsenDates(s, mode string) string {
	re := regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`)
	return re.ReplaceAllStringFunc(s, func(value string) string {
		t, err := time.Parse("2006-01-02", value)
		if err != nil {
			return value
		}
		switch strings.ToLower(mode) {
		case "quarter", "quarters":
			q := (int(t.Month())-1)/3 + 1
			return fmt.Sprintf("%04d-Q%d", t.Year(), q)
		default:
			return t.Format("2006-01")
		}
	})
}

func bucketNumber(value string) string {
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return value
	}
	switch {
	case n < 10:
		return "0-9"
	case n < 100:
		return "10-99"
	case n < 1000:
		return "100-999"
	case n < 10000:
		return "1000-9999"
	default:
		return "10000+"
	}
}

func LooksCSV(r io.Reader) bool {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	records, err := cr.ReadAll()
	return err == nil && len(records) > 0 && len(records[0]) > 1
}
