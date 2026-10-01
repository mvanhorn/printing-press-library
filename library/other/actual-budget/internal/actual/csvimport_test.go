// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual_test

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

func TestParseBankDate(t *testing.T) {
	cases := []struct {
		in, layout string
		dayFirst   bool
		want       string
		err        bool
	}{
		{"2026-09-03", "", false, "2026-09-03", false},
		{"09/03/2026", "", false, "2026-09-03", false},
		{"9/3/2026", "", false, "2026-09-03", false},
		{"03/09/2026", "", true, "2026-09-03", false},
		{"13/09/2026", "", false, "", true},
		{"03.09.2026", "02.01.2006", false, "2026-09-03", false},
		{"Sep 3 2026", "Jan 2 2006", false, "2026-09-03", false},
		{"", "", false, "", true},
	}
	for _, c := range cases {
		got, err := actual.ParseBankDate(c.in, c.layout, c.dayFirst)
		if (err != nil) != c.err || got != c.want {
			t.Errorf("ParseBankDate(%q,%q,%v) = %q,%v want %q err=%v", c.in, c.layout, c.dayFirst, got, err, c.want, c.err)
		}
	}
}

func TestParseBankAmount(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
		err  bool
	}{
		{"-84.50", -8450, true, false},
		{"(12.00)", -1200, true, false},
		{"$1,234.56", 123456, true, false},
		{"12.00-", -1200, true, false},
		{"", 0, false, false},
		{"abc", 0, false, true},
	}
	for _, c := range cases {
		got, ok, err := actual.ParseBankAmount(c.in)
		if got != c.want || ok != c.ok || (err != nil) != c.err {
			t.Errorf("ParseBankAmount(%q) = %d,%v,%v", c.in, got, ok, err)
		}
	}
}

func TestCSVMappingValidate(t *testing.T) {
	cases := []struct {
		m   actual.CSVMapping
		err bool
	}{
		{actual.CSVMapping{DateCol: "Date", AmountCol: "Amount"}, false},
		{actual.CSVMapping{DateCol: "Date", DebitCol: "Debit"}, false},
		{actual.CSVMapping{AmountCol: "Amount"}, true},
		{actual.CSVMapping{DateCol: "Date"}, true},
		{actual.CSVMapping{DateCol: "Date", AmountCol: "A", CreditCol: "C"}, true},
	}
	for i, c := range cases {
		if err := c.m.Validate(); (err != nil) != c.err {
			t.Errorf("case %d: err = %v", i, err)
		}
	}
}

func TestParseBankCSV(t *testing.T) {
	cases := []struct {
		name    string
		csv     string
		m       actual.CSVMapping
		amounts []int64
		badLine []int
		hardErr bool
	}{
		{
			name:    "header mapping is case-insensitive with BOM",
			csv:     "\ufeffDate,Description,Amount,Ref\n2026-09-03,KROGER #123,-84.50,abc1\n2026-09-04,Payroll,4000.00,abc2\n",
			m:       actual.CSVMapping{DateCol: "date", AmountCol: "AMOUNT", PayeeCol: "Description", IDCol: "Ref"},
			amounts: []int64{-8450, 400000},
		},
		{
			name:    "debit/credit split",
			csv:     "Posted,Payee,Debit,Credit\n09/03/2026,Kroger,84.50,\n09/04/2026,Employer,,4000\n",
			m:       actual.CSVMapping{DateCol: "Posted", DebitCol: "Debit", CreditCol: "Credit", PayeeCol: "Payee"},
			amounts: []int64{-8450, 400000},
		},
		{
			name:    "parentheses negatives and invert",
			csv:     "Date,Amount\n2026-09-03,(84.50)\n2026-09-04,10\n",
			m:       actual.CSVMapping{DateCol: "Date", AmountCol: "Amount", Invert: true},
			amounts: []int64{8450, -1000},
		},
		{
			name:    "invalid rows reported with line numbers",
			csv:     "Date,Amount\n2026-09-03,1.00\nnot-a-date,2.00\n2026-09-05,\n\n2026-09-06,x\n2026-09-07,3\n",
			m:       actual.CSVMapping{DateCol: "Date", AmountCol: "Amount"},
			amounts: []int64{100, 300},
			badLine: []int{3, 4, 6},
		},
		{
			name:    "missing column is a hard error",
			csv:     "Date,Amount\n2026-09-03,1\n",
			m:       actual.CSVMapping{DateCol: "Date", AmountCol: "Total"},
			hardErr: true,
		},
		{
			name:    "empty file",
			csv:     "",
			m:       actual.CSVMapping{DateCol: "Date", AmountCol: "Amount"},
			hardErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			txns, bad, err := actual.ParseBankCSV(strings.NewReader(c.csv), c.m)
			if (err != nil) != c.hardErr {
				t.Fatalf("err = %v", err)
			}
			if c.hardErr {
				return
			}
			if len(txns) != len(c.amounts) {
				t.Fatalf("txns = %+v", txns)
			}
			for i, a := range c.amounts {
				if txns[i].Amount != a {
					t.Errorf("row %d amount = %d, want %d", i, txns[i].Amount, a)
				}
			}
			if len(bad) != len(c.badLine) {
				t.Fatalf("bad = %+v, want lines %v", bad, c.badLine)
			}
			for i, l := range c.badLine {
				if bad[i].Line != l {
					t.Errorf("bad[%d].Line = %d, want %d", i, bad[i].Line, l)
				}
			}
		})
	}
	// Field mapping detail.
	txns, _, _ := actual.ParseBankCSV(strings.NewReader("Date,Description,Amount,Memo,Ref\n2026-09-03,KROGER #123,-84.50,weekly,id-1\n"),
		actual.CSVMapping{DateCol: "Date", AmountCol: "Amount", PayeeCol: "Description", NotesCol: "Memo", IDCol: "Ref"})
	if len(txns) != 1 || txns[0].Date != "2026-09-03" || txns[0].PayeeName != "KROGER #123" || txns[0].ImportedPayee != "KROGER #123" || txns[0].Notes != "weekly" || txns[0].ImportedID != "id-1" || txns[0].Line != 2 {
		t.Fatalf("mapping = %+v", txns)
	}
}
