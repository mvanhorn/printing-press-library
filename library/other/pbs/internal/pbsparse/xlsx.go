// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package pbsparse

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Sheet is a decoded worksheet: a sparse cell grid plus its merged ranges.
//
// Sparseness is load-bearing. A blank cell is OMITTED from the xlsx XML
// entirely rather than stored as present-and-empty, so "does this cell exist"
// is the only way to tell a structurally absent value from a numeric zero.
// Iterating only the cells that exist cannot see blanks at all; that mistake
// produced a false "weekly uses zero, monthly uses blank" dichotomy before the
// counts were retaken by grid position.
type Sheet struct {
	Name    string
	Cells   map[int]map[int]string // row -> col -> raw text
	Merges  []Merge
	MaxRow  int
	MaxCol  int
	rowKeys []int
}

// Merge is one merged cell range, 1-based and inclusive.
type Merge struct {
	R1, C1, R2, C2 int
}

// Cols returns how many columns the merge spans.
func (m Merge) Cols() int { return m.C2 - m.C1 + 1 }

// Cell returns the raw text at a 1-based position and whether it exists.
func (s *Sheet) Cell(row, col int) (string, bool) {
	r, ok := s.Cells[row]
	if !ok {
		return "", false
	}
	v, ok := r[col]
	return v, ok
}

// Value classifies the cell at a position, preserving absence.
func (s *Sheet) Value(row, col int) Value {
	raw, ok := s.Cell(row, col)
	return ParseValue(raw, !ok)
}

// Rows returns the sheet's populated row numbers in ascending order.
func (s *Sheet) Rows() []int { return s.rowKeys }

var reCellRef = regexp.MustCompile(`^([A-Z]+)(\d+)$`)

// colNum converts an A1-style column label to a 1-based index.
func colNum(s string) int {
	n := 0
	for _, ch := range s {
		if ch < 'A' || ch > 'Z' {
			return 0
		}
		n = n*26 + int(ch-'A') + 1
	}
	return n
}

// ColName converts a 1-based column index back to its A1-style label.
func ColName(n int) string {
	var sb []byte
	for n > 0 {
		n--
		sb = append([]byte{byte('A' + n%26)}, sb...)
		n /= 26
	}
	return string(sb)
}

type xlSST struct {
	Items []struct {
		Text  string   `xml:"t"`
		Runs  []string `xml:"r>t"`
		Inner string   `xml:",innerxml"`
	} `xml:"si"`
}

type xlSheetData struct {
	Rows []struct {
		R     int `xml:"r,attr"`
		Cells []struct {
			R string `xml:"r,attr"`
			T string `xml:"t,attr"`
			V string `xml:"v"`
			// Inline strings live under <is><t>, not <v>. Header cells in PBS
			// annexures use this form, so a reader that only looks at <v>
			// returns an empty string for every city name.
			IS struct {
				T    []string `xml:"t"`
				RunT []string `xml:"r>t"`
			} `xml:"is"`
		} `xml:"c"`
	} `xml:"sheetData>row"`
	Merges []struct {
		Ref string `xml:"ref,attr"`
	} `xml:"mergeCells>mergeCell"`
}

// OpenXLSX decodes every worksheet in an xlsx byte payload.
func OpenXLSX(data []byte) ([]*Sheet, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}

	var sst []string
	if f, ok := files["xl/sharedStrings.xml"]; ok {
		b, err := readZip(f)
		if err != nil {
			return nil, err
		}
		var s xlSST
		if err := xml.Unmarshal(b, &s); err != nil {
			return nil, fmt.Errorf("sharedStrings: %w", err)
		}
		for _, it := range s.Items {
			if it.Text != "" {
				sst = append(sst, it.Text)
				continue
			}
			if len(it.Runs) > 0 {
				sst = append(sst, strings.Join(it.Runs, ""))
				continue
			}
			sst = append(sst, stripTags(it.Inner))
		}
	}

	var names []string
	for n := range files {
		if strings.HasPrefix(n, "xl/worksheets/sheet") && strings.HasSuffix(n, ".xml") {
			names = append(names, n)
		}
	}
	sort.Slice(names, func(i, j int) bool { return sheetOrd(names[i]) < sheetOrd(names[j]) })

	var out []*Sheet
	for _, n := range names {
		b, err := readZip(files[n])
		if err != nil {
			return nil, err
		}
		var sd xlSheetData
		if err := xml.Unmarshal(b, &sd); err != nil {
			return nil, fmt.Errorf("%s: %w", n, err)
		}
		sh := &Sheet{Name: n, Cells: map[int]map[int]string{}}
		for _, row := range sd.Rows {
			for _, c := range row.Cells {
				m := reCellRef.FindStringSubmatch(c.R)
				if m == nil {
					continue
				}
				col := colNum(m[1])
				rn, err := strconv.Atoi(m[2])
				if err != nil || col == 0 {
					continue
				}
				txt := ""
				switch {
				case len(c.IS.T) > 0:
					txt = strings.Join(c.IS.T, "")
				case len(c.IS.RunT) > 0:
					txt = strings.Join(c.IS.RunT, "")
				case c.T == "s":
					if i, err := strconv.Atoi(strings.TrimSpace(c.V)); err == nil && i >= 0 && i < len(sst) {
						txt = sst[i]
					}
				default:
					txt = c.V
				}
				// A cell element with no content at all is treated as absent,
				// matching how the file represents a genuinely blank cell.
				if txt == "" && c.V == "" && len(c.IS.T) == 0 && len(c.IS.RunT) == 0 {
					continue
				}
				if sh.Cells[rn] == nil {
					sh.Cells[rn] = map[int]string{}
				}
				sh.Cells[rn][col] = txt
				if rn > sh.MaxRow {
					sh.MaxRow = rn
				}
				if col > sh.MaxCol {
					sh.MaxCol = col
				}
			}
		}
		for _, mc := range sd.Merges {
			if m, ok := parseMergeRef(mc.Ref); ok {
				sh.Merges = append(sh.Merges, m)
			}
		}
		for r := range sh.Cells {
			sh.rowKeys = append(sh.rowKeys, r)
		}
		sort.Ints(sh.rowKeys)
		out = append(out, sh)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("xlsx contains no worksheets")
	}
	return out, nil
}

var reSheetOrd = regexp.MustCompile(`sheet(\d+)\.xml$`)

func sheetOrd(n string) int {
	if m := reSheetOrd.FindStringSubmatch(n); m != nil {
		i, _ := strconv.Atoi(m[1])
		return i
	}
	return 1 << 30
}

func parseMergeRef(ref string) (Merge, bool) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 {
		return Merge{}, false
	}
	a := reCellRef.FindStringSubmatch(parts[0])
	b := reCellRef.FindStringSubmatch(parts[1])
	if a == nil || b == nil {
		return Merge{}, false
	}
	r1, _ := strconv.Atoi(a[2])
	r2, _ := strconv.Atoi(b[2])
	return Merge{R1: r1, C1: colNum(a[1]), R2: r2, C2: colNum(b[1])}, true
}

// maxZipEntryBytes caps a single decompressed worksheet.
//
// The largest real PBS annexure worksheet measured is well under a megabyte, so
// 64 MiB is generous by two orders of magnitude while still refusing a
// decompression bomb. Without a cap, one hostile or corrupt entry can exhaust
// memory and kill a long backfill partway through.
const maxZipEntryBytes = 64 << 20

func readZip(f *zip.File) ([]byte, error) {
	// The declared size is a hint, not a guarantee, so it is checked first as a
	// cheap refusal and the read is capped regardless.
	if f.UncompressedSize64 > maxZipEntryBytes {
		return nil, fmt.Errorf("%s declares %d uncompressed bytes, over the %d-byte cap", f.Name, f.UncompressedSize64, uint64(maxZipEntryBytes))
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", f.Name, err)
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, maxZipEntryBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", f.Name, err)
	}
	if len(b) > maxZipEntryBytes {
		return nil, fmt.Errorf("%s exceeds the %d-byte decompression cap", f.Name, maxZipEntryBytes)
	}
	return b, nil
}

var reTag = regexp.MustCompile(`<[^>]*>`)

func stripTags(s string) string { return reTag.ReplaceAllString(s, "") }
