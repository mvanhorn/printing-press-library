// Package format renders query results as compact tables, JSON, or CSV.
//
// Each row is a slice of Cell values that carry both a display string (with
// $/K/M/B/% formatting) and a raw numeric value. Table mode uses the display
// string; JSON and CSV emit the raw number so downstream tools can parse them
// without scientific notation or currency symbols.
package format

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Mode int

const (
	ModeTable Mode = iota
	ModeJSON
	ModeCSV
)

func ParseMode(jsonF, csvF bool) Mode {
	switch {
	case jsonF:
		return ModeJSON
	case csvF:
		return ModeCSV
	default:
		return ModeTable
	}
}

// Cell carries both a display form and an optional raw numeric value.
type Cell struct {
	Display string
	Num     float64
	IsNum   bool
}

// Str returns a non-numeric cell.
func Str(s string) Cell { return Cell{Display: s} }

// USD formats a USD amount with K/M/B/T suffix and retains the raw number.
func USD(v float64) Cell {
	return Cell{Display: USDString(v), Num: v, IsNum: true}
}

// Pct formats a percentage value already on a 0-100 scale.
func Pct(v float64) Cell {
	return Cell{Display: PctString(v), Num: v, IsNum: true}
}

// Num formats a number with K/M/B suffix (no $) and retains the raw value.
func Num(v float64) Cell {
	d := USDString(v)
	d = strings.TrimPrefix(d, "$")
	d = strings.TrimPrefix(d, "-$")
	if v < 0 && !strings.HasPrefix(d, "-") {
		d = "-" + d
	}
	return Cell{Display: d, Num: v, IsNum: true}
}

// Int formats an integer; useful for counts.
func Int(v int64) Cell {
	return Cell{Display: strconv.FormatInt(v, 10), Num: float64(v), IsNum: true}
}

// Raw renders a float with no SI suffix, avoiding scientific notation.
func Raw(v float64) Cell {
	return Cell{Display: rawFloatStr(v), Num: v, IsNum: true}
}

// Options controls table rendering.
type Options struct {
	NoHeader   bool
	RightAlign map[int]bool
}

// Recorder accumulates typed rows for the same headers.
type Recorder struct {
	Headers []string
	Rows    [][]Cell
}

func NewRecorder(headers []string) *Recorder {
	return &Recorder{Headers: headers}
}

func (r *Recorder) Append(cells []Cell) {
	r.Rows = append(r.Rows, cells)
}

// RenderRec writes the Recorder out in the requested mode.
func RenderRec(w io.Writer, mode Mode, r *Recorder, opts Options) error {
	switch mode {
	case ModeJSON:
		out := make([]map[string]any, 0, len(r.Rows))
		for _, row := range r.Rows {
			m := make(map[string]any, len(r.Headers))
			for i, h := range r.Headers {
				if i >= len(row) {
					m[h] = nil
					continue
				}
				c := row[i]
				if c.IsNum {
					if math.IsNaN(c.Num) || math.IsInf(c.Num, 0) {
						m[h] = nil
					} else {
						m[h] = c.Num
					}
				} else {
					m[h] = c.Display
				}
			}
			out = append(out, m)
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	case ModeCSV:
		cw := csv.NewWriter(w)
		defer cw.Flush()
		if !opts.NoHeader {
			if err := cw.Write(r.Headers); err != nil {
				return err
			}
		}
		for _, row := range r.Rows {
			cells := make([]string, len(r.Headers))
			for i := range r.Headers {
				if i >= len(row) {
					continue
				}
				c := row[i]
				if c.IsNum {
					cells[i] = rawFloatStr(c.Num)
				} else {
					cells[i] = c.Display
				}
			}
			if err := cw.Write(cells); err != nil {
				return err
			}
		}
		return cw.Error()
	default:
		rows := make([][]string, len(r.Rows))
		for i, row := range r.Rows {
			cells := make([]string, len(row))
			for j, c := range row {
				cells[j] = c.Display
			}
			rows[i] = cells
		}
		return renderTable(w, r.Headers, rows, opts)
	}
}

// Render is a back-compat helper for string-only data (used by `sync --status`,
// `sql`, and the config show command). For typed numeric data prefer RenderRec.
func Render(w io.Writer, mode Mode, headers []string, rows [][]string, opts Options) error {
	rec := &Recorder{Headers: headers, Rows: make([][]Cell, len(rows))}
	for i, r := range rows {
		cells := make([]Cell, len(r))
		for j, v := range r {
			cells[j] = Str(v)
		}
		rec.Rows[i] = cells
	}
	return RenderRec(w, mode, rec, opts)
}

func renderTable(w io.Writer, headers []string, rows [][]string, opts Options) error {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = utf8.RuneCountInString(h)
	}
	for _, r := range rows {
		for i, c := range r {
			if i >= len(widths) {
				continue
			}
			if n := utf8.RuneCountInString(c); n > widths[i] {
				widths[i] = n
			}
		}
	}
	pad := func(s string, w int, right bool) string {
		n := utf8.RuneCountInString(s)
		if n >= w {
			return s
		}
		sp := strings.Repeat(" ", w-n)
		if right {
			return sp + s
		}
		return s + sp
	}
	writeRow := func(cells []string) error {
		parts := make([]string, len(cells))
		for i, c := range cells {
			parts[i] = pad(c, widths[i], opts.RightAlign[i])
		}
		_, err := fmt.Fprintln(w, strings.Join(parts, "  "))
		return err
	}
	if !opts.NoHeader {
		if err := writeRow(headers); err != nil {
			return err
		}
	}
	for _, r := range rows {
		if len(r) < len(headers) {
			r = append(r, make([]string, len(headers)-len(r))...)
		}
		if err := writeRow(r); err != nil {
			return err
		}
	}
	return nil
}

// USDString formats a USD amount with K/M/B/T suffix.
func USDString(v float64) string {
	if math.IsNaN(v) || v == 0 {
		return "$0"
	}
	neg := ""
	if v < 0 {
		neg = "-"
		v = -v
	}
	switch {
	case v >= 1e12:
		return fmt.Sprintf("%s$%.2fT", neg, v/1e12)
	case v >= 1e9:
		return fmt.Sprintf("%s$%.2fB", neg, v/1e9)
	case v >= 1e6:
		return fmt.Sprintf("%s$%.2fM", neg, v/1e6)
	case v >= 1e3:
		return fmt.Sprintf("%s$%.2fK", neg, v/1e3)
	case v >= 1:
		return fmt.Sprintf("%s$%.2f", neg, v)
	default:
		return fmt.Sprintf("%s$%.4f", neg, v)
	}
}

// PctString formats a percentage value already on a 0-100 scale.
func PctString(v float64) string {
	if math.IsNaN(v) {
		return "-"
	}
	return fmt.Sprintf("%.2f%%", v)
}

func rawFloatStr(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return ""
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// Truncate shortens s to n runes with an ellipsis.
func Truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	if n <= 1 {
		return string([]rune(s)[:n])
	}
	return string([]rune(s)[:n-1]) + "…"
}
