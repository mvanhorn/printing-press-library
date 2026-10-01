// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual

// Bank CSV parsing for import: maps header columns to Actual's import
// transaction shape (date, amount in minor units, payee_name, imported_payee,
// notes, imported_id).

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// CSVMapping names the CSV header columns to read. Either AmountCol or at
// least one of DebitCol/CreditCol is required, plus DateCol.
type CSVMapping struct {
	DateCol    string
	AmountCol  string
	DebitCol   string
	CreditCol  string
	PayeeCol   string
	NotesCol   string
	IDCol      string
	DateFormat string // Go layout; empty = auto-detect
	DayFirst   bool   // auto-detect DD/MM/YYYY instead of MM/DD/YYYY
	Invert     bool   // flip the sign of every amount
}

// ImportTxn is one transaction in the sidecar's import body.
type ImportTxn struct {
	Date          string `json:"date"`
	Amount        int64  `json:"amount"`
	PayeeName     string `json:"payee_name,omitempty"`
	ImportedPayee string `json:"imported_payee,omitempty"`
	Notes         string `json:"notes,omitempty"`
	ImportedID    string `json:"imported_id,omitempty"`
	Line          int    `json:"-"`
}

// CSVRowError reports a row that could not be parsed.
type CSVRowError struct {
	Line  int    `json:"line"`
	Error string `json:"error"`
}

// Validate checks the mapping has the columns it needs.
func (m CSVMapping) Validate() error {
	if strings.TrimSpace(m.DateCol) == "" {
		return errors.New("--date-col is required")
	}
	if strings.TrimSpace(m.AmountCol) == "" && strings.TrimSpace(m.DebitCol) == "" && strings.TrimSpace(m.CreditCol) == "" {
		return errors.New("--amount-col (or --debit-col/--credit-col) is required")
	}
	if m.AmountCol != "" && (m.DebitCol != "" || m.CreditCol != "") {
		return errors.New("use either --amount-col or --debit-col/--credit-col, not both")
	}
	return nil
}

var autoDateLayouts = []string{"2006-01-02", "1/2/2006", "2006/1/2", "1-2-2006"}
var autoDateLayoutsDayFirst = []string{"2006-01-02", "2/1/2006", "2006/1/2", "2-1-2006", "2.1.2006"}

// ParseBankDate parses a CSV date into YYYY-MM-DD with layout, or by trying
// YYYY-MM-DD, M/D/YYYY (or D/M/YYYY with dayFirst) and close variants.
func ParseBankDate(s, layout string, dayFirst bool) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("empty date")
	}
	layouts := autoDateLayouts
	if dayFirst {
		layouts = autoDateLayoutsDayFirst
	}
	if layout != "" {
		layouts = []string{layout}
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("unrecognized date %q", s)
}

// ParseBankAmount parses a bank amount ("1,234.56", "(12.00)", "$-5", "12.00-")
// into minor units. Empty strings and dash placeholders ("-", "--", "—", "–",
// common in the unused column of a Debit/Credit export) parse as 0 with
// ok=false.
func ParseBankAmount(s string) (int64, bool, error) {
	s = strings.TrimSpace(s)
	switch s {
	case "", "-", "--", "\u2014", "\u2013":
		return 0, false, nil
	}
	if strings.HasSuffix(s, "-") && !strings.HasPrefix(s, "-") {
		s = "-" + strings.TrimSuffix(s, "-")
	}
	v, err := ParseAmount(s)
	if err != nil {
		return 0, false, err
	}
	return v, true, nil
}

// ParseBankCSV reads a CSV with a header row and maps each data row with m.
// Rows that fail to parse are reported with their 1-based line numbers and
// skipped; a missing mapped column is a hard error.
func ParseBankCSV(r io.Reader, m CSVMapping) ([]ImportTxn, []CSVRowError, error) {
	if err := m.Validate(); err != nil {
		return nil, nil, err
	}
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	header, err := cr.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil, errors.New("CSV is empty (expected a header row)")
		}
		return nil, nil, fmt.Errorf("reading CSV header: %w", err)
	}
	cols := map[string]int{}
	for i, h := range header {
		h = strings.TrimSpace(strings.TrimPrefix(h, "\ufeff"))
		cols[strings.ToLower(h)] = i
	}
	col := func(name string) (int, error) {
		if name == "" {
			return -1, nil
		}
		i, ok := cols[strings.ToLower(strings.TrimSpace(name))]
		if !ok {
			return -1, fmt.Errorf("column %q not found in the CSV header row (%d columns)", name, len(header))
		}
		return i, nil
	}
	var idx struct{ date, amount, debit, credit, payee, notes, id int }
	for _, c := range []struct {
		name string
		dst  *int
	}{{m.DateCol, &idx.date}, {m.AmountCol, &idx.amount}, {m.DebitCol, &idx.debit}, {m.CreditCol, &idx.credit}, {m.PayeeCol, &idx.payee}, {m.NotesCol, &idx.notes}, {m.IDCol, &idx.id}} {
		i, err := col(c.name)
		if err != nil {
			return nil, nil, err
		}
		*c.dst = i
	}
	get := func(rec []string, i int) string {
		if i < 0 || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}
	txns := make([]ImportTxn, 0)
	bad := make([]CSVRowError, 0)
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var pe *csv.ParseError
			if errors.As(err, &pe) {
				bad = append(bad, CSVRowError{Line: pe.Line, Error: pe.Err.Error()})
				continue
			}
			return nil, nil, fmt.Errorf("reading CSV: %w", err)
		}
		line := 0
		if len(rec) > 0 {
			line, _ = cr.FieldPos(0)
		}
		blank := true
		for _, f := range rec {
			if strings.TrimSpace(f) != "" {
				blank = false
				break
			}
		}
		if blank {
			continue
		}
		date, err := ParseBankDate(get(rec, idx.date), m.DateFormat, m.DayFirst)
		if err != nil {
			bad = append(bad, CSVRowError{Line: line, Error: err.Error()})
			continue
		}
		var amount int64
		if idx.amount >= 0 {
			v, ok, err := ParseBankAmount(get(rec, idx.amount))
			if err != nil || !ok {
				msg := "empty amount"
				if err != nil {
					msg = err.Error()
				}
				bad = append(bad, CSVRowError{Line: line, Error: msg})
				continue
			}
			amount = v
		} else {
			debit, okD, errD := ParseBankAmount(get(rec, idx.debit))
			credit, okC, errC := ParseBankAmount(get(rec, idx.credit))
			if errD != nil || errC != nil {
				bad = append(bad, CSVRowError{Line: line, Error: errors.Join(errD, errC).Error()})
				continue
			}
			if !okD && !okC {
				bad = append(bad, CSVRowError{Line: line, Error: "empty debit and credit"})
				continue
			}
			amount = abs64(credit) - abs64(debit)
		}
		if m.Invert {
			amount = -amount
		}
		payee := get(rec, idx.payee)
		txns = append(txns, ImportTxn{
			Date: date, Amount: amount, PayeeName: payee, ImportedPayee: payee,
			Notes: get(rec, idx.notes), ImportedID: get(rec, idx.id), Line: line,
		})
	}
	return txns, bad, nil
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
